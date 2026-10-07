package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/discovery"
	"skillswap/backend/internal/profile"
)

const discoveryTimeout = 5 * time.Second

type studentSummary struct {
	ID         string    `json:"id"`
	FirstName  string    `json:"firstName"`
	LastName   string    `json:"lastName"`
	University reference `json:"university"`
	City       string    `json:"city"`
}

type mutualMatch struct {
	Student       studentSummary `json:"student"`
	CanTeachYou   []userSkill    `json:"canTeachYou"`
	WantsToLearn  []userSkill    `json:"wantsToLearn"`
	CommonFormats []string       `json:"commonFormats"`
}

type studentCard struct {
	Student        studentSummary `json:"student"`
	TeachingSkills []userSkill    `json:"teachingSkills"`
	Formats        []string       `json:"formats"`
	Mutual         bool           `json:"mutual"`
}

func toStudentSummary(id, firstName, lastName, universityID, universityName, city string) studentSummary {
	return studentSummary{
		ID: id, FirstName: firstName, LastName: lastName,
		University: reference{ID: universityID, Name: universityName}, City: city,
	}
}

func toMutualMatch(m data.Match) mutualMatch {
	return mutualMatch{
		Student:       toStudentSummary(m.UserID, m.FirstName, m.LastName, m.UniversityID, m.UniversityName, m.City),
		CanTeachYou:   toUserSkills(m.CanTeach),
		WantsToLearn:  toUserSkills(m.WantsToLearn),
		CommonFormats: append([]string{}, m.CommonFormats...),
	}
}

func (a *API) mutualMatches(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), discoveryTimeout)
	defer cancel()
	result, err := a.app.MutualMatches(ctx, current.ID, page)
	if err != nil {
		a.discoveryError(w, err)
		return
	}
	items := make([]mutualMatch, 0, len(result.Items))
	for _, m := range result.Items {
		items = append(items, toMutualMatch(m))
	}
	respond(w, http.StatusOK, map[string]any{
		"items":               items,
		"page":                result.Page,
		"pageSize":            discovery.PageSize,
		"total":               result.Total,
		"eligibleForMatching": result.Eligible,
	})
}

func (a *API) searchStudents(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	filter := data.StudentFilter{
		Query:      query.Get("q"),
		CategoryID: query.Get("categoryId"),
		Level:      query.Get("level"),
		Format:     query.Get("format"),
	}
	switch query.Get("mutual") {
	case "", "false":
	case "true":
		filter.MutualOnly = true
	default:
		validationProblem(w, map[string]string{"mutual": "mutual must be true or false"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), discoveryTimeout)
	defer cancel()
	result, err := a.app.SearchStudents(ctx, current.ID, filter, page)
	if err != nil {
		a.discoveryError(w, err)
		return
	}
	items := make([]studentCard, 0, len(result.Items))
	for _, c := range result.Items {
		items = append(items, studentCard{
			Student:        toStudentSummary(c.UserID, c.FirstName, c.LastName, c.UniversityID, c.UniversityName, c.City),
			TeachingSkills: toUserSkills(c.TeachingSkills),
			Formats:        append([]string{}, c.Formats...),
			Mutual:         c.Mutual,
		})
	}
	respond(w, http.StatusOK, map[string]any{
		"items":    items,
		"page":     result.Page,
		"pageSize": discovery.PageSize,
		"total":    result.Total,
	})
}

// pageParam reads the optional 1-based "page" query parameter.
func pageParam(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("page")
	if raw == "" {
		return 1, true
	}
	page, err := strconv.Atoi(raw)
	if err != nil || page < 1 || page > discovery.MaxPage {
		problem(w, http.StatusBadRequest, "invalid_page", fmt.Sprintf("page must be a whole number from 1 to %d", discovery.MaxPage))
		return 0, false
	}
	return page, true
}

func (a *API) discoveryError(w http.ResponseWriter, err error) {
	var invalid *profile.ValidationError
	switch {
	case errors.As(err, &invalid):
		validationProblem(w, invalid.Fields)
	case errors.Is(err, discovery.ErrStudentNotFound):
		problem(w, http.StatusNotFound, "student_not_found", "Student was not found")
	case errors.Is(err, discovery.ErrInvalidPage):
		problem(w, http.StatusBadRequest, "invalid_page", fmt.Sprintf("page must be a whole number from 1 to %d", discovery.MaxPage))
	case errors.Is(err, data.ErrNotFound):
		problem(w, http.StatusNotFound, "not_found", "Resource was not found")
	default:
		a.queryError(w, err)
	}
}

type reviewResponse struct {
	ID        string    `json:"id"`
	Author    person    `json:"author"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
}

type person struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type mutualDetails struct {
	CanTeachYou   []userSkill `json:"canTeachYou"`
	WantsToLearn  []userSkill `json:"wantsToLearn"`
	CommonFormats []string    `json:"commonFormats"`
}

type studentProfileResponse struct {
	ID             string           `json:"id"`
	FirstName      string           `json:"firstName"`
	LastName       string           `json:"lastName"`
	University     reference        `json:"university"`
	Faculty        *reference       `json:"faculty"`
	Course         *int             `json:"course"`
	City           string           `json:"city"`
	Bio            string           `json:"bio"`
	Formats        []string         `json:"formats"`
	TeachingSkills []userSkill      `json:"teachingSkills"`
	LearningSkills []userSkill      `json:"learningSkills"`
	AverageRating  *float64         `json:"averageRating"`
	ReviewCount    int              `json:"reviewCount"`
	Reviews        []reviewResponse `json:"reviews"`
	Mutual         *mutualDetails   `json:"mutual"`
}

func toStudentProfile(s data.StudentProfile) studentProfileResponse {
	p := toProfile(s.Profile)
	response := studentProfileResponse{
		ID: p.ID, FirstName: p.FirstName, LastName: p.LastName, University: p.University, Faculty: p.Faculty,
		Course: p.Course, City: p.City, Bio: p.Bio, Formats: p.Formats,
		TeachingSkills: p.TeachingSkills, LearningSkills: p.LearningSkills,
		ReviewCount: s.ReviewCount, Reviews: make([]reviewResponse, 0, len(s.Reviews)),
	}
	if s.ReviewCount > 0 {
		average := math.Round(s.AverageRating*100) / 100
		response.AverageRating = &average
	}
	for _, r := range s.Reviews {
		response.Reviews = append(response.Reviews, reviewResponse{
			ID: r.ID, Author: person{ID: r.AuthorID, FirstName: r.AuthorFirstName, LastName: r.AuthorLastName},
			Rating: r.Rating, Comment: r.Comment, CreatedAt: r.CreatedAt,
		})
	}
	if s.Match != nil {
		response.Mutual = &mutualDetails{
			CanTeachYou:   toUserSkills(s.Match.CanTeach),
			WantsToLearn:  toUserSkills(s.Match.WantsToLearn),
			CommonFormats: append([]string{}, s.Match.CommonFormats...),
		}
	}
	return response
}

func (a *API) studentProfile(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), discoveryTimeout)
	defer cancel()
	result, err := a.app.StudentProfile(ctx, current.ID, r.PathValue("userId"))
	if err != nil {
		a.discoveryError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"student": toStudentProfile(result)})
}
