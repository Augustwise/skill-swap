package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	_ "time/tzdata"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
	sessionsvc "skillswap/backend/internal/session"
)

const (
	sessionTimeout  = 5 * time.Second
	defaultTimezone = "Europe/Kyiv"
)

type sessionProposalResponse struct {
	ID           string     `json:"id"`
	SessionID    string     `json:"sessionId"`
	Status       string     `json:"status"`
	ProposedBy   person     `json:"proposedBy"`
	StartsAt     time.Time  `json:"startsAt"`
	EndsAt       time.Time  `json:"endsAt"`
	Format       string     `json:"format"`
	MeetingPlace *string    `json:"meetingPlace"`
	MeetingURL   *string    `json:"meetingUrl"`
	RespondedBy  *person    `json:"respondedBy"`
	RespondedAt  *time.Time `json:"respondedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type sessionResponse struct {
	ID                         string                   `json:"id"`
	ExchangeID                 string                   `json:"exchangeId"`
	PlanOrder                  int                      `json:"planOrder"`
	Title                      string                   `json:"title"`
	Description                *string                  `json:"description"`
	Status                     string                   `json:"status"`
	Format                     string                   `json:"format"`
	Skill                      requestSkill             `json:"skill"`
	PlannedDurationMinutes     int                      `json:"plannedDurationMinutes"`
	Teacher                    studentSummary           `json:"teacher"`
	Learner                    studentSummary           `json:"learner"`
	Partner                    studentSummary           `json:"partner"`
	Role                       string                   `json:"role"`
	StartsAt                   *time.Time               `json:"startsAt"`
	EndsAt                     *time.Time               `json:"endsAt"`
	DurationMinutes            *int                     `json:"durationMinutes"`
	MeetingPlace               *string                  `json:"meetingPlace"`
	MeetingURL                 *string                  `json:"meetingUrl"`
	ActiveProposal             *sessionProposalResponse `json:"activeProposal"`
	ScheduleConfirmedByMe      bool                     `json:"scheduleConfirmedByMe"`
	ScheduleConfirmedByPartner bool                     `json:"scheduleConfirmedByPartner"`
	CanRespondToProposal       bool                     `json:"canRespondToProposal"`
	CanCancel                  bool                     `json:"canCancel"`
	CompletedAt                *time.Time               `json:"completedAt"`
	CancelledAt                *time.Time               `json:"cancelledAt"`
	CancellationReason         *string                  `json:"cancellationReason"`
	CreatedAt                  time.Time                `json:"createdAt"`
	UpdatedAt                  time.Time                `json:"updatedAt"`
}

func inTimezone(t time.Time, tz string) time.Time {
	if tz == "" {
		tz = defaultTimezone
	}
	if loc, err := time.LoadLocation(tz); err == nil {
		return t.In(loc)
	}
	return t
}

func toSessionProposal(sp data.SessionProposal, tz ...string) sessionProposalResponse {
	zone := defaultTimezone
	if len(tz) > 0 && tz[0] != "" {
		zone = tz[0]
	}
	response := sessionProposalResponse{
		ID:        sp.ID,
		SessionID: sp.SessionID,
		Status:    sp.Status,
		ProposedBy: person{
			ID:        sp.ProposedByID,
			FirstName: sp.ProposedByFirstName,
			LastName:  sp.ProposedByLastName,
		},
		StartsAt:    inTimezone(sp.StartsAt, zone),
		EndsAt:      inTimezone(sp.EndsAt, zone),
		Format:      sp.Format,
		RespondedAt: sp.RespondedAt,
		CreatedAt:   sp.CreatedAt,
	}
	if sp.MeetingPlace != "" {
		response.MeetingPlace = &sp.MeetingPlace
	}
	if sp.MeetingURL != "" {
		response.MeetingURL = &sp.MeetingURL
	}
	if sp.RespondedByID != "" {
		response.RespondedBy = &person{
			ID:        sp.RespondedByID,
			FirstName: sp.RespondedByFirstName,
			LastName:  sp.RespondedByLastName,
		}
	}
	return response
}

func toSession(s data.Session, viewerID string, tz ...string) sessionResponse {
	zone := defaultTimezone
	if len(tz) > 0 && tz[0] != "" {
		zone = tz[0]
	}
	requester := toRequestParty(s.Requester)
	recipient := toRequestParty(s.Recipient)
	teacher := toRequestParty(s.Teacher)
	learner := toRequestParty(s.Learner)

	partner := recipient
	partnerID := s.Recipient.UserID
	if viewerID == s.Recipient.UserID {
		partner = requester
		partnerID = s.Requester.UserID
	}
	role := "LEARNER"
	if viewerID == s.Teacher.UserID {
		role = "TEACHER"
	}

	confirmedByMe := slices.Contains(s.ConfirmedUserIDs, viewerID)
	confirmedByPartner := slices.Contains(s.ConfirmedUserIDs, partnerID)
	canRespond := s.Status == data.SessionProposed &&
		s.ExchangeStatus == data.ExchangeActive &&
		s.CanInteract &&
		s.ActiveProposal != nil &&
		s.ActiveProposal.Status == data.ProposalPending &&
		s.ActiveProposal.ProposedByID != viewerID
	canCancel := (s.Status == data.SessionProposed || s.Status == data.SessionScheduled) &&
		s.ExchangeStatus == data.ExchangeActive

	response := sessionResponse{
		ID:         s.ID,
		ExchangeID: s.ExchangeID,
		PlanOrder:  s.PlanOrder,
		Title:      s.Title,
		Status:     s.Status,
		Format:     s.Format,
		Skill: requestSkill{
			SkillID:    s.SkillID,
			CategoryID: s.SkillCategoryID,
			Name:       s.SkillName,
		},
		PlannedDurationMinutes:     s.PlannedDurationMinutes,
		Teacher:                    teacher,
		Learner:                    learner,
		Partner:                    partner,
		Role:                       role,
		ScheduleConfirmedByMe:      confirmedByMe,
		ScheduleConfirmedByPartner: confirmedByPartner,
		CanRespondToProposal:       canRespond,
		CanCancel:                  canCancel,
		CompletedAt:                s.CompletedAt,
		CancelledAt:                s.CancelledAt,
		CreatedAt:                  s.CreatedAt,
		UpdatedAt:                  s.UpdatedAt,
	}
	if s.Description != "" {
		response.Description = &s.Description
	}
	if s.StartsAt != nil {
		starts := inTimezone(*s.StartsAt, zone)
		response.StartsAt = &starts
	}
	if s.EndsAt != nil {
		ends := inTimezone(*s.EndsAt, zone)
		response.EndsAt = &ends
	}
	if s.StartsAt != nil && s.EndsAt != nil {
		minutes := int(s.EndsAt.Sub(*s.StartsAt).Minutes())
		response.DurationMinutes = &minutes
	}
	if s.MeetingPlace != "" {
		response.MeetingPlace = &s.MeetingPlace
	}
	if s.MeetingURL != "" {
		response.MeetingURL = &s.MeetingURL
	}
	if s.ActiveProposal != nil {
		proposal := toSessionProposal(*s.ActiveProposal, zone)
		response.ActiveProposal = &proposal
	}
	if s.CancellationReason != "" {
		response.CancellationReason = &s.CancellationReason
	}
	return response
}

type proposeSessionBody struct {
	SessionID    string `json:"sessionId"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	TeacherID    string `json:"teacherId"`
	StartsAt     string `json:"startsAt"`
	EndsAt       string `json:"endsAt"`
	Format       string `json:"format"`
	MeetingPlace string `json:"meetingPlace"`
	MeetingURL   string `json:"meetingUrl"`
}

func (a *API) listExchangeSessions(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), sessionTimeout)
	defer cancel()
	result, err := a.app.ExchangeSessions(ctx, current.ID, r.PathValue("exchangeId"))
	if err != nil {
		a.sessionError(w, err)
		return
	}
	items := make([]sessionResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toSession(item, current.ID))
	}
	respond(w, http.StatusOK, map[string]any{
		"exchange": toExchange(result.Exchange, current.ID),
		"items":    items,
	})
}

func (a *API) proposeExchangeSession(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	var body proposeSessionBody
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), sessionTimeout)
	defer cancel()
	result, err := a.app.ProposeExchangeSession(ctx, current.ID, r.PathValue("exchangeId"), sessionsvc.ProposalInput(body))
	if err != nil {
		a.sessionError(w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]any{
		"session":  toSession(result.Session, current.ID),
		"proposal": toSessionProposal(result.Proposal),
	})
}

func (a *API) sessionDetails(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), sessionTimeout)
	defer cancel()
	result, err := a.app.SessionDetails(ctx, current.ID, r.PathValue("sessionId"))
	if err != nil {
		a.sessionError(w, err)
		return
	}
	proposals := make([]sessionProposalResponse, 0, len(result.Proposals))
	for _, p := range result.Proposals {
		proposals = append(proposals, toSessionProposal(p))
	}
	respond(w, http.StatusOK, map[string]any{
		"session":   toSession(result.Session, current.ID),
		"proposals": proposals,
	})
}

func (a *API) proposeSession(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	var body proposeSessionBody
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), sessionTimeout)
	defer cancel()
	result, err := a.app.ProposeSession(ctx, current.ID, r.PathValue("sessionId"), sessionsvc.ProposalInput(body))
	if err != nil {
		a.sessionError(w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]any{
		"session":  toSession(result.Session, current.ID),
		"proposal": toSessionProposal(result.Proposal),
	})
}

func (a *API) acceptSessionProposal(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), sessionTimeout)
	defer cancel()
	result, err := a.app.AcceptSessionProposal(ctx, current.ID, r.PathValue("sessionId"))
	if err != nil {
		a.sessionError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"session":  toSession(result.Session, current.ID),
		"proposal": toSessionProposal(result.Proposal),
	})
}

func (a *API) declineSessionProposal(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), sessionTimeout)
	defer cancel()
	result, err := a.app.DeclineSessionProposal(ctx, current.ID, r.PathValue("sessionId"))
	if err != nil {
		a.sessionError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"session":  toSession(result.Session, current.ID),
		"proposal": toSessionProposal(result.Proposal),
	})
}

type cancelSessionBody struct {
	Reason string `json:"reason"`
}

func (a *API) cancelSession(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	var body cancelSessionBody
	if r.ContentLength > 0 || strings.TrimSpace(r.Header.Get("Content-Type")) != "" {
		if !decodeBody(w, r, &body) {
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), sessionTimeout)
	defer cancel()
	cancelled, err := a.app.CancelSession(ctx, current.ID, r.PathValue("sessionId"), body.Reason)
	if err != nil {
		a.sessionError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"session": toSession(cancelled, current.ID),
	})
}

func (a *API) listSchedule(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	upcoming := false
	switch r.URL.Query().Get("upcoming") {
	case "", "false":
	case "true":
		upcoming = true
	default:
		validationProblem(w, map[string]string{"upcoming": "upcoming must be true or false"})
		return
	}
	filter := data.ScheduleFilter{
		Status:       r.URL.Query().Get("status"),
		UpcomingOnly: upcoming,
	}
	ctx, cancel := context.WithTimeout(r.Context(), sessionTimeout)
	defer cancel()
	result, err := a.app.Schedule(ctx, current.ID, filter, page)
	if err != nil {
		a.sessionError(w, err)
		return
	}
	items := make([]sessionResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toSession(item, current.ID, result.Timezone))
	}
	respond(w, http.StatusOK, map[string]any{
		"items":                 items,
		"page":                  result.Page,
		"pageSize":              sessionsvc.PageSize,
		"total":                 result.Total,
		"timezone":              result.Timezone,
		"pendingProposalsCount": result.PendingProposalsCount,
	})
}

func (a *API) sessionError(w http.ResponseWriter, err error) {
	var invalid *profile.ValidationError
	switch {
	case errors.As(err, &invalid):
		validationProblem(w, invalid.Fields)
	case errors.Is(err, sessionsvc.ErrExchangeNotFound):
		problem(w, http.StatusNotFound, "exchange_not_found", "Exchange was not found")
	case errors.Is(err, sessionsvc.ErrSessionNotFound):
		problem(w, http.StatusNotFound, "session_not_found", "Session was not found")
	case errors.Is(err, sessionsvc.ErrActionNotAllowed):
		problem(w, http.StatusForbidden, "action_not_allowed", "This action is not available for your role in the session proposal")
	case errors.Is(err, sessionsvc.ErrExchangeNotActive):
		problem(w, http.StatusConflict, "exchange_not_active", "The exchange is no longer active")
	case errors.Is(err, sessionsvc.ErrExchangeBlocked):
		problem(w, http.StatusConflict, "exchange_blocked", "Session actions are blocked because a participant is unavailable or blocked")
	case errors.Is(err, sessionsvc.ErrPlanLimitReached):
		problem(w, http.StatusConflict, "plan_limit_reached", "All planned sessions for this teacher are already scheduled or proposed")
	case errors.Is(err, sessionsvc.ErrSessionNotSchedulable):
		problem(w, http.StatusConflict, "session_not_schedulable", "This session can no longer be scheduled")
	case errors.Is(err, sessionsvc.ErrSessionNotProposed):
		problem(w, http.StatusConflict, "session_not_proposed", "The session does not have a pending proposal")
	case errors.Is(err, sessionsvc.ErrSessionNotCancellable):
		problem(w, http.StatusConflict, "session_not_cancellable", "Only proposed or scheduled sessions can be cancelled")
	case errors.Is(err, sessionsvc.ErrSessionInPast):
		problem(w, http.StatusConflict, "session_in_past", "The proposed start time has already passed")
	case errors.Is(err, sessionsvc.ErrSessionConflict):
		problem(w, http.StatusConflict, "session_conflict", "One of the participants already has a scheduled session at this time")
	case errors.Is(err, sessionsvc.ErrInvalidPage):
		problem(w, http.StatusBadRequest, "invalid_page", fmt.Sprintf("page must be a whole number from 1 to %d", sessionsvc.MaxPage))
	default:
		a.queryError(w, err)
	}
}
