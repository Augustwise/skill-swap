package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"skillswap/backend/internal/auth"
	"skillswap/backend/internal/core"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/mailer/mailertest"
	"skillswap/backend/internal/profile"
)

// Paths for the demo skills, used by several tests below.
const (
	guitarPath    = "/api/v1/me/teaching-skills/" + datatest.GuitarSkillID
	photoshopPath = "/api/v1/me/learning-skills/" + datatest.PhotoshopSkillID
)

// newProfileHandler builds the API on an in-memory store, registers a user,
// and returns the handler together with that user's logged-in session.
func newProfileHandler(t *testing.T) (http.Handler, *session) {
	t.Helper()
	store := datatest.NewMemory()
	access := auth.NewService(store, store, &mailertest.Recorder{}, testAuthConfig, discardLogger())
	app := core.NewApplication(profile.NewService(store, store, store))
	handler := New(app, access, &fakeApp{}, testSettings, discardLogger())
	send(t, handler, http.MethodPost, "/api/v1/auth/register", registerBody("a@students.example.test", testPassword), nil)
	return handler, login(t, handler, "a@students.example.test", testPassword)
}

// decodeProfile reads the "profile" object from a JSON response.
func decodeProfile(t *testing.T, response *httptest.ResponseRecorder) profileResponse {
	t.Helper()
	var body struct {
		Profile profileResponse `json:"profile"`
	}
	decodeJSON(t, response, &body)
	return body.Profile
}

// expectStatus fails the test if the response has a different HTTP status.
func expectStatus(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, status, response.Body.String())
	}
}

// assertFieldError checks for a 422 validation error that names the given field.
func assertFieldError(t *testing.T, response *httptest.ResponseRecorder, field string) {
	t.Helper()
	assertProblem(t, response, http.StatusUnprocessableEntity, "validation_failed")
	var body struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	decodeJSON(t, response, &body)
	if body.Error.Fields[field] == "" {
		t.Fatalf("missing %s field error: %s", field, response.Body.String())
	}
}

// Every profile endpoint must reject requests that have no session cookie.
func TestProfileRequiresSession(t *testing.T) {
	handler, _ := newProfileHandler(t)
	for _, test := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/me/profile", ""},
		{http.MethodPatch, "/api/v1/me/profile", `{"city":"Київ"}`},
		{http.MethodPost, "/api/v1/me/teaching-skills", `{"skillId":"` + datatest.GuitarSkillID + `","level":"BEGINNER"}`},
		{http.MethodDelete, guitarPath, ""},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			response := send(t, handler, test.method, test.path, test.body, nil)
			assertProblem(t, response, http.StatusUnauthorized, "unauthenticated")
		})
	}
}

// Requests that change data must carry a valid CSRF token and an allowed Origin.
func TestProfileChangesRequireCSRFAndOrigin(t *testing.T) {
	handler, s := newProfileHandler(t)
	// Same session cookie, but no CSRF token.
	withoutCSRF := &session{cookie: s.cookie}
	for _, test := range []struct{ method, path, body string }{
		{http.MethodPatch, "/api/v1/me/profile", `{"city":"Київ"}`},
		{http.MethodPatch, guitarPath, `{"level":"ADVANCED"}`},
		{http.MethodDelete, guitarPath, ""},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			response := send(t, handler, test.method, test.path, test.body, withoutCSRF)
			assertProblem(t, response, http.StatusForbidden, "csrf_invalid")

			// A valid CSRF token is not enough: a foreign or missing Origin is also refused.
			for _, origin := range []string{"https://evil.example", ""} {
				req := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
				if origin != "" {
					req.Header.Set("Origin", origin)
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set(csrfHeaderName, s.csrf)
				req.AddCookie(s.cookie)
				response = httptest.NewRecorder()
				handler.ServeHTTP(response, req)
				assertProblem(t, response, http.StatusForbidden, "origin_forbidden")
			}
		})
	}
}

// Unsupported methods return 405 with an Allow header, and CORS preflight
// allows PATCH and DELETE.
func TestProfileMethodsAndCORS(t *testing.T) {
	handler, s := newProfileHandler(t)
	response := send(t, handler, http.MethodPut, "/api/v1/me/profile", `{}`, s)
	assertProblem(t, response, http.StatusMethodNotAllowed, "method_not_allowed")
	if got := response.Header().Get("Allow"); got != "GET, PATCH" {
		t.Fatalf("Allow = %q", got)
	}

	preflight := httptest.NewRequest(http.MethodOptions, guitarPath, nil)
	preflight.Header.Set("Origin", "http://localhost:3000")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodDelete)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, preflight)
	methods := response.Header().Get("Access-Control-Allow-Methods")
	if response.Code != http.StatusNoContent || !strings.Contains(methods, "PATCH") || !strings.Contains(methods, "DELETE") {
		t.Fatalf("preflight status = %d, methods = %q", response.Code, methods)
	}
}

// A new profile starts mostly empty; a PATCH changes only the fields sent,
// and explicit nulls clear optional fields.
func TestProfileUpdateAndRead(t *testing.T) {
	handler, s := newProfileHandler(t)
	// Fresh profile: name and university come from registration, the rest is empty.
	response := send(t, handler, http.MethodGet, "/api/v1/me/profile", "", s)
	expectStatus(t, response, http.StatusOK)
	empty := decodeProfile(t, response)
	if empty.FirstName != "Олена" || empty.University.ID != datatest.DemoUniversityID || empty.Faculty != nil ||
		empty.Course != nil || empty.Formats == nil || empty.TeachingSkills == nil || empty.EligibleForMatching {
		t.Fatalf("unexpected new profile: %s", response.Body.String())
	}

	// The bio is stored as plain text; escaping is left to the client.
	body := fmt.Sprintf(`{"facultyId":%q,"course":2,"city":"Київ","bio":"<script>alert(1)</script>","formats":["ONLINE","OFFLINE"]}`,
		datatest.DemoFacultyID)
	response = send(t, handler, http.MethodPatch, "/api/v1/me/profile", body, s)
	expectStatus(t, response, http.StatusOK)
	updated := decodeProfile(t, response)
	if updated.Faculty == nil || updated.Faculty.ID != datatest.DemoFacultyID || updated.Course == nil || *updated.Course != 2 ||
		updated.Bio != "<script>alert(1)</script>" || strings.Join(updated.Formats, ",") != "ONLINE,OFFLINE" {
		t.Fatalf("unexpected profile: %s", response.Body.String())
	}
	// The response must not leak account or credential data.
	for _, private := range []string{"password", "email", "accountStatus", "role"} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatalf("profile response contains %q: %s", private, response.Body.String())
		}
	}

	// Null clears faculty and course; fields not in the request (city) stay as they were.
	response = send(t, handler, http.MethodPatch, "/api/v1/me/profile", `{"facultyId":null,"course":null}`, s)
	expectStatus(t, response, http.StatusOK)
	cleared := decodeProfile(t, response)
	if cleared.Faculty != nil || cleared.Course != nil || cleared.City != "Київ" {
		t.Fatalf("unexpected profile after null patch: %s", response.Body.String())
	}
}

// Invalid values give a 422 naming the bad field; unknown or wrongly typed
// JSON gives a 400.
func TestProfileValidationErrors(t *testing.T) {
	handler, s := newProfileHandler(t)
	tests := []struct {
		name  string
		body  string
		field string
	}{
		{"601-character description", fmt.Sprintf(`{"bio":%q}`, strings.Repeat("я", 601)), "bio"},
		{"offline without city", `{"formats":["OFFLINE"]}`, "city"},
		{"course out of range", `{"course":7}`, "course"},
		{"foreign faculty", fmt.Sprintf(`{"facultyId":%q}`, datatest.OtherFacultyID), "facultyId"},
		{"empty first name", `{"firstName":""}`, "firstName"},
		{"null last name", `{"lastName":null}`, "lastName"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := send(t, handler, http.MethodPatch, "/api/v1/me/profile", test.body, s)
			assertFieldError(t, response, test.field)
		})
	}

	// universityId cannot be changed, so it is an unknown field; a string course is the wrong type.
	response := send(t, handler, http.MethodPatch, "/api/v1/me/profile", `{"universityId":"x"}`, s)
	assertProblem(t, response, http.StatusBadRequest, "invalid_json")
	response = send(t, handler, http.MethodPatch, "/api/v1/me/profile", `{"course":"third"}`, s)
	assertProblem(t, response, http.StatusBadRequest, "invalid_json")
}

// Covers adding, changing and removing teaching and learning skills, and
// when a profile counts as eligible for matching.
func TestSkillListEndpoints(t *testing.T) {
	handler, s := newProfileHandler(t)
	// add posts a skill to the given list (teaching or learning).
	add := func(path, skillID, level string) *httptest.ResponseRecorder {
		return send(t, handler, http.MethodPost, path, fmt.Sprintf(`{"skillId":%q,"level":%q}`, skillID, level), s)
	}

	response := add("/api/v1/me/teaching-skills", datatest.GuitarSkillID, "ADVANCED")
	expectStatus(t, response, http.StatusCreated)
	response = add("/api/v1/me/learning-skills", datatest.PhotoshopSkillID, "BEGINNER")
	expectStatus(t, response, http.StatusCreated)
	// One teaching skill, one learning skill and a format make the profile eligible.
	response = send(t, handler, http.MethodPatch, "/api/v1/me/profile", `{"formats":["ONLINE"]}`, s)
	if p := decodeProfile(t, response); !p.EligibleForMatching {
		t.Fatalf("profile with both lists and a format must be eligible: %s", response.Body.String())
	}

	// Error cases: duplicate skill, inactive skill, invalid level.
	response = add("/api/v1/me/teaching-skills", datatest.GuitarSkillID, "BEGINNER")
	assertProblem(t, response, http.StatusConflict, "skill_already_added")
	response = add("/api/v1/me/teaching-skills", datatest.InactiveSkillID, "BEGINNER")
	assertProblem(t, response, http.StatusNotFound, "skill_not_found")
	response = add("/api/v1/me/teaching-skills", datatest.PhotoshopSkillID, "EXPERT")
	assertFieldError(t, response, "level")

	// Change the level of an existing skill.
	response = send(t, handler, http.MethodPatch, guitarPath, `{"level":"INTERMEDIATE"}`, s)
	expectStatus(t, response, http.StatusOK)
	if p := decodeProfile(t, response); p.TeachingSkills[0].Level != "INTERMEDIATE" {
		t.Fatalf("level was not changed: %s", response.Body.String())
	}

	// Removing the only learning skill makes the profile ineligible again.
	response = send(t, handler, http.MethodDelete, photoshopPath, "", s)
	expectStatus(t, response, http.StatusOK)
	if p := decodeProfile(t, response); len(p.LearningSkills) != 0 || len(p.TeachingSkills) != 1 || p.EligibleForMatching {
		t.Fatalf("unexpected profile after removal: %s", response.Body.String())
	}
	// Deleting it again, or using a malformed ID, returns "not found".
	response = send(t, handler, http.MethodDelete, photoshopPath, "", s)
	assertProblem(t, response, http.StatusNotFound, "skill_not_found")
	response = send(t, handler, http.MethodDelete, "/api/v1/me/learning-skills/not-a-uuid", "", s)
	assertProblem(t, response, http.StatusNotFound, "skill_not_found")
}

// Profile data is saved on the server, so it is still there after logout and login.
func TestProfileSurvivesSignInAgain(t *testing.T) {
	handler, s := newProfileHandler(t)
	send(t, handler, http.MethodPatch, "/api/v1/me/profile", `{"city":"Одеса","bio":"Привіт"}`, s)
	send(t, handler, http.MethodPost, "/api/v1/me/teaching-skills", fmt.Sprintf(`{"skillId":%q,"level":"ADVANCED"}`, datatest.GuitarSkillID), s)
	expectStatus(t, send(t, handler, http.MethodPost, "/api/v1/auth/logout", "", s), http.StatusNoContent)

	s = login(t, handler, "a@students.example.test", testPassword)
	response := send(t, handler, http.MethodGet, "/api/v1/me/profile", "", s)
	expectStatus(t, response, http.StatusOK)
	if p := decodeProfile(t, response); p.City != "Одеса" || p.Bio != "Привіт" || len(p.TeachingSkills) != 1 {
		t.Fatalf("profile was not kept: %s", response.Body.String())
	}
}

// The faculties list is public and returns only the faculties of the given
// university; an unknown university gives 404.
func TestFacultiesEndpoint(t *testing.T) {
	handler, _ := newProfileHandler(t)
	response := send(t, handler, http.MethodGet, "/api/v1/universities/"+datatest.DemoUniversityID+"/faculties", "", nil)
	expectStatus(t, response, http.StatusOK)
	var body struct {
		Items []reference `json:"items"`
	}
	decodeJSON(t, response, &body)
	if len(body.Items) != 1 || body.Items[0].ID != datatest.DemoFacultyID {
		t.Fatalf("unexpected faculties: %s", response.Body.String())
	}
	response = send(t, handler, http.MethodGet, "/api/v1/universities/unknown/faculties", "", nil)
	assertProblem(t, response, http.StatusNotFound, "not_found")
}

// End-to-end check of the profile flow against a real PostgreSQL database.
// Skipped unless TEST_DATABASE_URL points to a migrated, seeded database.
func TestProfileAgainstLocalPostgres(t *testing.T) {
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
	// A unique email per run avoids clashes with users from earlier runs.
	email := fmt.Sprintf("profile-%d@students.example.test", time.Now().UnixNano())
	expectStatus(t, send(t, handler, http.MethodPost, "/api/v1/auth/register", registerBody(email, testPassword), nil), http.StatusCreated)
	s := login(t, handler, email, testPassword)

	// Update the profile; the 600-character bio is the maximum allowed length.
	const facultyID = "50000000-0000-0000-0000-000000000001"
	body := fmt.Sprintf(`{"facultyId":%q,"course":3,"city":"Київ","bio":%q,"formats":["OFFLINE","ONLINE"]}`,
		facultyID, strings.Repeat("ї", 600))
	expectStatus(t, send(t, handler, http.MethodPatch, "/api/v1/me/profile", body, s), http.StatusOK)
	assertFieldError(t, send(t, handler, http.MethodPatch, "/api/v1/me/profile", `{"course":7}`, s), "course")

	// Add, duplicate, change and remove skills.
	addGuitar := fmt.Sprintf(`{"skillId":%q,"level":"ADVANCED"}`, datatest.GuitarSkillID)
	expectStatus(t, send(t, handler, http.MethodPost, "/api/v1/me/teaching-skills", addGuitar, s), http.StatusCreated)
	assertProblem(t, send(t, handler, http.MethodPost, "/api/v1/me/teaching-skills", addGuitar, s),
		http.StatusConflict, "skill_already_added")
	addPhotoshop := fmt.Sprintf(`{"skillId":%q,"level":"BEGINNER"}`, datatest.PhotoshopSkillID)
	expectStatus(t, send(t, handler, http.MethodPost, "/api/v1/me/learning-skills", addPhotoshop, s), http.StatusCreated)
	expectStatus(t, send(t, handler, http.MethodPatch, photoshopPath, `{"level":"INTERMEDIATE"}`, s), http.StatusOK)

	expectStatus(t, send(t, handler, http.MethodDelete, guitarPath, "", s), http.StatusOK)
	assertProblem(t, send(t, handler, http.MethodDelete, guitarPath, "", s), http.StatusNotFound, "skill_not_found")
	expectStatus(t, send(t, handler, http.MethodPost, "/api/v1/me/teaching-skills", addGuitar, s), http.StatusCreated)

	// Log in again and check that everything was saved in the database.
	expectStatus(t, send(t, handler, http.MethodPost, "/api/v1/auth/logout", "", s), http.StatusNoContent)
	s = login(t, handler, email, testPassword)
	response := send(t, handler, http.MethodGet, "/api/v1/me/profile", "", s)
	expectStatus(t, response, http.StatusOK)
	p := decodeProfile(t, response)
	if p.Faculty == nil || p.Faculty.ID != facultyID || p.Course == nil || *p.Course != 3 || p.City != "Київ" ||
		len([]rune(p.Bio)) != 600 || strings.Join(p.Formats, ",") != "ONLINE,OFFLINE" {
		t.Fatalf("profile fields were not kept: %s", response.Body.String())
	}
	if len(p.TeachingSkills) != 1 || p.TeachingSkills[0].Name != "Гітара" ||
		len(p.LearningSkills) != 1 || p.LearningSkills[0].Level != "INTERMEDIATE" || !p.EligibleForMatching {
		t.Fatalf("skill lists were not kept: %s", response.Body.String())
	}
}
