package profile

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/validate"
)

const (
	maxCityLength = 100
	maxBioLength  = 600
	minCourse     = 1
	maxCourse     = 6
)

var (
	levels        = []string{"BEGINNER", "INTERMEDIATE", "ADVANCED"}
	formats       = []string{data.FormatOnline, data.FormatOffline}
	requestScopes = []string{data.ScopeAnyone, data.ScopeSameUniversity}
)

var (
	ErrUniversityNotFound = errors.New("university was not found")
	ErrSkillNotFound      = errors.New("skill is not in the catalog or is inactive")
	ErrSkillNotInList     = errors.New("skill is not in this list")
	ErrSkillAlreadyAdded  = errors.New("skill is already in this list")
)

type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid fields: %v", e.Fields)
}

type Service struct {
	catalog  data.ICatalogData
	profiles data.IProfileData
	tx       data.ITransaction
}

func NewService(catalog data.ICatalogData, profiles data.IProfileData, tx data.ITransaction) *Service {
	return &Service{catalog: catalog, profiles: profiles, tx: tx}
}

func (s *Service) Universities(ctx context.Context) ([]data.University, error) {
	return s.catalog.Universities(ctx)
}

func (s *Service) SkillCategories(ctx context.Context) ([]data.SkillCategory, error) {
	return s.catalog.SkillCategories(ctx)
}

func (s *Service) Skills(ctx context.Context, query string, limit int) ([]data.Skill, error) {
	return s.catalog.Skills(ctx, query, limit)
}

func (s *Service) Faculties(ctx context.Context, universityID string) ([]data.Faculty, error) {
	if !validate.UUID(universityID) {
		return nil, ErrUniversityNotFound
	}
	return s.profiles.FacultiesByUniversity(ctx, universityID)
}

func (s *Service) Profile(ctx context.Context, userID string) (data.Profile, error) {
	return s.profiles.ProfileByUserID(ctx, userID)
}

func EligibleForMatching(p data.Profile) bool {
	if len(p.TeachingSkills) == 0 || len(p.LearningSkills) == 0 || len(p.Formats) == 0 {
		return false
	}
	return !slices.Contains(p.Formats, data.FormatOffline) || p.City != ""
}

type ProfilePatch struct {
	FirstName    *string
	LastName     *string
	UniversityID *string
	FacultyID    *string
	Course       *int
	City         *string
	Bio          *string
	Formats      *[]string
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, patch ProfilePatch) (data.Profile, error) {
	var updated data.Profile
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.profiles.ProfileByUserID(ctx, userID)
		if err != nil {
			return err
		}
		update, err := s.validProfile(ctx, applyPatch(current, patch))
		if err != nil {
			return err
		}
		if err := s.profiles.UpdateProfile(ctx, userID, update); err != nil {
			return err
		}
		updated, err = s.profiles.ProfileByUserID(ctx, userID)
		return err
	})
	return updated, err
}

func applyPatch(current data.Profile, patch ProfilePatch) data.ProfileUpdate {
	update := data.ProfileUpdate{
		FirstName:    current.FirstName,
		LastName:     current.LastName,
		UniversityID: current.UniversityID,
		FacultyID:    current.FacultyID,
		Course:       current.Course,
		City:         current.City,
		Bio:          current.Bio,
		Formats:      current.Formats,
	}
	if patch.FirstName != nil {
		update.FirstName = *patch.FirstName
	}
	if patch.LastName != nil {
		update.LastName = *patch.LastName
	}
	if patch.UniversityID != nil && *patch.UniversityID != current.UniversityID {
		update.UniversityID = *patch.UniversityID
		// The old faculty belongs to the previous university.
		update.FacultyID = ""
	}
	if patch.FacultyID != nil {
		update.FacultyID = *patch.FacultyID
	}
	if patch.Course != nil {
		update.Course = *patch.Course
	}
	if patch.City != nil {
		update.City = *patch.City
	}
	if patch.Bio != nil {
		update.Bio = *patch.Bio
	}
	if patch.Formats != nil {
		update.Formats = *patch.Formats
	}
	return update
}

func (s *Service) validProfile(ctx context.Context, in data.ProfileUpdate) (data.ProfileUpdate, error) {
	fields := map[string]string{}
	out := data.ProfileUpdate{}
	var ok bool

	if out.FirstName, ok = validate.Name(in.FirstName); !ok {
		fields["firstName"] = "First name is required and must be at most 100 characters"
	}
	if out.LastName, ok = validate.Name(in.LastName); !ok {
		fields["lastName"] = "Last name is required and must be at most 100 characters"
	}
	if in.Course != 0 && (in.Course < minCourse || in.Course > maxCourse) {
		fields["course"] = "Course must be between 1 and 6"
	}
	out.Course = in.Course
	if out.City, ok = validate.Line(in.City, maxCityLength); !ok {
		fields["city"] = "City must be at most 100 characters"
	}
	if out.Bio, ok = validate.Text(in.Bio, maxBioLength); !ok {
		fields["bio"] = "Description must be at most 600 characters"
	}

	for _, format := range in.Formats {
		if !slices.Contains(formats, format) {
			fields["formats"] = "Formats may only be ONLINE and OFFLINE"
		}
	}
	for _, format := range formats {
		if slices.Contains(in.Formats, format) {
			out.Formats = append(out.Formats, format)
		}
	}
	if slices.Contains(out.Formats, data.FormatOffline) && out.City == "" && fields["city"] == "" {
		fields["city"] = "City is required for offline lessons"
	}

	universityOK, err := s.universityExists(ctx, in.UniversityID)
	if err != nil {
		return data.ProfileUpdate{}, err
	}
	if !universityOK {
		fields["universityId"] = "Choose a university from the list"
	}
	out.UniversityID = in.UniversityID

	if in.FacultyID != "" && universityOK {
		faculties, err := s.profiles.FacultiesByUniversity(ctx, in.UniversityID)
		if err != nil {
			return data.ProfileUpdate{}, err
		}
		if !slices.ContainsFunc(faculties, func(f data.Faculty) bool { return f.ID == in.FacultyID }) {
			fields["facultyId"] = "Choose a faculty of your university"
		}
		out.FacultyID = in.FacultyID
	}

	if len(fields) > 0 {
		return data.ProfileUpdate{}, &ValidationError{Fields: fields}
	}
	return out, nil
}

func (s *Service) universityExists(ctx context.Context, id string) (bool, error) {
	if !validate.UUID(id) {
		return false, nil
	}
	universities, err := s.catalog.Universities(ctx)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(universities, func(u data.University) bool { return u.ID == id }), nil
}

func (s *Service) AddSkill(ctx context.Context, userID string, list data.SkillList, skillID, level string) (data.Profile, error) {
	if err := validLevel(level); err != nil {
		return data.Profile{}, err
	}
	if !validate.UUID(skillID) {
		return data.Profile{}, ErrSkillNotFound
	}
	return s.changeList(ctx, userID, func(ctx context.Context) error {
		exists, err := s.profiles.ActiveSkillExists(ctx, skillID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrSkillNotFound
		}
		err = s.profiles.AddUserSkill(ctx, list, userID, skillID, level)
		if errors.Is(err, data.ErrSkillAlreadyAdded) {
			return ErrSkillAlreadyAdded
		}
		return err
	})
}

func (s *Service) UpdateSkillLevel(ctx context.Context, userID string, list data.SkillList, skillID, level string) (data.Profile, error) {
	if err := validLevel(level); err != nil {
		return data.Profile{}, err
	}
	if !validate.UUID(skillID) {
		return data.Profile{}, ErrSkillNotInList
	}
	return s.changeList(ctx, userID, func(ctx context.Context) error {
		err := s.profiles.UpdateUserSkillLevel(ctx, list, userID, skillID, level)
		if errors.Is(err, data.ErrNotFound) {
			return ErrSkillNotInList
		}
		return err
	})
}

func (s *Service) RemoveSkill(ctx context.Context, userID string, list data.SkillList, skillID string) (data.Profile, error) {
	if !validate.UUID(skillID) {
		return data.Profile{}, ErrSkillNotInList
	}
	return s.changeList(ctx, userID, func(ctx context.Context) error {
		err := s.profiles.RemoveUserSkill(ctx, list, userID, skillID)
		if errors.Is(err, data.ErrNotFound) {
			return ErrSkillNotInList
		}
		return err
	})
}

func (s *Service) changeList(ctx context.Context, userID string, change func(ctx context.Context) error) (data.Profile, error) {
	var updated data.Profile
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := change(ctx); err != nil {
			return err
		}
		var err error
		updated, err = s.profiles.ProfileByUserID(ctx, userID)
		return err
	})
	return updated, err
}

func validLevel(level string) error {
	if !slices.Contains(levels, level) {
		return &ValidationError{Fields: map[string]string{"level": "Level must be BEGINNER, INTERMEDIATE or ADVANCED"}}
	}
	return nil
}

// Privacy returns who can find the profile and send new requests (FR-15).
func (s *Service) Privacy(ctx context.Context, userID string) (data.PrivacySettings, error) {
	return s.profiles.PrivacySettings(ctx, userID)
}

// PrivacyPatch holds the settings to change; nil keeps the current value.
type PrivacyPatch struct {
	Discoverable    *bool
	AcceptsRequests *bool
	RequestScope    *string
}

// UpdatePrivacy hides the profile from search and new requests or shows it again. Existing
// requests, exchanges and chats keep working.
func (s *Service) UpdatePrivacy(ctx context.Context, userID string, patch PrivacyPatch) (data.PrivacySettings, error) {
	if patch.RequestScope != nil && !slices.Contains(requestScopes, *patch.RequestScope) {
		return data.PrivacySettings{}, &ValidationError{Fields: map[string]string{
			"requestScope": "Request scope must be ANYONE or SAME_UNIVERSITY",
		}}
	}
	var updated data.PrivacySettings
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		settings, err := s.profiles.PrivacySettings(ctx, userID)
		if err != nil {
			return err
		}
		if patch.Discoverable != nil {
			settings.Discoverable = *patch.Discoverable
		}
		if patch.AcceptsRequests != nil {
			settings.AcceptsRequests = *patch.AcceptsRequests
		}
		if patch.RequestScope != nil {
			settings.RequestScope = *patch.RequestScope
		}
		if err := s.profiles.UpdatePrivacySettings(ctx, userID, settings); err != nil {
			return err
		}
		updated, err = s.profiles.PrivacySettings(ctx, userID)
		return err
	})
	return updated, err
}
