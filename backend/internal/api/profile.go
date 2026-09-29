package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
)

const profileTimeout = 5 * time.Second

type reference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type userSkill struct {
	SkillID    string `json:"skillId"`
	CategoryID string `json:"categoryId"`
	Name       string `json:"name"`
	Level      string `json:"level"`
}

type profileResponse struct {
	ID                  string      `json:"id"`
	FirstName           string      `json:"firstName"`
	LastName            string      `json:"lastName"`
	University          reference   `json:"university"`
	Faculty             *reference  `json:"faculty"`
	Course              *int        `json:"course"`
	City                string      `json:"city"`
	Bio                 string      `json:"bio"`
	Formats             []string    `json:"formats"`
	TeachingSkills      []userSkill `json:"teachingSkills"`
	LearningSkills      []userSkill `json:"learningSkills"`
	EligibleForMatching bool        `json:"eligibleForMatching"`
}

func toProfile(p data.Profile) profileResponse {
	response := profileResponse{
		ID:                  p.UserID,
		FirstName:           p.FirstName,
		LastName:            p.LastName,
		University:          reference{ID: p.UniversityID, Name: p.UniversityName},
		City:                p.City,
		Bio:                 p.Bio,
		Formats:             append([]string{}, p.Formats...),
		TeachingSkills:      toUserSkills(p.TeachingSkills),
		LearningSkills:      toUserSkills(p.LearningSkills),
		EligibleForMatching: profile.EligibleForMatching(p),
	}
	if p.FacultyID != "" {
		response.Faculty = &reference{ID: p.FacultyID, Name: p.FacultyName}
	}
	if p.Course != 0 {
		course := p.Course
		response.Course = &course
	}
	return response
}

func toUserSkills(rows []data.UserSkill) []userSkill {
	items := make([]userSkill, 0, len(rows))
	for _, row := range rows {
		items = append(items, userSkill{SkillID: row.SkillID, CategoryID: row.CategoryID, Name: row.Name, Level: row.Level})
	}
	return items
}

func (a *API) faculties(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	rows, err := a.app.Faculties(ctx, r.PathValue("universityId"))
	if err != nil {
		a.profileError(w, err)
		return
	}
	items := make([]reference, 0, len(rows))
	for _, row := range rows {
		items = append(items, reference{ID: row.ID, Name: row.Name})
	}
	respond(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) getProfile(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), profileTimeout)
	defer cancel()
	p, err := a.app.Profile(ctx, current.ID)
	a.respondProfile(w, http.StatusOK, p, err)
}

type optional[T any] struct {
	set   bool
	value *T
}

func (o *optional[T]) UnmarshalJSON(raw []byte) error {
	o.set = true
	return json.Unmarshal(raw, &o.value)
}

func (o optional[T]) patch() *T {
	if !o.set {
		return nil
	}
	if o.value == nil {
		return new(T)
	}
	return o.value
}

type updateProfileRequest struct {
	FirstName optional[string]   `json:"firstName"`
	LastName  optional[string]   `json:"lastName"`
	FacultyID optional[string]   `json:"facultyId"`
	Course    optional[int]      `json:"course"`
	City      optional[string]   `json:"city"`
	Bio       optional[string]   `json:"bio"`
	Formats   optional[[]string] `json:"formats"`
}

func (a *API) updateProfile(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	var body updateProfileRequest
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), profileTimeout)
	defer cancel()
	p, err := a.app.UpdateProfile(ctx, current.ID, profile.ProfilePatch{
		FirstName: body.FirstName.patch(),
		LastName:  body.LastName.patch(),
		FacultyID: body.FacultyID.patch(),
		Course:    body.Course.patch(),
		City:      body.City.patch(),
		Bio:       body.Bio.patch(),
		Formats:   body.Formats.patch(),
	})
	a.respondProfile(w, http.StatusOK, p, err)
}

type addSkillRequest struct {
	SkillID string `json:"skillId"`
	Level   string `json:"level"`
}

type skillLevelRequest struct {
	Level string `json:"level"`
}

func (a *API) addSkill(list data.SkillList) userHandler {
	return func(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
		var body addSkillRequest
		if !decodeBody(w, r, &body) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), profileTimeout)
		defer cancel()
		p, err := a.app.AddSkill(ctx, current.ID, list, body.SkillID, body.Level)
		a.respondProfile(w, http.StatusCreated, p, err)
	}
}

func (a *API) updateSkillLevel(list data.SkillList) userHandler {
	return func(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
		var body skillLevelRequest
		if !decodeBody(w, r, &body) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), profileTimeout)
		defer cancel()
		p, err := a.app.UpdateSkillLevel(ctx, current.ID, list, r.PathValue("skillId"), body.Level)
		a.respondProfile(w, http.StatusOK, p, err)
	}
}

func (a *API) removeSkill(list data.SkillList) userHandler {
	return func(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
		ctx, cancel := context.WithTimeout(r.Context(), profileTimeout)
		defer cancel()
		p, err := a.app.RemoveSkill(ctx, current.ID, list, r.PathValue("skillId"))
		a.respondProfile(w, http.StatusOK, p, err)
	}
}

func (a *API) respondProfile(w http.ResponseWriter, status int, p data.Profile, err error) {
	if err != nil {
		a.profileError(w, err)
		return
	}
	respond(w, status, map[string]any{"profile": toProfile(p)})
}

func (a *API) profileError(w http.ResponseWriter, err error) {
	var invalid *profile.ValidationError
	switch {
	case errors.As(err, &invalid):
		validationProblem(w, invalid.Fields)
	case errors.Is(err, profile.ErrSkillNotFound):
		problem(w, http.StatusNotFound, "skill_not_found", "Skill was not found in the catalog")
	case errors.Is(err, profile.ErrSkillNotInList):
		problem(w, http.StatusNotFound, "skill_not_found", "Skill is not in this list")
	case errors.Is(err, profile.ErrSkillAlreadyAdded):
		problem(w, http.StatusConflict, "skill_already_added", "Skill is already in this list")
	case errors.Is(err, profile.ErrUniversityNotFound), errors.Is(err, data.ErrNotFound):
		problem(w, http.StatusNotFound, "not_found", "Resource was not found")
	default:
		a.queryError(w, err)
	}
}
