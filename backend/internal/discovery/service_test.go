package discovery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/expect"
	"skillswap/backend/internal/profile"
)

// recordingData returns no results and remembers the last request.
type recordingData struct {
	calls         int
	limit, offset int
	filter        data.StudentFilter
}

func (r *recordingData) MutualMatches(_ context.Context, _ string, limit, offset int) ([]data.Match, int, error) {
	r.calls++
	r.limit, r.offset = limit, offset
	return []data.Match{}, 45, nil
}

func (r *recordingData) SearchStudents(_ context.Context, _ string, filter data.StudentFilter, limit, offset int) ([]data.StudentCard, int, error) {
	r.calls++
	r.filter, r.limit, r.offset = filter, limit, offset
	return []data.StudentCard{}, 0, nil
}

func (r *recordingData) StudentProfile(context.Context, string, string) (data.StudentProfile, error) {
	r.calls++
	return data.StudentProfile{}, data.ErrNotFound
}

func newTestService(t *testing.T, eligible bool) (*Service, *recordingData, string) {
	t.Helper()
	ctx := context.Background()
	store := datatest.NewMemory()
	userID, err := store.CreateUser(ctx, data.NewUser{
		UniversityID: datatest.DemoUniversityID, Email: "a@students.example.test", FirstName: "Ольга", LastName: "Гнатюк",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateProfile(ctx, userID, data.ProfileUpdate{FirstName: "Ольга", LastName: "Гнатюк",
		Formats: []string{data.FormatOnline}}); err != nil {
		t.Fatal(err)
	}
	if err := store.AddUserSkill(ctx, data.TeachingList, userID, datatest.GuitarSkillID, "ADVANCED"); err != nil {
		t.Fatal(err)
	}
	if eligible {
		if err := store.AddUserSkill(ctx, data.LearningList, userID, datatest.PhotoshopSkillID, "BEGINNER"); err != nil {
			t.Fatal(err)
		}
	}
	matches := &recordingData{}
	return NewService(store, matches), matches, userID
}

func TestMutualMatchesPageWindow(t *testing.T) {
	service, matches, userID := newTestService(t, true)
	result, err := service.MutualMatches(context.Background(), userID, 3)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "limit", matches.limit, PageSize)
	expect.Equal(t, "offset", matches.offset, 2*PageSize)
	expect.Equal(t, "eligible for matching", result.Eligible, true)
	expect.Equal(t, "page", result.Page, 3)
	expect.Equal(t, "total", result.Total, 45)
	expect.Equal(t, "items are set", result.Items != nil, true)
}

// An incomplete own profile gives an empty list without querying matches.
func TestMutualMatchesSkipsIncompleteProfile(t *testing.T) {
	service, matches, userID := newTestService(t, false)
	result, err := service.MutualMatches(context.Background(), userID, 1)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "match queries", matches.calls, 0)
	expect.Equal(t, "eligible for matching", result.Eligible, false)
	expect.Equal(t, "total", result.Total, 0)
	expect.Equal(t, "items are set", result.Items != nil, true)
	expect.Equal(t, "items", len(result.Items), 0)
}

func TestMutualMatchesRejectsInvalidPage(t *testing.T) {
	service, matches, userID := newTestService(t, true)
	for _, page := range []int{0, -1, MaxPage + 1} {
		_, err := service.MutualMatches(context.Background(), userID, page)
		expect.Error(t, fmt.Sprintf("page %d", page), err, ErrInvalidPage)
	}
	expect.Equal(t, "match queries", matches.calls, 0)
}

func TestSearchStudentsNormalizesFilter(t *testing.T) {
	service, search, userID := newTestService(t, true)
	filter := data.StudentFilter{Query: "  Photoshop ", CategoryID: "20000000-0000-0000-0000-00000000000A",
		Level: "ADVANCED", Format: data.FormatOnline, MutualOnly: true}
	result, err := service.SearchStudents(context.Background(), userID, filter, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := data.StudentFilter{Query: "Photoshop", CategoryID: "20000000-0000-0000-0000-00000000000a",
		Level: "ADVANCED", Format: data.FormatOnline, MutualOnly: true}
	expect.Equal(t, "normalized filter", search.filter, want)
	expect.Equal(t, "limit", search.limit, PageSize)
	expect.Equal(t, "offset", search.offset, PageSize)
	expect.Equal(t, "page", result.Page, 2)
}

func TestSearchStudentsRejectsInvalidFilters(t *testing.T) {
	service, search, userID := newTestService(t, true)
	_, err := service.SearchStudents(context.Background(), userID, data.StudentFilter{
		Query: strings.Repeat("я", 101), CategoryID: "design", Level: "EXPERT", Format: "CAMPUS",
	}, 1)
	var invalid *profile.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v", err)
	}
	for _, field := range []string{"q", "categoryId", "level", "format"} {
		if invalid.Fields[field] == "" {
			t.Fatalf("missing %s error: %v", field, invalid.Fields)
		}
	}
	_, err = service.SearchStudents(context.Background(), userID, data.StudentFilter{}, 0)
	expect.Error(t, "page 0", err, ErrInvalidPage)
	expect.Equal(t, "search queries", search.calls, 0)
}

func TestStudentProfileNotFound(t *testing.T) {
	service, profiles, userID := newTestService(t, true)
	_, err := service.StudentProfile(context.Background(), userID, "not-a-uuid")
	expect.Error(t, "profile with an invalid ID", err, ErrStudentNotFound)
	expect.Equal(t, "profile queries", profiles.calls, 0)

	_, err = service.StudentProfile(context.Background(), userID, "60000000-0000-0000-0000-000000000099")
	expect.Error(t, "profile of a hidden student", err, ErrStudentNotFound)
}
