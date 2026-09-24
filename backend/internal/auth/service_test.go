package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"skillswap/backend/internal/data/datatest"
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
	if err != nil || !sent {
		t.Fatalf("register: sent = %v, err = %v", sent, err)
	}
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
	if tx.count != 1 || mail.Count() != 1 {
		t.Fatalf("transactions = %d, emails = %d", tx.count, mail.Count())
	}
	_, _, err := s.Register(context.Background(), RegisterInput{
		Email: "A@students.example.test", Password: testPassword, FirstName: "A", LastName: "B",
	})
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("duplicate register err = %v", err)
	}
}

func TestLoginLockoutLastsTwoHours(t *testing.T) {
	s, tx, _ := newTestService(t)
	register(t, s, "a@students.example.test")
	ctx := context.Background()
	for attempt := 1; attempt <= 5; attempt++ {
		_, err := s.Login(ctx, LoginInput{Email: "a@students.example.test", Password: "wrong password!!"})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d err = %v", attempt, err)
		}
	}
	_, err := s.Login(ctx, LoginInput{Email: "a@students.example.test", Password: testPassword})
	var locked *LockedError
	if !errors.As(err, &locked) {
		t.Fatalf("err = %v, want LockedError", err)
	}
	if left := time.Until(locked.Until); left < 2*time.Hour-time.Minute || left > 2*time.Hour {
		t.Fatalf("lockout left = %s, want 2h", left)
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
	if session.CSRFToken != CSRFToken(session.Token) || !ValidCSRF(session.Token, session.CSRFToken) || ValidCSRF(session.Token, "x") {
		t.Fatal("CSRF token does not match the session")
	}
	if user, err := s.Authenticate(ctx, session.Token); err != nil || user.Email != "a@students.example.test" {
		t.Fatalf("authenticate: %+v, %v", user, err)
	}
	if err := s.Logout(ctx, session.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, session.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("after logout err = %v", err)
	}
	if _, err := s.Authenticate(ctx, "garbage"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("garbage token err = %v", err)
	}
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
	if tx.count != before+1 {
		t.Fatalf("reset used %d transactions, want 1", tx.count-before)
	}
	if _, err := s.Authenticate(ctx, old.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("old session err = %v", err)
	}
	if err := s.ResetPassword(ctx, token, "another new passphrase"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("reused token err = %v", err)
	}
	if err := s.ForgotPassword(ctx, "nobody@students.example.test"); err != nil {
		t.Fatalf("unknown email err = %v", err)
	}
}
