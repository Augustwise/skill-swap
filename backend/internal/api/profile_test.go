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
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/expect"
	"skillswap/backend/internal/mailer/mailertest"
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
	mail := &mailertest.Recorder{}
	access := auth.NewService(store, store, mail, testAuthConfig, discardLogger())
	app := memoryApp(store)
	handler := New(app, access, &fakeApp{}, testSettings, discardLogger())
	send(t, handler, http.MethodPost, "/api/v1/auth/register", registerBody("a@students.example.test", testPassword), nil)
	expectStatus(t, send(t, handler, http.MethodPost, "/api/v1/auth/verify-email",
		fmt.Sprintf(`{"token":%q}`, tokenFromMail(t, mail)), nil), http.StatusOK)
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

func TestProfileRequiresVerifiedEmail(t *testing.T) {
	store := datatest.NewMemory()
	mail := &mailertest.Recorder{}
	access := auth.NewService(store, store, mail, testAuthConfig, discardLogger())
	app := memoryApp(store)
	handler := New(app, access, &fakeApp{}, testSettings, discardLogger())
	expectStatus(t, send(t, handler, http.MethodPost, "/api/v1/auth/register",
		registerBody("pending@students.example.test", testPassword), nil), http.StatusCreated)
	s := login(t, handler, "pending@students.example.test", testPassword)
	originalToken := tokenFromMail(t, mail)

	for _, test := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/me/profile", ""},
		{http.MethodPatch, "/api/v1/me/profile", `{"city":"Київ"}`},
		{http.MethodPost, "/api/v1/me/teaching-skills", `{"skillId":"` + datatest.GuitarSkillID + `","level":"BEGINNER"}`},
		{http.MethodPatch, guitarPath, `{"level":"ADVANCED"}`},
		{http.MethodDelete, guitarPath, ""},
		{http.MethodPost, "/api/v1/me/learning-skills", `{"skillId":"` + datatest.PhotoshopSkillID + `","level":"BEGINNER"}`},
		{http.MethodPatch, photoshopPath, `{"level":"ADVANCED"}`},
		{http.MethodDelete, photoshopPath, ""},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			assertProblem(t, send(t, handler, test.method, test.path, test.body, s),
				http.StatusForbidden, "email_not_verified")
		})
	}

	// Authentication and resend remain available while onboarding is blocked.
	expectStatus(t, send(t, handler, http.MethodGet, "/api/v1/auth/me", "", s), http.StatusOK)
	expectStatus(t, send(t, handler, http.MethodPost, "/api/v1/auth/resend-verification", "", s), http.StatusAccepted)
	assertProblem(t, send(t, handler, http.MethodPost, "/api/v1/auth/verify-email",
		fmt.Sprintf(`{"token":%q}`, originalToken), nil), http.StatusBadRequest, "invalid_token")
	assertProblem(t, send(t, handler, http.MethodGet, "/api/v1/me/profile", "", s), http.StatusForbidden, "email_not_verified")
	expectStatus(t, send(t, handler, http.MethodPost, "/api/v1/auth/verify-email",
		fmt.Sprintf(`{"token":%q}`, tokenFromMail(t, mail)), nil), http.StatusOK)

	// The existing session gains access only after a valid link has been used.
	response := send(t, handler, http.MethodGet, "/api/v1/me/profile", "", s)
	expectStatus(t, response, http.StatusOK)
	expect.Equal(t, "city after unverified requests", decodeProfile(t, response).City, "")
	expectStatus(t, send(t, handler, http.MethodPatch, "/api/v1/me/profile", `{"city":"Київ"}`, s), http.StatusOK)
	expectStatus(t, send(t, handler, http.MethodPost, "/api/v1/auth/logout", "", s), http.StatusNoContent)
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
	expect.Equal(t, "Allow", response.Header().Get("Allow"), "GET, PATCH")

	preflight := httptest.NewRequest(http.MethodOptions, guitarPath, nil)
	preflight.Header.Set("Origin", "http://localhost:3000")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodDelete)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, preflight)
	expectStatus(t, response, http.StatusNoContent)
	methods := response.Header().Get("Access-Control-Allow-Methods")
	expect.Equal(t, "preflight allows PATCH", strings.Contains(methods, "PATCH"), true)
	expect.Equal(t, "preflight allows DELETE", strings.Contains(methods, "DELETE"), true)
}

// A new profile starts mostly empty; a PATCH changes only the fields sent,
// and explicit nulls clear optional fields.
func TestProfileUpdateAndRead(t *testing.T) {
	handler, s := newProfileHandler(t)
	// Fresh profile: name and university come from registration, the rest is empty.
	response := send(t, handler, http.MethodGet, "/api/v1/me/profile", "", s)
	expectStatus(t, response, http.StatusOK)
	empty := decodeProfile(t, response)
	expect.Equal(t, "first name from registration", empty.FirstName, "Олена")
	expect.Equal(t, "university from registration", empty.University.ID, datatest.DemoUniversityID)
	expect.Equal(t, "new profile has a faculty", empty.Faculty != nil, false)
	expect.Equal(t, "new profile has a course", empty.Course != nil, false)
	expect.Equal(t, "formats are an array", empty.Formats != nil, true)
	expect.Equal(t, "teaching skills are an array", empty.TeachingSkills != nil, true)
	expect.Equal(t, "new profile eligible for matching", empty.EligibleForMatching, false)

	// The bio is stored as plain text; escaping is left to the client.
	body := fmt.Sprintf(`{"facultyId":%q,"course":2,"city":"Київ","bio":"<script>alert(1)</script>","formats":["ONLINE","OFFLINE"]}`,
		datatest.DemoFacultyID)
	response = send(t, handler, http.MethodPatch, "/api/v1/me/profile", body, s)
	expectStatus(t, response, http.StatusOK)
	updated := decodeProfile(t, response)
	expect.Equal(t, "faculty after update", valueOf(updated.Faculty).ID, datatest.DemoFacultyID)
	expect.Equal(t, "course after update", valueOf(updated.Course), 2)
	expect.Equal(t, "bio", updated.Bio, "<script>alert(1)</script>")
	expect.Slice(t, "formats", updated.Formats, []string{"ONLINE", "OFFLINE"})
	// The response must not leak account or credential data.
	for _, private := range []string{"password", "email", "accountStatus", "role"} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatalf("profile response contains %q: %s", private, response.Body.String())
		}
	}

	// A new university clears the faculty of the previous one.
	response = send(t, handler, http.MethodPatch, "/api/v1/me/profile",
		fmt.Sprintf(`{"universityId":%q}`, datatest.OtherUniversityID), s)
	expectStatus(t, response, http.StatusOK)
	moved := decodeProfile(t, response)
	expect.Equal(t, "university after the move", moved.University.ID, datatest.OtherUniversityID)
	expect.Equal(t, "university name is set", moved.University.Name != "", true)
	expect.Equal(t, "faculty kept after the move", moved.Faculty != nil, false)
	response = send(t, handler, http.MethodPatch, "/api/v1/me/profile",
		fmt.Sprintf(`{"universityId":%q,"facultyId":%q}`, datatest.DemoUniversityID, datatest.DemoFacultyID), s)
	expectStatus(t, response, http.StatusOK)

	// Null clears faculty and course; fields not in the request (city) stay as they were.
	response = send(t, handler, http.MethodPatch, "/api/v1/me/profile", `{"facultyId":null,"course":null}`, s)
	expectStatus(t, response, http.StatusOK)
	cleared := decodeProfile(t, response)
	expect.Equal(t, "faculty kept after null", cleared.Faculty != nil, false)
	expect.Equal(t, "course kept after null", cleared.Course != nil, false)
	expect.Equal(t, "city after null patch", cleared.City, "Київ")
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
		{"unknown university", `{"universityId":"x"}`, "universityId"},
		{"null university", `{"universityId":null}`, "universityId"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := send(t, handler, http.MethodPatch, "/api/v1/me/profile", test.body, s)
			assertFieldError(t, response, test.field)
		})
	}

	// Unknown fields and a string course are rejected as invalid JSON.
	response := send(t, handler, http.MethodPatch, "/api/v1/me/profile", `{"email":"x@example.test"}`, s)
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
	expect.Equal(t, "eligible with both lists and a format", decodeProfile(t, response).EligibleForMatching, true)

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
	expect.Equal(t, "guitar level", decodeProfile(t, response).TeachingSkills[0].Level, "INTERMEDIATE")

	// Removing the only learning skill makes the profile ineligible again.
	response = send(t, handler, http.MethodDelete, photoshopPath, "", s)
	expectStatus(t, response, http.StatusOK)
	afterRemoval := decodeProfile(t, response)
	expect.Equal(t, "learning skills after removal", len(afterRemoval.LearningSkills), 0)
	expect.Equal(t, "teaching skills after removal", len(afterRemoval.TeachingSkills), 1)
	expect.Equal(t, "eligible after removal", afterRemoval.EligibleForMatching, false)
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
	kept := decodeProfile(t, response)
	expect.Equal(t, "city after signing in again", kept.City, "Одеса")
	expect.Equal(t, "bio after signing in again", kept.Bio, "Привіт")
	expect.Equal(t, "teaching skills after signing in again", len(kept.TeachingSkills), 1)
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
	expect.Equal(t, "faculties", len(body.Items), 1)
	expect.Equal(t, "faculty", body.Items[0].ID, datatest.DemoFacultyID)
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
	mail := &mailertest.Recorder{}
	handler := postgresHandler(pool, mail)
	// A unique email per run avoids clashes with users from earlier runs.
	email := fmt.Sprintf("profile-%d@students.example.test", time.Now().UnixNano())
	expectStatus(t, send(t, handler, http.MethodPost, "/api/v1/auth/register", registerBody(email, testPassword), nil), http.StatusCreated)
	expectStatus(t, send(t, handler, http.MethodPost, "/api/v1/auth/verify-email",
		fmt.Sprintf(`{"token":%q}`, tokenFromMail(t, mail)), nil), http.StatusOK)
	s := login(t, handler, email, testPassword)

	// Update the profile; the 600-character bio is the maximum allowed length.
	const facultyID = "50000000-0000-0000-0000-000000000001"
	body := fmt.Sprintf(`{"facultyId":%q,"course":3,"city":"Київ","bio":%q,"formats":["OFFLINE","ONLINE"]}`,
		facultyID, strings.Repeat("ї", 600))
	expectStatus(t, send(t, handler, http.MethodPatch, "/api/v1/me/profile", body, s), http.StatusOK)
	assertFieldError(t, send(t, handler, http.MethodPatch, "/api/v1/me/profile", `{"course":7}`, s), "course")

	// Move to the second demo university and back; the faculty must follow the university.
	const secondUniversityID = "10000000-0000-0000-0000-000000000002"
	response := send(t, handler, http.MethodPatch, "/api/v1/me/profile", fmt.Sprintf(`{"universityId":%q}`, secondUniversityID), s)
	expectStatus(t, response, http.StatusOK)
	moved := decodeProfile(t, response)
	expect.Equal(t, "university after the move", moved.University.ID, secondUniversityID)
	expect.Equal(t, "faculty kept after the move", moved.Faculty != nil, false)
	expectStatus(t, send(t, handler, http.MethodPatch, "/api/v1/me/profile",
		fmt.Sprintf(`{"universityId":%q,"facultyId":%q}`, datatest.DemoUniversityID, facultyID), s), http.StatusOK)

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
	response = send(t, handler, http.MethodGet, "/api/v1/me/profile", "", s)
	expectStatus(t, response, http.StatusOK)
	p := decodeProfile(t, response)
	expect.Equal(t, "faculty", valueOf(p.Faculty).ID, facultyID)
	expect.Equal(t, "course", valueOf(p.Course), 3)
	expect.Equal(t, "city", p.City, "Київ")
	expect.Equal(t, "bio length in characters", len([]rune(p.Bio)), 600)
	expect.Slice(t, "formats", p.Formats, []string{"ONLINE", "OFFLINE"})
	expect.Equal(t, "teaching skills", len(p.TeachingSkills), 1)
	expect.Equal(t, "teaching skill", p.TeachingSkills[0].Name, "Гітара")
	expect.Equal(t, "learning skills", len(p.LearningSkills), 1)
	expect.Equal(t, "learning skill level", p.LearningSkills[0].Level, "INTERMEDIATE")
	expect.Equal(t, "eligible for matching", p.EligibleForMatching, true)
}
