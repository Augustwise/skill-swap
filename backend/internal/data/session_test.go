package data

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

const (
	demoIrynaMarkoExchange = "80000000-0000-0000-0000-000000000001"
	demoScheduledSessionID = "81000000-0000-0000-0000-000000000001"
	demoProposedSessionID  = "81000000-0000-0000-0000-000000000003"
	demoPendingProposalID  = "82000000-0000-0000-0000-000000000002"
)

// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestSessionsAndScheduleOnDemoData(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	exchanges, total, err := store.UserExchanges(ctx, demoIryna, ExchangeFilter{Status: ExchangeActive}, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total < 1 || !slices.ContainsFunc(exchanges, func(e Exchange) bool { return e.ID == demoIrynaMarkoExchange }) {
		t.Fatalf("Iryna exchanges = %+v, total %d", exchanges, total)
	}

	ex, err := store.ExchangeByID(ctx, demoIrynaMarkoExchange)
	if err != nil {
		t.Fatal(err)
	}
	if ex.Status != ExchangeActive || ex.TotalSessions != 4 ||
		ex.Progress.TotalSessions != 4 || ex.Progress.ScheduledSessions != 1 ||
		ex.Progress.ProposedSessions != 1 || ex.Progress.DraftSessions != 2 ||
		ex.Progress.CompletedSessions != 0 || ex.NextSession == nil || ex.NextSession.ID != demoScheduledSessionID {
		t.Fatalf("demo exchange = %+v", ex)
	}

	sessions, err := store.ExchangeSessions(ctx, demoIrynaMarkoExchange)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 4 || sessions[0].ID != demoScheduledSessionID || sessions[0].Status != SessionScheduled ||
		sessions[2].ID != demoProposedSessionID || sessions[2].Status != SessionProposed ||
		sessions[2].ActiveProposal == nil || sessions[2].ActiveProposal.ID != demoPendingProposalID {
		t.Fatalf("demo sessions = %+v", sessions)
	}

	schedule, schedTotal, pendingCount, tz, err := store.UserSchedule(ctx, demoIryna, ScheduleFilter{UpcomingOnly: true}, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if schedTotal < 1 || pendingCount < 1 || tz != "Europe/Kyiv" ||
		!slices.ContainsFunc(schedule, func(s Session) bool { return s.ID == demoScheduledSessionID }) {
		t.Fatalf("Iryna schedule = %+v, total %d, pending %d, tz %q", schedule, schedTotal, pendingCount, tz)
	}

	// Olha's schedule does not include Iryna and Marko's sessions.
	olhaSchedule, _, _, _, err := store.UserSchedule(ctx, demoOlha, ScheduleFilter{Status: ScheduleAll}, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(olhaSchedule, func(s Session) bool { return s.ExchangeID == demoIrynaMarkoExchange }) {
		t.Fatalf("Olha schedule leaked another exchange's session: %+v", olhaSchedule)
	}
}

// Runs inside a transaction that is rolled back.
// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestSessionLifecycleAndConflictsAgainstPostgres(t *testing.T) {
	store := openTestStore(t)
	rollback := errors.New("roll back test data")
	err := store.WithinTx(context.Background(), func(ctx context.Context) error {
		if err := store.UpdateRequestStatus(ctx, demoTarasToOlha, RequestAccepted, demoOlha); err != nil {
			t.Fatal(err)
		}
		exchangeID, err := store.CreateExchange(ctx, demoTarasToOlha, demoOlha)
		if err != nil {
			t.Fatal(err)
		}

		// CreateExchange creates the 4 initial DRAFT sessions.
		initial, err := store.ExchangeSessions(ctx, exchangeID)
		if err != nil || len(initial) != 4 {
			t.Fatalf("initial sessions = %+v, err %v", initial, err)
		}
		if initial[0].Status != SessionDraft || initial[0].Teacher.UserID != demoTaras ||
			initial[2].Status != SessionDraft || initial[2].Teacher.UserID != demoOlha {
			t.Fatalf("initial draft teachers = %+v", initial)
		}

		// Swap draft session teacher between slot 1 (Taras) and slot 3 (Olha).
		if err := store.SwapDraftSessionTeacher(ctx, exchangeID, initial[0].ID, demoOlha); err != nil {
			t.Fatal(err)
		}
		swapped, err := store.SessionByID(ctx, initial[0].ID)
		if err != nil || swapped.Teacher.UserID != demoOlha || swapped.Learner.UserID != demoTaras {
			t.Fatalf("swapped session = %+v, err %v", swapped, err)
		}

		// Propose an offline session on slot 1.
		start1 := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Minute)
		end1 := start1.Add(60 * time.Minute)
		prop1, err := store.SaveSessionProposal(ctx, NewSessionProposal{
			SessionID:    initial[0].ID,
			ProposerID:   demoOlha,
			TeacherID:    demoOlha,
			LearnerID:    demoTaras,
			Title:        "Гітара біля корпусу",
			Description:  "Перша зустріч наживо",
			StartsAt:     start1,
			EndsAt:       end1,
			Format:       FormatOffline,
			MeetingPlace: "Науково-технічна бібліотека КПІ",
		})
		if err != nil {
			t.Fatal(err)
		}
		if prop1.Status != ProposalPending || prop1.MeetingPlace != "Науково-технічна бібліотека КПІ" {
			t.Fatalf("prop1 = %+v", prop1)
		}

		// Taras declines the first proposal -> session returns to DRAFT.
		declined, err := store.DeclineSessionProposal(ctx, initial[0].ID, demoTaras)
		if err != nil || declined.Status != ProposalDeclined {
			t.Fatalf("declined = %+v, err %v", declined, err)
		}
		afterDecline, err := store.SessionByID(ctx, initial[0].ID)
		if err != nil || afterDecline.Status != SessionDraft || afterDecline.StartsAt != nil || len(afterDecline.ConfirmedUserIDs) != 0 {
			t.Fatalf("after decline = %+v, err %v", afterDecline, err)
		}

		// Taras proposes a new time and Olha accepts it.
		start2 := start1.Add(24 * time.Hour)
		end2 := start2.Add(60 * time.Minute)
		if _, err := store.SaveSessionProposal(ctx, NewSessionProposal{
			SessionID:    initial[0].ID,
			ProposerID:   demoTaras,
			TeacherID:    demoOlha,
			LearnerID:    demoTaras,
			Title:        "Гітара біля корпусу",
			StartsAt:     start2,
			EndsAt:       end2,
			Format:       FormatOffline,
			MeetingPlace: "Парк КПІ",
		}); err != nil {
			t.Fatal(err)
		}
		accepted, err := store.AcceptSessionProposal(ctx, initial[0].ID, demoOlha)
		if err != nil || accepted.Status != ProposalAccepted {
			t.Fatalf("accepted = %+v, err %v", accepted, err)
		}
		scheduled, err := store.SessionByID(ctx, initial[0].ID)
		if err != nil || scheduled.Status != SessionScheduled || len(scheduled.ConfirmedUserIDs) != 2 ||
			scheduled.MeetingPlace != "Парк КПІ" {
			t.Fatalf("scheduled = %+v, err %v", scheduled, err)
		}

		// Conflict checks: overlapping interval conflicts; adjacent interval [end2, end2+1h) does not.
		conflict, err := store.HasScheduleConflict(ctx, "", demoOlha, demoTaras, start2.Add(30*time.Minute), end2.Add(30*time.Minute))
		if err != nil || !conflict {
			t.Fatalf("overlapping conflict = %v, err %v", conflict, err)
		}
		adjacent, err := store.HasScheduleConflict(ctx, "", demoOlha, demoTaras, end2, end2.Add(time.Hour))
		if err != nil || adjacent {
			t.Fatalf("adjacent conflict = %v, err %v", adjacent, err)
		}
		selfExcluded, err := store.HasScheduleConflict(ctx, initial[0].ID, demoOlha, demoTaras, start2, end2)
		if err != nil || selfExcluded {
			t.Fatalf("self-excluded conflict = %v, err %v", selfExcluded, err)
		}

		// Cancel the scheduled session -> status becomes CANCELLED and a 5th replacement DRAFT session is added.
		if err := store.CancelSession(ctx, initial[0].ID, demoOlha, "Перенесемо на наступний тиждень"); err != nil {
			t.Fatal(err)
		}
		afterCancel, err := store.ExchangeSessions(ctx, exchangeID)
		if err != nil || len(afterCancel) != 5 || afterCancel[0].Status != SessionCancelled ||
			afterCancel[4].Status != SessionDraft || afterCancel[4].PlanOrder != 5 {
			t.Fatalf("after cancel = %+v, err %v", afterCancel, err)
		}

		proposals, err := store.SessionProposals(ctx, initial[0].ID)
		if err != nil || len(proposals) != 2 || proposals[0].Status != ProposalDeclined || proposals[1].Status != ProposalAccepted {
			t.Fatalf("proposals = %+v, err %v", proposals, err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTx err = %v", err)
	}
}
