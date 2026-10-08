package data

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

const (
	demoNataliia    = "60000000-0000-0000-0000-000000000008"
	demoViktor      = "60000000-0000-0000-0000-000000000009"
	demoTarasToOlha = "70000000-0000-0000-0000-000000000001"
)

// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestRequestRecipientOnDemoData(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	andrii, err := store.RequestRecipient(ctx, demoOlha, demoAndrii)
	if err != nil {
		t.Fatal(err)
	}
	if !andrii.AcceptsRequests || andrii.SameUniversityOnly || andrii.UserID != demoAndrii || andrii.City != "Київ" ||
		len(andrii.TeachingSkills) != 2 || len(andrii.LearningSkills) != 1 {
		t.Fatalf("Andrii = %+v", andrii)
	}
	dmytro, err := store.RequestRecipient(ctx, demoIryna, demoDmytro)
	if err != nil || dmytro.AcceptsRequests {
		t.Fatalf("Dmytro accepts requests = %v, err %v", dmytro.AcceptsRequests, err)
	}

	for _, test := range []struct{ name, sender, recipient string }{
		{"hidden profile", demoOlha, demoNataliia},
		{"unverified email", demoOlha, demoViktor},
		{"blocked by the sender", demoOlha, demoOleh},
		{"sender blocked by the recipient", demoOleh, demoOlha},
		{"the sender", demoOlha, demoOlha},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.RequestRecipient(ctx, test.sender, test.recipient); !errors.Is(err, ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
		})
	}
}

// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestRequestByIDOnDemoData(t *testing.T) {
	store := openTestStore(t)
	r, err := store.RequestByID(context.Background(), demoTarasToOlha)
	if err != nil {
		t.Fatal(err)
	}
	if r.Requester.UserID != demoTaras || r.Recipient.UserID != demoOlha || r.Requester.FirstName != "Тарас" ||
		r.Recipient.UniversityName == "" || r.Format != "OFFLINE" || r.TotalSessions != 4 || r.ExchangeID != "" {
		t.Fatalf("request = %+v", r)
	}
	if r.RequesterTeaches.SkillID != photoshop || r.RequesterTeaches.TeacherLevel != "BEGINNER" ||
		r.RequesterTeaches.LearnerLevel != "BEGINNER" || r.RequesterTeaches.Sessions != 2 || r.RequesterTeaches.DurationMinutes != 60 {
		t.Fatalf("Taras teaches %+v", r.RequesterTeaches)
	}
	if r.RecipientTeaches.SkillID != guitarSkill || r.RecipientTeaches.TeacherLevel != "ADVANCED" || r.RecipientTeaches.Name != "Гітара" {
		t.Fatalf("Olha teaches %+v", r.RecipientTeaches)
	}
	if _, err := store.RequestByID(context.Background(), "70000000-0000-0000-0000-000000000999"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown request: err = %v", err)
	}
}

// Runs inside a transaction that is rolled back.
// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestCreateRequestAgainstPostgres(t *testing.T) {
	store := openTestStore(t)
	rollback := errors.New("roll back test data")
	err := store.WithinTx(context.Background(), func(ctx context.Context) error {
		olhaToAndrii := NewRequest{RequesterID: demoOlha, RecipientID: demoAndrii, TeachSkillID: guitarSkill,
			LearnSkillID: photoshop, Format: FormatOnline, TeachSessions: 2, TeachDurationMinutes: 60,
			LearnSessions: 1, LearnDurationMinutes: 120, Message: "Привіт!"}
		andriiToOlha := NewRequest{RequesterID: demoAndrii, RecipientID: demoOlha, TeachSkillID: photoshop,
			LearnSkillID: guitarSkill, Format: FormatOnline, TeachSessions: 1, TeachDurationMinutes: 60,
			LearnSessions: 1, LearnDurationMinutes: 60}
		// A failed statement aborts the transaction, so each one runs in a savepoint.
		create := func(r NewRequest) (string, error) {
			t.Helper()
			if _, err := store.conn(ctx).Exec(ctx, `SAVEPOINT create_request`); err != nil {
				t.Fatal(err)
			}
			id, err := store.CreateRequest(ctx, r)
			if err != nil {
				if _, err := store.conn(ctx).Exec(ctx, `ROLLBACK TO SAVEPOINT create_request`); err != nil {
					t.Fatal(err)
				}
			}
			return id, err
		}

		id, err := create(olhaToAndrii)
		if err != nil {
			t.Fatal(err)
		}
		r, err := store.RequestByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != RequestPending || r.Requester.UserID != demoOlha || r.Recipient.UserID != demoAndrii ||
			r.TotalSessions != 3 || r.Message != "Привіт!" || r.RespondedAt != nil {
			t.Fatalf("request = %+v", r)
		}
		if r.RequesterTeaches.SkillID != guitarSkill || r.RequesterTeaches.TeacherLevel != "ADVANCED" ||
			r.RequesterTeaches.LearnerLevel != "BEGINNER" || r.RequesterTeaches.Sessions != 2 ||
			r.RecipientTeaches.SkillID != photoshop || r.RecipientTeaches.DurationMinutes != 120 {
			t.Fatalf("terms: %+v, %+v", r.RequesterTeaches, r.RecipientTeaches)
		}
		var history int
		if err := store.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM exchange_request_status_history
			WHERE request_id = $1 AND status = 'PENDING' AND changed_by_user_id = $2`, id, demoOlha).Scan(&history); err != nil || history != 1 {
			t.Fatalf("history rows = %d, err %v", history, err)
		}

		if _, err := create(olhaToAndrii); !errors.Is(err, ErrDuplicateRequest) {
			t.Fatalf("the same request again: err = %v", err)
		}
		if _, err := create(andriiToOlha); !errors.Is(err, ErrDuplicateRequest) {
			t.Fatalf("the reverse request: err = %v", err)
		}
		// The seed has a pending request from Taras with the same skills.
		olhaToTaras := olhaToAndrii
		olhaToTaras.RecipientID, olhaToTaras.Format = demoTaras, FormatOffline
		if _, err := create(olhaToTaras); !errors.Is(err, ErrDuplicateRequest) {
			t.Fatalf("the reverse of the demo request: err = %v", err)
		}

		for _, status := range []string{"DECLINED", "WITHDRAWN"} {
			if _, err := store.conn(ctx).Exec(ctx, `UPDATE exchange_requests SET status = $2, responded_at = now()
				WHERE id = $1`, id, status); err != nil {
				t.Fatal(err)
			}
			if id, err = create(andriiToOlha); err != nil {
				t.Fatalf("after %s: err = %v", status, err)
			}
		}
		empty, err := store.RequestByID(ctx, id)
		if err != nil || empty.Message != "" {
			t.Fatalf("no message = %q, err %v", empty.Message, err)
		}

		if err := store.RemoveUserSkill(ctx, TeachingList, demoAndrii, photoshop); err != nil {
			t.Fatal(err)
		}
		if _, err := create(olhaToAndrii); !errors.Is(err, ErrNotFound) {
			t.Fatalf("after a skill was removed: err = %v", err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTx err = %v", err)
	}
}

// Runs inside a transaction that is rolled back.
// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestRequestRecipientScope(t *testing.T) {
	store := openTestStore(t)
	rollback := errors.New("roll back test data")
	err := store.WithinTx(context.Background(), func(ctx context.Context) error {
		var universityID string
		if err := store.conn(ctx).QueryRow(ctx, `INSERT INTO universities (name, email_domain) VALUES ('Інший університет', $1)
			RETURNING id`, fmt.Sprintf("other-%d.example.test", time.Now().UnixNano())).Scan(&universityID); err != nil {
			t.Fatal(err)
		}
		id, err := store.CreateUser(ctx, NewUser{UniversityID: universityID,
			Email: fmt.Sprintf("scope-%d@students.example.test", time.Now().UnixNano()), FirstName: "S", LastName: "Test"})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.MarkEmailVerified(ctx, id); err != nil {
			t.Fatal(err)
		}
		if _, err := store.conn(ctx).Exec(ctx, `INSERT INTO user_profile_settings (user_id, request_scope)
			VALUES ($1, 'SAME_UNIVERSITY')`, id); err != nil {
			t.Fatal(err)
		}
		recipient, err := store.RequestRecipient(ctx, demoOlha, id)
		if err != nil {
			t.Fatal(err)
		}
		if !recipient.AcceptsRequests || !recipient.SameUniversityOnly || recipient.UniversityID != universityID {
			t.Fatalf("recipient = %+v", recipient)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTx err = %v", err)
	}
}
