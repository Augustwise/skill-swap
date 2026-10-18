package exchange

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"skillswap/backend/internal/communication"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/expect"
)

// Two acceptances run in separate transactions, so this test commits its data: it creates
// its own students and removes everything they left behind.
// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestAcceptRequestConcurrentlyAgainstPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for the local PostgreSQL integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store := data.NewPostgres(pool)
	comm := communication.NewService(store, store, store)
	service := NewService(store, store, store, comm)

	var users []string
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
			if _, err := pool.Exec(context.Background(), statement, users); err != nil {
				t.Errorf("clean up: %v", err)
			}
		}
	})
	student := func(name, teach, learn string) string {
		t.Helper()
		id, err := store.CreateUser(ctx, data.NewUser{UniversityID: datatest.DemoUniversityID,
			Email: fmt.Sprintf("accept-%s-%d@students.example.test", name, time.Now().UnixNano()), FirstName: name, LastName: "Test"})
		if err != nil {
			t.Fatal(err)
		}
		users = append(users, id)
		if err := store.MarkEmailVerified(ctx, id); err != nil {
			t.Fatal(err)
		}
		if err := store.UpdateProfile(ctx, id, data.ProfileUpdate{FirstName: name, LastName: "Test", City: "Київ",
			Formats: []string{data.FormatOnline}}); err != nil {
			t.Fatal(err)
		}
		if err := store.AddUserSkill(ctx, data.TeachingList, id, teach, "ADVANCED"); err != nil {
			t.Fatal(err)
		}
		if err := store.AddUserSkill(ctx, data.LearningList, id, learn, "BEGINNER"); err != nil {
			t.Fatal(err)
		}
		return id
	}
	const (
		uiuxSkill = "30000000-0000-0000-0000-000000000006"
		webSkill  = "30000000-0000-0000-0000-000000000007"
	)
	requester := student("A", uiuxSkill, webSkill)
	recipient := student("B", webSkill, uiuxSkill)
	created, err := service.CreateRequest(ctx, requester, NewRequest{RecipientID: recipient,
		TeachSkillID: uiuxSkill, LearnSkillID: webSkill, Format: data.FormatOnline,
		TeachSessions: 1, TeachDurationMinutes: 60, LearnSessions: 1, LearnDurationMinutes: 60})
	if err != nil {
		t.Fatal(err)
	}

	const attempts = 2
	results := make([]Acceptance, attempts)
	errs := make([]error, attempts)
	var wg sync.WaitGroup
	for i := range attempts {
		wg.Go(func() { results[i], errs[i] = service.AcceptRequest(ctx, recipient, created.ID) })
	}
	wg.Wait()
	for i := range attempts {
		name := fmt.Sprintf("acceptance %d", i)
		expect.NoError(t, name, errs[i])
		expect.Equal(t, name+": exchange ID is set", results[i].Exchange.ID != "", true)
		expect.Equal(t, name+": exchange", results[i].Exchange.ID, results[0].Exchange.ID)
	}
	var exchanges, accepted, conversations, notifications, draftSessions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM exchanges WHERE source_request_id = $1`, created.ID).Scan(&exchanges); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM exchange_request_status_history
		WHERE request_id = $1 AND status = 'ACCEPTED'`, created.ID).Scan(&accepted); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM conversations
		WHERE exchange_request_id = $1 AND exchange_id = $2 AND type = 'EXCHANGE'`,
		created.ID, results[0].Exchange.ID).Scan(&conversations); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notifications
		WHERE recipient_id = $1 AND event_type = 'REQUEST_RESPONSE' AND exchange_request_id = $2`,
		requester, created.ID).Scan(&notifications); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sessions
		WHERE exchange_id = $1 AND status = 'DRAFT'`, results[0].Exchange.ID).Scan(&draftSessions); err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "exchanges", exchanges, 1)
	expect.Equal(t, "acceptances in history", accepted, 1)
	expect.Equal(t, "exchange conversations", conversations, 1)
	expect.Equal(t, "requester's notifications", notifications, 1)
	expect.Equal(t, "draft sessions", draftSessions, 2)
}
