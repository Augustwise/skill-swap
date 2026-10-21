package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
	"skillswap/backend/internal/trust"
)

const (
	safetyTimeout = 5 * time.Second
	// Deleting an account cancels every exchange of the user in one transaction.
	deleteAccountTimeout = 10 * time.Second
)

type privacyResponse struct {
	IsDiscoverable  bool   `json:"isDiscoverable"`
	AcceptsRequests bool   `json:"acceptsRequests"`
	RequestScope    string `json:"requestScope"`
}

func toPrivacy(s data.PrivacySettings) privacyResponse {
	return privacyResponse{IsDiscoverable: s.Discoverable, AcceptsRequests: s.AcceptsRequests, RequestScope: s.RequestScope}
}

func (a *API) getPrivacy(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), safetyTimeout)
	defer cancel()
	settings, err := a.app.Privacy(ctx, current.ID)
	if err != nil {
		a.profileError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"privacy": toPrivacy(settings)})
}

type updatePrivacyRequest struct {
	IsDiscoverable  *bool   `json:"isDiscoverable"`
	AcceptsRequests *bool   `json:"acceptsRequests"`
	RequestScope    *string `json:"requestScope"`
}

func (a *API) updatePrivacy(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	var body updatePrivacyRequest
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), safetyTimeout)
	defer cancel()
	settings, err := a.app.UpdatePrivacy(ctx, current.ID, profile.PrivacyPatch{
		Discoverable: body.IsDiscoverable, AcceptsRequests: body.AcceptsRequests, RequestScope: body.RequestScope,
	})
	if err != nil {
		a.profileError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"privacy": toPrivacy(settings)})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (a *API) changePassword(w http.ResponseWriter, r *http.Request, current data.User, sessionToken string) {
	var body changePasswordRequest
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), authTimeout)
	defer cancel()
	if err := a.access.ChangePassword(ctx, current, sessionToken, body.CurrentPassword, body.NewPassword); err != nil {
		a.authError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "password_changed"})
}

type deleteAccountRequest struct {
	Password string `json:"password"`
}

func (a *API) deleteAccount(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	var body deleteAccountRequest
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), deleteAccountTimeout)
	defer cancel()
	if err := a.access.ConfirmPassword(ctx, current, body.Password); err != nil {
		a.authError(w, err)
		return
	}
	if err := a.app.DeleteAccount(ctx, current.ID); err != nil {
		a.safetyError(w, err)
		return
	}
	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

type blockResponse struct {
	User      person    `json:"user"`
	CreatedAt time.Time `json:"createdAt"`
}

func toPerson(u data.UserRef) person {
	return person{ID: u.ID, FirstName: u.FirstName, LastName: u.LastName}
}

func toBlock(b data.Block) blockResponse {
	return blockResponse{User: toPerson(b.User), CreatedAt: b.CreatedAt}
}

func (a *API) blockStudent(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), safetyTimeout)
	defer cancel()
	result, err := a.app.BlockStudent(ctx, current.ID, r.PathValue("userId"))
	if err != nil {
		a.safetyError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"block":              toBlock(result.Block),
		"cancelledExchanges": result.CancelledExchanges,
		"closedRequests":     result.ClosedRequests,
	})
}

func (a *API) unblockStudent(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), safetyTimeout)
	defer cancel()
	if err := a.app.UnblockStudent(ctx, current.ID, r.PathValue("userId")); err != nil {
		a.safetyError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listBlocks(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), safetyTimeout)
	defer cancel()
	result, err := a.app.Blocks(ctx, current.ID, page)
	if err != nil {
		a.safetyError(w, err)
		return
	}
	items := make([]blockResponse, 0, len(result.Items))
	for _, b := range result.Items {
		items = append(items, toBlock(b))
	}
	respond(w, http.StatusOK, map[string]any{"items": items, "page": result.Page, "pageSize": trust.PageSize, "total": result.Total})
}

// reportResponse is what the author of a report sees: the decision itself stays with the administrators.
type reportResponse struct {
	ID           string     `json:"id"`
	ReportedUser person     `json:"reportedUser"`
	Reason       string     `json:"reason"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"createdAt"`
	ResolvedAt   *time.Time `json:"resolvedAt"`
}

func toReport(r data.Report) reportResponse {
	return reportResponse{
		ID: r.ID, ReportedUser: toPerson(r.ReportedUser), Reason: r.Reason, Status: r.Status,
		CreatedAt: r.CreatedAt, ResolvedAt: r.ResolvedAt,
	}
}

type createReportRequest struct {
	ReportedUserID string `json:"reportedUserId"`
	Reason         string `json:"reason"`
}

func (a *API) createReport(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	var body createReportRequest
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), safetyTimeout)
	defer cancel()
	report, err := a.app.CreateReport(ctx, current.ID, trust.ReportInput(body))
	if err != nil {
		a.safetyError(w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]any{"report": toReport(report)})
}

func (a *API) myReports(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), safetyTimeout)
	defer cancel()
	result, err := a.app.MyReports(ctx, current.ID, page)
	if err != nil {
		a.safetyError(w, err)
		return
	}
	items := make([]reportResponse, 0, len(result.Items))
	for _, report := range result.Items {
		items = append(items, toReport(report))
	}
	respond(w, http.StatusOK, map[string]any{"items": items, "page": result.Page, "pageSize": trust.PageSize, "total": result.Total})
}

// safetyError maps the errors of blocks, reports, account deletion and the administrator tools.
func (a *API) safetyError(w http.ResponseWriter, err error) {
	var invalid *profile.ValidationError
	switch {
	case errors.As(err, &invalid):
		validationProblem(w, invalid.Fields)
	case errors.Is(err, trust.ErrStudentNotFound):
		problem(w, http.StatusNotFound, "student_not_found", "Student was not found")
	case errors.Is(err, trust.ErrUserNotFound):
		problem(w, http.StatusNotFound, "user_not_found", "User was not found")
	case errors.Is(err, trust.ErrReportNotFound):
		problem(w, http.StatusNotFound, "report_not_found", "Report was not found")
	case errors.Is(err, trust.ErrSkillNotFound):
		problem(w, http.StatusNotFound, "skill_not_found", "Skill was not found in the catalog")
	case errors.Is(err, trust.ErrReportExists):
		problem(w, http.StatusConflict, "report_already_open", "You already have an open report about this student")
	case errors.Is(err, trust.ErrReportClosed):
		problem(w, http.StatusConflict, "report_already_resolved", "This report has already been decided")
	case errors.Is(err, trust.ErrSelfModeration):
		problem(w, http.StatusConflict, "self_moderation", "You cannot moderate your own account")
	case errors.Is(err, trust.ErrSkillExists):
		problem(w, http.StatusConflict, "skill_already_exists", "A skill with this name or slug already exists")
	case errors.Is(err, trust.ErrInvalidPage):
		problem(w, http.StatusBadRequest, "invalid_page", fmt.Sprintf("page must be a whole number from 1 to %d", trust.MaxPage))
	default:
		a.queryError(w, err)
	}
}
