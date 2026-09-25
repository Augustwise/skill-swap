package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"slices"
	"time"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/mailer"
)

const (
	SessionTTL       = 7 * 24 * time.Hour
	verificationTTL  = 24 * time.Hour
	passwordResetTTL = time.Hour
	mailTimeout      = 10 * time.Second
)

var loginThrottle = data.ThrottlePolicy{
	MaxFailures: 5,
	Window:      15 * time.Minute,
	Lockout:     2 * time.Hour,
}

var (
	ErrEmailTaken         = errors.New("email is already registered")
	ErrInvalidCredentials = errors.New("email or password is incorrect")
	ErrAccountSuspended   = errors.New("account is suspended")
	ErrUnauthenticated    = errors.New("session is missing, expired, or revoked")
	ErrInvalidToken       = errors.New("token is invalid, used, or expired")
	ErrAlreadyVerified    = errors.New("email is already verified")
	ErrMailUnavailable    = errors.New("email could not be sent")
)

type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid fields: %v", e.Fields)
}

type LockedError struct {
	Until time.Time
}

func (e *LockedError) Error() string {
	return "too many failed sign-in attempts"
}

type Config struct {
	FrontendOrigin      string
	AllowedEmailDomains []string
}

type Service struct {
	data   data.IAuthData
	tx     data.ITransaction
	mailer mailer.IMailer
	config Config
	logger *slog.Logger
}

func NewService(store data.IAuthData, tx data.ITransaction, mail mailer.IMailer, config Config, logger *slog.Logger) *Service {
	return &Service{data: store, tx: tx, mailer: mail, config: config, logger: logger}
}

type RegisterInput struct {
	Email     string
	Password  string
	FirstName string
	LastName  string
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (user data.User, emailSent bool, err error) {
	fields := map[string]string{}
	email, ok := normalizeEmail(in.Email)
	if !ok {
		fields["email"] = "Enter a valid email address"
	} else if !slices.Contains(s.config.AllowedEmailDomains, emailDomain(email)) {
		fields["email"] = "Use your university email address"
	}
	if message := validatePassword(in.Password); message != "" {
		fields["password"] = message
	}
	firstName, ok := cleanName(in.FirstName)
	if !ok {
		fields["firstName"] = "First name is required and must be at most 100 characters"
	}
	lastName, ok := cleanName(in.LastName)
	if !ok {
		fields["lastName"] = "Last name is required and must be at most 100 characters"
	}
	if len(fields) > 0 {
		return data.User{}, false, &ValidationError{Fields: fields}
	}

	universityID, err := s.data.UniversityIDByEmailDomain(ctx, emailDomain(email))
	if errors.Is(err, data.ErrNotFound) {
		return data.User{}, false, &ValidationError{Fields: map[string]string{
			"email": "Your university is not registered in SkillSwap yet",
		}}
	}
	if err != nil {
		return data.User{}, false, err
	}
	passwordHash, err := hashPassword(in.Password)
	if err != nil {
		return data.User{}, false, err
	}
	token, err := newToken()
	if err != nil {
		return data.User{}, false, err
	}

	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		userID, err := s.data.CreateUser(ctx, data.NewUser{
			UniversityID: universityID,
			Email:        email,
			FirstName:    firstName,
			LastName:     lastName,
		})
		if err != nil {
			return err
		}
		if err := s.data.CreateLocalIdentity(ctx, userID, passwordHash); err != nil {
			return err
		}
		if err := s.data.CreateEmailVerification(ctx, userID, email); err != nil {
			return err
		}
		verification := data.StoredToken{Hash: hashToken(token), ExpiresAt: time.Now().Add(verificationTTL)}
		if err := s.data.CreateToken(ctx, userID, data.PurposeEmailVerification, verification); err != nil {
			return err
		}
		user, err = s.data.UserByID(ctx, userID)
		return err
	})
	if errors.Is(err, data.ErrEmailTaken) {
		return data.User{}, false, ErrEmailTaken
	}
	if err != nil {
		return data.User{}, false, err
	}

	return user, s.sendVerificationEmail(ctx, user, token) == nil, nil
}

func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	if !validToken(token) {
		return ErrInvalidToken
	}
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		userID, err := s.data.UseToken(ctx, hashToken(token), data.PurposeEmailVerification)
		if err != nil {
			return err
		}
		return s.data.MarkEmailVerified(ctx, userID)
	})
	if errors.Is(err, data.ErrNotFound) {
		return ErrInvalidToken
	}
	return err
}

func (s *Service) ResendVerification(ctx context.Context, user data.User) error {
	if user.EmailVerified {
		return ErrAlreadyVerified
	}
	token, err := s.replaceToken(ctx, user.ID, data.PurposeEmailVerification, verificationTTL)
	if err != nil {
		return err
	}
	if err := s.sendVerificationEmail(ctx, user, token); err != nil {
		return ErrMailUnavailable
	}
	return nil
}

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IP        netip.Addr

	PreviousSession string
}

type Session struct {
	User      data.User
	Token     string
	CSRFToken string
	ExpiresAt time.Time
}

func (s *Service) Login(ctx context.Context, in LoginInput) (Session, error) {
	email, ok := normalizeEmail(in.Email)
	if !ok || in.Password == "" {
		return Session{}, ErrInvalidCredentials
	}
	lockedUntil, err := s.data.LoginLockedUntil(ctx, email)
	if err != nil {
		return Session{}, err
	}
	if !lockedUntil.IsZero() {
		return Session{}, &LockedError{Until: lockedUntil}
	}
	user, passwordHash, err := s.data.UserByEmail(ctx, email)
	if err != nil && !errors.Is(err, data.ErrNotFound) {
		return Session{}, err
	}
	if !passwordMatches(passwordHash, in.Password) {
		if err := s.data.RecordFailedLogin(ctx, email, loginThrottle); err != nil {
			return Session{}, err
		}
		return Session{}, ErrInvalidCredentials
	}
	if user.Status != "ACTIVE" {
		return Session{}, ErrAccountSuspended
	}

	token, err := newToken()
	if err != nil {
		return Session{}, err
	}
	session := Session{User: user, Token: token, CSRFToken: CSRFToken(token), ExpiresAt: time.Now().Add(SessionTTL)}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		stored := data.StoredToken{Hash: hashToken(token), ExpiresAt: session.ExpiresAt}
		if err := s.data.CreateSession(ctx, user.ID, stored, in.UserAgent, in.IP); err != nil {
			return err
		}
		if err := s.data.RecordLogin(ctx, user.ID); err != nil {
			return err
		}
		if err := s.data.ClearFailedLogins(ctx, email); err != nil {
			return err
		}
		if validToken(in.PreviousSession) {
			return s.data.RevokeSession(ctx, hashToken(in.PreviousSession))
		}
		return nil
	})
	if err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Service) Authenticate(ctx context.Context, sessionToken string) (data.User, error) {
	if !validToken(sessionToken) {
		return data.User{}, ErrUnauthenticated
	}
	user, err := s.data.UserBySession(ctx, hashToken(sessionToken))
	if errors.Is(err, data.ErrNotFound) {
		return data.User{}, ErrUnauthenticated
	}
	return user, err
}

func (s *Service) Logout(ctx context.Context, sessionToken string) error {
	return s.data.RevokeSession(ctx, hashToken(sessionToken))
}

func (s *Service) ForgotPassword(ctx context.Context, rawEmail string) error {
	email, ok := normalizeEmail(rawEmail)
	if !ok {
		return &ValidationError{Fields: map[string]string{"email": "Enter a valid email address"}}
	}
	user, _, err := s.data.UserByEmail(ctx, email)
	if errors.Is(err, data.ErrNotFound) || (err == nil && user.Status != "ACTIVE") {
		return nil
	}
	if err != nil {
		return err
	}
	token, err := s.replaceToken(ctx, user.ID, data.PurposePasswordReset, passwordResetTTL)
	if err != nil {
		return err
	}
	link := s.config.FrontendOrigin + "/reset-password?token=" + url.QueryEscape(token)
	err = s.sendEmail(ctx, mailer.Message{
		To:      []string{user.Email},
		Subject: "Відновлення пароля SkillSwap",
		TextBody: fmt.Sprintf("Вітаємо, %s!\n\nЩоб задати новий пароль, відкрийте посилання:\n%s\n\n"+
			"Посилання дійсне 1 годину і спрацьовує лише один раз. "+
			"Якщо ви не просили відновити пароль, просто проігноруйте цей лист.\n", user.FirstName, link),
	})
	if err != nil {
		s.logger.Error("password reset email failed", "user_id", user.ID, "error", err)
	}
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if message := validatePassword(password); message != "" {
		return &ValidationError{Fields: map[string]string{"password": message}}
	}
	if !validToken(token) {
		return ErrInvalidToken
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return err
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		userID, err := s.data.UseToken(ctx, hashToken(token), data.PurposePasswordReset)
		if err != nil {
			return err
		}
		if err := s.data.SetPassword(ctx, userID, passwordHash); err != nil {
			return err
		}
		if err := s.data.RevokeUserSessions(ctx, userID); err != nil {
			return err
		}
		user, err := s.data.UserByID(ctx, userID)
		if err != nil {
			return err
		}
		return s.data.ClearFailedLogins(ctx, user.Email)
	})
	if errors.Is(err, data.ErrNotFound) {
		return ErrInvalidToken
	}
	return err
}

func (s *Service) replaceToken(ctx context.Context, userID, purpose string, ttl time.Duration) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.data.InvalidateTokens(ctx, userID, purpose); err != nil {
			return err
		}
		return s.data.CreateToken(ctx, userID, purpose, data.StoredToken{Hash: hashToken(token), ExpiresAt: time.Now().Add(ttl)})
	})
	return token, err
}

func (s *Service) sendVerificationEmail(ctx context.Context, user data.User, token string) error {
	link := s.config.FrontendOrigin + "/onboarding?token=" + url.QueryEscape(token)
	err := s.sendEmail(ctx, mailer.Message{
		To:      []string{user.Email},
		Subject: "Підтвердіть пошту SkillSwap",
		TextBody: fmt.Sprintf("Вітаємо, %s!\n\nЩоб підтвердити адресу електронної пошти, відкрийте посилання:\n%s\n\n"+
			"Посилання дійсне 24 години і спрацьовує лише один раз. "+
			"Якщо ви не реєструвалися в SkillSwap, просто проігноруйте цей лист.\n", user.FirstName, link),
	})
	if err != nil {
		s.logger.Error("verification email failed", "user_id", user.ID, "error", err)
	}
	return err
}

func (s *Service) sendEmail(ctx context.Context, message mailer.Message) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mailTimeout)
	defer cancel()
	return s.mailer.Send(ctx, message)
}
