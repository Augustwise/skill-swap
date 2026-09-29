package profile

import (
	"context"
	"errors"
	"strings"
	"testing"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
)

// newTestService creates a profile service on an in-memory store with one demo
// user, and returns the service and that user's ID.
func newTestService(t *testing.T) (*Service, string) {
	t.Helper()
	store := datatest.NewMemory()
	userID, err := store.CreateUser(context.Background(), data.NewUser{
		UniversityID: datatest.DemoUniversityID, Email: "a@students.example.test", FirstName: "Олена", LastName: "Коваль",
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewService(store, store, store), userID
}

// ptr returns a pointer to value. Patch fields are pointers, so nil means "not sent".
func ptr[T any](value T) *T {
	return &value
}

// fieldErrors checks that err is a ValidationError and returns its per-field messages.
func fieldErrors(t *testing.T, err error) map[string]string {
	t.Helper()
	var invalid *ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	return invalid.Fields
}

// A profile update trims spaces, removes duplicate formats and orders them
// (ONLINE first). A later update changes only the fields it sends.
func TestUpdateProfileSavesAndNormalizes(t *testing.T) {
	s, userID := newTestService(t)
	// Send messy input: extra spaces and a duplicate format.
	updated, err := s.UpdateProfile(context.Background(), userID, ProfilePatch{
		FirstName: ptr("  Марія "),
		FacultyID: ptr(datatest.DemoFacultyID),
		Course:    ptr(3),
		City:      ptr(" Київ "),
		Bio:       ptr("Граю на гітарі.\nХочу вивчити Photoshop."),
		Formats:   ptr([]string{"OFFLINE", "ONLINE", "ONLINE"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.FirstName != "Марія" || updated.LastName != "Коваль" || updated.City != "Київ" || updated.Course != 3 {
		t.Fatalf("unexpected profile: %+v", updated)
	}
	if updated.FacultyID != datatest.DemoFacultyID || updated.FacultyName == "" {
		t.Fatalf("faculty = %q %q", updated.FacultyID, updated.FacultyName)
	}
	if strings.Join(updated.Formats, ",") != "ONLINE,OFFLINE" {
		t.Fatalf("formats = %v", updated.Formats)
	}

	// Partial update: an empty faculty and course 0 clear those fields; name and city stay.
	updated, err = s.UpdateProfile(context.Background(), userID, ProfilePatch{
		FacultyID: ptr(""), Course: ptr(0), Formats: ptr([]string{"ONLINE"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.FirstName != "Марія" || updated.City != "Київ" || updated.FacultyID != "" || updated.Course != 0 {
		t.Fatalf("unexpected profile after partial update: %+v", updated)
	}
}

// Each invalid patch must fail with an error for the expected field.
func TestUpdateProfileValidation(t *testing.T) {
	tests := []struct {
		name  string
		patch ProfilePatch
		field string
	}{
		{"empty first name", ProfilePatch{FirstName: ptr("  ")}, "firstName"},
		{"long last name", ProfilePatch{LastName: ptr(strings.Repeat("я", 101))}, "lastName"},
		{"control character in name", ProfilePatch{FirstName: ptr("Оле\x00на")}, "firstName"},
		{"faculty of another university", ProfilePatch{FacultyID: ptr(datatest.OtherFacultyID)}, "facultyId"},
		{"unknown faculty", ProfilePatch{FacultyID: ptr("not-a-uuid")}, "facultyId"},
		{"course 7", ProfilePatch{Course: ptr(7)}, "course"},
		{"negative course", ProfilePatch{Course: ptr(-1)}, "course"},
		{"long city", ProfilePatch{City: ptr(strings.Repeat("м", 101))}, "city"},
		{"601-character description", ProfilePatch{Bio: ptr(strings.Repeat("ї", 601))}, "bio"},
		{"unknown format", ProfilePatch{Formats: ptr([]string{"ONLINE", "CAMPUS"})}, "formats"},
		{"offline without city", ProfilePatch{Formats: ptr([]string{"OFFLINE"})}, "city"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, userID := newTestService(t)
			_, err := s.UpdateProfile(context.Background(), userID, test.patch)
			if fields := fieldErrors(t, err); fields[test.field] == "" {
				t.Fatalf("missing %s error in %v", test.field, fields)
			}
		})
	}
}

// The 600 limit counts characters, not bytes (Cyrillic letters take 2 bytes each).
func TestBioLimitCountsCharacters(t *testing.T) {
	s, userID := newTestService(t)
	bio := strings.Repeat("ї", 600)
	updated, err := s.UpdateProfile(context.Background(), userID, ProfilePatch{Bio: ptr(bio)})
	if err != nil {
		t.Fatalf("600 Cyrillic characters must be accepted: %v", err)
	}
	if updated.Bio != bio {
		t.Fatal("description was not saved")
	}
}

// If one field in a patch is invalid, nothing from that patch is saved,
// not even the valid fields.
func TestInvalidPatchDoesNotChangeProfile(t *testing.T) {
	s, userID := newTestService(t)
	// The city is valid, but course 9 is not.
	_, err := s.UpdateProfile(context.Background(), userID, ProfilePatch{City: ptr("Львів"), Course: ptr(9)})
	fieldErrors(t, err)
	current, err := s.Profile(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if current.City != "" || current.Course != 0 {
		t.Fatalf("invalid patch changed the profile: %+v", current)
	}
}

// Adding, changing and removing skills works on one list at a time; the
// teaching and learning lists do not affect each other.
func TestSkillLists(t *testing.T) {
	s, userID := newTestService(t)
	ctx := context.Background()

	updated, err := s.AddSkill(ctx, userID, data.TeachingList, datatest.GuitarSkillID, "ADVANCED")
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.TeachingSkills) != 1 || updated.TeachingSkills[0].Name != "Гітара" || updated.TeachingSkills[0].Level != "ADVANCED" {
		t.Fatalf("teaching skills = %+v", updated.TeachingSkills)
	}

	// The same skill can be in both lists, but not twice in one list.
	if _, err := s.AddSkill(ctx, userID, data.LearningList, datatest.GuitarSkillID, "BEGINNER"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddSkill(ctx, userID, data.TeachingList, datatest.GuitarSkillID, "BEGINNER"); !errors.Is(err, ErrSkillAlreadyAdded) {
		t.Fatalf("duplicate err = %v, want ErrSkillAlreadyAdded", err)
	}

	// Changing the learning level must not change the teaching level.
	updated, err = s.UpdateSkillLevel(ctx, userID, data.LearningList, datatest.GuitarSkillID, "INTERMEDIATE")
	if err != nil {
		t.Fatal(err)
	}
	if updated.LearningSkills[0].Level != "INTERMEDIATE" || updated.TeachingSkills[0].Level != "ADVANCED" {
		t.Fatalf("level change touched the wrong list: %+v", updated)
	}

	// Removing from teaching must keep the learning entry; removing twice fails.
	updated, err = s.RemoveSkill(ctx, userID, data.TeachingList, datatest.GuitarSkillID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.TeachingSkills) != 0 || len(updated.LearningSkills) != 1 {
		t.Fatalf("removal touched the other list: %+v", updated)
	}
	if _, err := s.RemoveSkill(ctx, userID, data.TeachingList, datatest.GuitarSkillID); !errors.Is(err, ErrSkillNotInList) {
		t.Fatalf("second removal err = %v, want ErrSkillNotInList", err)
	}
	if _, err := s.AddSkill(ctx, userID, data.TeachingList, datatest.GuitarSkillID, "INTERMEDIATE"); err != nil {
		t.Fatalf("adding a removed skill again: %v", err)
	}
}

// Skill operations return the right error for missing skills and invalid levels.
func TestSkillListErrors(t *testing.T) {
	s, userID := newTestService(t)
	ctx := context.Background()
	tests := []struct {
		name string
		call func() error
		want error
	}{
		{"inactive skill", func() error {
			_, err := s.AddSkill(ctx, userID, data.TeachingList, datatest.InactiveSkillID, "BEGINNER")
			return err
		}, ErrSkillNotFound},
		{"unknown skill", func() error {
			_, err := s.AddSkill(ctx, userID, data.TeachingList, "30000000-0000-0000-0000-000000009999", "BEGINNER")
			return err
		}, ErrSkillNotFound},
		{"malformed skill id", func() error {
			_, err := s.AddSkill(ctx, userID, data.LearningList, "guitar", "BEGINNER")
			return err
		}, ErrSkillNotFound},
		{"level of a skill not in the list", func() error {
			_, err := s.UpdateSkillLevel(ctx, userID, data.TeachingList, datatest.PhotoshopSkillID, "BEGINNER")
			return err
		}, ErrSkillNotInList},
		{"remove a skill not in the list", func() error {
			_, err := s.RemoveSkill(ctx, userID, data.LearningList, datatest.PhotoshopSkillID)
			return err
		}, ErrSkillNotInList},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, test.want) {
				t.Fatalf("err = %v, want %v", err, test.want)
			}
		})
	}

	// Levels must be exactly BEGINNER, INTERMEDIATE or ADVANCED (uppercase).
	for _, level := range []string{"", "EXPERT", "beginner"} {
		_, err := s.AddSkill(ctx, userID, data.TeachingList, datatest.GuitarSkillID, level)
		if fields := fieldErrors(t, err); fields["level"] == "" {
			t.Fatalf("level %q: missing level error", level)
		}
	}
}

// A profile can be matched only if it has at least one teaching skill, one
// learning skill and one format; OFFLINE also needs a city.
func TestEligibleForMatching(t *testing.T) {
	skill := []data.UserSkill{{SkillID: datatest.GuitarSkillID}}
	tests := []struct {
		name    string
		profile data.Profile
		want    bool
	}{
		{"empty teaching list", data.Profile{LearningSkills: skill, Formats: []string{"ONLINE"}}, false},
		{"empty learning list", data.Profile{TeachingSkills: skill, Formats: []string{"ONLINE"}}, false},
		{"no format", data.Profile{TeachingSkills: skill, LearningSkills: skill}, false},
		{"offline without city", data.Profile{TeachingSkills: skill, LearningSkills: skill, Formats: []string{"OFFLINE"}}, false},
		{"online", data.Profile{TeachingSkills: skill, LearningSkills: skill, Formats: []string{"ONLINE"}}, true},
		{"offline with city", data.Profile{TeachingSkills: skill, LearningSkills: skill, Formats: []string{"ONLINE", "OFFLINE"}, City: "Київ"}, true},
	}
	for _, test := range tests {
		if got := EligibleForMatching(test.profile); got != test.want {
			t.Errorf("%s: EligibleForMatching = %t, want %t", test.name, got, test.want)
		}
	}
}

// A malformed university ID (such as an SQL injection attempt) is treated as
// "not found"; a valid ID returns that university's faculties.
func TestFacultiesRejectMalformedUniversityID(t *testing.T) {
	s, _ := newTestService(t)
	if _, err := s.Faculties(context.Background(), "1 OR 1=1"); !errors.Is(err, ErrUniversityNotFound) {
		t.Fatalf("err = %v, want ErrUniversityNotFound", err)
	}
	faculties, err := s.Faculties(context.Background(), datatest.DemoUniversityID)
	if err != nil || len(faculties) != 1 {
		t.Fatalf("faculties = %v, err = %v", faculties, err)
	}
}
