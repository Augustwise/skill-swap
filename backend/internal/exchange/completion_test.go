package exchange

import (
	"context"
	"errors"
	"strings"
	"testing"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
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

// activeExchange returns an exchange where Olha teaches two sessions and Andrii one.
func (f *fixture) activeExchange(t *testing.T) data.Exchange {
	t.Helper()
	ctx := context.Background()
	created, err := f.service.CreateRequest(ctx, f.olha, f.request())
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := f.service.AcceptRequest(ctx, f.andrii, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	return accepted.Exchange
}

// completeSessions marks the first count sessions of the plan as completed.
func (f *fixture) completeSessions(t *testing.T, exchangeID string, count int) {
	t.Helper()
	sessions, err := f.store.ExchangeSessions(context.Background(), exchangeID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sessions[:count] {
		f.store.SetSessionStatus(s.ID, data.SessionCompleted)
	}
}

func (f *fixture) unreadCount(t *testing.T, userID string) int {
	t.Helper()
	page, err := f.comm.Notifications(context.Background(), userID, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	return page.UnreadCount
}

func (f *fixture) stranger(t *testing.T) string {
	t.Helper()
	return f.student(t, "marko@students.example.test", datatest.DemoUniversityID, "Київ", []string{data.FormatOnline},
		datatest.PhotoshopSkillID, datatest.GuitarSkillID)
}

func TestCompleteExchangeErrors(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.activeExchange(t)
	stranger := f.stranger(t)
	complete := func(userID, exchangeID string) error {
		_, err := f.service.CompleteExchange(ctx, userID, exchangeID)
		return err
	}

	expectError(t, "complete someone else's exchange", complete(stranger, ex.ID), ErrExchangeNotFound)
	expectError(t, "complete an unknown exchange", complete(f.olha, "80000000-0000-0000-0000-000000000999"), ErrExchangeNotFound)
	expectError(t, "complete with an invalid ID", complete(f.olha, "exchange"), ErrExchangeNotFound)

	// Olha's two sessions are done, Andrii's one is not.
	f.completeSessions(t, ex.ID, 2)
	expectError(t, "complete with one direction left", complete(f.olha, ex.ID), ErrPlanNotCompleted)

	f.completeSessions(t, ex.ID, 3)
	f.store.SetBlocked(f.olha, f.andrii)
	expectError(t, "complete after a block", complete(f.andrii, ex.ID), ErrExchangeBlocked)

	// Cancelling stays available as a way out of a blocked exchange.
	if _, err := f.service.CancelExchange(ctx, f.andrii, ex.ID, "Блокування"); err != nil {
		t.Fatalf("cancel after a block: err = %v", err)
	}
	expectError(t, "complete a cancelled exchange", complete(f.olha, ex.ID), ErrExchangeNotActive)
}

func TestCompleteExchange(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.activeExchange(t)
	f.completeSessions(t, ex.ID, 3)
	andriiUnread := f.unreadCount(t, f.andrii)

	// Olha confirms first: the exchange waits for Andrii.
	first, err := f.service.CompleteExchange(ctx, f.olha, strings.ToUpper(ex.ID))
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "status after Olha's confirmation", first.Status, data.ExchangeActive)
	expectEqual(t, "confirmations after Olha's confirmation", len(first.ConfirmedUserIDs), 1)
	expectEqual(t, "completedAt is set", first.CompletedAt != nil, false)

	// A repeated confirmation changes nothing and does not notify Andrii twice.
	if _, err := f.service.CompleteExchange(ctx, f.olha, ex.ID); err != nil {
		t.Fatalf("repeated confirmation: err = %v", err)
	}
	expectEqual(t, "Andrii's new notifications", f.unreadCount(t, f.andrii)-andriiUnread, 1)

	// Andrii confirms too: the exchange is completed.
	done, err := f.service.CompleteExchange(ctx, f.andrii, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "status after both confirmations", done.Status, data.ExchangeCompleted)
	expectEqual(t, "completedAt is set", done.CompletedAt != nil, true)
	expectEqual(t, "confirmations after both confirmations", len(done.ConfirmedUserIDs), 2)

	// Confirming a completed exchange again is harmless, but it can no longer be cancelled.
	again, err := f.service.CompleteExchange(ctx, f.olha, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "status after a repeat", again.Status, data.ExchangeCompleted)
	_, err = f.service.CancelExchange(ctx, f.olha, ex.ID, "Передумала")
	expectError(t, "cancel a completed exchange", err, ErrExchangeNotActive)

	details, err := f.service.ExchangeDetails(ctx, f.olha, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "history items", len(details.History), 2)
	expectEqual(t, "last history status", details.History[1].Status, data.ExchangeCompleted)
	expectEqual(t, "completed by", details.History[1].ChangedByID, f.andrii)
}

func TestCancelExchangeErrors(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.activeExchange(t)
	stranger := f.stranger(t)

	for _, reason := range []string{"", "   ", strings.Repeat("я", 501), "bad\x00reason"} {
		_, err := f.service.CancelExchange(ctx, f.olha, ex.ID, reason)
		if fieldErrors(t, err)["reason"] == "" {
			t.Fatalf("reason %q was accepted", reason)
		}
	}

	cancel := func(userID, exchangeID string) error {
		_, err := f.service.CancelExchange(ctx, userID, exchangeID, "Причина")
		return err
	}
	expectError(t, "cancel someone else's exchange", cancel(stranger, ex.ID), ErrExchangeNotFound)
	expectError(t, "cancel an unknown exchange", cancel(f.olha, "80000000-0000-0000-0000-000000000999"), ErrExchangeNotFound)
	expectError(t, "cancel with an invalid ID", cancel(f.olha, "exchange"), ErrExchangeNotFound)
}

func TestCancelExchange(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.activeExchange(t)
	f.completeSessions(t, ex.ID, 1)
	olhaUnread := f.unreadCount(t, f.olha)

	cancelled, err := f.service.CancelExchange(ctx, f.andrii, ex.ID, "  Змінився розклад занять  ")
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "status", cancelled.Status, data.ExchangeCancelled)
	expectEqual(t, "cancelledAt is set", cancelled.CancelledAt != nil, true)
	expectEqual(t, "cancellation reason", cancelled.CancellationReason, "Змінився розклад занять")
	expectEqual(t, "completed sessions", cancelled.Progress.CompletedSessions, 1)
	expectEqual(t, "cancelled sessions", cancelled.Progress.CancelledSessions, 2)
	expectEqual(t, "draft sessions", cancelled.Progress.DraftSessions, 0)
	expectEqual(t, "Olha's new notifications", f.unreadCount(t, f.olha)-olhaUnread, 1)

	// A repeated cancellation keeps the first reason.
	again, err := f.service.CancelExchange(ctx, f.olha, ex.ID, "Інша причина")
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "reason after a repeat", again.CancellationReason, "Змінився розклад занять")

	details, err := f.service.ExchangeDetails(ctx, f.olha, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, "history items", len(details.History), 2)
	last := details.History[1]
	expectEqual(t, "last history status", last.Status, data.ExchangeCancelled)
	expectEqual(t, "cancelled by", last.ChangedByID, f.andrii)
	expectEqual(t, "reason in history", last.Reason, "Змінився розклад занять")
}
