package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"skillswap/backend/internal/auth"
	"skillswap/backend/internal/communication"
	"skillswap/backend/internal/core"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/discovery"
	"skillswap/backend/internal/exchange"
	"skillswap/backend/internal/expect"
	"skillswap/backend/internal/mailer"
	"skillswap/backend/internal/mailer/mailertest"
	"skillswap/backend/internal/profile"
	sessionsvc "skillswap/backend/internal/session"
	"skillswap/backend/internal/trust"
)

var errDatabaseUnavailable = errors.New("database unavailable")

type fakeApp struct {
	core.IApplication
	readyErr        error
	universities    []data.University
	universitiesErr error
	categories      []data.SkillCategory
	categoriesErr   error
	skills          []data.Skill
	skillsErr       error
	lastQuery       string
	lastLimit       int
}

func (s *fakeApp) Ready(context.Context) error {
	return s.readyErr
}

func (s *fakeApp) Universities(context.Context) ([]data.University, error) {
	return s.universities, s.universitiesErr
}

func (s *fakeApp) SkillCategories(context.Context) ([]data.SkillCategory, error) {
	return s.categories, s.categoriesErr
}

func (s *fakeApp) Skills(_ context.Context, query string, limit int) ([]data.Skill, error) {
	s.lastQuery = query
	s.lastLimit = limit
	return s.skills, s.skillsErr
}

var testSettings = Settings{FrontendOrigin: "http://localhost:3000"}

var testAuthConfig = auth.Config{
	FrontendOrigin:      "http://localhost:3000",
	AllowedEmailDomains: []string{"students.example.test"},
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testHandler(app *fakeApp) http.Handler {
	store := datatest.NewMemory()
	access := auth.NewService(store, store, &mailertest.Recorder{}, testAuthConfig, discardLogger())
	return New(app, access, app, testSettings, discardLogger())
}

func postgresHandler(pool *pgxpool.Pool, mail mailer.IMailer) http.Handler {
	store := data.NewPostgres(pool)
	access := auth.NewService(store, store, mail, testAuthConfig, discardLogger())
	comm := communication.NewService(store, store, store)
	sessions := sessionsvc.NewService(store, store, store, comm)
	app := core.NewApplication(profile.NewService(store, store, store), discovery.NewService(store, store),
		exchange.NewService(store, store, store, comm), comm, sessions, trust.NewService(store, store, store, store, comm))
	return New(app, access, store, testSettings, discardLogger())
}

func memoryApp(store *datatest.Memory) *core.Application {
	comm := communication.NewService(store, store, store)
	sessions := sessionsvc.NewService(store, store, store, comm)
	return core.NewApplication(profile.NewService(store, store, store), discovery.NewService(store, store),
		exchange.NewService(store, store, store, comm), comm, sessions, trust.NewService(store, store, store, store, comm))
}

func request(t *testing.T, handler http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, target, nil))
	return response
}

func TestHealthAndOrigin(t *testing.T) {
	handler := testHandler(&fakeApp{})
	response := request(t, handler, http.MethodGet, "/api/v1/health")
	expectStatus(t, response, http.StatusOK)
	expect.Equal(t, "Cache-Control", response.Header().Get("Cache-Control"), "no-store")

	foreignRequest := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	foreignRequest.Header.Set("Origin", "https://example.org")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, foreignRequest)
	assertProblem(t, response, http.StatusForbidden, "origin_forbidden")
}

func TestOpenAPIContract(t *testing.T) {
	response := request(t, testHandler(&fakeApp{}), http.MethodGet, "/api/v1/openapi.yaml")
	expectStatus(t, response, http.StatusOK)
	expect.Equal(t, "Content-Type", response.Header().Get("Content-Type"), "application/yaml; charset=utf-8")
	for _, fragment := range []string{"openapi: 3.0.3", "/universities:", "/skills:", "/auth/login:", "/auth/reset-password:",
		"/universities/{universityId}/faculties:", "/me/profile:", "/me/teaching-skills/{skillId}:", "/me/learning-skills:", "/me/matches:", "/students:", "/students/{userId}:", "/exchange-requests:",
		"/me/exchange-requests:", "/exchange-requests/{requestId}:",
		"/exchange-requests/{requestId}/accept:", "/exchange-requests/{requestId}/decline:", "/exchange-requests/{requestId}/withdraw:",
		"/exchange-requests/{requestId}/conversation:", "/me/exchanges:", "/exchanges/{exchangeId}:",
		"/exchanges/{exchangeId}/conversation:", "/exchanges/{exchangeId}/sessions:",
		"/exchanges/{exchangeId}/complete:", "/exchanges/{exchangeId}/cancel:",
		"/sessions/{sessionId}:", "/sessions/{sessionId}/proposals:", "/sessions/{sessionId}/accept:",
		"/sessions/{sessionId}/decline:", "/sessions/{sessionId}/cancel:", "/sessions/{sessionId}/confirm:", "/me/schedule:",
		"/me/conversations:", "/conversations/{conversationId}:", "/conversations/{conversationId}/messages:",
		"/conversations/{conversationId}/read:", "/me/notifications:", "/me/notifications/{notificationId}/read:",
		"/me/notifications/read-all:", "/exchanges/{exchangeId}/reviews:", "/students/{userId}/reviews:", "/me/reviews:",
		"/auth/change-password:", "/me/account:", "/me/privacy:", "/students/{userId}/block:", "/me/blocks:", "/reports:",
		"/me/reports:", "/admin/reports:", "/admin/reports/{reportId}:", "/admin/reports/{reportId}/resolve:",
		"/admin/users/{userId}/suspend:", "/admin/users/{userId}/restore:", "/admin/moderation-actions:",
		"/admin/skills:", "/admin/skills/{skillId}:"} {
		if !strings.Contains(response.Body.String(), fragment) {
			t.Fatalf("OpenAPI contract does not contain %q", fragment)
		}
	}
}

func TestReady(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		response := request(t, testHandler(&fakeApp{}), http.MethodGet, "/api/v1/ready")
		expectStatus(t, response, http.StatusOK)
		assertJSONEqual(t, response.Body.Bytes(), `{"status":"ready"}`)
	})

	t.Run("database unavailable", func(t *testing.T) {
		store := &fakeApp{readyErr: errDatabaseUnavailable}
		response := request(t, testHandler(store), http.MethodGet, "/api/v1/ready")
		assertProblem(t, response, http.StatusServiceUnavailable, "database_unavailable")
	})
}

func TestUniversities(t *testing.T) {
	store := &fakeApp{universities: []data.University{{
		ID:          "10000000-0000-0000-0000-000000000001",
		Name:        "Демонстраційний університет",
		ShortName:   "Demo Uni",
		EmailDomain: "students.example.test",
		City:        "Київ",
	}}}
	response := request(t, testHandler(store), http.MethodGet, "/api/v1/universities")
	expectStatus(t, response, http.StatusOK)
	var body struct {
		Items []university `json:"items"`
	}
	decodeJSON(t, response, &body)
	expect.Equal(t, "universities", len(body.Items), 1)
	expect.Equal(t, "university email domain", body.Items[0].EmailDomain, "students.example.test")
}

func TestSkillCategories(t *testing.T) {
	store := &fakeApp{categories: []data.SkillCategory{{
		ID: "20000000-0000-0000-0000-000000000001", Name: "Музика", Slug: "music",
	}}}
	response := request(t, testHandler(store), http.MethodGet, "/api/v1/skill-categories")
	expectStatus(t, response, http.StatusOK)
	var body struct {
		Items []skillCategory `json:"items"`
	}
	decodeJSON(t, response, &body)
	expect.Equal(t, "categories", len(body.Items), 1)
	expect.Equal(t, "category slug", body.Items[0].Slug, "music")
}

func TestSkillsSearchAndLimit(t *testing.T) {
	store := &fakeApp{skills: []data.Skill{{
		ID:         "30000000-0000-0000-0000-000000000004",
		CategoryID: "20000000-0000-0000-0000-000000000003",
		Name:       "Go",
		Slug:       "go",
	}}}
	response := request(t, testHandler(store), http.MethodGet, "/api/v1/skills?q=%20Go%20&limit=1")
	expectStatus(t, response, http.StatusOK)
	expect.Equal(t, "trimmed query", store.lastQuery, "Go")
	expect.Equal(t, "limit", store.lastLimit, 1)
	var body struct {
		Items []skill `json:"items"`
	}
	decodeJSON(t, response, &body)
	expect.Equal(t, "skills", len(body.Items), 1)
	expect.Equal(t, "skill name", body.Items[0].Name, "Go")

	store = &fakeApp{skills: []data.Skill{}}
	response = request(t, testHandler(store), http.MethodGet, "/api/v1/skills")
	expectStatus(t, response, http.StatusOK)
	expect.Equal(t, "default limit", store.lastLimit, 50)
}

func TestSkillsRejectsInvalidQuery(t *testing.T) {
	target := "/api/v1/skills?q=" + url.QueryEscape(strings.Repeat("я", 151))
	response := request(t, testHandler(&fakeApp{}), http.MethodGet, target)
	assertProblem(t, response, http.StatusBadRequest, "invalid_query")
}

func TestSkillsRejectsInvalidLimit(t *testing.T) {
	for _, limit := range []string{"0", "101", "-1", "1.5", "many"} {
		t.Run(limit, func(t *testing.T) {
			response := request(t, testHandler(&fakeApp{}), http.MethodGet,
				"/api/v1/skills?limit="+url.QueryEscape(limit))
			assertProblem(t, response, http.StatusBadRequest, "invalid_limit")
		})
	}
}

func TestRoutingErrorsUseProblemJSON(t *testing.T) {
	handler := testHandler(&fakeApp{})
	response := request(t, handler, http.MethodGet, "/api/v1/missing")
	assertProblem(t, response, http.StatusNotFound, "not_found")

	response = request(t, handler, http.MethodPost, "/api/v1/skills")
	assertProblem(t, response, http.StatusMethodNotAllowed, "method_not_allowed")
	expect.Equal(t, "Allow", response.Header().Get("Allow"), http.MethodGet)
}

// A panic in a handler is answered with problem JSON, and the next request still works.
func TestPanicReturnsInternalError(t *testing.T) {
	handler := testHandler(&fakeApp{})
	// fakeApp does not implement Faculties, so the call panics on the nil IApplication.
	response := request(t, handler, http.MethodGet, "/api/v1/universities/"+datatest.DemoUniversityID+"/faculties")
	assertProblem(t, response, http.StatusInternalServerError, "internal_error")
	expectStatus(t, request(t, handler, http.MethodGet, "/api/v1/health"), http.StatusOK)
}

func TestCORSPreflight(t *testing.T) {
	handler := testHandler(&fakeApp{})
	preflight := httptest.NewRequest(http.MethodOptions, "/api/v1/skills", nil)
	preflight.Header.Set("Origin", "http://localhost:3000")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, preflight)
	expectStatus(t, response, http.StatusNoContent)
	expect.Equal(t, "Access-Control-Allow-Origin", response.Header().Get("Access-Control-Allow-Origin"), "http://localhost:3000")
	if got := response.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodGet) {
		t.Fatalf("Access-Control-Allow-Methods = %q", got)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("preflight body = %q", response.Body.String())
	}
}

func TestDatabaseUnavailable(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		store *fakeApp
	}{
		{name: "universities", path: "/api/v1/universities", store: &fakeApp{universitiesErr: errDatabaseUnavailable}},
		{name: "categories", path: "/api/v1/skill-categories", store: &fakeApp{categoriesErr: errDatabaseUnavailable}},
		{name: "skills", path: "/api/v1/skills", store: &fakeApp{skillsErr: errDatabaseUnavailable}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, testHandler(test.store), http.MethodGet, test.path)
			assertProblem(t, response, http.StatusServiceUnavailable, "database_unavailable")
		})
	}
}

func TestCatalogAgainstLocalPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for the local PostgreSQL integration test")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	handler := postgresHandler(pool, &mailertest.Recorder{})

	response := request(t, handler, http.MethodGet, "/api/v1/ready")
	expectStatus(t, response, http.StatusOK)

	for _, test := range []struct {
		path      string
		wantCount int
	}{
		{path: "/api/v1/universities", wantCount: 2},
		{path: "/api/v1/skill-categories", wantCount: 4},
		{path: "/api/v1/universities/10000000-0000-0000-0000-000000000001/faculties", wantCount: 4},
		{path: "/api/v1/skills?q=Photoshop", wantCount: 1},
		{path: "/api/v1/skills?q=%25", wantCount: 0},
		{path: "/api/v1/skills?limit=1", wantCount: 1},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := request(t, handler, http.MethodGet, test.path)
			expectStatus(t, response, http.StatusOK)
			var body struct {
				Items []json.RawMessage `json:"items"`
			}
			decodeJSON(t, response, &body)
			expect.Equal(t, "item count", len(body.Items), test.wantCount)
		})
	}
}

func assertProblem(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	expectStatus(t, response, status)
	expect.Equal(t, "Content-Type", response.Header().Get("Content-Type"), "application/json; charset=utf-8")
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeJSON(t, response, &body)
	expect.Equal(t, "problem code", body.Error.Code, code)
	expect.Equal(t, "problem message is set", body.Error.Message != "", true)
}

func decodeJSON(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("invalid JSON %q: %v", response.Body.String(), err)
	}
}

func assertJSONEqual(t *testing.T, actual []byte, expected string) {
	t.Helper()
	var actualValue, expectedValue any
	if err := json.Unmarshal(actual, &actualValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(expected), &expectedValue); err != nil {
		t.Fatal(err)
	}
	actualJSON, _ := json.Marshal(actualValue)
	expectedJSON, _ := json.Marshal(expectedValue)
	if string(actualJSON) != string(expectedJSON) {
		t.Fatalf("JSON = %s, want %s", actualJSON, expectedJSON)
	}
}
