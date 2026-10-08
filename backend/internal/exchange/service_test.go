package exchange

import (
	"context"
	"errors"
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
