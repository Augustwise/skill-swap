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
	"skillswap/backend/internal/core"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/mailer"
	"skillswap/backend/internal/mailer/mailertest"
	"skillswap/backend/internal/profile"
)

var errDatabaseUnavailable = errors.New("database unavailable")

type fakeApp struct {
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
	return New(core.NewApplication(profile.NewService(store)), access, store, testSettings, discardLogger())
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
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d", response.Code)
	}

	foreignRequest := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	foreignRequest.Header.Set("Origin", "https://example.org")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, foreignRequest)
	assertProblem(t, response, http.StatusForbidden, "origin_forbidden")
}

func TestOpenAPIContract(t *testing.T) {
	response := request(t, testHandler(&fakeApp{}), http.MethodGet, "/api/v1/openapi.yaml")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/yaml; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	for _, fragment := range []string{"openapi: 3.0.3", "/universities:", "/skills:", "/auth/login:", "/auth/reset-password:"} {
		if !strings.Contains(response.Body.String(), fragment) {
			t.Fatalf("OpenAPI contract does not contain %q", fragment)
		}
	}
}

func TestReady(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		response := request(t, testHandler(&fakeApp{}), http.MethodGet, "/api/v1/ready")
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
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
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Items []university `json:"items"`
	}
	decodeJSON(t, response, &body)
	if len(body.Items) != 1 || body.Items[0].EmailDomain != "students.example.test" {
		t.Fatalf("unexpected response: %+v", body.Items)
	}
}

func TestSkillCategories(t *testing.T) {
	store := &fakeApp{categories: []data.SkillCategory{{
		ID: "20000000-0000-0000-0000-000000000001", Name: "Музика", Slug: "music",
	}}}
	response := request(t, testHandler(store), http.MethodGet, "/api/v1/skill-categories")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Items []skillCategory `json:"items"`
	}
	decodeJSON(t, response, &body)
	if len(body.Items) != 1 || body.Items[0].Slug != "music" {
		t.Fatalf("unexpected response: %+v", body.Items)
	}
}

func TestSkillsSearchAndLimit(t *testing.T) {
	store := &fakeApp{skills: []data.Skill{{
		ID:         "30000000-0000-0000-0000-000000000004",
		CategoryID: "20000000-0000-0000-0000-000000000003",
		Name:       "Go",
		Slug:       "go",
	}}}
	response := request(t, testHandler(store), http.MethodGet, "/api/v1/skills?q=%20Go%20&limit=1")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if store.lastQuery != "Go" || store.lastLimit != 1 {
		t.Fatalf("query = %q, limit = %d", store.lastQuery, store.lastLimit)
	}
	var body struct {
		Items []skill `json:"items"`
	}
	decodeJSON(t, response, &body)
	if len(body.Items) != 1 || body.Items[0].Name != "Go" {
		t.Fatalf("unexpected response: %+v", body.Items)
	}

	store = &fakeApp{skills: []data.Skill{}}
	response = request(t, testHandler(store), http.MethodGet, "/api/v1/skills")
	if response.Code != http.StatusOK || store.lastLimit != 50 {
		t.Fatalf("default limit = %d, status = %d", store.lastLimit, response.Code)
	}
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
	if got := response.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("Allow = %q", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	handler := testHandler(&fakeApp{})
	preflight := httptest.NewRequest(http.MethodOptions, "/api/v1/skills", nil)
	preflight.Header.Set("Origin", "http://localhost:3000")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, preflight)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
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
	if response.Code != http.StatusOK {
		t.Fatalf("readiness status = %d, body = %s", response.Code, response.Body.String())
	}

	for _, test := range []struct {
		path      string
		wantCount int
	}{
		{path: "/api/v1/universities", wantCount: 1},
		{path: "/api/v1/skill-categories", wantCount: 3},
		{path: "/api/v1/skills?q=Photoshop", wantCount: 1},
		{path: "/api/v1/skills?q=%25", wantCount: 0},
		{path: "/api/v1/skills?limit=1", wantCount: 1},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := request(t, handler, http.MethodGet, test.path)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var body struct {
				Items []json.RawMessage `json:"items"`
			}
			decodeJSON(t, response, &body)
			if len(body.Items) != test.wantCount {
				t.Fatalf("item count = %d, want %d", len(body.Items), test.wantCount)
			}
		})
	}
}

func assertProblem(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, status, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeJSON(t, response, &body)
	if body.Error.Code != code || body.Error.Message == "" {
		t.Fatalf("unexpected problem: %+v", body.Error)
	}
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
