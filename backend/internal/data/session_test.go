package data

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"skillswap/backend/internal/expect"
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
	expect.Equal(t, "at least 1 active exchange of Iryna in total", total >= 1, true)
	expect.Equal(t, "Iryna and Marko's exchange is in Iryna's list",
		slices.ContainsFunc(exchanges, func(e Exchange) bool { return e.ID == demoIrynaMarkoExchange }), true)

	ex, err := store.ExchangeByID(ctx, demoIrynaMarkoExchange)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "exchange status", ex.Status, ExchangeActive)
	expect.Equal(t, "total sessions", ex.TotalSessions, 4)
	expect.Equal(t, "total sessions in progress", ex.Progress.TotalSessions, 4)
	expect.Equal(t, "scheduled sessions", ex.Progress.ScheduledSessions, 1)
	expect.Equal(t, "proposed sessions", ex.Progress.ProposedSessions, 1)
	expect.Equal(t, "draft sessions", ex.Progress.DraftSessions, 2)
	expect.Equal(t, "completed sessions", ex.Progress.CompletedSessions, 0)
	expect.Equal(t, "next session is set", ex.NextSession != nil, true)
	expect.Equal(t, "next session", ex.NextSession.ID, demoScheduledSessionID)

	sessions, err := store.ExchangeSessions(ctx, demoIrynaMarkoExchange)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "sessions", len(sessions), 4)
	expect.Equal(t, "first session", sessions[0].ID, demoScheduledSessionID)
	expect.Equal(t, "first session status", sessions[0].Status, SessionScheduled)
	expect.Equal(t, "third session", sessions[2].ID, demoProposedSessionID)
	expect.Equal(t, "third session status", sessions[2].Status, SessionProposed)
	expect.Equal(t, "third session has an active proposal", sessions[2].ActiveProposal != nil, true)
	expect.Equal(t, "active proposal of the third session", sessions[2].ActiveProposal.ID, demoPendingProposalID)

	schedule, schedTotal, pendingCount, tz, err := store.UserSchedule(ctx, demoIryna, ScheduleFilter{UpcomingOnly: true}, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "at least 1 session in Iryna's schedule", schedTotal >= 1, true)
	expect.Equal(t, "at least 1 proposal waiting for Iryna", pendingCount >= 1, true)
	expect.Equal(t, "Iryna's time zone", tz, "Europe/Kyiv")
	expect.Equal(t, "scheduled demo session is in Iryna's schedule",
		slices.ContainsFunc(schedule, func(s Session) bool { return s.ID == demoScheduledSessionID }), true)

	// Olha's schedule does not include Iryna and Marko's sessions.
	olhaSchedule, _, _, _, err := store.UserSchedule(ctx, demoOlha, ScheduleFilter{Status: ScheduleAll}, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "Olha's schedule has a session of Iryna and Marko's exchange",
		slices.ContainsFunc(olhaSchedule, func(s Session) bool { return s.ExchangeID == demoIrynaMarkoExchange }), false)
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
		expect.NoError(t, "load the initial sessions", err)
		expect.Equal(t, "initial sessions", len(initial), 4)
		expect.Equal(t, "first session status", initial[0].Status, SessionDraft)
		expect.Equal(t, "first session teacher", initial[0].Teacher.UserID, demoTaras)
		expect.Equal(t, "third session status", initial[2].Status, SessionDraft)
		expect.Equal(t, "third session teacher", initial[2].Teacher.UserID, demoOlha)

		// Swap draft session teacher between slot 1 (Taras) and slot 3 (Olha).
		if err := store.SwapDraftSessionTeacher(ctx, exchangeID, initial[0].ID, demoOlha); err != nil {
			t.Fatal(err)
		}
		swapped, err := store.SessionByID(ctx, initial[0].ID)
		expect.NoError(t, "load the swapped session", err)
		expect.Equal(t, "teacher after the swap", swapped.Teacher.UserID, demoOlha)
		expect.Equal(t, "learner after the swap", swapped.Learner.UserID, demoTaras)

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
		expect.Equal(t, "status of Olha's proposal", prop1.Status, ProposalPending)
		expect.Equal(t, "meeting place of Olha's proposal", prop1.MeetingPlace, "Науково-технічна бібліотека КПІ")

		// Taras declines the first proposal -> session returns to DRAFT.
		declined, err := store.DeclineSessionProposal(ctx, initial[0].ID, demoTaras)
		expect.NoError(t, "DeclineSessionProposal", err)
		expect.Equal(t, "status of the declined proposal", declined.Status, ProposalDeclined)
		afterDecline, err := store.SessionByID(ctx, initial[0].ID)
		expect.NoError(t, "load the session after decline", err)
		expect.Equal(t, "status after decline", afterDecline.Status, SessionDraft)
		expect.Equal(t, "start time is set after decline", afterDecline.StartsAt != nil, false)
		expect.Equal(t, "confirmations after decline", len(afterDecline.ConfirmedUserIDs), 0)

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
		expect.NoError(t, "AcceptSessionProposal", err)
		expect.Equal(t, "status of the accepted proposal", accepted.Status, ProposalAccepted)
		scheduled, err := store.SessionByID(ctx, initial[0].ID)
		expect.NoError(t, "load the scheduled session", err)
		expect.Equal(t, "status after acceptance", scheduled.Status, SessionScheduled)
		expect.Equal(t, "confirmations after acceptance", len(scheduled.ConfirmedUserIDs), 2)
		expect.Equal(t, "meeting place after acceptance", scheduled.MeetingPlace, "Парк КПІ")

		// Conflict checks: overlapping interval conflicts; adjacent interval [end2, end2+1h) does not.
		conflict, err := store.HasScheduleConflict(ctx, "", demoOlha, demoTaras, start2.Add(30*time.Minute), end2.Add(30*time.Minute))
		expect.NoError(t, "check an overlapping interval", err)
		expect.Equal(t, "overlapping interval conflicts", conflict, true)
		adjacent, err := store.HasScheduleConflict(ctx, "", demoOlha, demoTaras, end2, end2.Add(time.Hour))
		expect.NoError(t, "check an adjacent interval", err)
		expect.Equal(t, "adjacent interval conflicts", adjacent, false)
		selfExcluded, err := store.HasScheduleConflict(ctx, initial[0].ID, demoOlha, demoTaras, start2, end2)
		expect.NoError(t, "check the session's own interval", err)
		expect.Equal(t, "session conflicts with itself", selfExcluded, false)

		// Cancel the scheduled session -> status becomes CANCELLED and a 5th replacement DRAFT session is added.
		if err := store.CancelSession(ctx, initial[0].ID, demoOlha, "Перенесемо на наступний тиждень"); err != nil {
			t.Fatal(err)
		}
		afterCancel, err := store.ExchangeSessions(ctx, exchangeID)
		expect.NoError(t, "load the sessions after cancel", err)
		expect.Equal(t, "sessions after cancel", len(afterCancel), 5)
		expect.Equal(t, "cancelled session status", afterCancel[0].Status, SessionCancelled)
		expect.Equal(t, "replacement session status", afterCancel[4].Status, SessionDraft)
		expect.Equal(t, "replacement session plan order", afterCancel[4].PlanOrder, 5)

		proposals, err := store.SessionProposals(ctx, initial[0].ID)
		expect.NoError(t, "load the proposals", err)
		expect.Equal(t, "proposals", len(proposals), 2)
		expect.Equal(t, "first proposal status", proposals[0].Status, ProposalDeclined)
		expect.Equal(t, "second proposal status", proposals[1].Status, ProposalAccepted)
		return rollback
	})
	expect.Error(t, "WithinTx", err, rollback)
}
