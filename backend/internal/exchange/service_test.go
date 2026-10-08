package exchange

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/profile"
)

type fixture struct {
	service *Service
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
	f := &fixture{service: NewService(store, store, store), store: store}
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
	if created.ID == "" || created.Status != data.RequestPending || created.Format != data.FormatOnline ||
		created.TotalSessions != 3 || created.Message != "Привіт! Давай обміняємось." || created.ExchangeID != "" {
		t.Fatalf("request = %+v", created)
	}
	if created.Requester.UserID != f.olha || created.Recipient.UserID != f.andrii {
		t.Fatalf("requester = %+v, recipient = %+v", created.Requester, created.Recipient)
	}
	want := data.RequestTerms{SkillID: datatest.GuitarSkillID, CategoryID: "20000000-0000-0000-0000-000000000001",
		Name: "Гітара", TeacherLevel: "ADVANCED", LearnerLevel: "BEGINNER", Sessions: 2, DurationMinutes: 60}
	if created.RequesterTeaches != want {
		t.Fatalf("requester teaches %+v", created.RequesterTeaches)
	}
	if got := created.RecipientTeaches; got.SkillID != datatest.PhotoshopSkillID || got.Sessions != 1 || got.DurationMinutes != 120 {
		t.Fatalf("recipient teaches %+v", got)
	}
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
			if fields := fieldErrors(t, err); fields[test.field] == "" || len(fields) != 1 {
				t.Fatalf("fields = %v, want only %s", fields, test.field)
			}
		})
	}
}

func TestCreateRequestRecipientRules(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	unknown := f.request()
	unknown.RecipientID = "40000000-0000-0000-0000-000000000999"
	if _, err := f.service.CreateRequest(ctx, f.olha, unknown); !errors.Is(err, ErrStudentNotFound) {
		t.Fatalf("unknown student: err = %v", err)
	}
	unverified, err := f.store.CreateUser(ctx, data.NewUser{UniversityID: datatest.DemoUniversityID,
		Email: "pending@students.example.test", FirstName: "P", LastName: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	in := f.request()
	in.RecipientID = unverified
	if _, err := f.service.CreateRequest(ctx, f.olha, in); !errors.Is(err, ErrStudentNotFound) {
		t.Fatalf("unverified student: err = %v", err)
	}

	f.store.SetRequestSettings(f.andrii, false, false)
	if _, err := f.service.CreateRequest(ctx, f.olha, f.request()); !errors.Is(err, ErrRequestsClosed) {
		t.Fatalf("closed: err = %v", err)
	}
	f.store.SetRequestSettings(f.andrii, true, true)
	if _, err := f.service.CreateRequest(ctx, f.olha, f.request()); err != nil {
		t.Fatalf("same university: err = %v", err)
	}

	other := f.student(t, "other@other.example.test", datatest.OtherUniversityID, "Київ", []string{data.FormatOnline},
		datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	f.store.SetRequestSettings(other, true, true)
	in = f.request()
	in.RecipientID = other
	if _, err := f.service.CreateRequest(ctx, f.olha, in); !errors.Is(err, ErrSameUniversityOnly) {
		t.Fatalf("other university: err = %v", err)
	}
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
			if len(fields) != len(test.fields) {
				t.Fatalf("fields = %v, want %v", fields, test.fields)
			}
			for _, field := range test.fields {
				if fields[field] == "" {
					t.Fatalf("fields = %v, want %v", fields, test.fields)
				}
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
	if _, err := f.service.CreateRequest(ctx, f.olha, f.request()); !errors.Is(err, ErrDuplicateRequest) {
		t.Fatalf("the same request again: err = %v", err)
	}
	back := NewRequest{RecipientID: f.olha, TeachSkillID: datatest.PhotoshopSkillID, LearnSkillID: datatest.GuitarSkillID,
		Format: data.FormatOnline, TeachSessions: 1, TeachDurationMinutes: 60, LearnSessions: 1, LearnDurationMinutes: 60}
	if _, err := f.service.CreateRequest(ctx, f.andrii, back); !errors.Is(err, ErrDuplicateRequest) {
		t.Fatalf("the reverse request: err = %v", err)
	}

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
	if got, total := ids(f.olha, outgoing, 1); !slices.Equal(got, []string{toMarko.ID, toAndrii.ID}) || total != 2 {
		t.Fatalf("Olha's sent requests = %v, total %d", got, total)
	}
	if got, total := ids(f.olha, outgoing, 2); len(got) != 0 || total != 2 {
		t.Fatalf("page 2 = %v, total %d", got, total)
	}
	if got, _ := ids(f.olha, data.RequestFilter{Direction: data.Incoming}, 1); len(got) != 0 {
		t.Fatalf("Olha's incoming requests = %v", got)
	}
	incoming := data.RequestFilter{Direction: data.Incoming, Status: data.RequestDeclined}
	if got, total := ids(f.andrii, incoming, 1); !slices.Equal(got, []string{toAndrii.ID}) || total != 1 {
		t.Fatalf("Andrii's declined requests = %v, total %d", got, total)
	}
	incoming.Status = data.RequestPending
	if got, _ := ids(f.andrii, incoming, 1); len(got) != 0 {
		t.Fatalf("Andrii's pending requests = %v", got)
	}
	if got, _ := ids(marko, incoming, 1); !slices.Equal(got, []string{toMarko.ID}) {
		t.Fatalf("Marko's pending requests = %v", got)
	}
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
		if fields := fieldErrors(t, err); fields[test.field] == "" || len(fields) != 1 {
			t.Fatalf("%+v: fields = %v, want only %s", test.filter, fields, test.field)
		}
	}
	for _, page := range []int{0, MaxPage + 1} {
		if _, err := f.service.Requests(ctx, f.olha, data.RequestFilter{Direction: data.Incoming}, page); !errors.Is(err, ErrInvalidPage) {
			t.Fatalf("page %d: err = %v", page, err)
		}
	}
}

func TestRequestDetails(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	created, err := f.service.CreateRequest(ctx, f.olha, f.request())
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{f.olha, f.andrii} {
		details, err := f.service.RequestDetails(ctx, user, strings.ToUpper(created.ID))
		if err != nil {
			t.Fatal(err)
		}
		if details.Request.ID != created.ID || len(details.History) != 1 {
			t.Fatalf("details = %+v", details)
		}
		if h := details.History[0]; h.Status != data.RequestPending || h.ChangedByID != f.olha || h.CreatedAt.IsZero() {
			t.Fatalf("history = %+v", h)
		}
	}

	stranger := f.student(t, "marko@students.example.test", datatest.DemoUniversityID, "Київ", []string{data.FormatOnline},
		datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	for _, test := range []struct{ name, user, id string }{
		{"not a participant", stranger, created.ID},
		{"unknown request", f.olha, "70000000-0000-0000-0000-000000000999"},
		{"not a UUID", f.olha, "request"},
	} {
		if _, err := f.service.RequestDetails(ctx, test.user, test.id); !errors.Is(err, ErrRequestNotFound) {
			t.Fatalf("%s: err = %v", test.name, err)
		}
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
			if !errors.Is(err, test.want) {
				t.Fatalf("err = %v, want %v", err, test.want)
			}
			stored, _ := f.store.RequestByID(ctx, created.ID)
			if test.want != nil {
				if stored.Status != test.status {
					t.Fatalf("status = %s, want it unchanged", stored.Status)
				}
				return
			}
			if answered.ID != created.ID || answered.Status != test.changed || stored.Status != test.changed {
				t.Fatalf("answered = %+v, stored status %s", answered, stored.Status)
			}
			history, _ := f.store.RequestHistory(ctx, created.ID)
			if test.status == test.changed {
				// A repeated action adds no history and keeps the request as it was.
				if len(history) != 1 || answered.RespondedAt != nil {
					t.Fatalf("history = %+v, responded at %v", history, answered.RespondedAt)
				}
				return
			}
			if answered.RespondedAt == nil || len(history) != 2 {
				t.Fatalf("responded at %v, history = %+v", answered.RespondedAt, history)
			}
			if h := history[1]; h.Status != test.changed || h.ChangedByID != user || h.ChangedByFirstName == "" {
				t.Fatalf("last change = %+v", h)
			}
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
		if _, err := f.service.DeclineRequest(ctx, test.user, test.id); !errors.Is(err, ErrRequestNotFound) {
			t.Fatalf("decline, %s: err = %v", test.name, err)
		}
		if _, err := f.service.WithdrawRequest(ctx, test.user, test.id); !errors.Is(err, ErrRequestNotFound) {
			t.Fatalf("withdraw, %s: err = %v", test.name, err)
		}
	}
	if r, _ := f.store.RequestByID(ctx, created.ID); r.Status != data.RequestPending {
		t.Fatalf("status = %s", r.Status)
	}
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
