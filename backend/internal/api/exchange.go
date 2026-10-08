package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/exchange"
	"skillswap/backend/internal/profile"
)

const exchangeTimeout = 5 * time.Second

type requestSkill struct {
	SkillID    string `json:"skillId"`
	CategoryID string `json:"categoryId"`
	Name       string `json:"name"`
}

type requestTerms struct {
	Skill           requestSkill `json:"skill"`
	TeacherLevel    string       `json:"teacherLevel"`
	LearnerLevel    string       `json:"learnerLevel"`
	Sessions        int          `json:"sessions"`
	DurationMinutes int          `json:"durationMinutes"`
}

type exchangeRequest struct {
	ID               string         `json:"id"`
	Status           string         `json:"status"`
	Format           string         `json:"format"`
	TotalSessions    int            `json:"totalSessions"`
	Message          *string        `json:"message"`
	Requester        studentSummary `json:"requester"`
	Recipient        studentSummary `json:"recipient"`
	RequesterTeaches requestTerms   `json:"requesterTeaches"`
	RecipientTeaches requestTerms   `json:"recipientTeaches"`
	ExchangeID       *string        `json:"exchangeId"`
	CreatedAt        time.Time      `json:"createdAt"`
	RespondedAt      *time.Time     `json:"respondedAt"`
}

func toRequestParty(p data.RequestParty) studentSummary {
	return toStudentSummary(p.UserID, p.FirstName, p.LastName, p.UniversityID, p.UniversityName, p.City)
}

func toRequestTerms(t data.RequestTerms) requestTerms {
	return requestTerms{
		Skill:        requestSkill{SkillID: t.SkillID, CategoryID: t.CategoryID, Name: t.Name},
		TeacherLevel: t.TeacherLevel, LearnerLevel: t.LearnerLevel,
		Sessions: t.Sessions, DurationMinutes: t.DurationMinutes,
	}
}

func toExchangeRequest(r data.ExchangeRequest) exchangeRequest {
	response := exchangeRequest{
		ID: r.ID, Status: r.Status, Format: r.Format, TotalSessions: r.TotalSessions,
		Requester: toRequestParty(r.Requester), Recipient: toRequestParty(r.Recipient),
		RequesterTeaches: toRequestTerms(r.RequesterTeaches), RecipientTeaches: toRequestTerms(r.RecipientTeaches),
		CreatedAt: r.CreatedAt, RespondedAt: r.RespondedAt,
	}
	if r.Message != "" {
		response.Message = &r.Message
	}
	if r.ExchangeID != "" {
		response.ExchangeID = &r.ExchangeID
	}
	return response
}

type createRequestBody struct {
	RecipientID          string `json:"recipientId"`
	TeachSkillID         string `json:"teachSkillId"`
	LearnSkillID         string `json:"learnSkillId"`
	Format               string `json:"format"`
	TeachSessions        int    `json:"teachSessions"`
	TeachDurationMinutes int    `json:"teachDurationMinutes"`
	LearnSessions        int    `json:"learnSessions"`
	LearnDurationMinutes int    `json:"learnDurationMinutes"`
	Message              string `json:"message"`
}

func (a *API) createRequest(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	var body createRequestBody
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), exchangeTimeout)
	defer cancel()
	created, err := a.app.CreateRequest(ctx, current.ID, exchange.NewRequest(body))
	if err != nil {
		a.exchangeError(w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]any{"request": toExchangeRequest(created)})
}

func (a *API) exchangeError(w http.ResponseWriter, err error) {
	var invalid *profile.ValidationError
	switch {
	case errors.As(err, &invalid):
		validationProblem(w, invalid.Fields)
	case errors.Is(err, exchange.ErrStudentNotFound):
		problem(w, http.StatusNotFound, "student_not_found", "Student was not found")
	case errors.Is(err, exchange.ErrRequestsClosed):
		problem(w, http.StatusConflict, "requests_closed", "The student does not accept requests")
	case errors.Is(err, exchange.ErrSameUniversityOnly):
		problem(w, http.StatusConflict, "same_university_only", "The student accepts requests only from their university")
	case errors.Is(err, exchange.ErrDuplicateRequest):
		problem(w, http.StatusConflict, "duplicate_request", "A request for these skills is already waiting for an answer")
	default:
		a.queryError(w, err)
	}
}
