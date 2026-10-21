package api

import (
	"context"
	"net/http"
	"time"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/trust"
)

const adminTimeout = 10 * time.Second

// withAdmin lets only verified administrators through (FR-14, NFR-08).
func (a *API) withAdmin(next userHandler) http.HandlerFunc {
	return a.withVerifiedUser(func(w http.ResponseWriter, r *http.Request, current data.User, sessionToken string) {
		if current.Role != data.RoleAdmin {
			problem(w, http.StatusForbidden, "admin_required", "Only administrators can do this")
			return
		}
		next(w, r, current, sessionToken)
	})
}

type skillReference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type moderationActionResponse struct {
	ID        string          `json:"id"`
	ReportID  *string         `json:"reportId"`
	Moderator person          `json:"moderator"`
	Subject   *person         `json:"subject"`
	Skill     *skillReference `json:"skill"`
	Action    string          `json:"action"`
	Reason    string          `json:"reason"`
	CreatedAt time.Time       `json:"createdAt"`
}

func toModerationAction(m data.ModerationAction) moderationActionResponse {
	response := moderationActionResponse{
		ID: m.ID, Moderator: toPerson(m.Moderator), Action: m.Action, Reason: m.Reason, CreatedAt: m.CreatedAt,
	}
	if m.ReportID != "" {
		response.ReportID = &m.ReportID
	}
	if m.Subject != nil {
		subject := toPerson(*m.Subject)
		response.Subject = &subject
	}
	if m.Skill != nil {
		response.Skill = &skillReference{ID: m.Skill.ID, Name: m.Skill.Name}
	}
	return response
}

func toModerationActions(actions []data.ModerationAction) []moderationActionResponse {
	items := make([]moderationActionResponse, 0, len(actions))
	for _, m := range actions {
		items = append(items, toModerationAction(m))
	}
	return items
}

type reportedUser struct {
	ID            string `json:"id"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	AccountStatus string `json:"accountStatus"`
}

type adminReportResponse struct {
	ID           string                    `json:"id"`
	Reporter     person                    `json:"reporter"`
	ReportedUser reportedUser              `json:"reportedUser"`
	Reason       string                    `json:"reason"`
	Status       string                    `json:"status"`
	CreatedAt    time.Time                 `json:"createdAt"`
	ResolvedAt   *time.Time                `json:"resolvedAt"`
	Decision     *moderationActionResponse `json:"decision"`
}

func toAdminReport(r data.Report) adminReportResponse {
	response := adminReportResponse{
		ID: r.ID, Reporter: toPerson(r.Reporter),
		ReportedUser: reportedUser{
			ID: r.ReportedUser.ID, FirstName: r.ReportedUser.FirstName, LastName: r.ReportedUser.LastName,
			AccountStatus: r.ReportedAccountStatus,
		},
		Reason: r.Reason, Status: r.Status, CreatedAt: r.CreatedAt, ResolvedAt: r.ResolvedAt,
	}
	if r.Decision != nil {
		decision := toModerationAction(*r.Decision)
		response.Decision = &decision
	}
	return response
}

func (a *API) adminReports(w http.ResponseWriter, r *http.Request, _ data.User, _ string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), adminTimeout)
	defer cancel()
	result, err := a.app.Reports(ctx, r.URL.Query().Get("status"), page)
	if err != nil {
		a.safetyError(w, err)
		return
	}
	items := make([]adminReportResponse, 0, len(result.Items))
	for _, report := range result.Items {
		items = append(items, toAdminReport(report))
	}
	respond(w, http.StatusOK, map[string]any{"items": items, "page": result.Page, "pageSize": trust.PageSize, "total": result.Total})
}

func (a *API) adminReportDetails(w http.ResponseWriter, r *http.Request, _ data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), adminTimeout)
	defer cancel()
	details, err := a.app.ReportDetails(ctx, r.PathValue("reportId"))
	if err != nil {
		a.safetyError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"report":  toAdminReport(details.Report),
		"history": toModerationActions(details.History),
	})
}

type resolveReportRequest struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

func (a *API) resolveReport(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	var body resolveReportRequest
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), adminTimeout)
	defer cancel()
	report, err := a.app.ResolveReport(ctx, current.ID, r.PathValue("reportId"), trust.DecisionInput(body))
	if err != nil {
		a.safetyError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"report": toAdminReport(report)})
}

type accountResponse struct {
	ID            string `json:"id"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	AccountStatus string `json:"accountStatus"`
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

type accountChange func(ctx context.Context, moderatorID, userID, reason string) (trust.AccountResult, error)

// moderateAccount runs SuspendAccount or RestoreAccount for the user in the path.
func (a *API) moderateAccount(change accountChange) userHandler {
	return func(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
		var body reasonRequest
		if !decodeBody(w, r, &body) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), adminTimeout)
		defer cancel()
		result, err := change(ctx, current.ID, r.PathValue("userId"), body.Reason)
		if err != nil {
			a.safetyError(w, err)
			return
		}
		u := result.User
		response := map[string]any{
			"user": accountResponse{
				ID: u.ID, FirstName: u.FirstName, LastName: u.LastName, Email: u.Email, Role: u.Role, AccountStatus: u.Status,
			},
			"action": nil,
		}
		if result.Action != nil {
			response["action"] = toModerationAction(*result.Action)
		}
		respond(w, http.StatusOK, response)
	}
}

func (a *API) moderationLog(w http.ResponseWriter, r *http.Request, _ data.User, _ string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), adminTimeout)
	defer cancel()
	result, err := a.app.ModerationLog(ctx, r.URL.Query().Get("userId"), page)
	if err != nil {
		a.safetyError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"items": toModerationActions(result.Items), "page": result.Page, "pageSize": trust.PageSize, "total": result.Total,
	})
}

type catalogSkillResponse struct {
	ID           string `json:"id"`
	CategoryID   string `json:"categoryId"`
	CategoryName string `json:"categoryName"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Description  string `json:"description"`
	IsActive     bool   `json:"isActive"`
}

func toCatalogSkill(s data.CatalogSkill) catalogSkillResponse {
	return catalogSkillResponse{
		ID: s.ID, CategoryID: s.CategoryID, CategoryName: s.CategoryName, Name: s.Name, Slug: s.Slug,
		Description: s.Description, IsActive: s.IsActive,
	}
}

func (a *API) adminSkills(w http.ResponseWriter, r *http.Request, _ data.User, _ string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), adminTimeout)
	defer cancel()
	result, err := a.app.CatalogSkills(ctx, r.URL.Query().Get("q"), page)
	if err != nil {
		a.safetyError(w, err)
		return
	}
	items := make([]catalogSkillResponse, 0, len(result.Items))
	for _, s := range result.Items {
		items = append(items, toCatalogSkill(s))
	}
	respond(w, http.StatusOK, map[string]any{"items": items, "page": result.Page, "pageSize": trust.CatalogPageSize, "total": result.Total})
}

// skillRequest leaves out the fields that do not change; on creation isActive defaults to true.
type skillRequest struct {
	CategoryID  *string `json:"categoryId"`
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"isActive"`
}

func (a *API) createSkill(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	var body skillRequest
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), adminTimeout)
	defer cancel()
	created, err := a.app.CreateSkill(ctx, current.ID, trust.SkillPatch(body))
	if err != nil {
		a.safetyError(w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]any{"skill": toCatalogSkill(created)})
}

func (a *API) updateSkill(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	var body skillRequest
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), adminTimeout)
	defer cancel()
	updated, err := a.app.UpdateSkill(ctx, current.ID, r.PathValue("skillId"), trust.SkillPatch(body))
	if err != nil {
		a.safetyError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"skill": toCatalogSkill(updated)})
}
