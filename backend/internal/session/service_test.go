package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"skillswap/backend/internal/communication"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/exchange"
	"skillswap/backend/internal/expect"
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
	base := time.Now().UTC().Truncate(24 * time.Hour).Add(34 * time.Hour)
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
	progress := res.Exchange.Progress
	expect.Equal(t, "exchange", res.Exchange.ID, ex.ID)
	expect.Equal(t, "total sessions", progress.TotalSessions, 4)
	expect.Equal(t, "draft sessions", progress.DraftSessions, 4)
	expect.Equal(t, "completed sessions", progress.CompletedSessions, 0)
	expect.Equal(t, "sessions", len(res.Items), 4)

	var teachers []string
	for i, s := range res.Items {
		name := fmt.Sprintf("draft session %d", i)
		expect.Equal(t, name+": plan order", s.PlanOrder, i+1)
		expect.Equal(t, name+": status", s.Status, data.SessionDraft)
		expect.Equal(t, name+": startsAt is set", s.StartsAt != nil, false)
		expect.Equal(t, name+": endsAt is set", s.EndsAt != nil, false)
		expect.Equal(t, name+": planned duration", s.PlannedDurationMinutes, 60)
		expect.Equal(t, name+": format", s.Format, data.FormatOnline)
		teachers = append(teachers, s.Teacher.UserID)
	}
	expect.Slice(t, "draft teachers", teachers, []string{f.olha, f.olha, f.andrii, f.andrii})

	// Stranger and unknown IDs are rejected with ErrExchangeNotFound.
	for _, test := range []struct{ name, user, id string }{
		{"not a participant", f.marko, ex.ID},
		{"unknown exchange", f.olha, "80000000-0000-0000-0000-000000000999"},
		{"not a UUID", f.olha, "not-a-uuid"},
	} {
		_, err := f.service.ExchangeSessions(ctx, test.user, test.id)
		expect.Error(t, test.name, err, ErrExchangeNotFound)
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
	expect.Equal(t, "plan order", proposed.Session.PlanOrder, 1)
	expect.Equal(t, "status after proposal", proposed.Session.Status, data.SessionProposed)
	expect.Equal(t, "title", proposed.Session.Title, "Акорди для початківців")
	expect.Equal(t, "description", proposed.Session.Description, "Розберемо Am, Dm, E.")
	expect.Equal(t, "meeting URL", proposed.Session.MeetingURL, "https://meet.example.test/guitar-lesson-1")
	expect.Slice(t, "confirmations after proposal", proposed.Session.ConfirmedUserIDs, []string{f.olha})
	expect.Equal(t, "proposal status", proposed.Proposal.Status, data.ProposalPending)
	expect.Equal(t, "proposed by", proposed.Proposal.ProposedByID, f.olha)

	// Andrii receives a SESSION_UPDATED notification.
	notifs, err := f.comm.Notifications(ctx, f.andrii, true, 1)
	expect.NoError(t, "load Andrii's notifications", err)
	expect.Equal(t, "Andrii has unread notifications", notifs.UnreadCount >= 1, true)
	expect.Equal(t, "Andrii's latest notification type", notifs.Items[0].EventType, data.EventSessionUpdated)
	expect.Equal(t, "Andrii's latest notification session", notifs.Items[0].SessionID, proposed.Session.ID)

	// Olha (the proposer) cannot accept or decline her own proposal.
	_, err = f.service.AcceptSessionProposal(ctx, f.olha, proposed.Session.ID)
	expect.Error(t, "proposer accepts", err, ErrActionNotAllowed)
	_, err = f.service.DeclineSessionProposal(ctx, f.olha, proposed.Session.ID)
	expect.Error(t, "proposer declines", err, ErrActionNotAllowed)

	// Andrii declines the first proposal -> session returns to DRAFT and confirmations are cleared.
	declined, err := f.service.DeclineSessionProposal(ctx, f.andrii, proposed.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "status after decline", declined.Session.Status, data.SessionDraft)
	expect.Equal(t, "startsAt is set after decline", declined.Session.StartsAt != nil, false)
	expect.Equal(t, "confirmations after decline", len(declined.Session.ConfirmedUserIDs), 0)
	expect.Equal(t, "proposal status after decline", declined.Proposal.Status, data.ProposalDeclined)

	// Repeating DeclineSessionProposal is idempotent.
	declinedAgain, err := f.service.DeclineSessionProposal(ctx, f.andrii, proposed.Session.ID)
	expect.NoError(t, "decline again", err)
	expect.Equal(t, "proposal after a repeated decline", declinedAgain.Proposal.ID, declined.Proposal.ID)

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
	expect.Equal(t, "status of Andrii's proposal", proposed2.Session.Status, data.SessionProposed)
	expect.Equal(t, "format of Andrii's proposal", proposed2.Session.Format, data.FormatOffline)
	expect.Equal(t, "meeting place of Andrii's proposal", proposed2.Session.MeetingPlace, "Бібліотека КПІ, 1 поверх")
	expect.Equal(t, "meeting URL of Andrii's proposal", proposed2.Session.MeetingURL, "")

	// Olha accepts Andrii's proposal -> session becomes SCHEDULED with both confirmations.
	accepted, err := f.service.AcceptSessionProposal(ctx, f.olha, proposed.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "status after accept", accepted.Session.Status, data.SessionScheduled)
	expect.Equal(t, "proposal status after accept", accepted.Proposal.Status, data.ProposalAccepted)
	expect.Equal(t, "confirmations after accept", len(accepted.Session.ConfirmedUserIDs), 2)

	// Repeating AcceptSessionProposal is idempotent.
	acceptedAgain, err := f.service.AcceptSessionProposal(ctx, f.olha, proposed.Session.ID)
	expect.NoError(t, "accept again", err)
	expect.Equal(t, "proposal after a repeated accept", acceptedAgain.Proposal.ID, accepted.Proposal.ID)
	expect.Equal(t, "confirmations after a repeated accept", len(acceptedAgain.Session.ConfirmedUserIDs), 2)

	// Rescheduling a SCHEDULED session clears the previous partner confirmation and requires new agreement.
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
	expect.Equal(t, "status after rescheduling", rescheduled.Session.Status, data.SessionProposed)
	expect.Slice(t, "confirmations after rescheduling", rescheduled.Session.ConfirmedUserIDs, []string{f.olha})

	// SessionDetails shows all 3 proposals in chronological order.
	details, err := f.service.SessionDetails(ctx, f.andrii, proposed.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "proposals in history", len(details.Proposals), 3)
	expect.Equal(t, "first proposal status", details.Proposals[0].Status, data.ProposalDeclined)
	expect.Equal(t, "second proposal status", details.Proposals[1].Status, data.ProposalAccepted)
	expect.Equal(t, "third proposal status", details.Proposals[2].Status, data.ProposalPending)
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
	expect.Error(t, "overlapping proposal in the same exchange", err, ErrSessionConflict)

	// Overlapping time [14:30, 15:30) in ANOTHER exchange (Olha <-> Marko) is also rejected because Olha is busy.
	_, err = f.service.ProposeExchangeSession(ctx, f.marko, ex2.ID, ProposalInput{
		Title:      "Photoshop з Марком",
		TeacherID:  f.marko,
		StartsAt:   t14.Add(30 * time.Minute).Format(time.RFC3339),
		EndsAt:     t15.Add(30 * time.Minute).Format(time.RFC3339),
		Format:     data.FormatOnline,
		MeetingURL: "https://meet.example.test/marko",
	})
	expect.Error(t, "overlapping proposal across exchanges", err, ErrSessionConflict)

	// Half-open interval [15:00, 16:00) starting at the exact end of [14:00, 15:00) is allowed.
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
	_, err = f.service.CancelSession(ctx, f.olha, sessions.Items[0].ID, "Ще не заплановано")
	expect.Error(t, "cancel a draft session", err, ErrSessionNotCancellable)

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
	expect.Equal(t, "status after cancel", cancelled.Status, data.SessionCancelled)
	expect.Equal(t, "cancelledAt is set", cancelled.CancelledAt != nil, true)
	expect.Equal(t, "cancellation reason", cancelled.CancellationReason, "Перенесли через іспит")

	// Repeating CancelSession is idempotent and does not create another replacement slot.
	again, err := f.service.CancelSession(ctx, f.andrii, prop.Session.ID, "")
	expect.NoError(t, "cancel again", err)
	expect.Equal(t, "session after a repeated cancel", again.ID, cancelled.ID)

	// Cancelled session does NOT increase progress, and a replacement DRAFT slot exists so Olha can propose again.
	afterCancel, err := f.service.ExchangeSessions(ctx, f.olha, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	progress := afterCancel.Exchange.Progress
	expect.Equal(t, "completed sessions after cancel", progress.CompletedSessions, 0)
	expect.Equal(t, "cancelled sessions after cancel", progress.CancelledSessions, 1)
	expect.Equal(t, "draft sessions after cancel", progress.DraftSessions, 2)
	expect.Equal(t, "percent after cancel", progress.Percent, 0)
	expect.Equal(t, "sessions after cancel", len(afterCancel.Items), 3)

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
	expect.Error(t, "exceeding plan limit", err, ErrPlanLimitReached)

	// Mark the replacement session COMPLETED and verify progress updates while cancelled session does not count.
	f.store.SetSessionStatus(replacement.Session.ID, data.SessionCompleted)
	withCompleted, err := f.service.ExchangeSessions(ctx, f.olha, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	progress = withCompleted.Exchange.Progress
	expect.Equal(t, "completed sessions", progress.CompletedSessions, 1)
	expect.Equal(t, "sessions Olha has taught", progress.RequesterCompletedSessions, 1)
	expect.Equal(t, "cancelled sessions", progress.CancelledSessions, 1)
	expect.Equal(t, "percent", progress.Percent, 50)
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
	for _, user := range []struct{ name, id string }{{"Olha", f.olha}, {"Andrii", f.andrii}} {
		sched, err := f.service.Schedule(ctx, user.id, data.ScheduleFilter{UpcomingOnly: true}, 1)
		if err != nil {
			t.Fatal(err)
		}
		expect.Equal(t, user.name+": total sessions", sched.Total, 2)
		expect.Equal(t, user.name+": listed sessions", len(sched.Items), 2)
		expect.Equal(t, user.name+": first session", sched.Items[0].ID, pEarlier.Session.ID)
		expect.Equal(t, user.name+": second session", sched.Items[1].ID, pLater.Session.ID)
	}

	// Exchange NextSession points to the earliest upcoming scheduled session.
	details, err := f.exchanges.ExchangeDetails(ctx, f.olha, ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	next := details.Exchange.NextSession
	expect.Equal(t, "next session is set", next != nil, true)
	expect.Equal(t, "next session", next.ID, pEarlier.Session.ID)

	// Marko (a third student) has an empty schedule and cannot see Olha & Andrii's sessions.
	markoSched, err := f.service.Schedule(ctx, f.marko, data.ScheduleFilter{}, 1)
	expect.NoError(t, "load Marko's schedule", err)
	expect.Equal(t, "Marko's total sessions", markoSched.Total, 0)
	expect.Equal(t, "Marko's listed sessions", len(markoSched.Items), 0)
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
	_, err := f.service.ProposeExchangeSession(ctx, f.olha, ex.ID, valid)
	expect.Error(t, "propose after a block", err, ErrExchangeBlocked)

	// Completed or cancelled exchange rejects session proposals.
	f.store.SetExchangeStatus(ex.ID, data.ExchangeCompleted)
	_, err = f.service.ProposeExchangeSession(ctx, f.olha, ex.ID, valid)
	expect.Error(t, "propose in a completed exchange", err, ErrExchangeNotActive)
}
