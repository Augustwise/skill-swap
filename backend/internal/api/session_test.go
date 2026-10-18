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
	response := send(t, e.handler, http.MethodGet, "/api/v1/me/exchanges?status=ACTIVE", "", olha)
	expectStatus(t, response, http.StatusOK)
	var exPage exchangePageResponse
	decodeJSON(t, response, &exPage)
	if exPage.Total != 1 || len(exPage.Items) != 1 || exPage.PageSize != 20 {
		t.Fatalf("exchanges page = %+v", exPage)
	}
	ex := exPage.Items[0]
	if ex.ID != exchangeID || ex.Partner.ID != andriiID || ex.Progress.TotalSessions != 3 ||
		ex.Progress.DraftSessions != 3 || ex.Progress.CompletedSessions != 0 ||
		ex.Progress.Percent != 0 || ex.NextSession != nil {
		t.Fatalf("exchange item = %+v", ex)
	}

	// 2. GET /api/v1/exchanges/{exchangeId}/sessions lists the 3 initial DRAFT slots.
	sessionsPath := "/api/v1/exchanges/" + exchangeID + "/sessions"
	response = send(t, e.handler, http.MethodGet, sessionsPath, "", andrii)
	expectStatus(t, response, http.StatusOK)
	var exSessions struct {
		Exchange exchangeResponse  `json:"exchange"`
		Items    []sessionResponse `json:"items"`
	}
	decodeJSON(t, response, &exSessions)
	if exSessions.Exchange.Partner.ID != olhaID || len(exSessions.Items) != 3 {
		t.Fatalf("exchange sessions = %+v", exSessions)
	}
	if exSessions.Items[0].Role != "LEARNER" || exSessions.Items[0].Teacher.ID != olhaID ||
		exSessions.Items[2].Role != "TEACHER" || exSessions.Items[2].Teacher.ID != andriiID {
		t.Fatalf("roles and teachers = %+v", exSessions.Items)
	}

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
	if s1.Status != "PROPOSED" || s1.Title != "Акорди та ритм" ||
		s1.Description == nil || *s1.Description != "Розберемо переходи між Am, Dm, E." ||
		s1.MeetingURL == nil || *s1.MeetingURL != "https://meet.example.test/skillswap-guitar-1" ||
		s1.MeetingPlace != nil || s1.DurationMinutes == nil || *s1.DurationMinutes != 60 ||
		!s1.ScheduleConfirmedByMe || s1.ScheduleConfirmedByPartner || s1.CanRespondToProposal || !s1.CanCancel {
		t.Fatalf("proposed session (Olha view) = %+v", s1)
	}

	// 4. Andrii inspects GET /api/v1/sessions/{sessionId} and sees CanRespondToProposal = true.
	s1Path := "/api/v1/sessions/" + s1.ID
	response = send(t, e.handler, http.MethodGet, s1Path, "", andrii)
	expectStatus(t, response, http.StatusOK)
	var details struct {
		Session   sessionResponse           `json:"session"`
		Proposals []sessionProposalResponse `json:"proposals"`
	}
	decodeJSON(t, response, &details)
	if !details.Session.CanRespondToProposal || details.Session.ScheduleConfirmedByMe ||
		!details.Session.ScheduleConfirmedByPartner || len(details.Proposals) != 1 {
		t.Fatalf("Andrii session details = %+v", details)
	}

	// Andrii sees pendingProposalsCount = 1 on GET /api/v1/me/schedule.
	response = send(t, e.handler, http.MethodGet, "/api/v1/me/schedule", "", andrii)
	expectStatus(t, response, http.StatusOK)
	var sched schedulePageResponse
	decodeJSON(t, response, &sched)
	if sched.Total != 0 || sched.PendingProposalsCount != 1 || sched.Timezone != "Europe/Kyiv" {
		t.Fatalf("Andrii schedule before accept = %+v", sched)
	}

	// 5. Olha cannot accept her own proposal (403 action_not_allowed); Andrii accepts it.
	assertProblem(t, send(t, e.handler, http.MethodPost, s1Path+"/accept", "", olha),
		http.StatusForbidden, "action_not_allowed")

	response = send(t, e.handler, http.MethodPost, s1Path+"/accept", "", andrii)
	expectStatus(t, response, http.StatusOK)
	decodeJSON(t, response, &proposed)
	if proposed.Session.Status != "SCHEDULED" || !proposed.Session.ScheduleConfirmedByMe ||
		!proposed.Session.ScheduleConfirmedByPartner || proposed.Session.CanRespondToProposal ||
		proposed.Proposal.Status != "ACCEPTED" || proposed.Proposal.RespondedBy == nil ||
		proposed.Proposal.RespondedBy.ID != andriiID {
		t.Fatalf("accepted session = %+v, proposal = %+v", proposed.Session, proposed.Proposal)
	}

	// 6. Both participants now see the scheduled meeting in GET /api/v1/me/schedule and in exchange NextSession.
	for _, s := range []*session{olha, andrii} {
		response = send(t, e.handler, http.MethodGet, "/api/v1/me/schedule?upcoming=true", "", s)
		expectStatus(t, response, http.StatusOK)
		decodeJSON(t, response, &sched)
		if sched.Total != 1 || len(sched.Items) != 1 || sched.Items[0].ID != s1.ID ||
			sched.Items[0].MeetingURL == nil || *sched.Items[0].MeetingURL != "https://meet.example.test/skillswap-guitar-1" ||
			sched.PendingProposalsCount != 0 {
			t.Fatalf("schedule after accept = %+v", sched)
		}
	}

	response = send(t, e.handler, http.MethodGet, "/api/v1/exchanges/"+exchangeID, "", olha)
	expectStatus(t, response, http.StatusOK)
	var exDetail struct {
		Exchange exchangeResponse  `json:"exchange"`
		Sessions []sessionResponse `json:"sessions"`
	}
	decodeJSON(t, response, &exDetail)
	if exDetail.Exchange.NextSession == nil || exDetail.Exchange.NextSession.ID != s1.ID ||
		exDetail.Exchange.Progress.ScheduledSessions != 1 || len(exDetail.Sessions) != 3 {
		t.Fatalf("exchange detail = %+v", exDetail)
	}

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
	if proposed.Session.Format != "OFFLINE" || proposed.Session.MeetingPlace == nil ||
		*proposed.Session.MeetingPlace != "Бібліотека КПІ, зал 4" || proposed.Session.MeetingURL != nil {
		t.Fatalf("offline proposal = %+v", proposed.Session)
	}

	response = send(t, e.handler, http.MethodPost, "/api/v1/sessions/"+s3ID+"/decline", "", olha)
	expectStatus(t, response, http.StatusOK)
	decodeJSON(t, response, &proposed)
	if proposed.Session.Status != "DRAFT" || proposed.Proposal.Status != "DECLINED" {
		t.Fatalf("declined session = %+v, proposal = %+v", proposed.Session, proposed.Proposal)
	}

	// 9. Cancel Session 1 with a reason -> removed from active schedule and does not increase progress.
	response = send(t, e.handler, http.MethodPost, s1Path+"/cancel", `{"reason":"Захворів"}`, andrii)
	expectStatus(t, response, http.StatusOK)
	var cancelResp struct {
		Session sessionResponse `json:"session"`
	}
	decodeJSON(t, response, &cancelResp)
	if cancelResp.Session.Status != "CANCELLED" || cancelResp.Session.CancellationReason == nil ||
		*cancelResp.Session.CancellationReason != "Захворів" || cancelResp.Session.CanCancel {
		t.Fatalf("cancelled session = %+v", cancelResp.Session)
	}

	// Cancel again without a body is idempotent.
	expectStatus(t, send(t, e.handler, http.MethodPost, s1Path+"/cancel", "", andrii), http.StatusOK)

	response = send(t, e.handler, http.MethodGet, "/api/v1/me/schedule", "", olha)
	expectStatus(t, response, http.StatusOK)
	decodeJSON(t, response, &sched)
	if sched.Total != 0 || len(sched.Items) != 0 {
		t.Fatalf("schedule after cancel = %+v", sched)
	}

	// Mark one session COMPLETED in the store and check progress on /me/exchanges.
	e.store.SetSessionStatus(exSessions.Items[1].ID, data.SessionCompleted)
	response = send(t, e.handler, http.MethodGet, "/api/v1/me/exchanges", "", olha)
	expectStatus(t, response, http.StatusOK)
	decodeJSON(t, response, &exPage)
	prog := exPage.Items[0].Progress
	if prog.CompletedSessions != 1 || prog.CancelledSessions != 1 || prog.TotalSessions != 3 || prog.Percent != 33 {
		t.Fatalf("progress = %+v", prog)
	}
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
