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
