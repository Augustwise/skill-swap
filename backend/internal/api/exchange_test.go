package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/expect"
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
	expect.Equal(t, "request ID is set", r.ID != "", true)
	expect.Equal(t, "status", r.Status, "PENDING")
	expect.Equal(t, "format", r.Format, "ONLINE")
	expect.Equal(t, "total sessions", r.TotalSessions, 3)
	expect.Equal(t, "message", valueOf(r.Message), "Привіт!")
	expect.Equal(t, "exchange ID is set", r.ExchangeID != nil, false)
	expect.Equal(t, "respondedAt is set", r.RespondedAt != nil, false)
	expect.Equal(t, "createdAt is set", !r.CreatedAt.IsZero(), true)

	expect.Equal(t, "requester", r.Requester.ID, e.meID(t, olha))
	expect.Equal(t, "recipient", r.Recipient.ID, andriiID)
	expect.Equal(t, "recipient city", r.Recipient.City, "Київ")
	expect.Equal(t, "recipient university is set", r.Recipient.University.Name != "", true)

	want := requestTerms{Skill: requestSkill{SkillID: datatest.GuitarSkillID, CategoryID: "20000000-0000-0000-0000-000000000001",
		Name: "Гітара"}, TeacherLevel: "ADVANCED", LearnerLevel: "BEGINNER", Sessions: 2, DurationMinutes: 60}
	expect.Equal(t, "terms Olha teaches", r.RequesterTeaches, want)
	expect.Equal(t, "skill Andrii teaches", r.RecipientTeaches.Skill.Name, "Photoshop")
	expect.Equal(t, "length of Andrii's lessons", r.RecipientTeaches.DurationMinutes, 120)

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
	expect.Equal(t, "GET status", response.Code, http.StatusMethodNotAllowed)
	expect.Equal(t, "Allow header", response.Header().Get("Allow"), http.MethodPost)
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
	return sendAndDecode[requestPageResponse](t, e, http.MethodGet, "/api/v1/me/exchange-requests"+query, "", s, http.StatusOK)
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
	expect.Equal(t, "Olha's sent requests", len(sent.Items), 1)
	expect.Equal(t, "sent request", sent.Items[0].ID, created.Request.ID)
	expect.Equal(t, "sent request recipient", sent.Items[0].Recipient.ID, andriiID)
	expect.Equal(t, "page", sent.Page, 1)
	expect.Equal(t, "page size", sent.PageSize, 20)
	expect.Equal(t, "total sent requests", sent.Total, 1)

	incoming := e.requests(t, andrii, "?direction=incoming&status=PENDING")
	expect.Equal(t, "Andrii's incoming requests", len(incoming.Items), 1)
	expect.Equal(t, "total incoming requests", incoming.Total, 1)

	declined := e.requests(t, andrii, "?direction=incoming&status=DECLINED")
	expect.Equal(t, "Andrii's declined requests", len(declined.Items), 0)
	expect.Equal(t, "total declined requests", declined.Total, 0)

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
	expect.Equal(t, "POST status", response.Code, http.StatusMethodNotAllowed)
	expect.Equal(t, "Allow header", response.Header().Get("Allow"), http.MethodGet)
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
	olhaID := e.meID(t, olha)

	for _, viewer := range []struct {
		name    string
		session *session
	}{{"Olha", olha}, {"Andrii", andrii}} {
		response := send(t, e.handler, http.MethodGet, path, "", viewer.session)
		expectStatus(t, response, http.StatusOK)
		var body struct {
			Request exchangeRequest       `json:"request"`
			History []requestStatusChange `json:"history"`
		}
		decodeJSON(t, response, &body)
		expect.Equal(t, viewer.name+": request", body.Request.ID, created.Request.ID)
		expect.Equal(t, viewer.name+": status", body.Request.Status, "PENDING")
		expect.Equal(t, viewer.name+": history items", len(body.History), 1)
		h := body.History[0]
		expect.Equal(t, viewer.name+": history status", h.Status, "PENDING")
		expect.Equal(t, viewer.name+": history author", valueOf(h.ChangedBy).ID, olhaID)
		expect.Equal(t, viewer.name+": history createdAt is set", !h.CreatedAt.IsZero(), true)
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
	expect.Equal(t, "status after decline", declined.Status, "DECLINED")
	expect.Equal(t, "respondedAt is set after decline", declined.RespondedAt != nil, true)

	// A repeated decline keeps the first answer time.
	again := answer(andrii, declinePath)
	expect.Equal(t, "status after a repeated decline", again.Status, "DECLINED")
	expect.Equal(t, "respondedAt unchanged by a repeated decline", valueOf(again.RespondedAt).Equal(*declined.RespondedAt), true)
	assertProblem(t, send(t, e.handler, http.MethodPost, strings.Replace(declinePath, "decline", "withdraw", 1), "", olha),
		http.StatusConflict, "request_not_pending")

	withdrawPath := requestsPath + "/" + create() + "/withdraw"
	assertProblem(t, send(t, e.handler, http.MethodPost, withdrawPath, "", andrii), http.StatusForbidden, "action_not_allowed")
	withdrawn := answer(olha, withdrawPath)
	expect.Equal(t, "status after withdrawal", withdrawn.Status, "WITHDRAWN")
	expect.Equal(t, "respondedAt is set after withdrawal", withdrawn.RespondedAt != nil, true)
	answer(olha, withdrawPath)
	assertProblem(t, send(t, e.handler, http.MethodPost, strings.Replace(withdrawPath, "withdraw", "decline", 1), "", andrii),
		http.StatusConflict, "request_not_pending")

	response := send(t, e.handler, http.MethodGet, strings.TrimSuffix(withdrawPath, "/withdraw"), "", andrii)
	expectStatus(t, response, http.StatusOK)
	var details struct {
		History []requestStatusChange `json:"history"`
	}
	decodeJSON(t, response, &details)
	expect.Equal(t, "history items", len(details.History), 2)
	expect.Equal(t, "last history status", details.History[1].Status, "WITHDRAWN")
	expect.Equal(t, "withdrawn by", valueOf(details.History[1].ChangedBy).ID, e.meID(t, olha))
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
	expect.Equal(t, "GET status", response.Code, http.StatusMethodNotAllowed)
	expect.Equal(t, "Allow header", response.Header().Get("Allow"), http.MethodPost)
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
		return sendAndDecode[acceptance](t, e, http.MethodPost, path, "", andrii, http.StatusOK)
	}
	first := accept()
	r, x := first.Request, first.Exchange
	expect.Equal(t, "request status", r.Status, "ACCEPTED")
	expect.Equal(t, "respondedAt is set", r.RespondedAt != nil, true)
	expect.Equal(t, "exchange ID in the request", valueOf(r.ExchangeID), x.ID)

	expect.Equal(t, "exchange ID is set", x.ID != "", true)
	expect.Equal(t, "request ID in the exchange", valueOf(x.RequestID), r.ID)
	expect.Equal(t, "exchange status", x.Status, "ACTIVE")
	expect.Equal(t, "exchange format", x.Format, "ONLINE")
	expect.Equal(t, "exchange total sessions", x.TotalSessions, 3)
	expect.Equal(t, "startedAt is set", x.StartedAt != nil, true)
	expect.Equal(t, "exchange requester", x.Requester.ID, e.meID(t, olha))
	expect.Equal(t, "exchange recipient", x.Recipient.ID, andriiID)
	expect.Equal(t, "terms Olha teaches", x.RequesterTeaches, created.Request.RequesterTeaches)
	expect.Equal(t, "terms Andrii teaches", x.RecipientTeaches, created.Request.RecipientTeaches)

	expect.Equal(t, "exchange after a repeated acceptance", accept().Exchange.ID, x.ID)

	exchangePath := "/api/v1/exchanges/" + x.ID
	for _, viewer := range []struct {
		name    string
		session *session
	}{{"Olha", olha}, {"Andrii", andrii}} {
		body := sendAndDecode[exchangeEnvelope](t, e, http.MethodGet, exchangePath, "", viewer.session, http.StatusOK)
		expect.Equal(t, viewer.name+": exchange", body.Exchange.ID, x.ID)
		expect.Equal(t, viewer.name+": terms Olha teaches", body.Exchange.RequesterTeaches, x.RequesterTeaches)
	}
	stranger := e.student(t, "marko@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	assertProblem(t, send(t, e.handler, http.MethodGet, exchangePath, "", stranger), http.StatusNotFound, "exchange_not_found")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/exchanges/latest", "", olha), http.StatusNotFound, "exchange_not_found")
	assertProblem(t, send(t, e.handler, http.MethodGet, exchangePath, "", nil), http.StatusUnauthorized, "unauthenticated")
	response = send(t, e.handler, http.MethodPost, exchangePath, "", olha)
	expect.Equal(t, "POST status", response.Code, http.StatusMethodNotAllowed)
	expect.Equal(t, "Allow header", response.Header().Get("Allow"), http.MethodGet)
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
