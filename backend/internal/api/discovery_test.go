package api

import (
	"fmt"
	"net/http"
	"slices"
	"testing"

	"skillswap/backend/internal/auth"
	"skillswap/backend/internal/core"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/discovery"
	"skillswap/backend/internal/mailer/mailertest"
	"skillswap/backend/internal/profile"
)

// discoveryEnv is the API on an in-memory store, with the mail recorder needed to
// verify new accounts.
type discoveryEnv struct {
	handler http.Handler
	mail    *mailertest.Recorder
}

func newDiscoveryEnv() *discoveryEnv {
	store := datatest.NewMemory()
	mail := &mailertest.Recorder{}
	access := auth.NewService(store, store, mail, testAuthConfig, discardLogger())
	app := core.NewApplication(profile.NewService(store, store, store), discovery.NewService(store, store))
	return &discoveryEnv{handler: New(app, access, &fakeApp{}, testSettings, discardLogger()), mail: mail}
}

// student registers and verifies a user, then fills the profile: the formats JSON
// array, the city, and one skill in each list (an empty skill ID skips that list).
func (e *discoveryEnv) student(t *testing.T, email, formats, city, teach, learn string) *session {
	t.Helper()
	expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/auth/register", registerBody(email, testPassword), nil), http.StatusCreated)
	expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/auth/verify-email",
		fmt.Sprintf(`{"token":%q}`, tokenFromMail(t, e.mail)), nil), http.StatusOK)
	s := login(t, e.handler, email, testPassword)
	expectStatus(t, send(t, e.handler, http.MethodPatch, "/api/v1/me/profile",
		fmt.Sprintf(`{"formats":%s,"city":%q}`, formats, city), s), http.StatusOK)
	if teach != "" {
		expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/me/teaching-skills",
			fmt.Sprintf(`{"skillId":%q,"level":"ADVANCED"}`, teach), s), http.StatusCreated)
	}
	if learn != "" {
		expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/me/learning-skills",
			fmt.Sprintf(`{"skillId":%q,"level":"BEGINNER"}`, learn), s), http.StatusCreated)
	}
	return s
}

type matchesResponse struct {
	Items               []mutualMatch `json:"items"`
	Page                int           `json:"page"`
	PageSize            int           `json:"pageSize"`
	Total               int           `json:"total"`
	EligibleForMatching bool          `json:"eligibleForMatching"`
}

func (e *discoveryEnv) matches(t *testing.T, s *session, query string) matchesResponse {
	t.Helper()
	response := send(t, e.handler, http.MethodGet, "/api/v1/me/matches"+query, "", s)
	expectStatus(t, response, http.StatusOK)
	var body matchesResponse
	decodeJSON(t, response, &body)
	return body
}

// meID returns the signed-in user's ID.
func (e *discoveryEnv) meID(t *testing.T, s *session) string {
	t.Helper()
	response := send(t, e.handler, http.MethodGet, "/api/v1/auth/me", "", s)
	expectStatus(t, response, http.StatusOK)
	var body struct {
		User user `json:"user"`
	}
	decodeJSON(t, response, &body)
	return body.User.ID
}

func TestMutualMatchesRequireVerifiedSession(t *testing.T) {
	e := newDiscoveryEnv()
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/matches", "", nil), http.StatusUnauthorized, "unauthenticated")

	expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/auth/register",
		registerBody("pending@students.example.test", testPassword), nil), http.StatusCreated)
	pending := login(t, e.handler, "pending@students.example.test", testPassword)
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/matches", "", pending), http.StatusForbidden, "email_not_verified")

	response := send(t, e.handler, http.MethodPost, "/api/v1/me/matches", "", pending)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("POST status = %d, Allow = %q", response.Code, response.Header().Get("Allow"))
	}
}

func TestMutualMatchesGuitarPhotoshop(t *testing.T) {
	e := newDiscoveryEnv()
	olha := e.student(t, "olha@students.example.test", `["ONLINE"]`, "Київ", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	andrii := e.student(t, "andrii@students.example.test", `["ONLINE","OFFLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	// Marko teaches Photoshop to Olha but wants nothing she teaches.
	e.student(t, "marko@students.example.test", `["ONLINE"]`, "Львів", datatest.PhotoshopSkillID, "")
	// Sofiia fits by skills, but shares only the offline format with Andrii in another city.
	e.student(t, "sofiia@students.example.test", `["OFFLINE"]`, "Львів", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	andriiID := e.meID(t, andrii)

	got := e.matches(t, olha, "")
	if got.Page != 1 || got.PageSize != discovery.PageSize || got.Total != 1 || !got.EligibleForMatching || len(got.Items) != 1 {
		t.Fatalf("matches = %+v", got)
	}
	match := got.Items[0]
	if match.Student.ID != andriiID || match.Student.University.Name == "" || match.Student.City != "Київ" {
		t.Fatalf("student = %+v", match.Student)
	}
	if len(match.CanTeachYou) != 1 || match.CanTeachYou[0].Name != "Photoshop" || match.CanTeachYou[0].Level != "ADVANCED" {
		t.Fatalf("canTeachYou = %+v", match.CanTeachYou)
	}
	if len(match.WantsToLearn) != 1 || match.WantsToLearn[0].Name != "Гітара" || match.WantsToLearn[0].Level != "BEGINNER" {
		t.Fatalf("wantsToLearn = %+v", match.WantsToLearn)
	}
	// Offline is not usable: Olha chose only online lessons.
	if !slices.Equal(match.CommonFormats, []string{"ONLINE"}) {
		t.Fatalf("commonFormats = %v", match.CommonFormats)
	}
	if got := e.matches(t, andrii, ""); got.Total != 1 || len(got.Items) != 1 {
		t.Fatalf("Andrii's matches = %+v", got)
	}

	// Andrii stops teaching Photoshop: the next request no longer shows the match.
	expectStatus(t, send(t, e.handler, http.MethodDelete, "/api/v1/me/teaching-skills/"+datatest.PhotoshopSkillID, "", andrii), http.StatusOK)
	if got := e.matches(t, olha, ""); got.Total != 0 || len(got.Items) != 0 || !got.EligibleForMatching {
		t.Fatalf("after the change = %+v", got)
	}
}

// A user whose own profile is incomplete gets an empty list with the reason.
func TestMutualMatchesForIncompleteProfile(t *testing.T) {
	e := newDiscoveryEnv()
	e.student(t, "andrii@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	olha := e.student(t, "olha@students.example.test", `["ONLINE"]`, "Київ", datatest.GuitarSkillID, "")

	got := e.matches(t, olha, "")
	if got.EligibleForMatching || got.Total != 0 || got.Items == nil || len(got.Items) != 0 {
		t.Fatalf("matches = %+v", got)
	}
}

func TestMutualMatchesPages(t *testing.T) {
	e := newDiscoveryEnv()
	olha := e.student(t, "olha@students.example.test", `["ONLINE"]`, "Київ", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	e.student(t, "andrii@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)

	if got := e.matches(t, olha, "?page=2"); got.Page != 2 || got.Total != 1 || len(got.Items) != 0 {
		t.Fatalf("page 2 = %+v", got)
	}
	for _, page := range []string{"0", "-1", "abc", "1.5", "501"} {
		t.Run(page, func(t *testing.T) {
			assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/matches?page="+page, "", olha),
				http.StatusBadRequest, "invalid_page")
		})
	}
}
