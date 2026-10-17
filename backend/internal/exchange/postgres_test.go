package exchange

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
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
	service := NewService(store, store, store)

	var users []string
	t.Cleanup(func() {
		for _, statement := range []string{
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
	requester := student("A", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	recipient := student("B", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	created, err := service.CreateRequest(ctx, requester, NewRequest{RecipientID: recipient,
		TeachSkillID: datatest.GuitarSkillID, LearnSkillID: datatest.PhotoshopSkillID, Format: data.FormatOnline,
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
		if errs[i] != nil {
			t.Fatalf("acceptance %d: err = %v", i, errs[i])
		}
		if results[i].Exchange.ID == "" || results[i].Exchange.ID != results[0].Exchange.ID {
			t.Fatalf("acceptance %d returned exchange %q, want %q", i, results[i].Exchange.ID, results[0].Exchange.ID)
		}
	}
	var exchanges, accepted int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM exchanges WHERE source_request_id = $1`, created.ID).Scan(&exchanges); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM exchange_request_status_history
		WHERE request_id = $1 AND status = 'ACCEPTED'`, created.ID).Scan(&accepted); err != nil {
		t.Fatal(err)
	}
	if exchanges != 1 || accepted != 1 {
		t.Fatalf("exchanges = %d, acceptances in history = %d", exchanges, accepted)
	}
}
