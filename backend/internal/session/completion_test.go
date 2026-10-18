package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/exchange"
)

// expectEqual stops the test with a readable message when a value is not the expected one.
func expectEqual[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

// expectError stops the test when an action did not fail with the expected error.
func expectError(t *testing.T, action string, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("%s: err = %v, want %v", action, err, want)
	}
}

func (f *fixture) scheduleSession(t *testing.T, exchangeID, proposerID, responderID, teacherID string, startsAt time.Time) data.Session {
	t.Helper()
	ctx := context.Background()
	proposed, err := f.service.ProposeExchangeSession(ctx, proposerID, exchangeID, ProposalInput{
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
	accepted, err := f.service.AcceptSessionProposal(ctx, responderID, proposed.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	return accepted.Session
}

func (f *fixture) sessionsByID(t *testing.T, exchangeID string) map[string]data.Session {
	t.Helper()
	result, err := f.service.ExchangeSessions(context.Background(), f.olha, exchangeID)
	if err != nil {
		t.Fatal(err)
	}
	sessions := map[string]data.Session{}
	for _, s := range result.Items {
		sessions[s.ID] = s
	}
	return sessions
}

// notificationCount counts the user's notifications whose event key starts with keyPrefix.
func (f *fixture) notificationCount(t *testing.T, userID, keyPrefix string) int {
	t.Helper()
	page, err := f.comm.Notifications(context.Background(), userID, false, 1)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, n := range page.Items {
		if strings.HasPrefix(n.EventKey, keyPrefix) {
			count++
		}
	}
	return count
}

func TestConfirmSessionCompletionErrors(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.createExchange(t, f.olha, f.andrii, data.FormatOnline, 1, 1)
	lesson := f.scheduleSession(t, ex.ID, f.olha, f.andrii, f.olha, f.baseTime.Add(24*time.Hour))
	confirm := func(userID, sessionID string) error {
		_, err := f.service.ConfirmSessionCompletion(ctx, userID, sessionID)
		return err
	}

	expectError(t, "confirm before the lesson ends", confirm(f.olha, lesson.ID), ErrSessionNotFinished)

	f.service.now = func() time.Time { return f.baseTime.Add(26 * time.Hour) }
	var draftID string
	for id, s := range f.sessionsByID(t, ex.ID) {
		if s.Status == data.SessionDraft {
			draftID = id
		}
	}
	expectError(t, "confirm someone else's lesson", confirm(f.marko, lesson.ID), ErrSessionNotFound)
	expectError(t, "confirm an unknown lesson", confirm(f.olha, "81000000-0000-0000-0000-000000000999"), ErrSessionNotFound)
	expectError(t, "confirm with an invalid ID", confirm(f.olha, "not-a-uuid"), ErrSessionNotFound)
	expectError(t, "confirm a lesson without a time", confirm(f.andrii, draftID), ErrSessionNotConfirmable)

	f.store.SetBlocked(f.andrii, f.olha)
	expectError(t, "confirm after a block", confirm(f.olha, lesson.ID), ErrExchangeBlocked)

	f.store.SetExchangeStatus(ex.ID, data.ExchangeCompleted)
	expectError(t, "confirm in a finished exchange", confirm(f.olha, lesson.ID), ErrExchangeNotActive)
}

func TestConfirmSessionCompletionByBothParticipants(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.createExchange(t, f.olha, f.andrii, data.FormatOnline, 1, 1)
	lesson := f.scheduleSession(t, ex.ID, f.olha, f.andrii, f.olha, f.baseTime.Add(24*time.Hour))
	f.service.now = func() time.Time { return f.baseTime.Add(26 * time.Hour) }

	// Olha confirms first: one confirmation does not count the lesson yet.
	first, err := f.service.ConfirmSessionCompletion(ctx, f.olha, lesson.ID)
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "status after Olha's confirmation", first.Session.Status, data.SessionScheduled)
	expectEqual(t, "confirmations after Olha's confirmation", len(first.Session.CompletionConfirmedUserIDs), 1)
	expectEqual(t, "completed sessions after Olha's confirmation", first.Exchange.Progress.CompletedSessions, 0)

	// A repeated confirmation changes nothing and does not notify Andrii twice.
	again, err := f.service.ConfirmSessionCompletion(ctx, f.olha, lesson.ID)
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "confirmations after a repeat", len(again.Session.CompletionConfirmedUserIDs), 1)
	expectEqual(t, "Andrii's requests to confirm", f.notificationCount(t, f.andrii, "session-completion:"), 1)

	// Andrii confirms too: the lesson is completed and counts toward the progress.
	second, err := f.service.ConfirmSessionCompletion(ctx, f.andrii, lesson.ID)
	if err != nil {
		t.Fatal(err)
	}
	session, progress := second.Session, second.Exchange.Progress
	expectEqual(t, "status after both confirmations", session.Status, data.SessionCompleted)
	expectEqual(t, "completedAt is set", session.CompletedAt != nil, true)
	expectEqual(t, "confirmations after both confirmations", len(session.CompletionConfirmedUserIDs), 2)
	expectEqual(t, "completed sessions", progress.CompletedSessions, 1)
	expectEqual(t, "sessions Olha has taught", progress.RequesterCompletedSessions, 1)
	expectEqual(t, "sessions Andrii has taught", progress.RecipientCompletedSessions, 0)
	expectEqual(t, "percent", progress.Percent, 50)
	expectEqual(t, "plan completed", progress.PlanCompleted(), false)
	expectEqual(t, "Olha's notices that the lesson counts", f.notificationCount(t, f.olha, "session-completed:"+lesson.ID), 1)

	// Confirming a completed lesson again is harmless, but it can no longer be cancelled.
	repeated, err := f.service.ConfirmSessionCompletion(ctx, f.andrii, lesson.ID)
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "status after a repeat", repeated.Session.Status, data.SessionCompleted)
	_, err = f.service.CancelSession(ctx, f.olha, lesson.ID, "")
	expectError(t, "cancel a completed lesson", err, ErrSessionNotCancellable)
}

func TestRescheduleClearsCompletionConfirmations(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.createExchange(t, f.olha, f.andrii, data.FormatOnline, 1, 1)
	lesson := f.scheduleSession(t, ex.ID, f.andrii, f.olha, f.andrii, f.baseTime.Add(24*time.Hour))

	f.service.now = func() time.Time { return f.baseTime.Add(26 * time.Hour) }
	if _, err := f.service.ConfirmSessionCompletion(ctx, f.olha, lesson.ID); err != nil {
		t.Fatal(err)
	}

	// Andrii says the lesson did not happen and proposes a new time.
	start := f.baseTime.Add(48 * time.Hour)
	rescheduled, err := f.service.ProposeSession(ctx, f.andrii, lesson.ID, ProposalInput{
		StartsAt:   start.Format(time.RFC3339),
		EndsAt:     start.Add(time.Hour).Format(time.RFC3339),
		MeetingURL: "https://meet.example.test/lesson",
	})
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "confirmations after rescheduling", len(rescheduled.Session.CompletionConfirmedUserIDs), 0)

	// After the new lesson Olha confirms again, and Andrii is asked again.
	if _, err := f.service.AcceptSessionProposal(ctx, f.olha, lesson.ID); err != nil {
		t.Fatal(err)
	}
	f.service.now = func() time.Time { return f.baseTime.Add(50 * time.Hour) }
	if _, err := f.service.ConfirmSessionCompletion(ctx, f.olha, lesson.ID); err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "Andrii's requests to confirm", f.notificationCount(t, f.andrii, "session-completion:"), 2)
}

func TestCompleteExchangeAfterConfirmedPlan(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.createExchange(t, f.olha, f.andrii, data.FormatOnline, 1, 1)
	olhaLesson := f.scheduleSession(t, ex.ID, f.olha, f.andrii, f.olha, f.baseTime.Add(24*time.Hour))
	andriiLesson := f.scheduleSession(t, ex.ID, f.andrii, f.olha, f.andrii, f.baseTime.Add(48*time.Hour))
	f.service.now = func() time.Time { return f.baseTime.Add(72 * time.Hour) }

	confirmByBoth := func(sessionID string) data.ExchangeProgress {
		t.Helper()
		if _, err := f.service.ConfirmSessionCompletion(ctx, f.olha, sessionID); err != nil {
			t.Fatal(err)
		}
		result, err := f.service.ConfirmSessionCompletion(ctx, f.andrii, sessionID)
		if err != nil {
			t.Fatal(err)
		}
		return result.Exchange.Progress
	}

	progress := confirmByBoth(olhaLesson.ID)
	expectEqual(t, "plan completed after Olha's lesson", progress.PlanCompleted(), false)
	_, err := f.exchanges.CompleteExchange(ctx, f.olha, ex.ID)
	expectError(t, "complete while Andrii's lesson is left", err, exchange.ErrPlanNotCompleted)

	progress = confirmByBoth(andriiLesson.ID)
	expectEqual(t, "plan completed after both lessons", progress.PlanCompleted(), true)
	expectEqual(t, "percent", progress.Percent, 100)

	// Andrii confirms the end first: the exchange waits for Olha.
	half, err := f.exchanges.CompleteExchange(ctx, f.andrii, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "status after Andrii's confirmation", half.Status, data.ExchangeActive)
	expectEqual(t, "confirmations after Andrii's confirmation", len(half.ConfirmedUserIDs), 1)
	expectEqual(t, "Olha's requests to confirm", f.notificationCount(t, f.olha, "exchange-completion:"+ex.ID+":"+f.andrii), 1)

	// Olha confirms too: the exchange is completed.
	done, err := f.exchanges.CompleteExchange(ctx, f.olha, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "status after both confirmations", done.Status, data.ExchangeCompleted)
	expectEqual(t, "completedAt is set", done.CompletedAt != nil, true)
	expectEqual(t, "confirmations after both confirmations", len(done.ConfirmedUserIDs), 2)
	expectEqual(t, "Andrii's notices that the exchange is completed", f.notificationCount(t, f.andrii, "exchange-completed:"+ex.ID), 1)
}

func TestCancelExchangeCancelsOpenSessions(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.createExchange(t, f.olha, f.andrii, data.FormatOnline, 2, 2)
	completed := f.scheduleSession(t, ex.ID, f.olha, f.andrii, f.olha, f.baseTime.Add(24*time.Hour))
	scheduled := f.scheduleSession(t, ex.ID, f.andrii, f.olha, f.andrii, f.baseTime.Add(96*time.Hour))
	start := f.baseTime.Add(120 * time.Hour)
	proposed, err := f.service.ProposeExchangeSession(ctx, f.olha, ex.ID, ProposalInput{
		Title:      "Гітара — заняття 2",
		TeacherID:  f.olha,
		StartsAt:   start.Format(time.RFC3339),
		EndsAt:     start.Add(time.Hour).Format(time.RFC3339),
		MeetingURL: "https://meet.example.test/lesson",
	})
	if err != nil {
		t.Fatal(err)
	}
	f.service.now = func() time.Time { return f.baseTime.Add(48 * time.Hour) }
	for _, user := range []string{f.olha, f.andrii} {
		if _, err := f.service.ConfirmSessionCompletion(ctx, user, completed.ID); err != nil {
			t.Fatal(err)
		}
	}

	const reason = "Не встигаю через сесію"
	cancelled, err := f.exchanges.CancelExchange(ctx, f.andrii, ex.ID, reason)
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "exchange status", cancelled.Status, data.ExchangeCancelled)
	expectEqual(t, "cancellation reason", cancelled.CancellationReason, reason)
	expectEqual(t, "completed sessions", cancelled.Progress.CompletedSessions, 1)
	expectEqual(t, "cancelled sessions", cancelled.Progress.CancelledSessions, 3)
	expectEqual(t, "scheduled sessions", cancelled.Progress.ScheduledSessions, 0)
	expectEqual(t, "draft sessions", cancelled.Progress.DraftSessions, 0)

	// The completed lesson stays; the lessons that have not taken place are cancelled.
	sessions := f.sessionsByID(t, ex.ID)
	expectEqual(t, "completed lesson status", sessions[completed.ID].Status, data.SessionCompleted)
	expectEqual(t, "scheduled lesson status", sessions[scheduled.ID].Status, data.SessionCancelled)
	expectEqual(t, "scheduled lesson reason", sessions[scheduled.ID].CancellationReason, reason)
	expectEqual(t, "proposed lesson status", sessions[proposed.Session.ID].Status, data.SessionCancelled)

	details, err := f.service.SessionDetails(ctx, f.olha, proposed.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "pending proposal status", details.Proposals[0].Status, data.ProposalSuperseded)

	_, err = f.service.ProposeSession(ctx, f.olha, scheduled.ID, ProposalInput{
		StartsAt:   start.Format(time.RFC3339),
		EndsAt:     start.Add(time.Hour).Format(time.RFC3339),
		MeetingURL: "https://meet.example.test/lesson",
	})
	expectError(t, "propose in a cancelled exchange", err, ErrExchangeNotActive)
	expectEqual(t, "Olha's notices about the cancellation", f.notificationCount(t, f.olha, "exchange-cancelled:"+ex.ID), 1)
}
