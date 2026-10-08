package api

import (
	"context"
	"errors"
	"fmt"
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

type requestStatusChange struct {
	Status    string    `json:"status"`
	ChangedBy *person   `json:"changedBy"`
	CreatedAt time.Time `json:"createdAt"`
}

func (a *API) listRequests(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	filter := data.RequestFilter{Direction: data.RequestDirection(query.Get("direction")), Status: query.Get("status")}
	ctx, cancel := context.WithTimeout(r.Context(), exchangeTimeout)
	defer cancel()
	result, err := a.app.Requests(ctx, current.ID, filter, page)
	if err != nil {
		a.exchangeError(w, err)
		return
	}
	items := make([]exchangeRequest, 0, len(result.Items))
	for _, request := range result.Items {
		items = append(items, toExchangeRequest(request))
	}
	respond(w, http.StatusOK, map[string]any{
		"items":    items,
		"page":     result.Page,
		"pageSize": exchange.PageSize,
		"total":    result.Total,
	})
}

func (a *API) requestDetails(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), exchangeTimeout)
	defer cancel()
	result, err := a.app.RequestDetails(ctx, current.ID, r.PathValue("requestId"))
	if err != nil {
		a.exchangeError(w, err)
		return
	}
	history := make([]requestStatusChange, 0, len(result.History))
	for _, c := range result.History {
		change := requestStatusChange{Status: c.Status, CreatedAt: c.CreatedAt}
		if c.ChangedByID != "" {
			change.ChangedBy = &person{ID: c.ChangedByID, FirstName: c.ChangedByFirstName, LastName: c.ChangedByLastName}
		}
		history = append(history, change)
	}
	respond(w, http.StatusOK, map[string]any{"request": toExchangeRequest(result.Request), "history": history})
}

type answerFunc func(ctx context.Context, userID, requestID string) (data.ExchangeRequest, error)

func (a *API) answerRequest(answer answerFunc) userHandler {
	return func(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
		ctx, cancel := context.WithTimeout(r.Context(), exchangeTimeout)
		defer cancel()
		answered, err := answer(ctx, current.ID, r.PathValue("requestId"))
		if err != nil {
			a.exchangeError(w, err)
			return
		}
		respond(w, http.StatusOK, map[string]any{"request": toExchangeRequest(answered)})
	}
}

type exchangeResponse struct {
	ID               string         `json:"id"`
	RequestID        *string        `json:"requestId"`
	Status           string         `json:"status"`
	Format           string         `json:"format"`
	TotalSessions    int            `json:"totalSessions"`
	Requester        studentSummary `json:"requester"`
	Recipient        studentSummary `json:"recipient"`
	RequesterTeaches requestTerms   `json:"requesterTeaches"`
	RecipientTeaches requestTerms   `json:"recipientTeaches"`
	StartedAt        *time.Time     `json:"startedAt"`
	CreatedAt        time.Time      `json:"createdAt"`
}

func toExchange(e data.Exchange) exchangeResponse {
	response := exchangeResponse{
		ID: e.ID, Status: e.Status, Format: e.Format, TotalSessions: e.TotalSessions,
		Requester: toRequestParty(e.Requester), Recipient: toRequestParty(e.Recipient),
		RequesterTeaches: toRequestTerms(e.RequesterTeaches), RecipientTeaches: toRequestTerms(e.RecipientTeaches),
		StartedAt: e.StartedAt, CreatedAt: e.CreatedAt,
	}
	if e.RequestID != "" {
		response.RequestID = &e.RequestID
	}
	return response
}

func (a *API) acceptRequest(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), exchangeTimeout)
	defer cancel()
	accepted, err := a.app.AcceptRequest(ctx, current.ID, r.PathValue("requestId"))
	if err != nil {
		a.exchangeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"request":  toExchangeRequest(accepted.Request),
		"exchange": toExchange(accepted.Exchange),
	})
}

func (a *API) exchangeDetails(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), exchangeTimeout)
	defer cancel()
	found, err := a.app.Exchange(ctx, current.ID, r.PathValue("exchangeId"))
	if err != nil {
		a.exchangeError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"exchange": toExchange(found)})
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
	case errors.Is(err, exchange.ErrRequestNotFound):
		problem(w, http.StatusNotFound, "request_not_found", "Request was not found")
	case errors.Is(err, exchange.ErrActionNotAllowed):
		problem(w, http.StatusForbidden, "action_not_allowed", "This action is not available for your role in the request")
	case errors.Is(err, exchange.ErrRequestNotPending):
		problem(w, http.StatusConflict, "request_not_pending", "The request has already been answered")
	case errors.Is(err, exchange.ErrRequestOutdated):
		problem(w, http.StatusConflict, "request_outdated",
			"The request can no longer be accepted: a student or one of the skills is no longer available")
	case errors.Is(err, exchange.ErrExchangeNotFound):
		problem(w, http.StatusNotFound, "exchange_not_found", "Exchange was not found")
	case errors.Is(err, exchange.ErrInvalidPage):
		problem(w, http.StatusBadRequest, "invalid_page", fmt.Sprintf("page must be a whole number from 1 to %d", exchange.MaxPage))
	default:
		a.queryError(w, err)
	}
}
