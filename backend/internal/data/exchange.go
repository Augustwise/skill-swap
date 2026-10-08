package data

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

const RequestPending = "PENDING"

var ErrDuplicateRequest = errors.New("a pending request for this pair of skills already exists")

// RequestRecipient is a visible student with the settings that decide who may send them requests.
type RequestRecipient struct {
	Profile
	AcceptsRequests    bool
	SameUniversityOnly bool
}

type NewRequest struct {
	RequesterID string
	RecipientID string
	// TeachSkillID is the catalog skill the requester teaches; LearnSkillID the one the recipient teaches.
	TeachSkillID         string
	LearnSkillID         string
	Format               string
	TeachSessions        int
	TeachDurationMinutes int
	LearnSessions        int
	LearnDurationMinutes int
	Message              string
}

type RequestParty struct {
	UserID         string
	FirstName      string
	LastName       string
	UniversityID   string
	UniversityName string
	City           string
}

// RequestTerms describe one direction of the exchange: who teaches which skill and how much.
type RequestTerms struct {
	SkillID         string
	CategoryID      string
	Name            string
	TeacherLevel    string
	LearnerLevel    string
	Sessions        int
	DurationMinutes int
}

type ExchangeRequest struct {
	ID               string
	Status           string
	Format           string
	TotalSessions    int
	Message          string // empty when there is no message
	Requester        RequestParty
	Recipient        RequestParty
	RequesterTeaches RequestTerms
	RecipientTeaches RequestTerms
	ExchangeID       string // empty until the request is accepted
	CreatedAt        time.Time
	RespondedAt      *time.Time
}

type IExchangeData interface {
	RequestRecipient(ctx context.Context, senderID, recipientID string) (RequestRecipient, error)
	CreateRequest(ctx context.Context, request NewRequest) (string, error)
	RequestByID(ctx context.Context, requestID string) (ExchangeRequest, error)
}

var _ IExchangeData = (*Postgres)(nil)

func (p *Postgres) RequestRecipient(ctx context.Context, senderID, recipientID string) (RequestRecipient, error) {
	var recipient RequestRecipient
	err := p.conn(ctx).QueryRow(ctx, `SELECT COALESCE(ps.accepts_requests, true),
			COALESCE(ps.request_scope = 'SAME_UNIVERSITY', false)
		FROM users c
		LEFT JOIN user_profile_settings ps ON ps.user_id = c.id
		WHERE c.id = $2 AND `+visibleStudent, senderID, recipientID).Scan(&recipient.AcceptsRequests, &recipient.SameUniversityOnly)
	if err != nil {
		return RequestRecipient{}, notFound(err)
	}
	if recipient.Profile, err = p.ProfileByUserID(ctx, recipientID); err != nil {
		return RequestRecipient{}, err
	}
	return recipient, nil
}

// CreateRequest finds the four active skill rows itself and returns ErrNotFound when one of them is gone.
func (p *Postgres) CreateRequest(ctx context.Context, r NewRequest) (string, error) {
	var id string
	err := p.conn(ctx).QueryRow(ctx, `WITH created AS (
			INSERT INTO exchange_requests (requester_id, recipient_id,
				requester_teaching_skill_id, recipient_learning_skill_id, recipient_teaching_skill_id, requester_learning_skill_id,
				format, total_sessions, requester_sessions, recipient_sessions,
				requester_duration_minutes, recipient_duration_minutes, message)
			SELECT rt.user_id, ct.user_id, rt.id, cl.id, ct.id, rl.id,
				$5::lesson_format, $6::smallint + $8::smallint, $6, $8, $7, $9, NULLIF($10, '')
			FROM user_teaching_skills rt
			JOIN user_learning_skills cl ON cl.user_id = $2 AND cl.skill_id = $3 AND cl.is_active
			JOIN user_teaching_skills ct ON ct.user_id = $2 AND ct.skill_id = $4 AND ct.is_active
			JOIN user_learning_skills rl ON rl.user_id = $1 AND rl.skill_id = $4 AND rl.is_active
			WHERE rt.user_id = $1 AND rt.skill_id = $3 AND rt.is_active
			RETURNING id, requester_id, created_at
		)
		INSERT INTO exchange_request_status_history (request_id, status, changed_by_user_id, created_at)
		SELECT id, 'PENDING', requester_id, created_at FROM created
		RETURNING request_id`,
		r.RequesterID, r.RecipientID, r.TeachSkillID, r.LearnSkillID, r.Format,
		r.TeachSessions, r.TeachDurationMinutes, r.LearnSessions, r.LearnDurationMinutes, r.Message).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_exchange_requests_pending_skill_pair" {
		return "", ErrDuplicateRequest
	}
	if err != nil {
		return "", notFound(err)
	}
	return id, nil
}

func requestParty(user string) string {
	return `json_build_object('UserID', ` + user + `.id, 'FirstName', ` + user + `.first_name,
		'LastName', ` + user + `.last_name, 'UniversityID', ` + user + `.university_id,
		'UniversityName', (SELECT un.name FROM universities un WHERE un.id = ` + user + `.university_id),
		'City', COALESCE(` + user + `.city, ''))`
}

func requestTerms(teaching, learning, sessions, duration string) string {
	return `(SELECT json_build_object('SkillID', s.id, 'CategoryID', s.category_id, 'Name', s.name,
		'TeacherLevel', t.level, 'LearnerLevel', l.current_level, 'Sessions', r.` + sessions + `,
		'DurationMinutes', r.` + duration + `)
		FROM user_teaching_skills t
		JOIN user_learning_skills l ON l.id = r.` + learning + `
		JOIN skills s ON s.id = t.skill_id
		WHERE t.id = r.` + teaching + `)`
}

var requestSelect = `SELECT r.id, r.status::text, r.format::text, r.total_sessions, COALESCE(r.message, ''),
		` + requestParty("rq") + `, ` + requestParty("rc") + `,
		` + requestTerms("requester_teaching_skill_id", "recipient_learning_skill_id", "requester_sessions", "requester_duration_minutes") + `,
		` + requestTerms("recipient_teaching_skill_id", "requester_learning_skill_id", "recipient_sessions", "recipient_duration_minutes") + `,
		COALESCE((SELECT e.id::text FROM exchanges e WHERE e.source_request_id = r.id), ''),
		r.created_at, r.responded_at
	FROM exchange_requests r
	JOIN users rq ON rq.id = r.requester_id
	JOIN users rc ON rc.id = r.recipient_id`

func (p *Postgres) RequestByID(ctx context.Context, requestID string) (ExchangeRequest, error) {
	var r ExchangeRequest
	err := p.conn(ctx).QueryRow(ctx, requestSelect+` WHERE r.id = $1`, requestID).Scan(
		&r.ID, &r.Status, &r.Format, &r.TotalSessions, &r.Message, &r.Requester, &r.Recipient,
		&r.RequesterTeaches, &r.RecipientTeaches, &r.ExchangeID, &r.CreatedAt, &r.RespondedAt)
	if err != nil {
		return ExchangeRequest{}, notFound(err)
	}
	return r, nil
}
