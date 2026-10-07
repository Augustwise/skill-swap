package discovery

import (
	"context"
	"errors"
	"testing"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
)

// recordingData returns no matches and remembers the requested page window.
type recordingData struct {
	calls         int
	limit, offset int
}

func (r *recordingData) MutualMatches(_ context.Context, _ string, limit, offset int) ([]data.Match, int, error) {
	r.calls++
	r.limit, r.offset = limit, offset
	return []data.Match{}, 45, nil
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
	if matches.limit != PageSize || matches.offset != 2*PageSize {
		t.Fatalf("limit = %d, offset = %d", matches.limit, matches.offset)
	}
	if !result.Eligible || result.Page != 3 || result.Total != 45 || result.Items == nil {
		t.Fatalf("result = %+v", result)
	}
}

// An incomplete own profile gives an empty list without querying matches.
func TestMutualMatchesSkipsIncompleteProfile(t *testing.T) {
	service, matches, userID := newTestService(t, false)
	result, err := service.MutualMatches(context.Background(), userID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if matches.calls != 0 || result.Eligible || result.Total != 0 || result.Items == nil || len(result.Items) != 0 {
		t.Fatalf("calls = %d, result = %+v", matches.calls, result)
	}
}

func TestMutualMatchesRejectsInvalidPage(t *testing.T) {
	service, matches, userID := newTestService(t, true)
	for _, page := range []int{0, -1, MaxPage + 1} {
		if _, err := service.MutualMatches(context.Background(), userID, page); !errors.Is(err, ErrInvalidPage) {
			t.Fatalf("page %d: err = %v", page, err)
		}
	}
	if matches.calls != 0 {
		t.Fatalf("calls = %d", matches.calls)
	}
}
