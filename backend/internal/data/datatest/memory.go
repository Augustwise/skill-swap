package datatest

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"skillswap/backend/internal/data"
)

const DemoUniversityID = "10000000-0000-0000-0000-000000000001"

type Memory struct {
	mu           sync.Mutex
	universities map[string]string
	users        map[string]*account
	sessions     map[string]*session
	tokens       map[string]*token
	throttles    map[string]*throttle
}

type account struct {
	user         data.User
	passwordHash string
}

type session struct {
	userID    string
	expiresAt time.Time
	revoked   bool
}

type token struct {
	userID    string
	purpose   string
	expiresAt time.Time
	used      bool
}

type throttle struct {
	failed      int
	windowStart time.Time
	lockedUntil time.Time
}

var (
	_ data.IAuthData    = (*Memory)(nil)
	_ data.ITransaction = (*Memory)(nil)
)

func NewMemory() *Memory {
	return &Memory{
		universities: map[string]string{"students.example.test": DemoUniversityID},
		users:        map[string]*account{},
		sessions:     map[string]*session{},
		tokens:       map[string]*token{},
		throttles:    map[string]*throttle{},
	}
}

func (m *Memory) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (m *Memory) ExpireTokens() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tokens {
		t.expiresAt = time.Now().Add(-time.Minute)
	}
}

func (m *Memory) LockedUntil(email string) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.throttles[email]; ok {
		return t.lockedUntil
	}
	return time.Time{}
}

func (m *Memory) byEmail(email string) *account {
	for _, a := range m.users {
		if a.user.Email == email {
			return a
		}
	}
	return nil
}

func (m *Memory) UniversityIDByEmailDomain(_ context.Context, domain string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.universities[domain]
	if !ok {
		return "", data.ErrNotFound
	}
	return id, nil
}

func (m *Memory) CreateUser(_ context.Context, user data.NewUser) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byEmail(user.Email) != nil {
		return "", data.ErrEmailTaken
	}
	id := fmt.Sprintf("40000000-0000-0000-0000-%012d", len(m.users)+1)
	m.users[id] = &account{user: data.User{
		ID:           id,
		Email:        user.Email,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		UniversityID: user.UniversityID,
		Role:         "STUDENT",
		Status:       "ACTIVE",
		CreatedAt:    time.Now().UTC(),
	}}
	return id, nil
}

func (m *Memory) CreateLocalIdentity(_ context.Context, userID, passwordHash string) error {
	return m.SetPassword(context.Background(), userID, passwordHash)
}

func (m *Memory) SetPassword(_ context.Context, userID, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.users[userID]; ok {
		a.passwordHash = passwordHash
	}
	return nil
}

func (m *Memory) CreateEmailVerification(context.Context, string, string) error {
	return nil
}

func (m *Memory) MarkEmailVerified(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.users[userID]; ok {
		a.user.EmailVerified = true
	}
	return nil
}

func (m *Memory) UserByID(_ context.Context, userID string) (data.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.users[userID]
	if !ok {
		return data.User{}, data.ErrNotFound
	}
	return a.user, nil
}

func (m *Memory) UserByEmail(_ context.Context, email string) (data.User, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a := m.byEmail(email)
	if a == nil {
		return data.User{}, "", data.ErrNotFound
	}
	return a.user, a.passwordHash, nil
}

func (m *Memory) UserBySession(_ context.Context, tokenHash string) (data.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[tokenHash]
	if !ok || s.revoked || !s.expiresAt.After(time.Now()) {
		return data.User{}, data.ErrNotFound
	}
	a, ok := m.users[s.userID]
	if !ok || a.user.Status != "ACTIVE" {
		return data.User{}, data.ErrNotFound
	}
	return a.user, nil
}

func (m *Memory) CreateSession(_ context.Context, userID string, stored data.StoredToken, _ string, _ netip.Addr) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[stored.Hash] = &session{userID: userID, expiresAt: stored.ExpiresAt}
	return nil
}

func (m *Memory) RecordLogin(context.Context, string) error {
	return nil
}

func (m *Memory) RevokeSession(_ context.Context, tokenHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[tokenHash]; ok {
		s.revoked = true
	}
	return nil
}

func (m *Memory) RevokeUserSessions(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.userID == userID {
			s.revoked = true
		}
	}
	return nil
}

func (m *Memory) CreateToken(_ context.Context, userID, purpose string, stored data.StoredToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[stored.Hash] = &token{userID: userID, purpose: purpose, expiresAt: stored.ExpiresAt}
	return nil
}

func (m *Memory) InvalidateTokens(_ context.Context, userID, purpose string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tokens {
		if t.userID == userID && t.purpose == purpose {
			t.used = true
		}
	}
	return nil
}

func (m *Memory) UseToken(_ context.Context, tokenHash, purpose string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tokens[tokenHash]
	if !ok || t.used || t.purpose != purpose || !t.expiresAt.After(time.Now()) {
		return "", data.ErrNotFound
	}
	t.used = true
	return t.userID, nil
}

func (m *Memory) LoginLockedUntil(_ context.Context, email string) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.throttles[email]; ok && t.lockedUntil.After(time.Now()) {
		return t.lockedUntil, nil
	}
	return time.Time{}, nil
}

func (m *Memory) RecordFailedLogin(_ context.Context, email string, policy data.ThrottlePolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	t, ok := m.throttles[email]
	if !ok || t.windowStart.Before(now.Add(-policy.Window)) {
		t = &throttle{windowStart: now}
		m.throttles[email] = t
	}
	t.failed++
	if t.failed >= policy.MaxFailures {
		t.lockedUntil = now.Add(policy.Lockout)
		t.failed = 0
		t.windowStart = now
	}
	return nil
}

func (m *Memory) ClearFailedLogins(_ context.Context, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.throttles, email)
	return nil
}
