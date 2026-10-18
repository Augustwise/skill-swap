package session

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"skillswap/backend/internal/communication"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/exchange"
	"skillswap/backend/internal/profile"
)

type fixture struct {
	service   *Service
	exchanges *exchange.Service
	comm      *communication.Service
	store     *datatest.Memory
	olha      string
	andrii    string
	marko     string
	baseTime  time.Time
}

func (f *fixture) student(t *testing.T, email, firstName, lastName, teach, learn string) string {
	t.Helper()
	ctx := context.Background()
	id, err := f.store.CreateUser(ctx, data.NewUser{
		UniversityID: datatest.DemoUniversityID,
		Email:        email,
		FirstName:    firstName,
		LastName:     lastName,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkEmailVerified(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := f.store.UpdateProfile(ctx, id, data.ProfileUpdate{
		FirstName: firstName,
		LastName:  lastName,
		City:      "Київ",
		Formats:   []string{data.FormatOnline, data.FormatOffline},
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.AddUserSkill(ctx, data.TeachingList, id, teach, "ADVANCED"); err != nil {
		t.Fatal(err)
	}
	if err := f.store.AddUserSkill(ctx, data.LearningList, id, learn, "BEGINNER"); err != nil {
		t.Fatal(err)
	}
	return id
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	store := datatest.NewMemory()
	comm := communication.NewService(store, store, store)
	exchanges := exchange.NewService(store, store, store, comm)
	sessions := NewService(store, store, store, comm)
	base := time.Date(2026, time.October, 25, 10, 0, 0, 0, time.UTC)
	sessions.now = func() time.Time { return base }

	f := &fixture{
		service:   sessions,
		exchanges: exchanges,
		comm:      comm,
		store:     store,
		baseTime:  base,
	}
	f.olha = f.student(t, "olha@students.example.test", "Ольга", "Гнатюк", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	f.andrii = f.student(t, "andrii@students.example.test", "Андрій", "Коваль", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	f.marko = f.student(t, "marko@students.example.test", "Марко", "Савчук", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	return f
}

func (f *fixture) createExchange(t *testing.T, requester, recipient, format string, teachSessions, learnSessions int) data.Exchange {
	t.Helper()
	ctx := context.Background()
	req, err := f.exchanges.CreateRequest(ctx, requester, exchange.NewRequest{
		RecipientID:          recipient,
		TeachSkillID:         datatest.GuitarSkillID,
		LearnSkillID:         datatest.PhotoshopSkillID,
		Format:               format,
		TeachSessions:        teachSessions,
		TeachDurationMinutes: 60,
		LearnSessions:        learnSessions,
		LearnDurationMinutes: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := f.exchanges.AcceptRequest(ctx, recipient, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	return accepted.Exchange
}

func fieldErrors(t *testing.T, err error) map[string]string {
	t.Helper()
	var invalid *profile.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	return invalid.Fields
}

func TestExchangeSessionsAndInitialDraftPlan(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.createExchange(t, f.olha, f.andrii, data.FormatOnline, 2, 2)

	res, err := f.service.ExchangeSessions(ctx, f.olha, strings.ToUpper(ex.ID))
	if err != nil {
		t.Fatal(err)
	}
	if res.Exchange.ID != ex.ID || res.Exchange.Progress.TotalSessions != 4 ||
		res.Exchange.Progress.DraftSessions != 4 || res.Exchange.Progress.CompletedSessions != 0 ||
		len(res.Items) != 4 {
		t.Fatalf("exchange sessions = %+v", res)
	}
	for i, s := range res.Items {
		if s.PlanOrder != i+1 || s.Status != data.SessionDraft || s.StartsAt != nil || s.EndsAt != nil ||
			s.PlannedDurationMinutes != 60 || s.Format != data.FormatOnline {
			t.Fatalf("draft session %d = %+v", i, s)
		}
	}
	if res.Items[0].Teacher.UserID != f.olha || res.Items[1].Teacher.UserID != f.olha ||
		res.Items[2].Teacher.UserID != f.andrii || res.Items[3].Teacher.UserID != f.andrii {
		t.Fatalf("unexpected draft teachers: %+v", res.Items)
	}

	// Stranger and unknown IDs are rejected with ErrExchangeNotFound.
	for _, test := range []struct{ user, id string }{
		{f.marko, ex.ID},
		{f.olha, "80000000-0000-0000-0000-000000000999"},
		{f.olha, "not-a-uuid"},
	} {
		if _, err := f.service.ExchangeSessions(ctx, test.user, test.id); !errors.Is(err, ErrExchangeNotFound) {
			t.Fatalf("ExchangeSessions(%s, %s): err = %v", test.user, test.id, err)
		}
	}
}

func TestProposeAcceptDeclineAndRescheduleFlow(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.createExchange(t, f.olha, f.andrii, data.FormatOnline, 2, 2)

	start1 := f.baseTime.Add(24 * time.Hour).Format(time.RFC3339)
	end1 := f.baseTime.Add(25 * time.Hour).Format(time.RFC3339)

	// Olha proposes a session on the exchange (picks her first DRAFT slot).
	proposed, err := f.service.ProposeExchangeSession(ctx, f.olha, ex.ID, ProposalInput{
		Title:       "  Акорди для початківців  ",
		Description: " Розберемо Am, Dm, E. ",
		TeacherID:   f.olha,
		StartsAt:    start1,
		EndsAt:      end1,
		Format:      data.FormatOnline,
		MeetingURL:  "https://meet.example.test/guitar-lesson-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if proposed.Session.PlanOrder != 1 || proposed.Session.Status != data.SessionProposed ||
		proposed.Session.Title != "Акорди для початківців" ||
		proposed.Session.Description != "Розберемо Am, Dm, E." ||
		proposed.Session.MeetingURL != "https://meet.example.test/guitar-lesson-1" ||
		!slices.Equal(proposed.Session.ConfirmedUserIDs, []string{f.olha}) {
		t.Fatalf("proposed session = %+v", proposed.Session)
	}
	if proposed.Proposal.Status != data.ProposalPending || proposed.Proposal.ProposedByID != f.olha {
		t.Fatalf("proposal = %+v", proposed.Proposal)
	}

	// Andrii receives a SESSION_UPDATED notification.
	notifs, err := f.comm.Notifications(ctx, f.andrii, true, 1)
	if err != nil || notifs.UnreadCount < 1 || notifs.Items[0].EventType != data.EventSessionUpdated ||
		notifs.Items[0].SessionID != proposed.Session.ID {
		t.Fatalf("Andrii notifications = %+v, err %v", notifs, err)
	}

	// Olha (the proposer) cannot accept or decline her own proposal.
	if _, err := f.service.AcceptSessionProposal(ctx, f.olha, proposed.Session.ID); !errors.Is(err, ErrActionNotAllowed) {
		t.Fatalf("proposer accept: err = %v", err)
	}
	if _, err := f.service.DeclineSessionProposal(ctx, f.olha, proposed.Session.ID); !errors.Is(err, ErrActionNotAllowed) {
		t.Fatalf("proposer decline: err = %v", err)
	}

	// Andrii declines the first proposal -> session returns to DRAFT and confirmations are cleared.
	declined, err := f.service.DeclineSessionProposal(ctx, f.andrii, proposed.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if declined.Session.Status != data.SessionDraft || declined.Session.StartsAt != nil ||
		len(declined.Session.ConfirmedUserIDs) != 0 || declined.Proposal.Status != data.ProposalDeclined {
		t.Fatalf("declined = %+v", declined)
	}
	// Repeating DeclineSessionProposal is idempotent.
	if again, err := f.service.DeclineSessionProposal(ctx, f.andrii, proposed.Session.ID); err != nil ||
		again.Proposal.ID != declined.Proposal.ID {
		t.Fatalf("declined again = %+v, err %v", again, err)
	}

	// Andrii now proposes an offline session on the same slot.
	start2 := f.baseTime.Add(48 * time.Hour).Format(time.RFC3339)
	end2 := f.baseTime.Add(49 * time.Hour).Format(time.RFC3339)
	proposed2, err := f.service.ProposeSession(ctx, f.andrii, proposed.Session.ID, ProposalInput{
		Title:        "Акорди вживу",
		StartsAt:     start2,
		EndsAt:       end2,
		Format:       data.FormatOffline,
		MeetingPlace: "Бібліотека КПІ, 1 поверх",
	})
	if err != nil {
		t.Fatal(err)
	}
	if proposed2.Session.Status != data.SessionProposed || proposed2.Session.Format != data.FormatOffline ||
		proposed2.Session.MeetingPlace != "Бібліотека КПІ, 1 поверх" || proposed2.Session.MeetingURL != "" {
		t.Fatalf("proposed2 = %+v", proposed2.Session)
	}

	// Olha accepts Andrii's proposal -> session becomes SCHEDULED with both confirmations.
	accepted, err := f.service.AcceptSessionProposal(ctx, f.olha, proposed.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Session.Status != data.SessionScheduled || accepted.Proposal.Status != data.ProposalAccepted ||
		len(accepted.Session.ConfirmedUserIDs) != 2 {
		t.Fatalf("accepted = %+v", accepted)
	}
	// Repeating AcceptSessionProposal is idempotent.
	if again, err := f.service.AcceptSessionProposal(ctx, f.olha, proposed.Session.ID); err != nil ||
		again.Proposal.ID != accepted.Proposal.ID || len(again.Session.ConfirmedUserIDs) != 2 {
		t.Fatalf("accepted again = %+v, err %v", again, err)
	}

	// Rescheduling a SCHEDULED session clears the previous partner confirmation and requires new agreement (FR-09, LR2 5.3).
	start3 := f.baseTime.Add(72 * time.Hour).Format(time.RFC3339)
	end3 := f.baseTime.Add(73 * time.Hour).Format(time.RFC3339)
	rescheduled, err := f.service.ProposeSession(ctx, f.olha, proposed.Session.ID, ProposalInput{
		StartsAt:     start3,
		EndsAt:       end3,
		Format:       data.FormatOffline,
		MeetingPlace: "Корпус 18, хол",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rescheduled.Session.Status != data.SessionProposed ||
		!slices.Equal(rescheduled.Session.ConfirmedUserIDs, []string{f.olha}) {
		t.Fatalf("rescheduled = %+v", rescheduled.Session)
	}

	// SessionDetails shows all 3 proposals in chronological order.
	details, err := f.service.SessionDetails(ctx, f.andrii, proposed.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(details.Proposals) != 3 ||
		details.Proposals[0].Status != data.ProposalDeclined ||
		details.Proposals[1].Status != data.ProposalAccepted ||
		details.Proposals[2].Status != data.ProposalPending {
		t.Fatalf("proposals history = %+v", details.Proposals)
	}
}

func TestScheduleConflictsAndHalfOpenIntervals(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex1 := f.createExchange(t, f.olha, f.andrii, data.FormatOnline, 2, 2)
	ex2 := f.createExchange(t, f.olha, f.marko, data.FormatOnline, 1, 1)

	// Schedule Session 1 of ex1 from 14:00 to 15:00 tomorrow.
	t14 := f.baseTime.Add(28 * time.Hour) // 14:00 UTC next day
	t15 := t14.Add(time.Hour)             // 15:00 UTC
	t16 := t15.Add(time.Hour)             // 16:00 UTC

	prop1, err := f.service.ProposeExchangeSession(ctx, f.olha, ex1.ID, ProposalInput{
		Title:      "Гітара 1",
		TeacherID:  f.olha,
		StartsAt:   t14.Format(time.RFC3339),
		EndsAt:     t15.Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.AcceptSessionProposal(ctx, f.andrii, prop1.Session.ID); err != nil {
		t.Fatal(err)
	}

	// Overlapping time [14:30, 15:30) in the same exchange is rejected with ErrSessionConflict.
	_, err = f.service.ProposeExchangeSession(ctx, f.andrii, ex1.ID, ProposalInput{
		Title:      "Photoshop 1",
		TeacherID:  f.andrii,
		StartsAt:   t14.Add(30 * time.Minute).Format(time.RFC3339),
		EndsAt:     t15.Add(30 * time.Minute).Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/2",
	})
	if !errors.Is(err, ErrSessionConflict) {
		t.Fatalf("overlapping proposal in same exchange: err = %v, want ErrSessionConflict", err)
	}

	// Overlapping time [14:30, 15:30) in ANOTHER exchange (Olha <-> Marko) is also rejected because Olha is busy.
	_, err = f.service.ProposeExchangeSession(ctx, f.marko, ex2.ID, ProposalInput{
		Title:      "Photoshop з Марком",
		TeacherID:  f.marko,
		StartsAt:   t14.Add(30 * time.Minute).Format(time.RFC3339),
		EndsAt:     t15.Add(30 * time.Minute).Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/marko",
	})
	if !errors.Is(err, ErrSessionConflict) {
		t.Fatalf("overlapping proposal across exchanges: err = %v, want ErrSessionConflict", err)
	}

	// Half-open interval [15:00, 16:00) starting at the exact end of [14:00, 15:00) is allowed (LR2 5.3).
	prop2, err := f.service.ProposeExchangeSession(ctx, f.andrii, ex1.ID, ProposalInput{
		Title:      "Photoshop одразу після гітари",
		TeacherID:  f.andrii,
		StartsAt:   t15.Format(time.RFC3339),
		EndsAt:     t16.Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/2",
	})
	if err != nil {
		t.Fatalf("adjacent interval [15:00, 16:00): err = %v", err)
	}
	if _, err := f.service.AcceptSessionProposal(ctx, f.olha, prop2.Session.ID); err != nil {
		t.Fatalf("accept adjacent interval: err = %v", err)
	}
}

func TestCancelSessionProgressAndReplacementSlot(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.createExchange(t, f.olha, f.andrii, data.FormatOnline, 1, 1)

	// Cannot cancel a DRAFT session.
	sessions, err := f.service.ExchangeSessions(ctx, f.olha, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.CancelSession(ctx, f.olha, sessions.Items[0].ID, "Ще не заплановано"); !errors.Is(err, ErrSessionNotCancellable) {
		t.Fatalf("cancel DRAFT: err = %v", err)
	}

	// Propose and accept Session 1.
	tStart := f.baseTime.Add(24 * time.Hour).Format(time.RFC3339)
	tEnd := f.baseTime.Add(25 * time.Hour).Format(time.RFC3339)
	prop, err := f.service.ProposeSession(ctx, f.olha, sessions.Items[0].ID, ProposalInput{
		Title:      "Гітара — заняття 1",
		StartsAt:   tStart,
		EndsAt:     tEnd,
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/g1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.AcceptSessionProposal(ctx, f.andrii, prop.Session.ID); err != nil {
		t.Fatal(err)
	}

	// Cancel the SCHEDULED session.
	cancelled, err := f.service.CancelSession(ctx, f.andrii, prop.Session.ID, "  Перенесли через іспит  ")
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != data.SessionCancelled || cancelled.CancelledAt == nil ||
		cancelled.CancellationReason != "Перенесли через іспит" {
		t.Fatalf("cancelled = %+v", cancelled)
	}
	// Repeating CancelSession is idempotent and does not create another replacement slot.
	if again, err := f.service.CancelSession(ctx, f.andrii, prop.Session.ID, ""); err != nil || again.ID != cancelled.ID {
		t.Fatalf("cancel again = %+v, err %v", again, err)
	}

	// Cancelled session does NOT increase progress, and a replacement DRAFT slot exists so Olha can propose again.
	afterCancel, err := f.service.ExchangeSessions(ctx, f.olha, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterCancel.Exchange.Progress.CompletedSessions != 0 ||
		afterCancel.Exchange.Progress.CancelledSessions != 1 ||
		afterCancel.Exchange.Progress.DraftSessions != 2 ||
		afterCancel.Exchange.Progress.Percent != 0 ||
		len(afterCancel.Items) != 3 {
		t.Fatalf("after cancel = %+v", afterCancel)
	}

	// Olha can still propose her 1 planned session using the replacement DRAFT slot.
	replacement, err := f.service.ProposeExchangeSession(ctx, f.olha, ex.ID, ProposalInput{
		Title:      "Гітара — нова дата",
		TeacherID:  f.olha,
		StartsAt:   f.baseTime.Add(48 * time.Hour).Format(time.RFC3339),
		EndsAt:     f.baseTime.Add(49 * time.Hour).Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/g1-new",
	})
	if err != nil {
		t.Fatalf("propose on replacement slot: err = %v", err)
	}
	// Now Olha's 1 planned slot is PROPOSED, so proposing another one for Olha hits ErrPlanLimitReached.
	_, err = f.service.ProposeExchangeSession(ctx, f.olha, ex.ID, ProposalInput{
		Title:      "Зайве заняття",
		TeacherID:  f.olha,
		StartsAt:   f.baseTime.Add(72 * time.Hour).Format(time.RFC3339),
		EndsAt:     f.baseTime.Add(73 * time.Hour).Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/extra",
	})
	if !errors.Is(err, ErrPlanLimitReached) {
		t.Fatalf("exceeding plan limit: err = %v, want ErrPlanLimitReached", err)
	}

	// Mark the replacement session COMPLETED and verify progress updates while cancelled session does not count.
	f.store.SetSessionStatus(replacement.Session.ID, data.SessionCompleted)
	withCompleted, err := f.service.ExchangeSessions(ctx, f.olha, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	if withCompleted.Exchange.Progress.CompletedSessions != 1 ||
		withCompleted.Exchange.Progress.RequesterCompletedSessions != 1 ||
		withCompleted.Exchange.Progress.CancelledSessions != 1 ||
		withCompleted.Exchange.Progress.Percent != 50 {
		t.Fatalf("progress with 1 completed and 1 cancelled = %+v", withCompleted.Exchange.Progress)
	}
}

func TestScheduleChronologicalOrderAndPrivacy(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.createExchange(t, f.olha, f.andrii, data.FormatOnline, 1, 1)

	// Schedule session 2 (later, +48h) first, then session 1 (earlier, +24h).
	laterStart := f.baseTime.Add(48 * time.Hour)
	earlierStart := f.baseTime.Add(24 * time.Hour)

	pLater, err := f.service.ProposeExchangeSession(ctx, f.andrii, ex.ID, ProposalInput{
		Title:      "Photoshop — пізніше заняття",
		TeacherID:  f.andrii,
		StartsAt:   laterStart.Format(time.RFC3339),
		EndsAt:     laterStart.Add(time.Hour).Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/ps",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.AcceptSessionProposal(ctx, f.olha, pLater.Session.ID); err != nil {
		t.Fatal(err)
	}

	pEarlier, err := f.service.ProposeExchangeSession(ctx, f.olha, ex.ID, ProposalInput{
		Title:      "Гітара — раніше заняття",
		TeacherID:  f.olha,
		StartsAt:   earlierStart.Format(time.RFC3339),
		EndsAt:     earlierStart.Add(time.Hour).Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/guitar",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.AcceptSessionProposal(ctx, f.andrii, pEarlier.Session.ID); err != nil {
		t.Fatal(err)
	}

	// Both Olha and Andrii see the two sessions in chronological order (earlier first).
	for _, user := range []string{f.olha, f.andrii} {
		sched, err := f.service.Schedule(ctx, user, data.ScheduleFilter{UpcomingOnly: true}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if sched.Total != 2 || len(sched.Items) != 2 ||
			sched.Items[0].ID != pEarlier.Session.ID || sched.Items[1].ID != pLater.Session.ID {
			t.Fatalf("Schedule(%s) = %+v", user, sched)
		}
	}

	// Exchange NextSession points to the earliest upcoming scheduled session.
	details, err := f.exchanges.ExchangeDetails(ctx, f.olha, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	next := details.Exchange.NextSession
	expectEqual(t, "next session is set", next != nil, true)
	expectEqual(t, "next session", next.ID, pEarlier.Session.ID)

	// Marko (a third student) has an empty schedule and cannot see Olha & Andrii's sessions (FR-10).
	markoSched, err := f.service.Schedule(ctx, f.marko, data.ScheduleFilter{}, 1)
	if err != nil || markoSched.Total != 0 || len(markoSched.Items) != 0 {
		t.Fatalf("Marko schedule = %+v, err %v", markoSched, err)
	}
}

func TestValidationAndStateErrors(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ex := f.createExchange(t, f.olha, f.andrii, data.FormatOnline, 1, 1)

	valid := ProposalInput{
		Title:      "Тема заняття",
		TeacherID:  f.olha,
		StartsAt:   f.baseTime.Add(24 * time.Hour).Format(time.RFC3339),
		EndsAt:     f.baseTime.Add(25 * time.Hour).Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/room",
	}

	for _, test := range []struct {
		name   string
		change func(in *ProposalInput)
		field  string
	}{
		{"empty title", func(in *ProposalInput) { in.Title = "   " }, "title"},
		{"long title", func(in *ProposalInput) { in.Title = strings.Repeat("а", MaxTitleLength+1) }, "title"},
		{"long description", func(in *ProposalInput) { in.Description = strings.Repeat("б", MaxDescriptionLength+1) }, "description"},
		{"invalid teacher UUID", func(in *ProposalInput) { in.TeacherID = "olha" }, "teacherId"},
		{"stranger as teacher", func(in *ProposalInput) { in.TeacherID = f.marko }, "teacherId"},
		{"startsAt without timezone", func(in *ProposalInput) { in.StartsAt = "2026-10-26T10:00:00" }, "startsAt"},
		{"startsAt in the past", func(in *ProposalInput) { in.StartsAt = f.baseTime.Add(-time.Hour).Format(time.RFC3339) }, "startsAt"},
		{"endsAt before startsAt", func(in *ProposalInput) {
			in.EndsAt = f.baseTime.Add(23 * time.Hour).Format(time.RFC3339)
		}, "endsAt"},
		{"duration too short", func(in *ProposalInput) {
			in.EndsAt = f.baseTime.Add(24*time.Hour + 10*time.Minute).Format(time.RFC3339)
		}, "endsAt"},
		{"duration too long", func(in *ProposalInput) {
			in.EndsAt = f.baseTime.Add(24*time.Hour + 241*time.Minute).Format(time.RFC3339)
		}, "endsAt"},
		{"unknown format", func(in *ProposalInput) { in.Format = "CAMPUS" }, "format"},
		{"online without meetingUrl", func(in *ProposalInput) { in.MeetingURL = "" }, "meetingUrl"},
		{"online with javascript URL", func(in *ProposalInput) { in.MeetingURL = "javascript:alert(1)" }, "meetingUrl"},
		{"online with meetingPlace", func(in *ProposalInput) { in.MeetingPlace = "КПІ" }, "meetingPlace"},
		{"offline without meetingPlace", func(in *ProposalInput) {
			in.Format = data.FormatOffline
			in.MeetingURL = ""
			in.MeetingPlace = ""
		}, "meetingPlace"},
		{"offline with meetingUrl", func(in *ProposalInput) {
			in.Format = data.FormatOffline
			in.MeetingPlace = "КПІ"
		}, "meetingUrl"},
	} {
		t.Run(test.name, func(t *testing.T) {
			in := valid
			test.change(&in)
			_, err := f.service.ProposeExchangeSession(ctx, f.olha, ex.ID, in)
			if fields := fieldErrors(t, err); fields[test.field] == "" {
				t.Fatalf("fields = %v, want %s", fields, test.field)
			}
		})
	}

	// Block between participants prevents proposing or accepting sessions.
	f.store.SetBlocked(f.andrii, f.olha)
	if _, err := f.service.ProposeExchangeSession(ctx, f.olha, ex.ID, valid); !errors.Is(err, ErrExchangeBlocked) {
		t.Fatalf("blocked ProposeExchangeSession: err = %v", err)
	}

	// Completed or cancelled exchange rejects session proposals.
	f.store.SetExchangeStatus(ex.ID, data.ExchangeCompleted)
	if _, err := f.service.ProposeExchangeSession(ctx, f.olha, ex.ID, valid); !errors.Is(err, ErrExchangeNotActive) {
		t.Fatalf("completed exchange: err = %v", err)
	}
}
