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
)

// Checks LR2 5.3 and NFR-05 against PostgreSQL:
// 1. Two concurrent acceptances of the same proposal return the same SCHEDULED session and create one notification.
// 2. Two concurrent acceptances of overlapping proposals serialize via LockUsers so only one succeeds and the other gets ErrSessionConflict.
// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestAcceptSessionProposalConcurrentlyAgainstPostgres(t *testing.T) {
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
	exchanges := exchange.NewService(store, store, store, comm)
	sessions := NewService(store, store, store, comm)

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
			`DELETE FROM meeting_places WHERE user_id = ANY($1)`,
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
		id, err := store.CreateUser(ctx, data.NewUser{
			UniversityID: datatest.DemoUniversityID,
			Email:        fmt.Sprintf("session-%s-%d@students.example.test", name, time.Now().UnixNano()),
			FirstName:    name,
			LastName:     "Test",
		})
		if err != nil {
			t.Fatal(err)
		}
		users = append(users, id)
		if err := store.MarkEmailVerified(ctx, id); err != nil {
			t.Fatal(err)
		}
		if err := store.UpdateProfile(ctx, id, data.ProfileUpdate{
			FirstName: name,
			LastName:  "Test",
			City:      "Київ",
			Formats:   []string{data.FormatOnline, data.FormatOffline},
		}); err != nil {
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
		motionSkill  = "30000000-0000-0000-0000-000000000008"
		blenderSkill = "30000000-0000-0000-0000-000000000009"
	)
	userA := student("A", motionSkill, blenderSkill)
	userB := student("B", blenderSkill, motionSkill)
	req, err := exchanges.CreateRequest(ctx, userA, exchange.NewRequest{
		RecipientID:          userB,
		TeachSkillID:         motionSkill,
		LearnSkillID:         blenderSkill,
		Format:               data.FormatOnline,
		TeachSessions:        2,
		TeachDurationMinutes: 60,
		LearnSessions:        2,
		LearnDurationMinutes: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	acc, err := exchanges.AcceptRequest(ctx, userB, req.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Part 1: two parallel acceptances of the same session proposal.
	tStart := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	tEnd := tStart.Add(60 * time.Minute)
	prop1, err := sessions.ProposeExchangeSession(ctx, userA, acc.Exchange.ID, ProposalInput{
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
		wg.Go(func() { results[i], errs[i] = sessions.AcceptSessionProposal(ctx, userB, prop1.Session.ID) })
	}
	wg.Wait()
	for i := range attempts {
		if errs[i] != nil {
			t.Fatalf("acceptance %d: err = %v", i, errs[i])
		}
		if results[i].Session.Status != data.SessionScheduled || results[i].Proposal.ID != prop1.Proposal.ID {
			t.Fatalf("acceptance %d = %+v", i, results[i])
		}
	}

	// Part 2: two overlapping proposals accepted concurrently -> only one can become SCHEDULED.
	overlapStart1 := time.Now().UTC().Add(96 * time.Hour).Truncate(time.Minute)
	overlapEnd1 := overlapStart1.Add(60 * time.Minute)
	overlapStart2 := overlapStart1.Add(30 * time.Minute)
	overlapEnd2 := overlapEnd1.Add(30 * time.Minute)

	p2, err := sessions.ProposeExchangeSession(ctx, userA, acc.Exchange.ID, ProposalInput{
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
	p3, err := sessions.ProposeExchangeSession(ctx, userB, acc.Exchange.ID, ProposalInput{
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
	wg.Go(func() { _, err2 = sessions.AcceptSessionProposal(ctx, userB, p2.Session.ID) })
	wg.Go(func() { _, err3 = sessions.AcceptSessionProposal(ctx, userA, p3.Session.ID) })
	wg.Wait()

	if (err2 == nil && !errors.Is(err3, ErrSessionConflict)) ||
		(err3 == nil && !errors.Is(err2, ErrSessionConflict)) ||
		(err2 != nil && err3 != nil) {
		t.Fatalf("concurrent overlapping acceptances: err2 = %v, err3 = %v, want one nil and one ErrSessionConflict", err2, err3)
	}
}
