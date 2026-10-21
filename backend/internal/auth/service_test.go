package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/expect"
	"skillswap/backend/internal/mailer"
	"skillswap/backend/internal/mailer/mailertest"
)

const testPassword = "correct horse battery"

var linkToken = regexp.MustCompile(`token=([A-Za-z0-9_-]{43})`)

type spyTx struct {
	*datatest.Memory
	count  int
	active bool
}

func (s *spyTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	s.count++
	s.active = true
	defer func() { s.active = false }()
	return fn(ctx)
}

type spyMailer struct {
	mailertest.Recorder
	tx *spyTx
	t  *testing.T
}

func (m *spyMailer) Send(ctx context.Context, message mailer.Message) error {
	if m.tx.active {
		m.t.Error("email sent inside an open transaction")
	}
	return m.Recorder.Send(ctx, message)
}

func newTestService(t *testing.T) (*Service, *spyTx, *spyMailer) {
	store := datatest.NewMemory()
	tx := &spyTx{Memory: store}
	mail := &spyMailer{tx: tx, t: t}
	config := Config{FrontendOrigin: "http://localhost:3000", AllowedEmailDomains: []string{"students.example.test"}}
	return NewService(store, tx, mail, config, slog.New(slog.NewTextHandler(io.Discard, nil))), tx, mail
}

func register(t *testing.T, s *Service, email string) {
	t.Helper()
	_, sent, err := s.Register(context.Background(), RegisterInput{
		Email: email, Password: testPassword, FirstName: "Олена", LastName: "Коваль",
	})
	expect.NoError(t, "register", err)
	expect.Equal(t, "verification email sent", sent, true)
}

func lastToken(t *testing.T, mail *spyMailer) string {
	t.Helper()
	match := linkToken.FindStringSubmatch(mail.Last().TextBody)
	if match == nil {
		t.Fatalf("no token link in %q", mail.Last().TextBody)
	}
	return match[1]
}

func TestPasswordRules(t *testing.T) {
	tests := []struct {
		password string
		valid    bool
	}{
		{strings.Repeat("я", 12), true},
		{strings.Repeat("я", 11), false},
		{strings.Repeat("a", 72), true},
		{strings.Repeat("я", 37), false},
	}
	for _, test := range tests {
		if got := validatePassword(test.password) == ""; got != test.valid {
			t.Errorf("validatePassword(%d bytes) valid = %v, want %v", len(test.password), got, test.valid)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	for raw, want := range map[string]string{
		" Olena@Students.Example.Test ": "olena@students.example.test",
		"Olena <olena@example.test>":    "",
		"a@b@example.test":              "",
		"":                              "",
	} {
		got, _ := normalizeEmail(raw)
		if got != want {
			t.Errorf("normalizeEmail(%q) = %q, want %q", raw, got, want)
		}
	}
}

// Атомарність
func TestRegisterWritesInOneTransactionAndMailsAfterCommit(t *testing.T) {
	s, tx, mail := newTestService(t)
	register(t, s, "a@students.example.test")
	expect.Equal(t, "transactions", tx.count, 1)
	expect.Equal(t, "emails", mail.Count(), 1)
	_, _, err := s.Register(context.Background(), RegisterInput{
		Email: "A@students.example.test", Password: testPassword, FirstName: "A", LastName: "B",
	})
	expect.Error(t, "register the same email again", err, ErrEmailTaken)
}

func TestLoginLockoutLastsFortyMinutes(t *testing.T) {
	s, tx, _ := newTestService(t)
	register(t, s, "a@students.example.test")
	ctx := context.Background()
	for attempt := 1; attempt <= 5; attempt++ {
		_, err := s.Login(ctx, LoginInput{Email: "a@students.example.test", Password: "wrong password!!"})
		expect.Error(t, fmt.Sprintf("login attempt %d", attempt), err, ErrInvalidCredentials)
	}
	_, err := s.Login(ctx, LoginInput{Email: "a@students.example.test", Password: testPassword})
	var locked *LockedError
	if !errors.As(err, &locked) {
		t.Fatalf("err = %v, want LockedError", err)
	}
	left := time.Until(locked.Until)
	if left < 40*time.Minute-time.Minute {
		t.Fatalf("lockout left = %s, want at least 39m", left)
	}
	if left > 40*time.Minute {
		t.Fatalf("lockout left = %s, want at most 40m", left)
	}
	if !tx.LockedUntil("a@students.example.test").Equal(locked.Until) {
		t.Fatal("lockout was not stored")
	}
}

func TestSessionLifecycle(t *testing.T) {
	s, _, _ := newTestService(t)
	register(t, s, "a@students.example.test")
	ctx := context.Background()
	session, err := s.Login(ctx, LoginInput{Email: "A@students.example.test", Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "CSRF token", session.CSRFToken, CSRFToken(session.Token))
	expect.Equal(t, "session CSRF token is valid", ValidCSRF(session.Token, session.CSRFToken), true)
	expect.Equal(t, "wrong CSRF token is valid", ValidCSRF(session.Token, "x"), false)

	user, err := s.Authenticate(ctx, session.Token)
	expect.NoError(t, "authenticate", err)
	expect.Equal(t, "authenticated email", user.Email, "a@students.example.test")

	if err := s.Logout(ctx, session.Token); err != nil {
		t.Fatal(err)
	}
	_, err = s.Authenticate(ctx, session.Token)
	expect.Error(t, "authenticate after logout", err, ErrUnauthenticated)
	_, err = s.Authenticate(ctx, "garbage")
	expect.Error(t, "authenticate with a garbage token", err, ErrUnauthenticated)
}

func TestPasswordResetRevokesSessionsInOneTransaction(t *testing.T) {
	s, tx, mail := newTestService(t)
	register(t, s, "a@students.example.test")
	ctx := context.Background()
	old, err := s.Login(ctx, LoginInput{Email: "a@students.example.test", Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ForgotPassword(ctx, "a@students.example.test"); err != nil {
		t.Fatal(err)
	}
	token := lastToken(t, mail)
	before := tx.count
	if err := s.ResetPassword(ctx, token, "a brand new passphrase"); err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "transactions used by the reset", tx.count-before, 1)

	_, err = s.Authenticate(ctx, old.Token)
	expect.Error(t, "authenticate with the old session", err, ErrUnauthenticated)
	err = s.ResetPassword(ctx, token, "another new passphrase")
	expect.Error(t, "reuse the reset token", err, ErrInvalidToken)
	if err := s.ForgotPassword(ctx, "nobody@students.example.test"); err != nil {
		t.Fatalf("unknown email err = %v", err)
	}
}

// Changing the password keeps the current session and signs out every other one.
func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	s, _, _ := newTestService(t)
	ctx := context.Background()
	register(t, s, "a@students.example.test")
	current, err := s.Login(ctx, LoginInput{Email: "a@students.example.test", Password: testPassword})
	expect.NoError(t, "sign in on this device", err)
	other, err := s.Login(ctx, LoginInput{Email: "a@students.example.test", Password: testPassword})
	expect.NoError(t, "sign in on another device", err)
	newPassword := "another long passphrase"

	var invalid *ValidationError
	err = s.ChangePassword(ctx, current.User, current.Token, "wrong password!!", newPassword)
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	expect.Equal(t, "wrong current password", invalid.Fields["currentPassword"], "Password is incorrect")
	err = s.ChangePassword(ctx, current.User, current.Token, testPassword, "short")
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	expect.Equal(t, "short new password", invalid.Fields["newPassword"], "Password must contain at least 12 characters")
	err = s.ChangePassword(ctx, current.User, current.Token, testPassword, testPassword)
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	expect.Equal(t, "unchanged password", invalid.Fields["newPassword"], "New password must differ from the current one")

	expect.NoError(t, "change the password", s.ChangePassword(ctx, current.User, current.Token, testPassword, newPassword))
	_, err = s.Authenticate(ctx, current.Token)
	expect.NoError(t, "current session", err)
	_, err = s.Authenticate(ctx, other.Token)
	expect.Error(t, "other session", err, ErrUnauthenticated)
	_, err = s.Login(ctx, LoginInput{Email: "a@students.example.test", Password: testPassword})
	expect.Error(t, "sign in with the old password", err, ErrInvalidCredentials)
	_, err = s.Login(ctx, LoginInput{Email: "a@students.example.test", Password: newPassword})
	expect.NoError(t, "sign in with the new password", err)
}

// Wrong passwords before a sensitive action count towards the sign-in lockout.
func TestConfirmPasswordUsesTheLoginLockout(t *testing.T) {
	s, _, _ := newTestService(t)
	ctx := context.Background()
	register(t, s, "a@students.example.test")
	session, err := s.Login(ctx, LoginInput{Email: "a@students.example.test", Password: testPassword})
	expect.NoError(t, "sign in", err)
	expect.NoError(t, "confirm the right password", s.ConfirmPassword(ctx, session.User, testPassword))
	for range 5 {
		var invalid *ValidationError
		if err := s.ConfirmPassword(ctx, session.User, "wrong password!!"); !errors.As(err, &invalid) {
			t.Fatalf("err = %v, want ValidationError", err)
		}
	}
	var locked *LockedError
	if err := s.ConfirmPassword(ctx, session.User, testPassword); !errors.As(err, &locked) {
		t.Fatalf("err = %v, want LockedError", err)
	}
}
