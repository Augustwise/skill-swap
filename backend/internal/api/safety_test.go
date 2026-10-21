package api

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/expect"
	"skillswap/backend/internal/mailer/mailertest"
)

type privacyBody struct {
	Privacy privacyResponse `json:"privacy"`
}

type blockResult struct {
	Block              blockResponse `json:"block"`
	CancelledExchanges int           `json:"cancelledExchanges"`
	ClosedRequests     int           `json:"closedRequests"`
}

type pageOf[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

type accountChangeResponse struct {
	User   accountResponse           `json:"user"`
	Action *moderationActionResponse `json:"action"`
}

func getConversation(t *testing.T, e *discoveryEnv, path string, s *session) conversationResponse {
	t.Helper()
	return sendAndDecode[struct {
		Conversation conversationResponse `json:"conversation"`
	}](t, e, http.MethodGet, path, "", s, http.StatusOK).Conversation
}

// admin registers a verified user with the administrator role.
func (e *discoveryEnv) admin(t *testing.T) (*session, string) {
	t.Helper()
	s := e.student(t, "admin@students.example.test", `["ONLINE"]`, "Київ", "", "")
	id := e.meID(t, s)
	e.store.SetAdmin(id)
	return s, id
}

func TestPrivacyRoutes(t *testing.T) {
	e, olha, andrii, olhaID, _, exchangeID := newExchangeEnv(t)
	settings := sendAndDecode[privacyBody](t, e, http.MethodGet, "/api/v1/me/privacy", "", olha, http.StatusOK).Privacy
	expect.Equal(t, "visible by default", settings.IsDiscoverable, true)
	expect.Equal(t, "default scope", settings.RequestScope, data.ScopeAnyone)
	expect.Equal(t, "Olha in Andrii's search", e.search(t, andrii, "").Total, 1)

	settings = sendAndDecode[privacyBody](t, e, http.MethodPatch, "/api/v1/me/privacy", `{"isDiscoverable":false}`, olha, http.StatusOK).Privacy
	expect.Equal(t, "hidden", settings.IsDiscoverable, false)
	expect.Equal(t, "still accepts requests", settings.AcceptsRequests, true)
	expect.Equal(t, "Olha in Andrii's search", e.search(t, andrii, "").Total, 0)
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/students/"+olhaID, "", andrii), http.StatusNotFound, "student_not_found")
	// The current exchange keeps working.
	expect.Equal(t, "exchange of a hidden student", getExchangeDetails(t, e, exchangeID, andrii).Exchange.Status, data.ExchangeActive)

	assertProblem(t, send(t, e.handler, http.MethodPatch, "/api/v1/me/privacy", `{"requestScope":"FRIENDS"}`, olha),
		http.StatusUnprocessableEntity, "validation_failed")
	assertProblem(t, send(t, e.handler, http.MethodPatch, "/api/v1/me/privacy", `{"isDiscoverable":"no"}`, olha),
		http.StatusBadRequest, "invalid_json")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/privacy", "", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestBlockAndReportRoutes(t *testing.T) {
	e, olha, andrii, olhaID, andriiID, exchangeID := newExchangeEnv(t)
	blockPath := "/api/v1/students/" + andriiID + "/block"

	assertProblem(t, send(t, e.handler, http.MethodPost, blockPath, "", &session{cookie: olha.cookie}), http.StatusForbidden, "csrf_invalid")
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/students/"+olhaID+"/block", "", olha),
		http.StatusUnprocessableEntity, "validation_failed")
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/students/not-a-uuid/block", "", olha),
		http.StatusNotFound, "student_not_found")

	blocked := sendAndDecode[blockResult](t, e, http.MethodPost, blockPath, "", olha, http.StatusOK)
	expect.Equal(t, "blocked user", blocked.Block.User.ID, andriiID)
	expect.Equal(t, "cancelled exchanges", blocked.CancelledExchanges, 1)
	expect.Equal(t, "exchange status", getExchangeDetails(t, e, exchangeID, andrii).Exchange.Status, data.ExchangeCancelled)

	conversation := getConversation(t, e, "/api/v1/exchanges/"+exchangeID+"/conversation", olha)
	expect.Equal(t, "Olha blocked the partner", conversation.BlockedByMe, true)
	expect.Equal(t, "Olha can send", conversation.CanSend, false)
	asAndrii := getConversation(t, e, "/api/v1/conversations/"+conversation.ID, andrii)
	expect.Equal(t, "Andrii blocked the partner", asAndrii.BlockedByMe, false)
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/conversations/"+conversation.ID+"/messages", `{"body":"Привіт"}`, andrii),
		http.StatusConflict, "conversation_blocked")

	blocks := sendAndDecode[pageOf[blockResponse]](t, e, http.MethodGet, "/api/v1/me/blocks", "", olha, http.StatusOK)
	expect.Equal(t, "Olha's blocks", blocks.Total, 1)
	expect.Equal(t, "page size", blocks.PageSize, 20)

	report := sendAndDecode[struct {
		Report reportResponse `json:"report"`
	}](t, e, http.MethodPost, "/api/v1/reports", `{"reportedUserId":"`+andriiID+`","reason":"Образливі повідомлення"}`, olha, http.StatusCreated).Report
	expect.Equal(t, "report status", report.Status, data.ReportOpen)
	expect.Equal(t, "reported user", report.ReportedUser.ID, andriiID)
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/reports", `{"reportedUserId":"`+andriiID+`","reason":"Ще"}`, olha),
		http.StatusConflict, "report_already_open")
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/reports", `{"reportedUserId":"`+andriiID+`","reason":""}`, olha),
		http.StatusUnprocessableEntity, "validation_failed")
	// A partner can still report the student who blocked them.
	expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/reports", `{"reportedUserId":"`+olhaID+`","reason":"Заблокувала без причини"}`, andrii),
		http.StatusCreated)
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/reports", `{"reportedUserId":"`+datatest.GuitarSkillID+`","reason":"Спам"}`, olha),
		http.StatusNotFound, "student_not_found")
	mine := sendAndDecode[pageOf[reportResponse]](t, e, http.MethodGet, "/api/v1/me/reports", "", olha, http.StatusOK)
	expect.Equal(t, "Olha's reports", mine.Total, 1)

	expectStatus(t, send(t, e.handler, http.MethodDelete, blockPath, "", olha), http.StatusNoContent)
	conversation = getConversation(t, e, "/api/v1/conversations/"+conversation.ID, olha)
	expect.Equal(t, "blocked after unblocking", conversation.BlockedByMe, false)
	expect.Equal(t, "can send after unblocking", conversation.CanSend, true)
}

func TestAdminRoutes(t *testing.T) {
	e, olha, andrii, _, andriiID, exchangeID := newExchangeEnv(t)
	admin, adminID := e.admin(t)
	report := sendAndDecode[struct {
		Report reportResponse `json:"report"`
	}](t, e, http.MethodPost, "/api/v1/reports", `{"reportedUserId":"`+andriiID+`","reason":"Спам"}`, olha, http.StatusCreated).Report

	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/admin/reports", "", olha), http.StatusForbidden, "admin_required")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/admin/reports", "", nil), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/admin/reports?status=CLOSED", "", admin), http.StatusUnprocessableEntity, "validation_failed")

	open := sendAndDecode[pageOf[adminReportResponse]](t, e, http.MethodGet, "/api/v1/admin/reports?status=OPEN", "", admin, http.StatusOK)
	expect.Equal(t, "open reports", open.Total, 1)
	expect.Equal(t, "reported account", open.Items[0].ReportedUser.AccountStatus, data.AccountActive)

	reportPath := "/api/v1/admin/reports/" + report.ID
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/admin/reports/"+datatest.GuitarSkillID, "", admin), http.StatusNotFound, "report_not_found")
	assertProblem(t, send(t, e.handler, http.MethodPost, reportPath+"/resolve", `{"decision":"BAN","reason":"x"}`, admin),
		http.StatusUnprocessableEntity, "validation_failed")
	resolved := sendAndDecode[struct {
		Report adminReportResponse `json:"report"`
	}](t, e, http.MethodPost, reportPath+"/resolve", `{"decision":"SUSPEND","reason":"Підтверджено скаргою"}`, admin, http.StatusOK).Report
	expect.Equal(t, "report status", resolved.Status, data.ReportResolved)
	expect.Equal(t, "decision author", resolved.Decision.Moderator.ID, adminID)
	expect.Equal(t, "reported account", resolved.ReportedUser.AccountStatus, data.AccountSuspended)
	assertProblem(t, send(t, e.handler, http.MethodPost, reportPath+"/resolve", `{"decision":"DISMISS","reason":"x"}`, admin),
		http.StatusConflict, "report_already_resolved")

	// Andrii is signed out, cannot sign in, and the exchange is cancelled.
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/auth/me", "", andrii), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/auth/login", loginBody("andrii@students.example.test", testPassword), nil),
		http.StatusForbidden, "account_suspended")
	expect.Equal(t, "exchange status", getExchangeDetails(t, e, exchangeID, olha).Exchange.Status, data.ExchangeCancelled)

	details := sendAndDecode[struct {
		Report  adminReportResponse        `json:"report"`
		History []moderationActionResponse `json:"history"`
	}](t, e, http.MethodGet, reportPath, "", admin, http.StatusOK)
	expect.Equal(t, "Andrii's history", len(details.History), 1)

	restorePath := "/api/v1/admin/users/" + andriiID + "/restore"
	assertProblem(t, send(t, e.handler, http.MethodPost, restorePath, `{"reason":""}`, admin), http.StatusUnprocessableEntity, "validation_failed")
	restored := sendAndDecode[accountChangeResponse](t, e, http.MethodPost, restorePath, `{"reason":"Апеляцію прийнято"}`, admin, http.StatusOK)
	expect.Equal(t, "Andrii's status", restored.User.AccountStatus, data.AccountActive)
	expect.Equal(t, "logged action", restored.Action.Action, data.ActionRestoreAccount)
	login(t, e.handler, "andrii@students.example.test", testPassword)
	repeated := sendAndDecode[accountChangeResponse](t, e, http.MethodPost, restorePath, `{"reason":"Ще раз"}`, admin, http.StatusOK)
	expect.Equal(t, "action of a repeated restore", repeated.Action == nil, true)
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/admin/users/"+adminID+"/suspend", `{"reason":"Тест"}`, admin),
		http.StatusConflict, "self_moderation")
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/admin/users/"+datatest.GuitarSkillID+"/suspend", `{"reason":"Тест"}`, admin),
		http.StatusNotFound, "user_not_found")

	log := sendAndDecode[pageOf[moderationActionResponse]](t, e, http.MethodGet, "/api/v1/admin/moderation-actions?userId="+andriiID, "", admin, http.StatusOK)
	expect.Equal(t, "Andrii's log entries", log.Total, 2)
	expect.Equal(t, "newest entry", log.Items[0].Action, data.ActionRestoreAccount)
	expect.Equal(t, "report of the suspension", valueOf(log.Items[1].ReportID), report.ID)
}

func TestAdminCatalogRoutes(t *testing.T) {
	e := newDiscoveryEnv()
	admin, _ := e.admin(t)
	student := e.student(t, "olha@students.example.test", `["ONLINE"]`, "Київ", "", "")
	skillBody := `{"categoryId":"20000000-0000-0000-0000-000000000002","name":"Figma","slug":"figma","description":"Дизайн інтерфейсів"}`

	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/admin/skills", skillBody, student), http.StatusForbidden, "admin_required")
	created := sendAndDecode[struct {
		Skill catalogSkillResponse `json:"skill"`
	}](t, e, http.MethodPost, "/api/v1/admin/skills", skillBody, admin, http.StatusCreated).Skill
	expect.Equal(t, "created skill", created.Slug, "figma")
	expect.Equal(t, "created skill is active", created.IsActive, true)
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/admin/skills", skillBody, admin), http.StatusConflict, "skill_already_exists")
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/admin/skills", `{"name":"Без категорії","slug":"Bad Slug"}`, admin),
		http.StatusUnprocessableEntity, "validation_failed")

	updated := sendAndDecode[struct {
		Skill catalogSkillResponse `json:"skill"`
	}](t, e, http.MethodPatch, "/api/v1/admin/skills/"+created.ID, `{"isActive":false}`, admin, http.StatusOK).Skill
	expect.Equal(t, "hidden skill", updated.IsActive, false)
	assertProblem(t, send(t, e.handler, http.MethodPatch, "/api/v1/admin/skills/"+datatest.DemoUniversityID, `{"isActive":false}`, admin),
		http.StatusNotFound, "skill_not_found")

	// The public catalog no longer offers the hidden skill.
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/me/teaching-skills", `{"skillId":"`+created.ID+`","level":"ADVANCED"}`, student),
		http.StatusNotFound, "skill_not_found")
	catalog := sendAndDecode[pageOf[catalogSkillResponse]](t, e, http.MethodGet, "/api/v1/admin/skills?q=fig", "", admin, http.StatusOK)
	expect.Equal(t, "hidden skill in the administrator catalog", catalog.Total, 1)
	expect.Equal(t, "catalog page size", catalog.PageSize, 50)

	log := sendAndDecode[pageOf[moderationActionResponse]](t, e, http.MethodGet, "/api/v1/admin/moderation-actions", "", admin, http.StatusOK)
	expect.Equal(t, "catalog log entries", log.Total, 2)
	expect.Equal(t, "logged skill", log.Items[0].Skill.ID, created.ID)
}

func TestChangePasswordAndDeleteAccountRoutes(t *testing.T) {
	e, olha, andrii, _, _, exchangeID := newExchangeEnv(t)
	newPassword := "another long passphrase"

	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/auth/change-password",
		`{"currentPassword":"wrong password!!","newPassword":"`+newPassword+`"}`, andrii), http.StatusUnprocessableEntity, "validation_failed")
	other := login(t, e.handler, "andrii@students.example.test", testPassword)
	expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/auth/change-password",
		`{"currentPassword":"`+testPassword+`","newPassword":"`+newPassword+`"}`, andrii), http.StatusOK)
	expectStatus(t, send(t, e.handler, http.MethodGet, "/api/v1/auth/me", "", andrii), http.StatusOK)
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/auth/me", "", other), http.StatusUnauthorized, "unauthenticated")

	assertProblem(t, send(t, e.handler, http.MethodDelete, "/api/v1/me/account", `{"password":"`+testPassword+`"}`, andrii),
		http.StatusUnprocessableEntity, "validation_failed")
	response := send(t, e.handler, http.MethodDelete, "/api/v1/me/account", `{"password":"`+newPassword+`"}`, andrii)
	expectStatus(t, response, http.StatusNoContent)
	expect.Equal(t, "cookie is cleared", sessionCookie(response).MaxAge, -1)

	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/auth/me", "", andrii), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/auth/login", loginBody("andrii@students.example.test", newPassword), nil),
		http.StatusUnauthorized, "invalid_credentials")

	exchange := getExchangeDetails(t, e, exchangeID, olha).Exchange
	expect.Equal(t, "exchange status", exchange.Status, data.ExchangeCancelled)
	expect.Equal(t, "partner name", exchange.Partner.FirstName+" "+exchange.Partner.LastName, "Видалений користувач")
}

// Checks the seeded administrator, reports and blocks through the real database. The privacy
// change is committed and undone at the end. Skipped unless TEST_DATABASE_URL is set.
func TestModerationAgainstLocalPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for the local PostgreSQL integration test")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	e := &discoveryEnv{handler: postgresHandler(pool, &mailertest.Recorder{})}
	const demoPassword = "SkillSwapDemo2026"
	admin := login(t, e.handler, "demo.admin@students.example.test", demoPassword)
	olha := login(t, e.handler, "demo.olha@students.example.test", demoPassword)

	open := sendAndDecode[pageOf[adminReportResponse]](t, e, http.MethodGet, "/api/v1/admin/reports?status=OPEN", "", admin, http.StatusOK)
	expect.Equal(t, "open reports", open.Total >= 1, true)
	decided := sendAndDecode[struct {
		Report adminReportResponse `json:"report"`
	}](t, e, http.MethodGet, "/api/v1/admin/reports/a0000000-0000-0000-0000-000000000002", "", admin, http.StatusOK).Report
	expect.Equal(t, "seeded decision", decided.Decision.Action, data.ActionDismissReport)
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/admin/reports", "", olha), http.StatusForbidden, "admin_required")

	blocks := sendAndDecode[pageOf[blockResponse]](t, e, http.MethodGet, "/api/v1/me/blocks", "", olha, http.StatusOK)
	expect.Equal(t, "Olha's blocked user", blocks.Items[0].User.ID, "60000000-0000-0000-0000-000000000010")

	t.Cleanup(func() {
		send(t, e.handler, http.MethodPatch, "/api/v1/me/privacy", `{"isDiscoverable":true}`, olha)
	})
	hidden := sendAndDecode[privacyBody](t, e, http.MethodPatch, "/api/v1/me/privacy", `{"isDiscoverable":false}`, olha, http.StatusOK)
	expect.Equal(t, "Olha is hidden", hidden.Privacy.IsDiscoverable, false)
	andrii := login(t, e.handler, "demo.andrii@students.example.test", demoPassword)
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/students/60000000-0000-0000-0000-000000000001", "", andrii),
		http.StatusNotFound, "student_not_found")
}
