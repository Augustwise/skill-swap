package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"skillswap/backend/internal/data/datatest"
)

const requestsPath = "/api/v1/exchange-requests"

func requestBody(recipientID, teach, learn string) string {
	return fmt.Sprintf(`{"recipientId":%q,"teachSkillId":%q,"learnSkillId":%q,"format":"ONLINE",
		"teachSessions":2,"teachDurationMinutes":60,"learnSessions":1,"learnDurationMinutes":120}`, recipientID, teach, learn)
}

// newRequestEnv has Olha (teaches guitar, learns Photoshop) and Andrii (the reverse).
func newRequestEnv(t *testing.T) (e *discoveryEnv, olha, andrii *session, andriiID string) {
	t.Helper()
	e = newDiscoveryEnv()
	olha = e.student(t, "olha@students.example.test", `["ONLINE"]`, "Київ", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	andrii = e.student(t, "andrii@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	return e, olha, andrii, e.meID(t, andrii)
}

func TestCreateRequest(t *testing.T) {
	e, olha, _, andriiID := newRequestEnv(t)
	body := strings.Replace(requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID),
		`"format"`, `"message":" Привіт! ","format"`, 1)
	response := send(t, e.handler, http.MethodPost, requestsPath, body, olha)
	expectStatus(t, response, http.StatusCreated)
	var created struct {
		Request exchangeRequest `json:"request"`
	}
	decodeJSON(t, response, &created)
	r := created.Request
	if r.ID == "" || r.Status != "PENDING" || r.Format != "ONLINE" || r.TotalSessions != 3 ||
		r.Message == nil || *r.Message != "Привіт!" || r.ExchangeID != nil || r.RespondedAt != nil || r.CreatedAt.IsZero() {
		t.Fatalf("request = %+v", r)
	}
	if r.Requester.ID != e.meID(t, olha) || r.Recipient.ID != andriiID || r.Recipient.City != "Київ" || r.Recipient.University.Name == "" {
		t.Fatalf("requester = %+v, recipient = %+v", r.Requester, r.Recipient)
	}
	want := requestTerms{Skill: requestSkill{SkillID: datatest.GuitarSkillID, CategoryID: "20000000-0000-0000-0000-000000000001",
		Name: "Гітара"}, TeacherLevel: "ADVANCED", LearnerLevel: "BEGINNER", Sessions: 2, DurationMinutes: 60}
	if r.RequesterTeaches != want || r.RecipientTeaches.Skill.Name != "Photoshop" || r.RecipientTeaches.DurationMinutes != 120 {
		t.Fatalf("terms: %+v, %+v", r.RequesterTeaches, r.RecipientTeaches)
	}

	// Without a message both optional values are null, not omitted.
	e, olha, _, andriiID = newRequestEnv(t)
	response = send(t, e.handler, http.MethodPost, requestsPath, requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID), olha)
	expectStatus(t, response, http.StatusCreated)
	for _, fragment := range []string{`"message":null`, `"exchangeId":null`, `"respondedAt":null`} {
		if !strings.Contains(response.Body.String(), fragment) {
			t.Fatalf("body %s has no %s", response.Body.String(), fragment)
		}
	}
}

func TestCreateRequestAccess(t *testing.T) {
	e, olha, _, andriiID := newRequestEnv(t)
	body := requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	assertProblem(t, send(t, e.handler, http.MethodPost, requestsPath, body, nil), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodPost, requestsPath, body, &session{cookie: olha.cookie}),
		http.StatusForbidden, "csrf_invalid")

	req := httptest.NewRequest(http.MethodPost, requestsPath, strings.NewReader(body))
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeaderName, olha.csrf)
	req.AddCookie(olha.cookie)
	response := httptest.NewRecorder()
	e.handler.ServeHTTP(response, req)
	assertProblem(t, response, http.StatusForbidden, "origin_forbidden")

	expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/auth/register",
		registerBody("pending@students.example.test", testPassword), nil), http.StatusCreated)
	pending := login(t, e.handler, "pending@students.example.test", testPassword)
	assertProblem(t, send(t, e.handler, http.MethodPost, requestsPath, body, pending), http.StatusForbidden, "email_not_verified")

	response = send(t, e.handler, http.MethodGet, requestsPath, "", olha)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("GET status = %d, Allow = %q", response.Code, response.Header().Get("Allow"))
	}
}

func TestCreateRequestErrors(t *testing.T) {
	e, olha, andrii, andriiID := newRequestEnv(t)
	body := requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID)

	assertFieldError(t, send(t, e.handler, http.MethodPost, requestsPath,
		strings.Replace(body, `"learnDurationMinutes":120`, `"learnDurationMinutes":90`, 1), olha), "learnSessions")
	assertFieldError(t, send(t, e.handler, http.MethodPost, requestsPath,
		requestBody(andriiID, datatest.PhotoshopSkillID, datatest.GuitarSkillID), olha), "teachSkillId")
	assertProblem(t, send(t, e.handler, http.MethodPost, requestsPath,
		strings.Replace(body, `"teachSessions":2`, `"teachSessions":"2"`, 1), olha), http.StatusBadRequest, "invalid_json")
	assertProblem(t, send(t, e.handler, http.MethodPost, requestsPath,
		requestBody("40000000-0000-0000-0000-000000000999", datatest.GuitarSkillID, datatest.PhotoshopSkillID), olha),
		http.StatusNotFound, "student_not_found")

	e.store.SetRequestSettings(andriiID, false, false)
	assertProblem(t, send(t, e.handler, http.MethodPost, requestsPath, body, olha), http.StatusConflict, "requests_closed")
	e.store.SetRequestSettings(andriiID, true, false)

	expectStatus(t, send(t, e.handler, http.MethodPost, requestsPath, body, olha), http.StatusCreated)
	assertProblem(t, send(t, e.handler, http.MethodPost, requestsPath, body, olha), http.StatusConflict, "duplicate_request")
	assertProblem(t, send(t, e.handler, http.MethodPost, requestsPath,
		requestBody(e.meID(t, olha), datatest.PhotoshopSkillID, datatest.GuitarSkillID), andrii), http.StatusConflict, "duplicate_request")
}

type requestPageResponse struct {
	Items    []exchangeRequest `json:"items"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
	Total    int               `json:"total"`
}

func (e *discoveryEnv) requests(t *testing.T, s *session, query string) requestPageResponse {
	t.Helper()
	response := send(t, e.handler, http.MethodGet, "/api/v1/me/exchange-requests"+query, "", s)
	expectStatus(t, response, http.StatusOK)
	var body requestPageResponse
	decodeJSON(t, response, &body)
	return body
}

func TestRequestLists(t *testing.T) {
	e, olha, andrii, andriiID := newRequestEnv(t)
	response := send(t, e.handler, http.MethodPost, requestsPath, requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID), olha)
	expectStatus(t, response, http.StatusCreated)
	var created struct {
		Request exchangeRequest `json:"request"`
	}
	decodeJSON(t, response, &created)

	sent := e.requests(t, olha, "?direction=outgoing")
	if len(sent.Items) != 1 || sent.Items[0].ID != created.Request.ID || sent.Items[0].Recipient.ID != andriiID ||
		sent.Page != 1 || sent.PageSize != 20 || sent.Total != 1 {
		t.Fatalf("Olha's sent requests = %+v", sent)
	}
	if incoming := e.requests(t, andrii, "?direction=incoming&status=PENDING"); len(incoming.Items) != 1 || incoming.Total != 1 {
		t.Fatalf("Andrii's incoming requests = %+v", incoming)
	}
	if declined := e.requests(t, andrii, "?direction=incoming&status=DECLINED"); len(declined.Items) != 0 || declined.Total != 0 {
		t.Fatalf("Andrii's declined requests = %+v", declined)
	}
	// An empty page is an empty array, not null.
	response = send(t, e.handler, http.MethodGet, "/api/v1/me/exchange-requests?direction=incoming&page=2", "", andrii)
	expectStatus(t, response, http.StatusOK)
	assertJSONEqual(t, response.Body.Bytes(), `{"items":[],"page":2,"pageSize":20,"total":1}`)

	path := "/api/v1/me/exchange-requests"
	assertProblem(t, send(t, e.handler, http.MethodGet, path+"?direction=incoming", "", nil), http.StatusUnauthorized, "unauthenticated")
	assertFieldError(t, send(t, e.handler, http.MethodGet, path, "", olha), "direction")
	assertFieldError(t, send(t, e.handler, http.MethodGet, path+"?direction=incoming&status=EXPIRED", "", olha), "status")
	assertProblem(t, send(t, e.handler, http.MethodGet, path+"?direction=incoming&page=0", "", olha), http.StatusBadRequest, "invalid_page")
	response = send(t, e.handler, http.MethodPost, path, "{}", olha)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("POST status = %d, Allow = %q", response.Code, response.Header().Get("Allow"))
	}
}

func TestRequestDetails(t *testing.T) {
	e, olha, andrii, andriiID := newRequestEnv(t)
	response := send(t, e.handler, http.MethodPost, requestsPath, requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID), olha)
	expectStatus(t, response, http.StatusCreated)
	var created struct {
		Request exchangeRequest `json:"request"`
	}
	decodeJSON(t, response, &created)
	path := requestsPath + "/" + created.Request.ID

	for _, s := range []*session{olha, andrii} {
		response := send(t, e.handler, http.MethodGet, path, "", s)
		expectStatus(t, response, http.StatusOK)
		var body struct {
			Request exchangeRequest       `json:"request"`
			History []requestStatusChange `json:"history"`
		}
		decodeJSON(t, response, &body)
		if body.Request.ID != created.Request.ID || body.Request.Status != "PENDING" || len(body.History) != 1 {
			t.Fatalf("details = %+v", body)
		}
		if h := body.History[0]; h.Status != "PENDING" || h.ChangedBy == nil || h.ChangedBy.ID != e.meID(t, olha) || h.CreatedAt.IsZero() {
			t.Fatalf("history = %+v", h)
		}
	}

	stranger := e.student(t, "marko@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	assertProblem(t, send(t, e.handler, http.MethodGet, path, "", stranger), http.StatusNotFound, "request_not_found")
	assertProblem(t, send(t, e.handler, http.MethodGet, requestsPath+"/70000000-0000-0000-0000-000000000999", "", olha),
		http.StatusNotFound, "request_not_found")
	assertProblem(t, send(t, e.handler, http.MethodGet, requestsPath+"/latest", "", olha), http.StatusNotFound, "request_not_found")
	assertProblem(t, send(t, e.handler, http.MethodGet, path, "", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestAnswerRequest(t *testing.T) {
	e, olha, andrii, andriiID := newRequestEnv(t)
	create := func() string {
		t.Helper()
		response := send(t, e.handler, http.MethodPost, requestsPath, requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID), olha)
		expectStatus(t, response, http.StatusCreated)
		var created struct {
			Request exchangeRequest `json:"request"`
		}
		decodeJSON(t, response, &created)
		return created.Request.ID
	}
	answer := func(s *session, path string) exchangeRequest {
		t.Helper()
		response := send(t, e.handler, http.MethodPost, path, "", s)
		expectStatus(t, response, http.StatusOK)
		var body struct {
			Request exchangeRequest `json:"request"`
		}
		decodeJSON(t, response, &body)
		return body.Request
	}

	declinePath := requestsPath + "/" + create() + "/decline"
	assertProblem(t, send(t, e.handler, http.MethodPost, declinePath, "", olha), http.StatusForbidden, "action_not_allowed")
	declined := answer(andrii, declinePath)
	if declined.Status != "DECLINED" || declined.RespondedAt == nil {
		t.Fatalf("declined = %+v", declined)
	}
	if again := answer(andrii, declinePath); again.Status != "DECLINED" || !again.RespondedAt.Equal(*declined.RespondedAt) {
		t.Fatalf("declined again = %+v", again)
	}
	assertProblem(t, send(t, e.handler, http.MethodPost, strings.Replace(declinePath, "decline", "withdraw", 1), "", olha),
		http.StatusConflict, "request_not_pending")

	withdrawPath := requestsPath + "/" + create() + "/withdraw"
	assertProblem(t, send(t, e.handler, http.MethodPost, withdrawPath, "", andrii), http.StatusForbidden, "action_not_allowed")
	if withdrawn := answer(olha, withdrawPath); withdrawn.Status != "WITHDRAWN" || withdrawn.RespondedAt == nil {
		t.Fatalf("withdrawn = %+v", withdrawn)
	}
	answer(olha, withdrawPath)
	assertProblem(t, send(t, e.handler, http.MethodPost, strings.Replace(withdrawPath, "withdraw", "decline", 1), "", andrii),
		http.StatusConflict, "request_not_pending")

	response := send(t, e.handler, http.MethodGet, strings.TrimSuffix(withdrawPath, "/withdraw"), "", andrii)
	expectStatus(t, response, http.StatusOK)
	var details struct {
		History []requestStatusChange `json:"history"`
	}
	decodeJSON(t, response, &details)
	if len(details.History) != 2 || details.History[1].Status != "WITHDRAWN" || details.History[1].ChangedBy.ID != e.meID(t, olha) {
		t.Fatalf("history = %+v", details.History)
	}
}

func TestAnswerRequestAccess(t *testing.T) {
	e, olha, andrii, andriiID := newRequestEnv(t)
	response := send(t, e.handler, http.MethodPost, requestsPath, requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID), olha)
	expectStatus(t, response, http.StatusCreated)
	var created struct {
		Request exchangeRequest `json:"request"`
	}
	decodeJSON(t, response, &created)
	path := requestsPath + "/" + created.Request.ID + "/decline"

	assertProblem(t, send(t, e.handler, http.MethodPost, path, "", nil), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodPost, path, "", &session{cookie: andrii.cookie}), http.StatusForbidden, "csrf_invalid")
	stranger := e.student(t, "marko@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	assertProblem(t, send(t, e.handler, http.MethodPost, path, "", stranger), http.StatusNotFound, "request_not_found")
	assertProblem(t, send(t, e.handler, http.MethodPost, requestsPath+"/latest/withdraw", "", olha), http.StatusNotFound, "request_not_found")
	response = send(t, e.handler, http.MethodGet, path, "", andrii)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("GET status = %d, Allow = %q", response.Code, response.Header().Get("Allow"))
	}
}

func TestAcceptRequest(t *testing.T) {
	e, olha, andrii, andriiID := newRequestEnv(t)
	response := send(t, e.handler, http.MethodPost, requestsPath, requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID), olha)
	expectStatus(t, response, http.StatusCreated)
	var created struct {
		Request exchangeRequest `json:"request"`
	}
	decodeJSON(t, response, &created)
	path := requestsPath + "/" + created.Request.ID + "/accept"

	assertProblem(t, send(t, e.handler, http.MethodPost, path, "", nil), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodPost, path, "", &session{cookie: andrii.cookie}), http.StatusForbidden, "csrf_invalid")
	assertProblem(t, send(t, e.handler, http.MethodPost, path, "", olha), http.StatusForbidden, "action_not_allowed")

	type acceptance struct {
		Request  exchangeRequest  `json:"request"`
		Exchange exchangeResponse `json:"exchange"`
	}
	accept := func() acceptance {
		t.Helper()
		response := send(t, e.handler, http.MethodPost, path, "", andrii)
		expectStatus(t, response, http.StatusOK)
		var body acceptance
		decodeJSON(t, response, &body)
		return body
	}
	first := accept()
	r, x := first.Request, first.Exchange
	if r.Status != "ACCEPTED" || r.RespondedAt == nil || r.ExchangeID == nil || *r.ExchangeID != x.ID {
		t.Fatalf("request = %+v", r)
	}
	if x.ID == "" || x.RequestID == nil || *x.RequestID != r.ID || x.Status != "ACTIVE" || x.Format != "ONLINE" ||
		x.TotalSessions != 3 || x.StartedAt == nil || x.Requester.ID != e.meID(t, olha) || x.Recipient.ID != andriiID ||
		x.RequesterTeaches != created.Request.RequesterTeaches || x.RecipientTeaches != created.Request.RecipientTeaches {
		t.Fatalf("exchange = %+v", x)
	}
	if again := accept(); again.Exchange.ID != x.ID {
		t.Fatalf("accepted again: exchange %s, want %s", again.Exchange.ID, x.ID)
	}

	exchangePath := "/api/v1/exchanges/" + x.ID
	for _, s := range []*session{olha, andrii} {
		response := send(t, e.handler, http.MethodGet, exchangePath, "", s)
		expectStatus(t, response, http.StatusOK)
		var body struct {
			Exchange exchangeResponse `json:"exchange"`
		}
		decodeJSON(t, response, &body)
		if body.Exchange.ID != x.ID || body.Exchange.RequesterTeaches != x.RequesterTeaches {
			t.Fatalf("exchange details = %+v", body.Exchange)
		}
	}
	stranger := e.student(t, "marko@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	assertProblem(t, send(t, e.handler, http.MethodGet, exchangePath, "", stranger), http.StatusNotFound, "exchange_not_found")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/exchanges/latest", "", olha), http.StatusNotFound, "exchange_not_found")
	assertProblem(t, send(t, e.handler, http.MethodGet, exchangePath, "", nil), http.StatusUnauthorized, "unauthenticated")
	response = send(t, e.handler, http.MethodPost, exchangePath, "", olha)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("POST status = %d, Allow = %q", response.Code, response.Header().Get("Allow"))
	}
}

func TestAcceptRequestErrors(t *testing.T) {
	e, olha, andrii, andriiID := newRequestEnv(t)
	create := func() string {
		t.Helper()
		response := send(t, e.handler, http.MethodPost, requestsPath, requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID), olha)
		expectStatus(t, response, http.StatusCreated)
		var created struct {
			Request exchangeRequest `json:"request"`
		}
		decodeJSON(t, response, &created)
		return requestsPath + "/" + created.Request.ID
	}

	declined := create()
	expectStatus(t, send(t, e.handler, http.MethodPost, declined+"/decline", "", andrii), http.StatusOK)
	assertProblem(t, send(t, e.handler, http.MethodPost, declined+"/accept", "", andrii), http.StatusConflict, "request_not_pending")

	outdated := create()
	expectStatus(t, send(t, e.handler, http.MethodDelete, "/api/v1/me/teaching-skills/"+datatest.GuitarSkillID, "", olha), http.StatusOK)
	assertProblem(t, send(t, e.handler, http.MethodPost, outdated+"/accept", "", andrii), http.StatusConflict, "request_outdated")
	assertProblem(t, send(t, e.handler, http.MethodPost, requestsPath+"/latest/accept", "", andrii), http.StatusNotFound, "request_not_found")
}
