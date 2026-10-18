package exchange

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"skillswap/backend/internal/communication"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/expect"
	"skillswap/backend/internal/profile"
)

type fixture struct {
	service *Service
	comm    *communication.Service
	store   *datatest.Memory
	olha    string
	andrii  string
}

func (f *fixture) student(t *testing.T, email, universityID, city string, formats []string, teach, learn string) string {
	t.Helper()
	ctx := context.Background()
	id, err := f.store.CreateUser(ctx, data.NewUser{UniversityID: universityID, Email: email, FirstName: email, LastName: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkEmailVerified(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := f.store.UpdateProfile(ctx, id, data.ProfileUpdate{FirstName: email, LastName: "Test", City: city, Formats: formats}); err != nil {
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

// newFixture has Olha (teaches guitar, learns Photoshop) and Andrii (the reverse), both in Kyiv.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	store := datatest.NewMemory()
	comm := communication.NewService(store, store, store)
	f := &fixture{service: NewService(store, store, store, comm), comm: comm, store: store}
	both := []string{data.FormatOnline, data.FormatOffline}
	f.olha = f.student(t, "olha@students.example.test", datatest.DemoUniversityID, "Київ", both,
		datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	f.andrii = f.student(t, "andrii@students.example.test", datatest.DemoUniversityID, "Київ", both,
		datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	return f
}

func (f *fixture) request() NewRequest {
	return NewRequest{
		RecipientID: f.andrii, TeachSkillID: datatest.GuitarSkillID, LearnSkillID: datatest.PhotoshopSkillID,
		Format: data.FormatOnline, TeachSessions: 2, TeachDurationMinutes: 60, LearnSessions: 1, LearnDurationMinutes: 120,
	}
}

func fieldErrors(t *testing.T, err error) map[string]string {
	t.Helper()
	var invalid *profile.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	return invalid.Fields
}

func TestCreateRequest(t *testing.T) {
	f := newFixture(t)
	in := f.request()
	in.RecipientID = strings.ToUpper(in.RecipientID)
	in.Message = "  Привіт! Давай обміняємось.\n "
	created, err := f.service.CreateRequest(context.Background(), f.olha, in)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "request ID is set", created.ID != "", true)
	expect.Equal(t, "status", created.Status, data.RequestPending)
	expect.Equal(t, "format", created.Format, data.FormatOnline)
	expect.Equal(t, "total sessions", created.TotalSessions, 3)
	expect.Equal(t, "message", created.Message, "Привіт! Давай обміняємось.")
	expect.Equal(t, "exchange ID", created.ExchangeID, "")
	expect.Equal(t, "requester", created.Requester.UserID, f.olha)
	expect.Equal(t, "recipient", created.Recipient.UserID, f.andrii)

	want := data.RequestTerms{SkillID: datatest.GuitarSkillID, CategoryID: "20000000-0000-0000-0000-000000000001",
		Name: "Гітара", TeacherLevel: "ADVANCED", LearnerLevel: "BEGINNER", Sessions: 2, DurationMinutes: 60}
	expect.Equal(t, "what the requester teaches", created.RequesterTeaches, want)
	recipientTeaches := created.RecipientTeaches
	expect.Equal(t, "skill the recipient teaches", recipientTeaches.SkillID, datatest.PhotoshopSkillID)
	expect.Equal(t, "sessions the recipient teaches", recipientTeaches.Sessions, 1)
	expect.Equal(t, "recipient's session length", recipientTeaches.DurationMinutes, 120)

	conv, err := f.comm.RequestConversation(context.Background(), f.olha, created.ID)
	expect.NoError(t, "load the request conversation", err)
	expect.Equal(t, "conversation request", conv.RequestID, created.ID)
	expect.Equal(t, "conversation type", conv.Type, data.ConversationRequest)

	notifs, err := f.comm.Notifications(context.Background(), f.andrii, true, 1)
	expect.NoError(t, "load Andrii's notifications", err)
	expect.Equal(t, "Andrii's notifications", notifs.Total, 1)
	expect.Equal(t, "Andrii's notification type", notifs.Items[0].EventType, data.EventExchangeRequest)
	expect.Equal(t, "Andrii's notification request", notifs.Items[0].RequestID, created.ID)
}

func TestCreateRequestValidation(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(f *fixture, in *NewRequest)
		field  string
	}{
		{"recipient is not a UUID", func(_ *fixture, in *NewRequest) { in.RecipientID = "andrii" }, "recipientId"},
		{"request to oneself", func(f *fixture, in *NewRequest) { in.RecipientID = f.olha }, "recipientId"},
		{"teach skill is not a UUID", func(_ *fixture, in *NewRequest) { in.TeachSkillID = "" }, "teachSkillId"},
		{"the same skill both ways", func(_ *fixture, in *NewRequest) { in.LearnSkillID = in.TeachSkillID }, "learnSkillId"},
		{"unknown format", func(_ *fixture, in *NewRequest) { in.Format = "CAMPUS" }, "format"},
		{"no sessions", func(_ *fixture, in *NewRequest) { in.TeachSessions = 0 }, "teachSessions"},
		{"too many sessions", func(_ *fixture, in *NewRequest) { in.LearnSessions, in.LearnDurationMinutes = 21, 15 }, "learnSessions"},
		{"too short", func(_ *fixture, in *NewRequest) { in.TeachDurationMinutes = 14 }, "teachDurationMinutes"},
		{"too long", func(_ *fixture, in *NewRequest) { in.LearnDurationMinutes = 241 }, "learnDurationMinutes"},
		{"different total time", func(_ *fixture, in *NewRequest) { in.LearnDurationMinutes = 90 }, "learnSessions"},
		{"long message", func(_ *fixture, in *NewRequest) { in.Message = strings.Repeat("я", 501) }, "message"},
		{"control character", func(_ *fixture, in *NewRequest) { in.Message = "hi\x00" }, "message"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			in := f.request()
			test.change(f, &in)
			_, err := f.service.CreateRequest(context.Background(), f.olha, in)
			fields := fieldErrors(t, err)
			expect.Equal(t, test.field+" has an error", fields[test.field] != "", true)
			expect.Equal(t, "field errors", len(fields), 1)
		})
	}
}

func TestCreateRequestRecipientRules(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	unknown := f.request()
	unknown.RecipientID = "40000000-0000-0000-0000-000000000999"
	_, err := f.service.CreateRequest(ctx, f.olha, unknown)
	expect.Error(t, "unknown student", err, ErrStudentNotFound)
	unverified, err := f.store.CreateUser(ctx, data.NewUser{UniversityID: datatest.DemoUniversityID,
		Email: "pending@students.example.test", FirstName: "P", LastName: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	in := f.request()
	in.RecipientID = unverified
	_, err = f.service.CreateRequest(ctx, f.olha, in)
	expect.Error(t, "unverified student", err, ErrStudentNotFound)

	f.store.SetRequestSettings(f.andrii, false, false)
	_, err = f.service.CreateRequest(ctx, f.olha, f.request())
	expect.Error(t, "closed requests", err, ErrRequestsClosed)
	f.store.SetRequestSettings(f.andrii, true, true)
	if _, err := f.service.CreateRequest(ctx, f.olha, f.request()); err != nil {
		t.Fatalf("same university: err = %v", err)
	}

	other := f.student(t, "other@other.example.test", datatest.OtherUniversityID, "Київ", []string{data.FormatOnline},
		datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	f.store.SetRequestSettings(other, true, true)
	in = f.request()
	in.RecipientID = other
	_, err = f.service.CreateRequest(ctx, f.olha, in)
	expect.Error(t, "other university", err, ErrSameUniversityOnly)
}

func TestCreateRequestChecksSkillsAndFormats(t *testing.T) {
	both := []string{data.FormatOnline, data.FormatOffline}
	for _, test := range []struct {
		name   string
		setup  func(t *testing.T, f *fixture, in *NewRequest)
		fields []string
	}{
		{"skills swapped", func(_ *testing.T, _ *fixture, in *NewRequest) {
			in.TeachSkillID, in.LearnSkillID = in.LearnSkillID, in.TeachSkillID
		}, []string{"teachSkillId", "learnSkillId"}},
		{"recipient does not want my skill", func(t *testing.T, f *fixture, _ *NewRequest) {
			if err := f.store.RemoveUserSkill(context.Background(), data.LearningList, f.andrii, datatest.GuitarSkillID); err != nil {
				t.Fatal(err)
			}
		}, []string{"teachSkillId"}},
		{"I do not want the recipient's skill", func(t *testing.T, f *fixture, _ *NewRequest) {
			if err := f.store.RemoveUserSkill(context.Background(), data.LearningList, f.olha, datatest.PhotoshopSkillID); err != nil {
				t.Fatal(err)
			}
		}, []string{"learnSkillId"}},
		{"format not in my profile", func(t *testing.T, f *fixture, _ *NewRequest) {
			if err := f.store.UpdateProfile(context.Background(), f.olha, data.ProfileUpdate{FirstName: "O", LastName: "T",
				City: "Київ", Formats: []string{data.FormatOffline}}); err != nil {
				t.Fatal(err)
			}
		}, []string{"format"}},
		{"format not in the recipient's profile", func(t *testing.T, f *fixture, in *NewRequest) {
			in.Format = data.FormatOffline
			if err := f.store.UpdateProfile(context.Background(), f.andrii, data.ProfileUpdate{FirstName: "A", LastName: "T",
				City: "Київ", Formats: []string{data.FormatOnline}}); err != nil {
				t.Fatal(err)
			}
		}, []string{"format"}},
		{"offline in different cities", func(t *testing.T, f *fixture, in *NewRequest) {
			in.Format = data.FormatOffline
			if err := f.store.UpdateProfile(context.Background(), f.andrii, data.ProfileUpdate{FirstName: "A", LastName: "T",
				City: "Львів", Formats: both}); err != nil {
				t.Fatal(err)
			}
		}, []string{"format"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			in := f.request()
			test.setup(t, f, &in)
			_, err := f.service.CreateRequest(context.Background(), f.olha, in)
			fields := fieldErrors(t, err)
			expect.Equal(t, "field errors", len(fields), len(test.fields))
			for _, field := range test.fields {
				expect.Equal(t, field+" has an error", fields[field] != "", true)
			}
		})
	}

	f := newFixture(t)
	in := f.request()
	in.Format = data.FormatOffline
	if _, err := f.service.CreateRequest(context.Background(), f.olha, in); err != nil {
		t.Fatalf("offline in the same city: err = %v", err)
	}
}

func TestCreateRequestDuplicate(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	first, err := f.service.CreateRequest(ctx, f.olha, f.request())
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.service.CreateRequest(ctx, f.olha, f.request())
	expect.Error(t, "the same request again", err, ErrDuplicateRequest)
	back := NewRequest{RecipientID: f.olha, TeachSkillID: datatest.PhotoshopSkillID, LearnSkillID: datatest.GuitarSkillID,
		Format: data.FormatOnline, TeachSessions: 1, TeachDurationMinutes: 60, LearnSessions: 1, LearnDurationMinutes: 60}
	_, err = f.service.CreateRequest(ctx, f.andrii, back)
	expect.Error(t, "the reverse request", err, ErrDuplicateRequest)

	f.store.SetRequestStatus(first.ID, "DECLINED")
	if _, err := f.service.CreateRequest(ctx, f.andrii, back); err != nil {
		t.Fatalf("after the answer: err = %v", err)
	}
}

func TestRequests(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	marko := f.student(t, "marko@students.example.test", datatest.DemoUniversityID, "Київ", []string{data.FormatOnline},
		datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	toAndrii, err := f.service.CreateRequest(ctx, f.olha, f.request())
	if err != nil {
		t.Fatal(err)
	}
	in := f.request()
	in.RecipientID = marko
	toMarko, err := f.service.CreateRequest(ctx, f.olha, in)
	if err != nil {
		t.Fatal(err)
	}
	f.store.SetRequestStatus(toAndrii.ID, data.RequestDeclined)

	ids := func(user string, filter data.RequestFilter, page int) ([]string, int) {
		t.Helper()
		result, err := f.service.Requests(ctx, user, filter, page)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, r := range result.Items {
			ids = append(ids, r.ID)
		}
		return ids, result.Total
	}
	outgoing := data.RequestFilter{Direction: data.Outgoing}
	got, total := ids(f.olha, outgoing, 1)
	expect.Slice(t, "Olha's sent requests", got, []string{toMarko.ID, toAndrii.ID})
	expect.Equal(t, "total of Olha's sent requests", total, 2)

	got, total = ids(f.olha, outgoing, 2)
	expect.Equal(t, "Olha's sent requests on page 2", len(got), 0)
	expect.Equal(t, "total on page 2", total, 2)

	got, _ = ids(f.olha, data.RequestFilter{Direction: data.Incoming}, 1)
	expect.Equal(t, "Olha's incoming requests", len(got), 0)

	incoming := data.RequestFilter{Direction: data.Incoming, Status: data.RequestDeclined}
	got, total = ids(f.andrii, incoming, 1)
	expect.Slice(t, "Andrii's declined requests", got, []string{toAndrii.ID})
	expect.Equal(t, "total of Andrii's declined requests", total, 1)

	incoming.Status = data.RequestPending
	got, _ = ids(f.andrii, incoming, 1)
	expect.Equal(t, "Andrii's pending requests", len(got), 0)
	got, _ = ids(marko, incoming, 1)
	expect.Slice(t, "Marko's pending requests", got, []string{toMarko.ID})
}

func TestRequestsValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for _, test := range []struct {
		filter data.RequestFilter
		field  string
	}{
		{data.RequestFilter{}, "direction"},
		{data.RequestFilter{Direction: "sent"}, "direction"},
		{data.RequestFilter{Direction: data.Incoming, Status: "EXPIRED"}, "status"},
		{data.RequestFilter{Direction: data.Incoming, Status: "pending"}, "status"},
	} {
		_, err := f.service.Requests(ctx, f.olha, test.filter, 1)
		fields := fieldErrors(t, err)
		name := fmt.Sprintf("filter %+v", test.filter)
		expect.Equal(t, name+": "+test.field+" has an error", fields[test.field] != "", true)
		expect.Equal(t, name+": field errors", len(fields), 1)
	}
	for _, page := range []int{0, MaxPage + 1} {
		_, err := f.service.Requests(ctx, f.olha, data.RequestFilter{Direction: data.Incoming}, page)
		expect.Error(t, fmt.Sprintf("page %d", page), err, ErrInvalidPage)
	}
}

func TestRequestDetails(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	created, err := f.service.CreateRequest(ctx, f.olha, f.request())
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range []struct{ name, id string }{{"Olha", f.olha}, {"Andrii", f.andrii}} {
		details, err := f.service.RequestDetails(ctx, user.id, strings.ToUpper(created.ID))
		if err != nil {
			t.Fatal(err)
		}
		expect.Equal(t, user.name+": request", details.Request.ID, created.ID)
		expect.Equal(t, user.name+": history items", len(details.History), 1)
		h := details.History[0]
		expect.Equal(t, user.name+": history status", h.Status, data.RequestPending)
		expect.Equal(t, user.name+": changed by", h.ChangedByID, f.olha)
		expect.Equal(t, user.name+": history time is set", !h.CreatedAt.IsZero(), true)
	}

	stranger := f.student(t, "marko@students.example.test", datatest.DemoUniversityID, "Київ", []string{data.FormatOnline},
		datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	for _, test := range []struct{ name, user, id string }{
		{"not a participant", stranger, created.ID},
		{"unknown request", f.olha, "70000000-0000-0000-0000-000000000999"},
		{"not a UUID", f.olha, "request"},
	} {
		_, err := f.service.RequestDetails(ctx, test.user, test.id)
		expect.Error(t, test.name, err, ErrRequestNotFound)
	}
}

func TestAnswerRequestMatrix(t *testing.T) {
	type action func(s *Service, ctx context.Context, userID, requestID string) (data.ExchangeRequest, error)
	decline, withdraw := (*Service).DeclineRequest, (*Service).WithdrawRequest
	for _, test := range []struct {
		name    string
		byOlha  bool // Olha sent the request; Andrii received it
		act     action
		status  string
		want    error
		changed string // status after the action; empty when the action fails
	}{
		{"recipient declines a pending request", false, decline, data.RequestPending, nil, data.RequestDeclined},
		{"recipient declines again", false, decline, data.RequestDeclined, nil, data.RequestDeclined},
		{"recipient declines a withdrawn request", false, decline, data.RequestWithdrawn, ErrRequestNotPending, ""},
		{"recipient declines an accepted request", false, decline, data.RequestAccepted, ErrRequestNotPending, ""},
		{"requester declines", true, decline, data.RequestPending, ErrActionNotAllowed, ""},
		{"requester withdraws a pending request", true, withdraw, data.RequestPending, nil, data.RequestWithdrawn},
		{"requester withdraws again", true, withdraw, data.RequestWithdrawn, nil, data.RequestWithdrawn},
		{"requester withdraws a declined request", true, withdraw, data.RequestDeclined, ErrRequestNotPending, ""},
		{"requester withdraws an accepted request", true, withdraw, data.RequestAccepted, ErrRequestNotPending, ""},
		{"recipient withdraws", false, withdraw, data.RequestPending, ErrActionNotAllowed, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			f := newFixture(t)
			created, err := f.service.CreateRequest(ctx, f.olha, f.request())
			if err != nil {
				t.Fatal(err)
			}
			f.store.SetRequestStatus(created.ID, test.status)
			user := f.andrii
			if test.byOlha {
				user = f.olha
			}
			answered, err := test.act(f.service, ctx, user, strings.ToUpper(created.ID))
			expect.Error(t, "answer the request", err, test.want)
			stored, _ := f.store.RequestByID(ctx, created.ID)
			if test.want != nil {
				expect.Equal(t, "stored status after a failed answer", stored.Status, test.status)
				return
			}
			expect.Equal(t, "answered request", answered.ID, created.ID)
			expect.Equal(t, "answered status", answered.Status, test.changed)
			expect.Equal(t, "stored status", stored.Status, test.changed)
			history, _ := f.store.RequestHistory(ctx, created.ID)
			if test.status == test.changed {
				// A repeated action adds no history and keeps the request as it was.
				expect.Equal(t, "history items after a repeat", len(history), 1)
				expect.Equal(t, "respondedAt is set after a repeat", answered.RespondedAt != nil, false)
				return
			}
			expect.Equal(t, "respondedAt is set", answered.RespondedAt != nil, true)
			expect.Equal(t, "history items", len(history), 2)
			last := history[1]
			expect.Equal(t, "last change status", last.Status, test.changed)
			expect.Equal(t, "last change by", last.ChangedByID, user)
			expect.Equal(t, "first name of who changed it is set", last.ChangedByFirstName != "", true)
		})
	}
}

func TestAnswerRequestNotFound(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	created, err := f.service.CreateRequest(ctx, f.olha, f.request())
	if err != nil {
		t.Fatal(err)
	}
	stranger := f.student(t, "marko@students.example.test", datatest.DemoUniversityID, "Київ", []string{data.FormatOnline},
		datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	for _, test := range []struct{ name, user, id string }{
		{"not a participant", stranger, created.ID},
		{"unknown request", f.andrii, "70000000-0000-0000-0000-000000000999"},
		{"not a UUID", f.andrii, "request"},
	} {
		_, err := f.service.DeclineRequest(ctx, test.user, test.id)
		expect.Error(t, test.name+": decline", err, ErrRequestNotFound)
		_, err = f.service.WithdrawRequest(ctx, test.user, test.id)
		expect.Error(t, test.name+": withdraw", err, ErrRequestNotFound)
	}
	r, _ := f.store.RequestByID(ctx, created.ID)
	expect.Equal(t, "status", r.Status, data.RequestPending)
}

func TestDeclinedRequestFreesSkillPair(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	created, err := f.service.CreateRequest(ctx, f.olha, f.request())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.DeclineRequest(ctx, f.andrii, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.CreateRequest(ctx, f.olha, f.request()); err != nil {
		t.Fatalf("a new request after the decline: err = %v", err)
	}
}

func TestAcceptRequest(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	created, err := f.service.CreateRequest(ctx, f.olha, f.request())
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := f.service.AcceptRequest(ctx, f.andrii, strings.ToUpper(created.ID))
	if err != nil {
		t.Fatal(err)
	}
	r, e := accepted.Request, accepted.Exchange
	expect.Equal(t, "request", r.ID, created.ID)
	expect.Equal(t, "request status", r.Status, data.RequestAccepted)
	expect.Equal(t, "respondedAt is set", r.RespondedAt != nil, true)
	expect.Equal(t, "request's exchange ID is set", r.ExchangeID != "", true)
	expect.Equal(t, "request's exchange", r.ExchangeID, e.ID)

	expect.Equal(t, "exchange's request", e.RequestID, created.ID)
	expect.Equal(t, "exchange status", e.Status, data.ExchangeActive)
	expect.Equal(t, "exchange format", e.Format, created.Format)
	expect.Equal(t, "exchange total sessions", e.TotalSessions, 3)
	expect.Equal(t, "exchange requester", e.Requester.UserID, f.olha)
	expect.Equal(t, "exchange recipient", e.Recipient.UserID, f.andrii)
	expect.Equal(t, "startedAt is set", e.StartedAt != nil, true)
	expect.Equal(t, "what the requester teaches", e.RequesterTeaches, created.RequesterTeaches)
	expect.Equal(t, "what the recipient teaches", e.RecipientTeaches, created.RecipientTeaches)

	history, _ := f.store.RequestHistory(ctx, created.ID)
	expect.Equal(t, "history items", len(history), 2)
	expect.Equal(t, "last history status", history[1].Status, data.RequestAccepted)
	expect.Equal(t, "accepted by", history[1].ChangedByID, f.andrii)

	// Accepting again, e.g. after a lost response, returns the same exchange.
	again, err := f.service.AcceptRequest(ctx, f.andrii, created.ID)
	expect.NoError(t, "accept again", err)
	expect.Equal(t, "exchange after a repeat", again.Exchange.ID, e.ID)
	expect.Equal(t, "respondedAt is set after a repeat", again.Request.RespondedAt != nil, true)
	expect.Equal(t, "respondedAt is unchanged after a repeat", again.Request.RespondedAt.Equal(*r.RespondedAt), true)
	history, _ = f.store.RequestHistory(ctx, created.ID)
	expect.Equal(t, "history items after a repeat", len(history), 2)

	_, err = f.service.DeclineRequest(ctx, f.andrii, created.ID)
	expect.Error(t, "decline an accepted request", err, ErrRequestNotPending)

	conv, err := f.comm.ExchangeConversation(ctx, f.olha, e.ID)
	expect.NoError(t, "load the exchange conversation", err)
	expect.Equal(t, "conversation exchange", conv.ExchangeID, e.ID)
	expect.Equal(t, "conversation request", conv.RequestID, created.ID)
	expect.Equal(t, "conversation type", conv.Type, data.ConversationExchange)

	notifs, err := f.comm.Notifications(ctx, f.olha, true, 1)
	expect.NoError(t, "load Olha's notifications", err)
	expect.Equal(t, "Olha's notifications", notifs.Total, 1)
	expect.Equal(t, "Olha's notification type", notifs.Items[0].EventType, data.EventRequestResponse)
	expect.Equal(t, "Olha's notification exchange", notifs.Items[0].ExchangeID, e.ID)
}

func TestAcceptRequestErrors(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name   string
		byOlha bool
		status string
		want   error
	}{
		{"requester accepts", true, data.RequestPending, ErrActionNotAllowed},
		{"declined request", false, data.RequestDeclined, ErrRequestNotPending},
		{"withdrawn request", false, data.RequestWithdrawn, ErrRequestNotPending},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			created, err := f.service.CreateRequest(ctx, f.olha, f.request())
			if err != nil {
				t.Fatal(err)
			}
			f.store.SetRequestStatus(created.ID, test.status)
			user := f.andrii
			if test.byOlha {
				user = f.olha
			}
			_, err = f.service.AcceptRequest(ctx, user, created.ID)
			expect.Error(t, "accept the request", err, test.want)
			r, _ := f.store.RequestByID(ctx, created.ID)
			expect.Equal(t, "status after a failed accept", r.Status, test.status)
			expect.Equal(t, "exchange ID after a failed accept", r.ExchangeID, "")
		})
	}

	f := newFixture(t)
	created, err := f.service.CreateRequest(ctx, f.olha, f.request())
	if err != nil {
		t.Fatal(err)
	}
	stranger := f.student(t, "marko@students.example.test", datatest.DemoUniversityID, "Київ", []string{data.FormatOnline},
		datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	for _, test := range []struct{ name, user, id string }{
		{"not a participant", stranger, created.ID},
		{"unknown request", f.andrii, "70000000-0000-0000-0000-000000000999"},
		{"not a UUID", f.andrii, "request"},
	} {
		_, err := f.service.AcceptRequest(ctx, test.user, test.id)
		expect.Error(t, test.name, err, ErrRequestNotFound)
	}

	// An outdated request stays pending, so its author can still withdraw it.
	if err := f.store.RemoveUserSkill(ctx, data.TeachingList, f.olha, datatest.GuitarSkillID); err != nil {
		t.Fatal(err)
	}
	_, err = f.service.AcceptRequest(ctx, f.andrii, created.ID)
	expect.Error(t, "a removed skill", err, ErrRequestOutdated)
	r, _ := f.store.RequestByID(ctx, created.ID)
	expect.Equal(t, "outdated request status", r.Status, data.RequestPending)
	expect.Equal(t, "outdated request exchange ID", r.ExchangeID, "")
	if _, err := f.service.WithdrawRequest(ctx, f.olha, created.ID); err != nil {
		t.Fatalf("withdraw an outdated request: err = %v", err)
	}
}

func TestExchange(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	created, err := f.service.CreateRequest(ctx, f.olha, f.request())
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := f.service.AcceptRequest(ctx, f.andrii, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{f.olha, f.andrii} {
		details, err := f.service.ExchangeDetails(ctx, user, strings.ToUpper(accepted.Exchange.ID))
		if err != nil {
			t.Fatal(err)
		}
		expect.Equal(t, "exchange", details.Exchange.ID, accepted.Exchange.ID)
		expect.Equal(t, "history items", len(details.History), 1)
		expect.Equal(t, "first history status", details.History[0].Status, data.ExchangeActive)
		expect.Equal(t, "accepted by", details.History[0].ChangedByID, f.andrii)
	}
	stranger := f.student(t, "marko@students.example.test", datatest.DemoUniversityID, "Київ", []string{data.FormatOnline},
		datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	for _, test := range []struct{ name, user, id string }{
		{"not a participant", stranger, accepted.Exchange.ID},
		{"unknown exchange", f.olha, "80000000-0000-0000-0000-000000000999"},
		{"not a UUID", f.olha, "exchange"},
	} {
		_, err := f.service.ExchangeDetails(ctx, test.user, test.id)
		expect.Error(t, test.name, err, ErrExchangeNotFound)
	}
}

func TestExchangesListAndFilters(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	marko := f.student(t, "marko@students.example.test", datatest.DemoUniversityID, "Київ", []string{data.FormatOnline},
		datatest.PhotoshopSkillID, datatest.GuitarSkillID)

	req1, err := f.service.CreateRequest(ctx, f.olha, f.request())
	if err != nil {
		t.Fatal(err)
	}
	acc1, err := f.service.AcceptRequest(ctx, f.andrii, req1.ID)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "initial total sessions", acc1.Exchange.Progress.TotalSessions, 3)
	expect.Equal(t, "initial draft sessions", acc1.Exchange.Progress.DraftSessions, 3)

	in2 := f.request()
	in2.RecipientID = marko
	req2, err := f.service.CreateRequest(ctx, f.olha, in2)
	if err != nil {
		t.Fatal(err)
	}
	acc2, err := f.service.AcceptRequest(ctx, marko, req2.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.store.SetExchangeStatus(acc1.Exchange.ID, data.ExchangeCompleted)

	// ACTIVE exchanges come first; status filter narrows the list.
	all, err := f.service.Exchanges(ctx, f.olha, data.ExchangeFilter{}, 1)
	expect.NoError(t, "list all exchanges", err)
	expect.Equal(t, "total exchanges", all.Total, 2)
	expect.Equal(t, "listed exchanges", len(all.Items), 2)
	expect.Equal(t, "first exchange", all.Items[0].ID, acc2.Exchange.ID)
	expect.Equal(t, "second exchange", all.Items[1].ID, acc1.Exchange.ID)

	activeOnly, err := f.service.Exchanges(ctx, f.olha, data.ExchangeFilter{Status: data.ExchangeActive}, 1)
	expect.NoError(t, "list active exchanges", err)
	expect.Equal(t, "total active exchanges", activeOnly.Total, 1)
	expect.Equal(t, "active exchange", activeOnly.Items[0].ID, acc2.Exchange.ID)

	completedOnly, err := f.service.Exchanges(ctx, f.olha, data.ExchangeFilter{Status: data.ExchangeCompleted}, 1)
	expect.NoError(t, "list completed exchanges", err)
	expect.Equal(t, "total completed exchanges", completedOnly.Total, 1)
	expect.Equal(t, "completed exchange", completedOnly.Items[0].ID, acc1.Exchange.ID)

	if _, err := f.service.Exchanges(ctx, f.olha, data.ExchangeFilter{Status: "PAUSED"}, 1); fieldErrors(t, err)["status"] == "" {
		t.Fatalf("invalid status: err = %v", err)
	}
	for _, p := range []int{0, MaxPage + 1} {
		_, err := f.service.Exchanges(ctx, f.olha, data.ExchangeFilter{}, p)
		expect.Error(t, fmt.Sprintf("page %d", p), err, ErrInvalidPage)
	}
}
