package data

import (
	"context"
	"errors"
	"fmt"
	"slices"
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

// Runs inside a transaction that is rolled back.
// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestUserRequestsAgainstPostgres(t *testing.T) {
	store := openTestStore(t)
	rollback := errors.New("roll back test data")
	err := store.WithinTx(context.Background(), func(ctx context.Context) error {
		id, err := store.CreateRequest(ctx, NewRequest{RequesterID: demoOlha, RecipientID: demoAndrii,
			TeachSkillID: guitarSkill, LearnSkillID: photoshop, Format: FormatOnline,
			TeachSessions: 1, TeachDurationMinutes: 60, LearnSessions: 1, LearnDurationMinutes: 60})
		if err != nil {
			t.Fatal(err)
		}
		list := func(userID string, filter RequestFilter, limit, offset int) ([]string, int) {
			t.Helper()
			requests, total, err := store.UserRequests(ctx, userID, filter, limit, offset)
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]string, 0, len(requests))
			for _, r := range requests {
				ids = append(ids, r.ID)
			}
			return ids, total
		}

		// Requests made inside the transaction are the newest.
		sent, total := list(demoOlha, RequestFilter{Direction: Outgoing}, 1, 0)
		if !slices.Equal(sent, []string{id}) || total < 1 {
			t.Fatalf("Olha's sent requests = %v, total %d", sent, total)
		}
		if past, pastTotal := list(demoOlha, RequestFilter{Direction: Outgoing}, 20, 400); len(past) != 0 || pastTotal != total {
			t.Fatalf("past the end = %v, total %d, want %d", past, pastTotal, total)
		}
		if incoming, _ := list(demoOlha, RequestFilter{Direction: Incoming}, 20, 0); slices.Contains(incoming, id) ||
			!slices.Contains(incoming, demoTarasToOlha) {
			t.Fatalf("Olha's incoming requests = %v", incoming)
		}
		if sent, _ := list(demoTaras, RequestFilter{Direction: Outgoing}, 20, 0); !slices.Contains(sent, demoTarasToOlha) {
			t.Fatalf("Taras's sent requests = %v", sent)
		}
		if incoming, _ := list(demoAndrii, RequestFilter{Direction: Incoming, Status: RequestPending}, 20, 0); !slices.Contains(incoming, id) {
			t.Fatalf("Andrii's pending requests = %v", incoming)
		}
		if declined, _ := list(demoAndrii, RequestFilter{Direction: Incoming, Status: RequestDeclined}, 20, 0); slices.Contains(declined, id) {
			t.Fatalf("Andrii's declined requests = %v", declined)
		}
		if _, err := store.conn(ctx).Exec(ctx, `UPDATE exchange_requests SET status = 'DECLINED', responded_at = now()
			WHERE id = $1`, id); err != nil {
			t.Fatal(err)
		}
		if declined, _ := list(demoAndrii, RequestFilter{Direction: Incoming, Status: RequestDeclined}, 20, 0); !slices.Contains(declined, id) {
			t.Fatalf("Andrii's declined requests after the update = %v", declined)
		}

		// Participants keep their requests after a block or a hidden profile.
		if _, err := store.conn(ctx).Exec(ctx, `INSERT INTO user_blocks (blocker_id, blocked_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, demoAndrii, demoOlha); err != nil {
			t.Fatal(err)
		}
		if _, err := store.conn(ctx).Exec(ctx, `INSERT INTO user_profile_settings (user_id, is_discoverable) VALUES ($1, false)
			ON CONFLICT (user_id) DO UPDATE SET is_discoverable = false`, demoAndrii); err != nil {
			t.Fatal(err)
		}
		if sent, _ := list(demoOlha, RequestFilter{Direction: Outgoing}, 20, 0); !slices.Contains(sent, id) {
			t.Fatalf("Olha's sent requests after the block = %v", sent)
		}
		if r, err := store.RequestByID(ctx, id); err != nil || r.Recipient.FirstName == "" {
			t.Fatalf("request after the block = %+v, err %v", r, err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTx err = %v", err)
	}
}

// Runs inside a transaction that is rolled back.
// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestRequestHistoryAgainstPostgres(t *testing.T) {
	store := openTestStore(t)
	rollback := errors.New("roll back test data")
	err := store.WithinTx(context.Background(), func(ctx context.Context) error {
		id, err := store.CreateRequest(ctx, NewRequest{RequesterID: demoOlha, RecipientID: demoAndrii,
			TeachSkillID: guitarSkill, LearnSkillID: photoshop, Format: FormatOnline,
			TeachSessions: 1, TeachDurationMinutes: 60, LearnSessions: 1, LearnDurationMinutes: 60})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.conn(ctx).Exec(ctx, `INSERT INTO exchange_request_status_history (request_id, status, created_at)
			VALUES ($1, 'WITHDRAWN', now() + interval '1 minute')`, id); err != nil {
			t.Fatal(err)
		}
		history, err := store.RequestHistory(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(history) != 2 {
			t.Fatalf("history = %+v", history)
		}
		if h := history[0]; h.Status != RequestPending || h.ChangedByID != demoOlha || h.ChangedByFirstName != "Ольга" || h.ChangedByLastName == "" {
			t.Fatalf("first change = %+v", h)
		}
		if h := history[1]; h.Status != RequestWithdrawn || h.ChangedByID != "" || h.ChangedByFirstName != "" {
			t.Fatalf("change without a user = %+v", h)
		}
		if empty, err := store.RequestHistory(ctx, "70000000-0000-0000-0000-000000000999"); err != nil || len(empty) != 0 {
			t.Fatalf("unknown request history = %+v, err %v", empty, err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTx err = %v", err)
	}
}

// Runs inside a transaction that is rolled back.
// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestUpdateRequestStatusAgainstPostgres(t *testing.T) {
	store := openTestStore(t)
	rollback := errors.New("roll back test data")
	unknown := "70000000-0000-0000-0000-000000000999"
	err := store.WithinTx(context.Background(), func(ctx context.Context) error {
		state, err := store.LockRequest(ctx, demoTarasToOlha)
		if err != nil {
			t.Fatal(err)
		}
		want := RequestState{ID: demoTarasToOlha, RequesterID: demoTaras, RecipientID: demoOlha, Status: RequestPending}
		if state != want {
			t.Fatalf("state = %+v", state)
		}
		if err := store.UpdateRequestStatus(ctx, demoTarasToOlha, RequestDeclined, demoOlha); err != nil {
			t.Fatal(err)
		}
		r, err := store.RequestByID(ctx, demoTarasToOlha)
		if err != nil || r.Status != RequestDeclined || r.RespondedAt == nil {
			t.Fatalf("request = %+v, err %v", r, err)
		}
		history, err := store.RequestHistory(ctx, demoTarasToOlha)
		if err != nil || len(history) != 2 {
			t.Fatalf("history = %+v, err %v", history, err)
		}
		last := history[0]
		if last.Status != RequestDeclined {
			last = history[1]
		}
		if last.Status != RequestDeclined || last.ChangedByID != demoOlha || !last.CreatedAt.Equal(*r.RespondedAt) {
			t.Fatalf("decline in history = %+v", last)
		}

		if _, err := store.LockRequest(ctx, unknown); !errors.Is(err, ErrNotFound) {
			t.Fatalf("lock an unknown request: err = %v", err)
		}
		if err := store.UpdateRequestStatus(ctx, unknown, RequestDeclined, demoOlha); !errors.Is(err, ErrNotFound) {
			t.Fatalf("update an unknown request: err = %v", err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTx err = %v", err)
	}
}

// Runs inside a transaction that is rolled back.
// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestCreateExchangeAgainstPostgres(t *testing.T) {
	store := openTestStore(t)
	rollback := errors.New("roll back test data")
	err := store.WithinTx(context.Background(), func(ctx context.Context) error {
		if err := store.UpdateRequestStatus(ctx, demoTarasToOlha, RequestAccepted, demoOlha); err != nil {
			t.Fatal(err)
		}
		id, err := store.CreateExchange(ctx, demoTarasToOlha, demoOlha)
		if err != nil {
			t.Fatal(err)
		}
		e, err := store.ExchangeByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if e.ID != id || e.RequestID != demoTarasToOlha || e.Status != ExchangeActive || e.Format != "OFFLINE" ||
			e.TotalSessions != 4 || e.StartedAt == nil || e.Requester.UserID != demoTaras || e.Recipient.UserID != demoOlha ||
			e.Recipient.UniversityName == "" {
			t.Fatalf("exchange = %+v", e)
		}
		taras := RequestTerms{SkillID: photoshop, CategoryID: e.RequesterTeaches.CategoryID, Name: "Photoshop",
			TeacherLevel: "BEGINNER", LearnerLevel: "BEGINNER", Sessions: 2, DurationMinutes: 60}
		if e.RequesterTeaches != taras || e.RequesterTeaches.CategoryID == "" {
			t.Fatalf("Taras teaches %+v", e.RequesterTeaches)
		}
		olha := e.RecipientTeaches
		if olha.SkillID != guitarSkill || olha.Name != "Гітара" || olha.TeacherLevel != "ADVANCED" || olha.Sessions != 2 {
			t.Fatalf("Olha teaches %+v", olha)
		}
		if r, err := store.RequestByID(ctx, demoTarasToOlha); err != nil || r.ExchangeID != id {
			t.Fatalf("request exchange ID = %q, err %v", r.ExchangeID, err)
		}
		var history int
		if err := store.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM exchange_status_history
			WHERE exchange_id = $1 AND status = 'ACTIVE' AND changed_by_user_id = $2`, id, demoOlha).Scan(&history); err != nil || history != 1 {
			t.Fatalf("history rows = %d, err %v", history, err)
		}

		// The terms are snapshots: later profile changes do not reach the exchange.
		if err := store.UpdateUserSkillLevel(ctx, TeachingList, demoOlha, guitarSkill, "BEGINNER"); err != nil {
			t.Fatal(err)
		}
		if err := store.RemoveUserSkill(ctx, TeachingList, demoTaras, photoshop); err != nil {
			t.Fatal(err)
		}
		if _, err := store.conn(ctx).Exec(ctx, `UPDATE skills SET name = 'Photoshop CC' WHERE id = $1`, photoshop); err != nil {
			t.Fatal(err)
		}
		after, err := store.ExchangeByID(ctx, id)
		if err != nil || after.RequesterTeaches != taras || after.RecipientTeaches != olha {
			t.Fatalf("after profile changes = %+v, err %v", after, err)
		}

		if _, err := store.ExchangeByID(ctx, "80000000-0000-0000-0000-000000000999"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("unknown exchange: err = %v", err)
		}
		if _, err := store.CreateExchange(ctx, "70000000-0000-0000-0000-000000000999", demoOlha); !errors.Is(err, ErrNotFound) {
			t.Fatalf("exchange for an unknown request: err = %v", err)
		}
		// The last statement: the unique source request fails and aborts the transaction.
		if _, err := store.CreateExchange(ctx, demoTarasToOlha, demoOlha); err == nil {
			t.Fatal("a second exchange for the same request was created")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTx err = %v", err)
	}
}

// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestCreateExchangeRollsBack(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	failure := errors.New("fail after the exchange was created")
	err := store.WithinTx(ctx, func(ctx context.Context) error {
		if err := store.UpdateRequestStatus(ctx, demoTarasToOlha, RequestAccepted, demoOlha); err != nil {
			return err
		}
		if _, err := store.CreateExchange(ctx, demoTarasToOlha, demoOlha); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("WithinTx err = %v", err)
	}
	r, err := store.RequestByID(ctx, demoTarasToOlha)
	if err != nil || r.Status != RequestPending || r.ExchangeID != "" || r.RespondedAt != nil {
		t.Fatalf("request after the rollback = %+v, err %v", r, err)
	}
	history, err := store.RequestHistory(ctx, demoTarasToOlha)
	if err != nil || len(history) != 1 {
		t.Fatalf("history after the rollback = %+v, err %v", history, err)
	}
}

// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestRequestIsCurrentAgainstPostgres(t *testing.T) {
	store := openTestStore(t)
	for _, test := range []struct {
		name   string
		change string // run before the check, then rolled back
		want   bool
	}{
		{"untouched", ``, true},
		{"recipient blocked the requester", `INSERT INTO user_blocks (blocker_id, blocked_id)
			VALUES ('` + demoOlha + `', '` + demoTaras + `')`, false},
		{"requester blocked the recipient", `INSERT INTO user_blocks (blocker_id, blocked_id)
			VALUES ('` + demoTaras + `', '` + demoOlha + `')`, false},
		{"requester suspended", `UPDATE users SET account_status = 'SUSPENDED' WHERE id = '` + demoTaras + `'`, false},
		{"recipient deleted", `UPDATE users SET deleted_at = now() WHERE id = '` + demoOlha + `'`, false},
		{"requester stopped teaching", `UPDATE user_teaching_skills SET is_active = false
			WHERE user_id = '` + demoTaras + `' AND skill_id = '` + photoshop + `'`, false},
		{"recipient stopped learning", `UPDATE user_learning_skills SET is_active = false
			WHERE user_id = '` + demoOlha + `' AND skill_id = '` + photoshop + `'`, false},
		{"recipient stopped teaching", `UPDATE user_teaching_skills SET is_active = false
			WHERE user_id = '` + demoOlha + `' AND skill_id = '` + guitarSkill + `'`, false},
		{"requester stopped learning", `UPDATE user_learning_skills SET is_active = false
			WHERE user_id = '` + demoTaras + `' AND skill_id = '` + guitarSkill + `'`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			rollback := errors.New("roll back test data")
			err := store.WithinTx(context.Background(), func(ctx context.Context) error {
				if test.change != "" {
					if _, err := store.conn(ctx).Exec(ctx, test.change); err != nil {
						t.Fatal(err)
					}
				}
				if current, err := store.RequestIsCurrent(ctx, demoTarasToOlha); err != nil || current != test.want {
					t.Fatalf("current = %v, err %v", current, err)
				}
				return rollback
			})
			if !errors.Is(err, rollback) {
				t.Fatalf("WithinTx err = %v", err)
			}
		})
	}
	if current, err := store.RequestIsCurrent(context.Background(), "70000000-0000-0000-0000-000000000999"); err != nil || current {
		t.Fatalf("unknown request: current = %v, err %v", current, err)
	}
}
