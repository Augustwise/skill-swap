package data

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"skillswap/backend/internal/expect"
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
	expect.Equal(t, "Andrii accepts requests", andrii.AcceptsRequests, true)
	expect.Equal(t, "Andrii accepts only his university", andrii.SameUniversityOnly, false)
	expect.Equal(t, "Andrii's user ID", andrii.UserID, demoAndrii)
	expect.Equal(t, "Andrii's city", andrii.City, "Київ")
	expect.Equal(t, "skills Andrii teaches", len(andrii.TeachingSkills), 2)
	expect.Equal(t, "skills Andrii learns", len(andrii.LearningSkills), 1)
	dmytro, err := store.RequestRecipient(ctx, demoIryna, demoDmytro)
	expect.NoError(t, "load Dmytro as a recipient", err)
	expect.Equal(t, "Dmytro accepts requests", dmytro.AcceptsRequests, false)

	for _, test := range []struct{ name, sender, recipient string }{
		{"hidden profile", demoOlha, demoNataliia},
		{"unverified email", demoOlha, demoViktor},
		{"blocked by the sender", demoOlha, demoOleh},
		{"sender blocked by the recipient", demoOleh, demoOlha},
		{"the sender", demoOlha, demoOlha},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := store.RequestRecipient(ctx, test.sender, test.recipient)
			expect.Error(t, "load the recipient", err, ErrNotFound)
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
	expect.Equal(t, "requester", r.Requester.UserID, demoTaras)
	expect.Equal(t, "recipient", r.Recipient.UserID, demoOlha)
	expect.Equal(t, "requester's first name", r.Requester.FirstName, "Тарас")
	expect.Equal(t, "recipient's university is set", r.Recipient.UniversityName != "", true)
	expect.Equal(t, "format", r.Format, "OFFLINE")
	expect.Equal(t, "total sessions", r.TotalSessions, 4)
	expect.Equal(t, "exchange ID", r.ExchangeID, "")

	expect.Equal(t, "skill Taras teaches", r.RequesterTeaches.SkillID, photoshop)
	expect.Equal(t, "Taras's teaching level", r.RequesterTeaches.TeacherLevel, "BEGINNER")
	expect.Equal(t, "Olha's Photoshop level", r.RequesterTeaches.LearnerLevel, "BEGINNER")
	expect.Equal(t, "sessions Taras teaches", r.RequesterTeaches.Sessions, 2)
	expect.Equal(t, "length of Taras's sessions", r.RequesterTeaches.DurationMinutes, 60)

	expect.Equal(t, "skill Olha teaches", r.RecipientTeaches.SkillID, guitarSkill)
	expect.Equal(t, "Olha's teaching level", r.RecipientTeaches.TeacherLevel, "ADVANCED")
	expect.Equal(t, "name of the skill Olha teaches", r.RecipientTeaches.Name, "Гітара")

	_, err = store.RequestByID(context.Background(), "70000000-0000-0000-0000-000000000999")
	expect.Error(t, "unknown request", err, ErrNotFound)
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
		expect.Equal(t, "status", r.Status, RequestPending)
		expect.Equal(t, "requester", r.Requester.UserID, demoOlha)
		expect.Equal(t, "recipient", r.Recipient.UserID, demoAndrii)
		expect.Equal(t, "total sessions", r.TotalSessions, 3)
		expect.Equal(t, "message", r.Message, "Привіт!")
		expect.Equal(t, "respondedAt is set", r.RespondedAt != nil, false)

		expect.Equal(t, "skill Olha teaches", r.RequesterTeaches.SkillID, guitarSkill)
		expect.Equal(t, "Olha's teaching level", r.RequesterTeaches.TeacherLevel, "ADVANCED")
		expect.Equal(t, "Andrii's guitar level", r.RequesterTeaches.LearnerLevel, "BEGINNER")
		expect.Equal(t, "sessions Olha teaches", r.RequesterTeaches.Sessions, 2)
		expect.Equal(t, "skill Andrii teaches", r.RecipientTeaches.SkillID, photoshop)
		expect.Equal(t, "length of Andrii's sessions", r.RecipientTeaches.DurationMinutes, 120)

		var history int
		err = store.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM exchange_request_status_history
			WHERE request_id = $1 AND status = 'PENDING' AND changed_by_user_id = $2`, id, demoOlha).Scan(&history)
		expect.NoError(t, "count history rows", err)
		expect.Equal(t, "PENDING history rows by Olha", history, 1)

		_, err = create(olhaToAndrii)
		expect.Error(t, "the same request again", err, ErrDuplicateRequest)
		_, err = create(andriiToOlha)
		expect.Error(t, "the reverse request", err, ErrDuplicateRequest)
		// The seed has a pending request from Taras with the same skills.
		olhaToTaras := olhaToAndrii
		olhaToTaras.RecipientID, olhaToTaras.Format = demoTaras, FormatOffline
		_, err = create(olhaToTaras)
		expect.Error(t, "the reverse of the demo request", err, ErrDuplicateRequest)

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
		expect.NoError(t, "load the request without a message", err)
		expect.Equal(t, "message", empty.Message, "")

		if err := store.RemoveUserSkill(ctx, TeachingList, demoAndrii, photoshop); err != nil {
			t.Fatal(err)
		}
		_, err = create(olhaToAndrii)
		expect.Error(t, "after a skill was removed", err, ErrNotFound)
		return rollback
	})
	expect.Error(t, "WithinTx", err, rollback)
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
		expect.Equal(t, "recipient accepts requests", recipient.AcceptsRequests, true)
		expect.Equal(t, "recipient accepts only their university", recipient.SameUniversityOnly, true)
		expect.Equal(t, "recipient's university", recipient.UniversityID, universityID)
		return rollback
	})
	expect.Error(t, "WithinTx", err, rollback)
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
		expect.Slice(t, "Olha's newest sent request", sent, []string{id})
		expect.Equal(t, "at least 1 sent request in total", total >= 1, true)
		past, pastTotal := list(demoOlha, RequestFilter{Direction: Outgoing}, 20, 400)
		expect.Equal(t, "sent requests past the end", len(past), 0)
		expect.Equal(t, "total past the end", pastTotal, total)
		olhaIncoming, _ := list(demoOlha, RequestFilter{Direction: Incoming}, 20, 0)
		expect.Equal(t, "Olha's own request is in her incoming requests", slices.Contains(olhaIncoming, id), false)
		expect.Equal(t, "Taras's request is in Olha's incoming requests", slices.Contains(olhaIncoming, demoTarasToOlha), true)
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
		r, err := store.RequestByID(ctx, id)
		expect.NoError(t, "load the request after the block", err)
		expect.Equal(t, "recipient's first name is set after the block", r.Recipient.FirstName != "", true)
		return rollback
	})
	expect.Error(t, "WithinTx", err, rollback)
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
		expect.Equal(t, "history items", len(history), 2)
		first := history[0]
		expect.Equal(t, "first change status", first.Status, RequestPending)
		expect.Equal(t, "first change author", first.ChangedByID, demoOlha)
		expect.Equal(t, "first change author's first name", first.ChangedByFirstName, "Ольга")
		expect.Equal(t, "first change author's last name is set", first.ChangedByLastName != "", true)
		// The second change was made without a user.
		second := history[1]
		expect.Equal(t, "second change status", second.Status, RequestWithdrawn)
		expect.Equal(t, "second change author", second.ChangedByID, "")
		expect.Equal(t, "second change author's first name", second.ChangedByFirstName, "")
		empty, err := store.RequestHistory(ctx, "70000000-0000-0000-0000-000000000999")
		expect.NoError(t, "load the history of an unknown request", err)
		expect.Equal(t, "history items of an unknown request", len(empty), 0)
		return rollback
	})
	expect.Error(t, "WithinTx", err, rollback)
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
		expect.Equal(t, "locked request", state, want)
		if err := store.UpdateRequestStatus(ctx, demoTarasToOlha, RequestDeclined, demoOlha); err != nil {
			t.Fatal(err)
		}
		r, err := store.RequestByID(ctx, demoTarasToOlha)
		expect.NoError(t, "load the declined request", err)
		expect.Equal(t, "status", r.Status, RequestDeclined)
		expect.Equal(t, "respondedAt is set", r.RespondedAt != nil, true)
		history, err := store.RequestHistory(ctx, demoTarasToOlha)
		expect.NoError(t, "load the history", err)
		expect.Equal(t, "history items", len(history), 2)
		last := history[0]
		if last.Status != RequestDeclined {
			last = history[1]
		}
		expect.Equal(t, "decline status in history", last.Status, RequestDeclined)
		expect.Equal(t, "declined by", last.ChangedByID, demoOlha)
		expect.Equal(t, "decline time matches respondedAt", last.CreatedAt.Equal(*r.RespondedAt), true)

		_, err = store.LockRequest(ctx, unknown)
		expect.Error(t, "lock an unknown request", err, ErrNotFound)
		err = store.UpdateRequestStatus(ctx, unknown, RequestDeclined, demoOlha)
		expect.Error(t, "update an unknown request", err, ErrNotFound)
		return rollback
	})
	expect.Error(t, "WithinTx", err, rollback)
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
		expect.Equal(t, "exchange ID", e.ID, id)
		expect.Equal(t, "source request", e.RequestID, demoTarasToOlha)
		expect.Equal(t, "status", e.Status, ExchangeActive)
		expect.Equal(t, "format", e.Format, "OFFLINE")
		expect.Equal(t, "total sessions", e.TotalSessions, 4)
		expect.Equal(t, "startedAt is set", e.StartedAt != nil, true)
		expect.Equal(t, "requester", e.Requester.UserID, demoTaras)
		expect.Equal(t, "recipient", e.Recipient.UserID, demoOlha)
		expect.Equal(t, "recipient's university is set", e.Recipient.UniversityName != "", true)

		taras := RequestTerms{SkillID: photoshop, CategoryID: e.RequesterTeaches.CategoryID, Name: "Photoshop",
			TeacherLevel: "BEGINNER", LearnerLevel: "BEGINNER", Sessions: 2, DurationMinutes: 60}
		expect.Equal(t, "terms Taras teaches", e.RequesterTeaches, taras)
		expect.Equal(t, "category of the skill Taras teaches is set", e.RequesterTeaches.CategoryID != "", true)
		olha := e.RecipientTeaches
		expect.Equal(t, "skill Olha teaches", olha.SkillID, guitarSkill)
		expect.Equal(t, "name of the skill Olha teaches", olha.Name, "Гітара")
		expect.Equal(t, "Olha's teaching level", olha.TeacherLevel, "ADVANCED")
		expect.Equal(t, "sessions Olha teaches", olha.Sessions, 2)

		r, err := store.RequestByID(ctx, demoTarasToOlha)
		expect.NoError(t, "load the request", err)
		expect.Equal(t, "request exchange ID", r.ExchangeID, id)
		var history int
		err = store.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM exchange_status_history
			WHERE exchange_id = $1 AND status = 'ACTIVE' AND changed_by_user_id = $2`, id, demoOlha).Scan(&history)
		expect.NoError(t, "count history rows", err)
		expect.Equal(t, "ACTIVE history rows by Olha", history, 1)

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
		expect.NoError(t, "load the exchange after profile changes", err)
		expect.Equal(t, "terms Taras teaches after profile changes", after.RequesterTeaches, taras)
		expect.Equal(t, "terms Olha teaches after profile changes", after.RecipientTeaches, olha)

		_, err = store.ExchangeByID(ctx, "80000000-0000-0000-0000-000000000999")
		expect.Error(t, "unknown exchange", err, ErrNotFound)
		_, err = store.CreateExchange(ctx, "70000000-0000-0000-0000-000000000999", demoOlha)
		expect.Error(t, "exchange for an unknown request", err, ErrNotFound)
		// The last statement: the unique source request fails and aborts the transaction.
		if _, err := store.CreateExchange(ctx, demoTarasToOlha, demoOlha); err == nil {
			t.Fatal("a second exchange for the same request was created")
		}
		return rollback
	})
	expect.Error(t, "WithinTx", err, rollback)
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
	expect.Error(t, "WithinTx", err, failure)
	r, err := store.RequestByID(ctx, demoTarasToOlha)
	expect.NoError(t, "load the request after the rollback", err)
	expect.Equal(t, "status after the rollback", r.Status, RequestPending)
	expect.Equal(t, "exchange ID after the rollback", r.ExchangeID, "")
	expect.Equal(t, "respondedAt is set after the rollback", r.RespondedAt != nil, false)
	history, err := store.RequestHistory(ctx, demoTarasToOlha)
	expect.NoError(t, "load the history after the rollback", err)
	expect.Equal(t, "history items after the rollback", len(history), 1)
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
				current, err := store.RequestIsCurrent(ctx, demoTarasToOlha)
				expect.NoError(t, "RequestIsCurrent", err)
				expect.Equal(t, "current", current, test.want)
				return rollback
			})
			expect.Error(t, "WithinTx", err, rollback)
		})
	}
	current, err := store.RequestIsCurrent(context.Background(), "70000000-0000-0000-0000-000000000999")
	expect.NoError(t, "RequestIsCurrent for an unknown request", err)
	expect.Equal(t, "unknown request is current", current, false)
}
