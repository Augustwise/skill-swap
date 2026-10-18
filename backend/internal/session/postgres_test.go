package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"skillswap/backend/internal/communication"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/exchange"
	"skillswap/backend/internal/expect"
)

const (
	motionSkill  = "30000000-0000-0000-0000-000000000008"
	blenderSkill = "30000000-0000-0000-0000-000000000009"
)

type postgresEnv struct {
	pool      *pgxpool.Pool
	store     *data.Postgres
	exchanges *exchange.Service
	sessions  *Service
	users     []string
}

// newPostgresEnv is skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
// The students it creates are deleted with all their data when the test ends.
func newPostgresEnv(t *testing.T) *postgresEnv {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for the local PostgreSQL integration test")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store := data.NewPostgres(pool)
	comm := communication.NewService(store, store, store)
	env := &postgresEnv{
		pool:      pool,
		store:     store,
		exchanges: exchange.NewService(store, store, store, comm),
		sessions:  NewService(store, store, store, comm),
	}
	t.Cleanup(func() {
		for _, statement := range []string{
			`DELETE FROM notifications WHERE recipient_id = ANY($1) OR actor_id = ANY($1)`,
			`DELETE FROM message_receipts WHERE user_id = ANY($1)
				OR message_id IN (SELECT id FROM messages WHERE author_id = ANY($1))`,
			`DELETE FROM messages WHERE conversation_id IN (SELECT id FROM conversations WHERE created_by_user_id = ANY($1))`,
			`DELETE FROM conversation_participants
				WHERE conversation_id IN (SELECT id FROM conversations WHERE created_by_user_id = ANY($1))`,
			`DELETE FROM conversations WHERE created_by_user_id = ANY($1)`,
			`DELETE FROM session_confirmations WHERE session_id IN (SELECT id FROM sessions WHERE exchange_id IN (SELECT id FROM exchanges WHERE user_a_id = ANY($1)))`,
			`DELETE FROM session_schedule_proposals WHERE session_id IN (SELECT id FROM sessions WHERE exchange_id IN (SELECT id FROM exchanges WHERE user_a_id = ANY($1)))`,
			`DELETE FROM sessions WHERE exchange_id IN (SELECT id FROM exchanges WHERE user_a_id = ANY($1))`,
			`DELETE FROM meeting_places WHERE user_id = ANY($1)`,
			`DELETE FROM exchange_confirmations WHERE exchange_id IN (SELECT id FROM exchanges WHERE user_a_id = ANY($1))`,
			`DELETE FROM exchange_status_history WHERE exchange_id IN (SELECT id FROM exchanges WHERE user_a_id = ANY($1))`,
			`DELETE FROM exchange_skill_commitments WHERE teacher_id = ANY($1)`,
			`DELETE FROM exchanges WHERE user_a_id = ANY($1)`,
			`DELETE FROM exchange_request_status_history
				WHERE request_id IN (SELECT id FROM exchange_requests WHERE requester_id = ANY($1))`,
			`DELETE FROM exchange_requests WHERE requester_id = ANY($1)`,
			`DELETE FROM user_teaching_skills WHERE user_id = ANY($1)`,
			`DELETE FROM user_learning_skills WHERE user_id = ANY($1)`,
			`DELETE FROM user_lesson_formats WHERE user_id = ANY($1)`,
			`DELETE FROM user_verifications WHERE user_id = ANY($1)`,
			`DELETE FROM users WHERE id = ANY($1)`,
		} {
			if _, err := pool.Exec(context.Background(), statement, env.users); err != nil {
				t.Errorf("clean up: %v", err)
			}
		}
	})
	return env
}

func (env *postgresEnv) student(t *testing.T, name, teach, learn string) string {
	t.Helper()
	ctx := context.Background()
	id, err := env.store.CreateUser(ctx, data.NewUser{
		UniversityID: datatest.DemoUniversityID,
		Email:        fmt.Sprintf("session-%s-%d@students.example.test", name, time.Now().UnixNano()),
		FirstName:    name,
		LastName:     "Test",
	})
	if err != nil {
		t.Fatal(err)
	}
	env.users = append(env.users, id)
	if err := env.store.MarkEmailVerified(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := env.store.UpdateProfile(ctx, id, data.ProfileUpdate{
		FirstName: name,
		LastName:  "Test",
		City:      "Київ",
		Formats:   []string{data.FormatOnline, data.FormatOffline},
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.store.AddUserSkill(ctx, data.TeachingList, id, teach, "ADVANCED"); err != nil {
		t.Fatal(err)
	}
	if err := env.store.AddUserSkill(ctx, data.LearningList, id, learn, "BEGINNER"); err != nil {
		t.Fatal(err)
	}
	return id
}

// exchange starts an exchange where userA teaches motion design and userB teaches Blender.
func (env *postgresEnv) exchange(t *testing.T, userA, userB string, sessionsEachWay int) data.Exchange {
	t.Helper()
	ctx := context.Background()
	req, err := env.exchanges.CreateRequest(ctx, userA, exchange.NewRequest{
		RecipientID:          userB,
		TeachSkillID:         motionSkill,
		LearnSkillID:         blenderSkill,
		Format:               data.FormatOnline,
		TeachSessions:        sessionsEachWay,
		TeachDurationMinutes: 60,
		LearnSessions:        sessionsEachWay,
		LearnDurationMinutes: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	acc, err := env.exchanges.AcceptRequest(ctx, userB, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	return acc.Exchange
}

// schedule proposes a one-hour online lesson taught by teacherID and lets the other participant accept it.
func (env *postgresEnv) schedule(t *testing.T, exchangeID, teacherID, learnerID string, startsAt time.Time) data.Session {
	t.Helper()
	ctx := context.Background()
	proposed, err := env.sessions.ProposeExchangeSession(ctx, teacherID, exchangeID, ProposalInput{
		Title:      "Заняття",
		TeacherID:  teacherID,
		StartsAt:   startsAt.Format(time.RFC3339),
		EndsAt:     startsAt.Add(time.Hour).Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/lesson",
	})
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := env.sessions.AcceptSessionProposal(ctx, learnerID, proposed.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	return accepted.Session
}

// count runs a SELECT count(*) query.
func (env *postgresEnv) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := env.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// text runs a query that returns one text value.
func (env *postgresEnv) text(t *testing.T, query string, args ...any) string {
	t.Helper()
	var value string
	if err := env.pool.QueryRow(context.Background(), query, args...).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

// Checks against PostgreSQL:
// 1. Two concurrent acceptances of the same proposal return the same SCHEDULED session and create one notification.
// 2. Two concurrent acceptances of overlapping proposals serialize via LockUsers so only one succeeds and the other gets ErrSessionConflict.
func TestAcceptSessionProposalConcurrentlyAgainstPostgres(t *testing.T) {
	env := newPostgresEnv(t)
	ctx := context.Background()
	userA := env.student(t, "A", motionSkill, blenderSkill)
	userB := env.student(t, "B", blenderSkill, motionSkill)
	acc := env.exchange(t, userA, userB, 2)

	// Part 1: two parallel acceptances of the same session proposal.
	tStart := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	tEnd := tStart.Add(60 * time.Minute)
	prop1, err := env.sessions.ProposeExchangeSession(ctx, userA, acc.ID, ProposalInput{
		Title:      "Гітара — заняття 1",
		TeacherID:  userA,
		StartsAt:   tStart.Format(time.RFC3339),
		EndsAt:     tEnd.Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/s1",
	})
	if err != nil {
		t.Fatal(err)
	}

	const attempts = 2
	results := make([]ProposalResult, attempts)
	errs := make([]error, attempts)
	var wg sync.WaitGroup
	for i := range attempts {
		wg.Go(func() { results[i], errs[i] = env.sessions.AcceptSessionProposal(ctx, userB, prop1.Session.ID) })
	}
	wg.Wait()
	for i := range attempts {
		if errs[i] != nil {
			t.Fatalf("acceptance %d: err = %v", i, errs[i])
		}
		expect.Equal(t, "session status after acceptance", results[i].Session.Status, data.SessionScheduled)
		expect.Equal(t, "accepted proposal", results[i].Proposal.ID, prop1.Proposal.ID)
	}

	// Part 2: two overlapping proposals accepted concurrently -> only one can become SCHEDULED.
	overlapStart1 := time.Now().UTC().Add(96 * time.Hour).Truncate(time.Minute)
	overlapEnd1 := overlapStart1.Add(60 * time.Minute)
	overlapStart2 := overlapStart1.Add(30 * time.Minute)
	overlapEnd2 := overlapEnd1.Add(30 * time.Minute)

	p2, err := env.sessions.ProposeExchangeSession(ctx, userA, acc.ID, ProposalInput{
		Title:      "Гітара — заняття 2",
		TeacherID:  userA,
		StartsAt:   overlapStart1.Format(time.RFC3339),
		EndsAt:     overlapEnd1.Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/s2",
	})
	if err != nil {
		t.Fatal(err)
	}
	p3, err := env.sessions.ProposeExchangeSession(ctx, userB, acc.ID, ProposalInput{
		Title:      "Photoshop — заняття 1",
		TeacherID:  userB,
		StartsAt:   overlapStart2.Format(time.RFC3339),
		EndsAt:     overlapEnd2.Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/s3",
	})
	if err != nil {
		t.Fatal(err)
	}

	var err2, err3 error
	wg.Go(func() { _, err2 = env.sessions.AcceptSessionProposal(ctx, userB, p2.Session.ID) })
	wg.Go(func() { _, err3 = env.sessions.AcceptSessionProposal(ctx, userA, p3.Session.ID) })
	wg.Wait()

	accepted, conflicts := 0, 0
	for _, err := range []error{err2, err3} {
		switch {
		case err == nil:
			accepted++
		case errors.Is(err, ErrSessionConflict):
			conflicts++
		default:
			t.Fatalf("overlapping acceptance: err = %v", err)
		}
	}
	expect.Equal(t, "accepted overlapping proposals", accepted, 1)
	expect.Equal(t, "rejected overlapping proposals", conflicts, 1)
}

// Both participants confirming a session or the whole exchange at the same time complete it
// exactly once, with one confirmation per participant.
func TestCompletionConcurrentlyAgainstPostgres(t *testing.T) {
	env := newPostgresEnv(t)
	ctx := context.Background()
	userA := env.student(t, "A", motionSkill, blenderSkill)
	userB := env.student(t, "B", blenderSkill, motionSkill)
	ex := env.exchange(t, userA, userB, 1)
	start := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	lessons := []data.Session{
		env.schedule(t, ex.ID, userA, userB, start),
		env.schedule(t, ex.ID, userB, userA, start.Add(2*time.Hour)),
	}
	env.sessions.now = func() time.Time { return start.Add(4 * time.Hour) }

	// Both participants confirm each lesson at the same time, and userA also sends a duplicate.
	var wg sync.WaitGroup
	for _, lesson := range lessons {
		errs := make([]error, 3)
		for i, user := range []string{userA, userB, userA} {
			wg.Go(func() { _, errs[i] = env.sessions.ConfirmSessionCompletion(ctx, user, lesson.ID) })
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Fatalf("confirmation of %s: err = %v", lesson.ID, err)
			}
		}
		expect.Equal(t, "lesson status",
			env.text(t, `SELECT status::text FROM sessions WHERE id = $1`, lesson.ID), data.SessionCompleted)
		expect.Equal(t, "completed lessons without completedAt",
			env.count(t, `SELECT count(*) FROM sessions WHERE id = $1 AND completed_at IS NULL`, lesson.ID), 0)
		expect.Equal(t, "completion confirmations",
			env.count(t, `SELECT count(*) FROM session_confirmations WHERE session_id = $1 AND kind = 'COMPLETION'`, lesson.ID), 2)
		expect.Equal(t, "notices that the lesson counts",
			env.count(t, `SELECT count(*) FROM notifications WHERE event_key = $1`, "session-completed:"+lesson.ID), 1)
	}

	// Both participants confirm the end of the exchange at the same time.
	results := make([]data.Exchange, 2)
	errs := make([]error, 2)
	for i, user := range []string{userA, userB} {
		wg.Go(func() { results[i], errs[i] = env.exchanges.CompleteExchange(ctx, user, ex.ID) })
	}
	wg.Wait()
	completedBy := 0
	for i := range results {
		if errs[i] != nil {
			t.Fatalf("final confirmation: err = %v", errs[i])
		}
		if results[i].Status == data.ExchangeCompleted {
			completedBy++
		}
	}
	expect.Equal(t, "calls that completed the exchange", completedBy, 1)
	expect.Equal(t, "exchange confirmations",
		env.count(t, `SELECT count(*) FROM exchange_confirmations WHERE exchange_id = $1`, ex.ID), 2)
	expect.Equal(t, "COMPLETED history items",
		env.count(t, `SELECT count(*) FROM exchange_status_history WHERE exchange_id = $1 AND status = 'COMPLETED'`, ex.ID), 1)
}

func TestCancelExchangeAgainstPostgres(t *testing.T) {
	env := newPostgresEnv(t)
	ctx := context.Background()
	userA := env.student(t, "A", motionSkill, blenderSkill)
	userB := env.student(t, "B", blenderSkill, motionSkill)
	ex := env.exchange(t, userA, userB, 2)
	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Minute)
	done := env.schedule(t, ex.ID, userA, userB, start)
	scheduled := env.schedule(t, ex.ID, userB, userA, start.Add(2*time.Hour))
	proposed, err := env.sessions.ProposeExchangeSession(ctx, userA, ex.ID, ProposalInput{
		Title:      "Заняття",
		TeacherID:  userA,
		StartsAt:   start.Add(4 * time.Hour).Format(time.RFC3339),
		EndsAt:     start.Add(5 * time.Hour).Format(time.RFC3339),
		MeetingURL: "https://meet.example.test/lesson",
	})
	if err != nil {
		t.Fatal(err)
	}
	env.sessions.now = func() time.Time { return start.Add(90 * time.Minute) }
	for _, user := range []string{userA, userB} {
		if _, err := env.sessions.ConfirmSessionCompletion(ctx, user, done.ID); err != nil {
			t.Fatal(err)
		}
	}

	const reason = "Змінився розклад"
	cancelled, err := env.exchanges.CancelExchange(ctx, userB, ex.ID, reason)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "exchange status", cancelled.Status, data.ExchangeCancelled)
	expect.Equal(t, "cancelledAt is set", cancelled.CancelledAt != nil, true)
	expect.Equal(t, "cancellation reason", cancelled.CancellationReason, reason)
	expect.Equal(t, "completed sessions", cancelled.Progress.CompletedSessions, 1)
	expect.Equal(t, "cancelled sessions", cancelled.Progress.CancelledSessions, 3)

	for _, id := range []string{scheduled.ID, proposed.Session.ID} {
		expect.Equal(t, "lesson status", env.text(t, `SELECT status::text FROM sessions WHERE id = $1`, id), data.SessionCancelled)
		expect.Equal(t, "lesson reason", env.text(t, `SELECT cancellation_reason FROM sessions WHERE id = $1`, id), reason)
	}
	expect.Equal(t, "pending proposals",
		env.count(t, `SELECT count(*) FROM session_schedule_proposals WHERE session_id = $1 AND status = 'PENDING'`, proposed.Session.ID), 0)
	expect.Equal(t, "cancelled by",
		env.text(t, `SELECT changed_by_user_id::text FROM exchange_status_history WHERE exchange_id = $1 AND status = 'CANCELLED'`, ex.ID), userB)
	expect.Equal(t, "reason in history",
		env.text(t, `SELECT reason FROM exchange_status_history WHERE exchange_id = $1 AND status = 'CANCELLED'`, ex.ID), reason)
}
