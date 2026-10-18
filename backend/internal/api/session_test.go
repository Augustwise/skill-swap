package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/expect"
)

type exchangePageResponse struct {
	Items    []exchangeResponse `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Total    int                `json:"total"`
}

type schedulePageResponse struct {
	Items                 []sessionResponse `json:"items"`
	Page                  int               `json:"page"`
	PageSize              int               `json:"pageSize"`
	Total                 int               `json:"total"`
	Timezone              string            `json:"timezone"`
	PendingProposalsCount int               `json:"pendingProposalsCount"`
}

func newExchangeEnv(t *testing.T) (e *discoveryEnv, olha, andrii *session, olhaID, andriiID, exchangeID string) {
	t.Helper()
	e = newDiscoveryEnv()
	olha = e.student(t, "olha@students.example.test", `["ONLINE","OFFLINE"]`, "Київ", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	andrii = e.student(t, "andrii@students.example.test", `["ONLINE","OFFLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	olhaID = e.meID(t, olha)
	andriiID = e.meID(t, andrii)

	response := send(t, e.handler, http.MethodPost, requestsPath,
		requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID), olha)
	expectStatus(t, response, http.StatusCreated)
	var created struct {
		Request exchangeRequest `json:"request"`
	}
	decodeJSON(t, response, &created)

	response = send(t, e.handler, http.MethodPost, requestsPath+"/"+created.Request.ID+"/accept", "", andrii)
	expectStatus(t, response, http.StatusOK)
	var accepted struct {
		Exchange exchangeResponse `json:"exchange"`
	}
	decodeJSON(t, response, &accepted)
	return e, olha, andrii, olhaID, andriiID, accepted.Exchange.ID
}

func TestExchangesSessionsAndScheduleFlow(t *testing.T) {
	e, olha, andrii, olhaID, andriiID, exchangeID := newExchangeEnv(t)

	// 1. GET /api/v1/me/exchanges shows the active exchange with partner, progress and null nextSession.
	exPage := sendAndDecode[exchangePageResponse](t, e, http.MethodGet, "/api/v1/me/exchanges?status=ACTIVE", "", olha, http.StatusOK)
	expect.Equal(t, "active exchanges total", exPage.Total, 1)
	expect.Equal(t, "active exchanges", len(exPage.Items), 1)
	expect.Equal(t, "page size", exPage.PageSize, 20)
	ex := exPage.Items[0]
	expect.Equal(t, "exchange", ex.ID, exchangeID)
	expect.Equal(t, "Olha's partner", ex.Partner.ID, andriiID)
	expect.Equal(t, "total sessions", ex.Progress.TotalSessions, 3)
	expect.Equal(t, "draft sessions", ex.Progress.DraftSessions, 3)
	expect.Equal(t, "completed sessions", ex.Progress.CompletedSessions, 0)
	expect.Equal(t, "percent", ex.Progress.Percent, 0)
	expect.Equal(t, "next session is set", ex.NextSession != nil, false)

	// 2. GET /api/v1/exchanges/{exchangeId}/sessions lists the 3 initial DRAFT slots.
	sessionsPath := "/api/v1/exchanges/" + exchangeID + "/sessions"
	response := send(t, e.handler, http.MethodGet, sessionsPath, "", andrii)
	expectStatus(t, response, http.StatusOK)
	var exSessions struct {
		Exchange exchangeResponse  `json:"exchange"`
		Items    []sessionResponse `json:"items"`
	}
	decodeJSON(t, response, &exSessions)
	expect.Equal(t, "Andrii's partner", exSessions.Exchange.Partner.ID, olhaID)
	expect.Equal(t, "exchange sessions", len(exSessions.Items), 3)
	expect.Equal(t, "Andrii's role in session 1", exSessions.Items[0].Role, "LEARNER")
	expect.Equal(t, "session 1 teacher", exSessions.Items[0].Teacher.ID, olhaID)
	expect.Equal(t, "Andrii's role in session 3", exSessions.Items[2].Role, "TEACHER")
	expect.Equal(t, "session 3 teacher", exSessions.Items[2].Teacher.ID, andriiID)

	// 3. Olha proposes Session 1 online via POST /api/v1/exchanges/{exchangeId}/sessions.
	t1Start := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Minute)
	t1End := t1Start.Add(60 * time.Minute)
	proposeBody := fmt.Sprintf(`{
		"title": "Акорди та ритм",
		"description": "Розберемо переходи між Am, Dm, E.",
		"teacherId": %q,
		"startsAt": %q,
		"endsAt": %q,
		"format": "ONLINE",
		"meetingUrl": "https://meet.example.test/skillswap-guitar-1"
	}`, olhaID, t1Start.Format(time.RFC3339), t1End.Format(time.RFC3339))

	response = send(t, e.handler, http.MethodPost, sessionsPath, proposeBody, olha)
	expectStatus(t, response, http.StatusCreated)
	var proposed struct {
		Session  sessionResponse         `json:"session"`
		Proposal sessionProposalResponse `json:"proposal"`
	}
	decodeJSON(t, response, &proposed)
	s1 := proposed.Session
	expect.Equal(t, "status after proposal", s1.Status, "PROPOSED")
	expect.Equal(t, "title", s1.Title, "Акорди та ритм")
	expect.Equal(t, "description", valueOf(s1.Description), "Розберемо переходи між Am, Dm, E.")
	expect.Equal(t, "meeting URL", valueOf(s1.MeetingURL), "https://meet.example.test/skillswap-guitar-1")
	expect.Equal(t, "meeting place is set", s1.MeetingPlace != nil, false)
	expect.Equal(t, "duration in minutes", valueOf(s1.DurationMinutes), 60)
	expect.Equal(t, "time confirmed by Olha", s1.ScheduleConfirmedByMe, true)
	expect.Equal(t, "time confirmed by Andrii, as Olha sees it", s1.ScheduleConfirmedByPartner, false)
	expect.Equal(t, "Olha can respond to her proposal", s1.CanRespondToProposal, false)
	expect.Equal(t, "Olha can cancel", s1.CanCancel, true)

	// 4. Andrii inspects GET /api/v1/sessions/{sessionId} and sees CanRespondToProposal = true.
	s1Path := "/api/v1/sessions/" + s1.ID
	response = send(t, e.handler, http.MethodGet, s1Path, "", andrii)
	expectStatus(t, response, http.StatusOK)
	var details struct {
		Session   sessionResponse           `json:"session"`
		Proposals []sessionProposalResponse `json:"proposals"`
	}
	decodeJSON(t, response, &details)
	expect.Equal(t, "Andrii can respond to the proposal", details.Session.CanRespondToProposal, true)
	expect.Equal(t, "time confirmed by Andrii", details.Session.ScheduleConfirmedByMe, false)
	expect.Equal(t, "time confirmed by Olha, as Andrii sees it", details.Session.ScheduleConfirmedByPartner, true)
	expect.Equal(t, "proposals", len(details.Proposals), 1)

	// Andrii sees pendingProposalsCount = 1 on GET /api/v1/me/schedule.
	sched := sendAndDecode[schedulePageResponse](t, e, http.MethodGet, "/api/v1/me/schedule", "", andrii, http.StatusOK)
	expect.Equal(t, "Andrii's scheduled sessions before accept", sched.Total, 0)
	expect.Equal(t, "Andrii's pending proposals", sched.PendingProposalsCount, 1)
	expect.Equal(t, "timezone", sched.Timezone, "Europe/Kyiv")

	// 5. Olha cannot accept her own proposal (403 action_not_allowed); Andrii accepts it.
	assertProblem(t, send(t, e.handler, http.MethodPost, s1Path+"/accept", "", olha),
		http.StatusForbidden, "action_not_allowed")

	response = send(t, e.handler, http.MethodPost, s1Path+"/accept", "", andrii)
	expectStatus(t, response, http.StatusOK)
	decodeJSON(t, response, &proposed)
	expect.Equal(t, "status after accept", proposed.Session.Status, "SCHEDULED")
	expect.Equal(t, "time confirmed by Andrii after accept", proposed.Session.ScheduleConfirmedByMe, true)
	expect.Equal(t, "time confirmed by Olha after accept", proposed.Session.ScheduleConfirmedByPartner, true)
	expect.Equal(t, "Andrii can respond to the proposal after accept", proposed.Session.CanRespondToProposal, false)
	expect.Equal(t, "proposal status after accept", proposed.Proposal.Status, "ACCEPTED")
	expect.Equal(t, "proposal accepted by", valueOf(proposed.Proposal.RespondedBy).ID, andriiID)

	// 6. Both participants now see the scheduled meeting in GET /api/v1/me/schedule and in exchange NextSession.
	for _, viewer := range []struct {
		name    string
		session *session
	}{{"Olha", olha}, {"Andrii", andrii}} {
		sched = sendAndDecode[schedulePageResponse](t, e, http.MethodGet, "/api/v1/me/schedule?upcoming=true", "", viewer.session, http.StatusOK)
		expect.Equal(t, viewer.name+": scheduled sessions", sched.Total, 1)
		expect.Equal(t, viewer.name+": schedule items", len(sched.Items), 1)
		expect.Equal(t, viewer.name+": scheduled session", sched.Items[0].ID, s1.ID)
		expect.Equal(t, viewer.name+": meeting URL", valueOf(sched.Items[0].MeetingURL), "https://meet.example.test/skillswap-guitar-1")
		expect.Equal(t, viewer.name+": pending proposals", sched.PendingProposalsCount, 0)
	}

	exDetail := getExchangeDetails(t, e, exchangeID, olha)
	expect.Equal(t, "next session", valueOf(exDetail.Exchange.NextSession).ID, s1.ID)
	expect.Equal(t, "scheduled sessions", exDetail.Exchange.Progress.ScheduledSessions, 1)
	expect.Equal(t, "sessions in exchange details", len(exDetail.Sessions), 3)

	// 7. Overlapping proposal [t1Start+30m, t1End+30m) is rejected with 409 session_conflict.
	conflictBody := fmt.Sprintf(`{
		"title": "Перетин часу",
		"teacherId": %q,
		"startsAt": %q,
		"endsAt": %q,
		"format": "ONLINE",
		"meetingUrl": "https://meet.example.test/conflict"
	}`, andriiID, t1Start.Add(30*time.Minute).Format(time.RFC3339), t1End.Add(30*time.Minute).Format(time.RFC3339))
	assertProblem(t, send(t, e.handler, http.MethodPost, sessionsPath, conflictBody, andrii),
		http.StatusConflict, "session_conflict")

	// 8. Offline proposal on Session 3, then declined by Olha.
	s3ID := exSessions.Items[2].ID
	t2Start := t1End // back-to-back with s1 is allowed!
	t2End := t2Start.Add(120 * time.Minute)
	offlineBody := fmt.Sprintf(`{
		"title": "Photoshop — ретуш наживо",
		"startsAt": %q,
		"endsAt": %q,
		"format": "OFFLINE",
		"meetingPlace": "Бібліотека КПІ, зал 4"
	}`, t2Start.Format(time.RFC3339), t2End.Format(time.RFC3339))
	response = send(t, e.handler, http.MethodPost, "/api/v1/sessions/"+s3ID+"/proposals", offlineBody, andrii)
	expectStatus(t, response, http.StatusCreated)
	decodeJSON(t, response, &proposed)
	expect.Equal(t, "offline proposal format", proposed.Session.Format, "OFFLINE")
	expect.Equal(t, "meeting place", valueOf(proposed.Session.MeetingPlace), "Бібліотека КПІ, зал 4")
	expect.Equal(t, "meeting URL is set for an offline session", proposed.Session.MeetingURL != nil, false)

	response = send(t, e.handler, http.MethodPost, "/api/v1/sessions/"+s3ID+"/decline", "", olha)
	expectStatus(t, response, http.StatusOK)
	decodeJSON(t, response, &proposed)
	expect.Equal(t, "status after decline", proposed.Session.Status, "DRAFT")
	expect.Equal(t, "proposal status after decline", proposed.Proposal.Status, "DECLINED")

	// 9. Cancel Session 1 with a reason -> removed from active schedule and does not increase progress.
	response = send(t, e.handler, http.MethodPost, s1Path+"/cancel", `{"reason":"Захворів"}`, andrii)
	expectStatus(t, response, http.StatusOK)
	var cancelResp struct {
		Session sessionResponse `json:"session"`
	}
	decodeJSON(t, response, &cancelResp)
	expect.Equal(t, "status after cancel", cancelResp.Session.Status, "CANCELLED")
	expect.Equal(t, "cancellation reason", valueOf(cancelResp.Session.CancellationReason), "Захворів")
	expect.Equal(t, "a cancelled session can be cancelled", cancelResp.Session.CanCancel, false)

	// Cancel again without a body is idempotent.
	expectStatus(t, send(t, e.handler, http.MethodPost, s1Path+"/cancel", "", andrii), http.StatusOK)

	sched = sendAndDecode[schedulePageResponse](t, e, http.MethodGet, "/api/v1/me/schedule", "", olha, http.StatusOK)
	expect.Equal(t, "Olha's scheduled sessions after cancel", sched.Total, 0)
	expect.Equal(t, "Olha's schedule items after cancel", len(sched.Items), 0)

	// Mark one session COMPLETED in the store and check progress on /me/exchanges.
	e.store.SetSessionStatus(exSessions.Items[1].ID, data.SessionCompleted)
	exPage = sendAndDecode[exchangePageResponse](t, e, http.MethodGet, "/api/v1/me/exchanges", "", olha, http.StatusOK)
	prog := exPage.Items[0].Progress
	expect.Equal(t, "final completed sessions", prog.CompletedSessions, 1)
	expect.Equal(t, "final cancelled sessions", prog.CancelledSessions, 1)
	expect.Equal(t, "final total sessions", prog.TotalSessions, 3)
	expect.Equal(t, "final percent", prog.Percent, 33)
}

func TestSessionAccessAndValidationErrors(t *testing.T) {
	e, olha, andrii, olhaID, _, exchangeID := newExchangeEnv(t)
	stranger := e.student(t, "marko@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)

	sessionsPath := "/api/v1/exchanges/" + exchangeID + "/sessions"
	response := send(t, e.handler, http.MethodGet, sessionsPath, "", olha)
	expectStatus(t, response, http.StatusOK)
	var exSessions struct {
		Items []sessionResponse `json:"items"`
	}
	decodeJSON(t, response, &exSessions)
	s1ID := exSessions.Items[0].ID
	s1Path := "/api/v1/sessions/" + s1ID

	// Unauthenticated and CSRF/Origin checks.
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/exchanges", "", nil), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/schedule", "", nil), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodGet, sessionsPath, "", nil), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodGet, s1Path, "", nil), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodPost, s1Path+"/accept", "", &session{cookie: olha.cookie}),
		http.StatusForbidden, "csrf_invalid")

	foreignReq := httptest.NewRequest(http.MethodPost, s1Path+"/accept", nil)
	foreignReq.Header.Set("Origin", "https://evil.example")
	foreignReq.Header.Set(csrfHeaderName, olha.csrf)
	foreignReq.AddCookie(olha.cookie)
	foreignResp := httptest.NewRecorder()
	e.handler.ServeHTTP(foreignResp, foreignReq)
	assertProblem(t, foreignResp, http.StatusForbidden, "origin_forbidden")

	// Third-party access is denied with 404.
	assertProblem(t, send(t, e.handler, http.MethodGet, sessionsPath, "", stranger), http.StatusNotFound, "exchange_not_found")
	assertProblem(t, send(t, e.handler, http.MethodPost, sessionsPath, `{}`, stranger), http.StatusNotFound, "exchange_not_found")
	assertProblem(t, send(t, e.handler, http.MethodGet, s1Path, "", stranger), http.StatusNotFound, "session_not_found")
	assertProblem(t, send(t, e.handler, http.MethodPost, s1Path+"/proposals", `{}`, stranger), http.StatusNotFound, "session_not_found")
	assertProblem(t, send(t, e.handler, http.MethodPost, s1Path+"/accept", "", stranger), http.StatusNotFound, "session_not_found")
	assertProblem(t, send(t, e.handler, http.MethodPost, s1Path+"/decline", "", stranger), http.StatusNotFound, "session_not_found")
	assertProblem(t, send(t, e.handler, http.MethodPost, s1Path+"/cancel", "", stranger), http.StatusNotFound, "session_not_found")

	// Validation errors.
	tStart := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	tEnd := time.Now().UTC().Add(25 * time.Hour).Format(time.RFC3339)
	assertFieldError(t, send(t, e.handler, http.MethodPost, sessionsPath,
		fmt.Sprintf(`{"title":"","teacherId":%q,"startsAt":%q,"endsAt":%q,"format":"ONLINE","meetingUrl":"https://meet.example.test/1"}`,
			olhaID, tStart, tEnd), olha), "title")
	assertFieldError(t, send(t, e.handler, http.MethodPost, sessionsPath,
		fmt.Sprintf(`{"title":"Тема","teacherId":%q,"startsAt":%q,"endsAt":%q,"format":"ONLINE","meetingUrl":"ftp://bad"}`,
			olhaID, tStart, tEnd), olha), "meetingUrl")
	assertFieldError(t, send(t, e.handler, http.MethodGet, "/api/v1/me/exchanges?status=PAUSED", "", olha), "status")
	assertFieldError(t, send(t, e.handler, http.MethodGet, "/api/v1/me/schedule?status=DRAFT", "", olha), "status")
	assertFieldError(t, send(t, e.handler, http.MethodGet, "/api/v1/me/schedule?upcoming=maybe", "", olha), "upcoming")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/exchanges?page=0", "", olha), http.StatusBadRequest, "invalid_page")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/schedule?page=0", "", olha), http.StatusBadRequest, "invalid_page")

	// State errors on a DRAFT session.
	assertProblem(t, send(t, e.handler, http.MethodPost, s1Path+"/accept", "", andrii), http.StatusConflict, "session_not_proposed")
	assertProblem(t, send(t, e.handler, http.MethodPost, s1Path+"/decline", "", andrii), http.StatusConflict, "session_not_proposed")
	assertProblem(t, send(t, e.handler, http.MethodPost, s1Path+"/cancel", "", andrii), http.StatusConflict, "session_not_cancellable")
	assertFieldError(t, send(t, e.handler, http.MethodPost, s1Path+"/cancel",
		fmt.Sprintf(`{"reason":%q}`, strings.Repeat("х", 501)), andrii), "reason")
}
