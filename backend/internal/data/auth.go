package data

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	PurposeEmailVerification = "EMAIL_VERIFICATION"
	PurposePasswordReset     = "PASSWORD_RESET"
)

type User struct {
	ID            string
	Email         string
	FirstName     string
	LastName      string
	UniversityID  string
	Role          string
	Status        string
	EmailVerified bool
	CreatedAt     time.Time
}

type NewUser struct {
	UniversityID string
	Email        string
	FirstName    string
	LastName     string
}

type StoredToken struct {
	Hash      string
	ExpiresAt time.Time
}

type ThrottlePolicy struct {
	MaxFailures int
	Window      time.Duration
	Lockout     time.Duration
}

type IAuthData interface {
	UniversityIDByEmailDomain(ctx context.Context, domain string) (string, error)
	CreateUser(ctx context.Context, user NewUser) (string, error)
	CreateLocalIdentity(ctx context.Context, userID, passwordHash string) error
	SetPassword(ctx context.Context, userID, passwordHash string) error
	CreateEmailVerification(ctx context.Context, userID, email string) error
	MarkEmailVerified(ctx context.Context, userID string) error
	UserByID(ctx context.Context, userID string) (User, error)
	UserByEmail(ctx context.Context, email string) (User, string, error)
	UserBySession(ctx context.Context, tokenHash string) (User, error)
	CreateSession(ctx context.Context, userID string, session StoredToken, userAgent string, ip netip.Addr) error
	RecordLogin(ctx context.Context, userID string) error
	RevokeSession(ctx context.Context, tokenHash string) error
	RevokeUserSessions(ctx context.Context, userID string) error
	CreateToken(ctx context.Context, userID, purpose string, token StoredToken) error
	InvalidateTokens(ctx context.Context, userID, purpose string) error
	UseToken(ctx context.Context, tokenHash, purpose string) (string, error)
	LoginLockedUntil(ctx context.Context, email string) (time.Time, error)
	RecordFailedLogin(ctx context.Context, email string, policy ThrottlePolicy) error
	ClearFailedLogins(ctx context.Context, email string) error
}

const selectUser = `SELECT u.id, u.email, u.first_name, u.last_name, u.university_id, u.role::text,
	u.account_status::text, u.created_at,
	EXISTS (SELECT 1 FROM user_verifications v WHERE v.user_id = u.id AND v.type = 'UNIVERSITY_EMAIL'
		AND v.identifier = u.email AND v.status = 'VERIFIED')`

func scanUser(row pgx.Row, extra ...any) (User, error) {
	var user User
	targets := []any{&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.UniversityID,
		&user.Role, &user.Status, &user.CreatedAt, &user.EmailVerified}
	if err := row.Scan(append(targets, extra...)...); err != nil {
		return User{}, notFound(err)
	}
	return user, nil
}

func (p *Postgres) UniversityIDByEmailDomain(ctx context.Context, domain string) (string, error) {
	var id string
	err := p.conn(ctx).QueryRow(ctx, `SELECT id FROM universities WHERE lower(email_domain) = $1`, domain).Scan(&id)
	return id, notFound(err)
}

func (p *Postgres) CreateUser(ctx context.Context, user NewUser) (string, error) {
	var id string
	err := p.conn(ctx).QueryRow(ctx, `INSERT INTO users (university_id, email, first_name, last_name, terms_accepted_at)
		VALUES ($1, $2, $3, $4, now()) RETURNING id`,
		user.UniversityID, user.Email, user.FirstName, user.LastName).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return "", ErrEmailTaken
	}
	return id, err
}

func (p *Postgres) CreateLocalIdentity(ctx context.Context, userID, passwordHash string) error {
	_, err := p.conn(ctx).Exec(ctx, `INSERT INTO auth_identities (user_id, provider, provider_subject, password_hash, password_changed_at)
		VALUES ($1, 'LOCAL', $2, $3, now())`, userID, userID, passwordHash)
	return err
}

func (p *Postgres) SetPassword(ctx context.Context, userID, passwordHash string) error {
	_, err := p.conn(ctx).Exec(ctx, `UPDATE auth_identities SET password_hash = $2, password_changed_at = now()
		WHERE user_id = $1 AND provider = 'LOCAL'`, userID, passwordHash)
	return err
}

func (p *Postgres) CreateEmailVerification(ctx context.Context, userID, email string) error {
	_, err := p.conn(ctx).Exec(ctx, `INSERT INTO user_verifications (user_id, type, identifier, status)
		VALUES ($1, 'UNIVERSITY_EMAIL', $2, 'PENDING')`, userID, email)
	return err
}

func (p *Postgres) MarkEmailVerified(ctx context.Context, userID string) error {
	_, err := p.conn(ctx).Exec(ctx, `INSERT INTO user_verifications (user_id, type, identifier, status, verified_at)
		SELECT id, 'UNIVERSITY_EMAIL', email, 'VERIFIED', now() FROM users WHERE id = $1
		ON CONFLICT (user_id, type, identifier)
		DO UPDATE SET status = 'VERIFIED', verified_at = now()`, userID)
	return err
}

func (p *Postgres) UserByID(ctx context.Context, userID string) (User, error) {
	return scanUser(p.conn(ctx).QueryRow(ctx, selectUser+` FROM users u WHERE u.id = $1 AND u.deleted_at IS NULL`, userID))
}

func (p *Postgres) UserByEmail(ctx context.Context, email string) (User, string, error) {
	var passwordHash string
	user, err := scanUser(p.conn(ctx).QueryRow(ctx, selectUser+`, COALESCE(a.password_hash, '')
		FROM users u
		JOIN auth_identities a ON a.user_id = u.id AND a.provider = 'LOCAL'
		WHERE u.email = $1 AND u.deleted_at IS NULL`, email), &passwordHash)
	return user, passwordHash, err
}

func (p *Postgres) UserBySession(ctx context.Context, tokenHash string) (User, error) {
	return scanUser(p.conn(ctx).QueryRow(ctx, `WITH session AS (
			UPDATE user_sessions SET last_used_at = now()
			WHERE session_token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
			RETURNING user_id
		)
		`+selectUser+` FROM users u JOIN session ON session.user_id = u.id
		WHERE u.deleted_at IS NULL AND u.account_status = 'ACTIVE'`, tokenHash))
}

func (p *Postgres) CreateSession(ctx context.Context, userID string, session StoredToken, userAgent string, ip netip.Addr) error {
	var ipValue any
	if ip.IsValid() {
		ipValue = ip
	}
	_, err := p.conn(ctx).Exec(ctx, `INSERT INTO user_sessions (user_id, session_token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)`, userID, session.Hash, userAgent, ipValue, session.ExpiresAt)
	return err
}

func (p *Postgres) RecordLogin(ctx context.Context, userID string) error {
	if _, err := p.conn(ctx).Exec(ctx, `UPDATE auth_identities SET last_login_at = now()
		WHERE user_id = $1 AND provider = 'LOCAL'`, userID); err != nil {
		return err
	}
	_, err := p.conn(ctx).Exec(ctx, `UPDATE users SET last_active_at = now() WHERE id = $1`, userID)
	return err
}

func (p *Postgres) RevokeSession(ctx context.Context, tokenHash string) error {
	_, err := p.conn(ctx).Exec(ctx, `UPDATE user_sessions SET revoked_at = now()
		WHERE session_token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	return err
}

func (p *Postgres) RevokeUserSessions(ctx context.Context, userID string) error {
	_, err := p.conn(ctx).Exec(ctx, `UPDATE user_sessions SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func (p *Postgres) CreateToken(ctx context.Context, userID, purpose string, token StoredToken) error {
	_, err := p.conn(ctx).Exec(ctx, `INSERT INTO one_time_tokens (user_id, purpose, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`, userID, purpose, token.Hash, token.ExpiresAt)
	return err
}

func (p *Postgres) InvalidateTokens(ctx context.Context, userID, purpose string) error {
	_, err := p.conn(ctx).Exec(ctx, `UPDATE one_time_tokens SET used_at = now()
		WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL`, userID, purpose)
	return err
}

func (p *Postgres) UseToken(ctx context.Context, tokenHash, purpose string) (string, error) {
	var userID string
	err := p.conn(ctx).QueryRow(ctx, `UPDATE one_time_tokens SET used_at = now()
		WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`, tokenHash, purpose).Scan(&userID)
	return userID, notFound(err)
}

func (p *Postgres) LoginLockedUntil(ctx context.Context, email string) (time.Time, error) {
	var lockedUntil time.Time
	err := p.conn(ctx).QueryRow(ctx, `SELECT locked_until FROM login_throttles
		WHERE email = $1 AND locked_until > now()`, email).Scan(&lockedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	return lockedUntil, err
}

func (p *Postgres) RecordFailedLogin(ctx context.Context, email string, policy ThrottlePolicy) error {
	var failedCount int
	err := p.conn(ctx).QueryRow(ctx, `INSERT INTO login_throttles AS t (email, failed_count, window_started_at)
		VALUES ($1, 1, now())
		ON CONFLICT (email) DO UPDATE SET
			failed_count = CASE WHEN t.window_started_at > now() - make_interval(secs => $2)
				THEN t.failed_count + 1 ELSE 1 END,
			window_started_at = CASE WHEN t.window_started_at > now() - make_interval(secs => $2)
				THEN t.window_started_at ELSE now() END
		RETURNING failed_count`, email, policy.Window.Seconds()).Scan(&failedCount)
	if err != nil || failedCount < policy.MaxFailures {
		return err
	}
	_, err = p.conn(ctx).Exec(ctx, `UPDATE login_throttles
		SET locked_until = now() + make_interval(secs => $2), failed_count = 0, window_started_at = now()
		WHERE email = $1`, email, policy.Lockout.Seconds())
	return err
}

func (p *Postgres) ClearFailedLogins(ctx context.Context, email string) error {
	_, err := p.conn(ctx).Exec(ctx, `DELETE FROM login_throttles WHERE email = $1`, email)
	return err
}
