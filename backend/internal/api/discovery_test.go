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

type searchResponse struct {
	Items    []studentCard `json:"items"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
	Total    int           `json:"total"`
}

func (e *discoveryEnv) search(t *testing.T, s *session, query string) searchResponse {
	t.Helper()
	response := send(t, e.handler, http.MethodGet, "/api/v1/students"+query, "", s)
	expectStatus(t, response, http.StatusOK)
	var body searchResponse
	decodeJSON(t, response, &body)
	return body
}

func TestSearchStudentsRequiresVerifiedSession(t *testing.T) {
	e := newDiscoveryEnv()
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/students?q=Photoshop", "", nil), http.StatusUnauthorized, "unauthenticated")
	expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/auth/register",
		registerBody("pending@students.example.test", testPassword), nil), http.StatusCreated)
	pending := login(t, e.handler, "pending@students.example.test", testPassword)
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/students", "", pending), http.StatusForbidden, "email_not_verified")
}

// FR-04 acceptance: "Photoshop" finds only students who offer it, filters apply
// together, and an empty result is a normal empty page.
func TestSearchStudents(t *testing.T) {
	e := newDiscoveryEnv()
	olha := e.student(t, "olha@students.example.test", `["ONLINE"]`, "Київ", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	andrii := e.student(t, "andrii@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	marko := e.student(t, "marko@students.example.test", `["ONLINE"]`, "Львів", datatest.PhotoshopSkillID, "")
	expectStatus(t, send(t, e.handler, http.MethodPatch, "/api/v1/me/teaching-skills/"+datatest.PhotoshopSkillID,
		`{"level":"BEGINNER"}`, marko), http.StatusOK)
	sofiia := e.student(t, "sofiia@students.example.test", `["OFFLINE"]`, "Львів", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	ids := map[string]string{e.meID(t, andrii): "andrii", e.meID(t, marko): "marko", e.meID(t, sofiia): "sofiia"}
	names := func(body searchResponse) []string {
		got := []string{}
		for _, item := range body.Items {
			got = append(got, ids[item.Student.ID])
		}
		return got
	}

	for _, test := range []struct {
		query string
		want  []string
	}{
		// Sofiia wants Photoshop but does not offer it; the mutual match comes first.
		{"?q=Photoshop", []string{"andrii", "marko"}},
		{"?q=photo&level=BEGINNER", []string{"marko"}},
		{"?format=OFFLINE", []string{"sofiia"}},
		{"?q=Photoshop&mutual=true", []string{"andrii"}},
		{"?categoryId=20000000-0000-0000-0000-000000000001", []string{"sofiia"}},
		{"?q=Скрипка", []string{}},
	} {
		t.Run(test.query, func(t *testing.T) {
			got := e.search(t, olha, test.query)
			if !slices.Equal(names(got), test.want) || got.Total != len(test.want) || got.Page != 1 || got.PageSize != discovery.PageSize {
				t.Fatalf("search = %v (%+v), want %v", names(got), got, test.want)
			}
		})
	}

	got := e.search(t, olha, "?q=Photoshop")
	andriiCard, markoCard := got.Items[0], got.Items[1]
	if !andriiCard.Mutual || markoCard.Mutual {
		t.Fatalf("mutual: Andrii %v, Marko %v", andriiCard.Mutual, markoCard.Mutual)
	}
	if len(markoCard.TeachingSkills) != 1 || markoCard.TeachingSkills[0].Level != "BEGINNER" ||
		!slices.Equal(markoCard.Formats, []string{"ONLINE"}) || markoCard.Student.City != "Львів" {
		t.Fatalf("Marko = %+v", markoCard)
	}
	if got.Items == nil || e.search(t, olha, "?q=Скрипка").Items == nil {
		t.Fatal("items must be an array, not null")
	}
}

func TestSearchStudentsRejectsInvalidFilters(t *testing.T) {
	e := newDiscoveryEnv()
	olha := e.student(t, "olha@students.example.test", `["ONLINE"]`, "Київ", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	for query, field := range map[string]string{
		"?level=EXPERT":     "level",
		"?format=CAMPUS":    "format",
		"?categoryId=music": "categoryId",
		"?mutual=yes":       "mutual",
	} {
		t.Run(query, func(t *testing.T) {
			assertFieldError(t, send(t, e.handler, http.MethodGet, "/api/v1/students"+query, "", olha), field)
		})
	}
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/students?page=0", "", olha), http.StatusBadRequest, "invalid_page")
}

func (e *discoveryEnv) studentProfile(t *testing.T, s *session, id string) studentProfileResponse {
	t.Helper()
	response := send(t, e.handler, http.MethodGet, "/api/v1/students/"+id, "", s)
	expectStatus(t, response, http.StatusOK)
	var body struct {
		Student studentProfileResponse `json:"student"`
	}
	decodeJSON(t, response, &body)
	return body.Student
}

func TestStudentProfile(t *testing.T) {
	e := newDiscoveryEnv()
	olha := e.student(t, "olha@students.example.test", `["ONLINE"]`, "Київ", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	andrii := e.student(t, "andrii@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	expectStatus(t, send(t, e.handler, http.MethodPatch, "/api/v1/me/profile",
		`{"bio":"Дизайнер","course":3,"facultyId":"`+datatest.DemoFacultyID+`"}`, andrii), http.StatusOK)
	marko := e.student(t, "marko@students.example.test", `["ONLINE"]`, "Львів", datatest.PhotoshopSkillID, "")
	andriiID, markoID := e.meID(t, andrii), e.meID(t, marko)

	got := e.studentProfile(t, olha, andriiID)
	if got.ID != andriiID || got.Bio != "Дизайнер" || got.Course == nil || *got.Course != 3 || got.Faculty == nil ||
		got.University.Name == "" || got.City != "Київ" || !slices.Equal(got.Formats, []string{"ONLINE"}) {
		t.Fatalf("profile = %+v", got)
	}
	if len(got.TeachingSkills) != 1 || got.TeachingSkills[0].Level != "ADVANCED" || len(got.LearningSkills) != 1 {
		t.Fatalf("skills: teaching %+v, learning %+v", got.TeachingSkills, got.LearningSkills)
	}
	if got.AverageRating != nil || got.ReviewCount != 0 || got.Reviews == nil || len(got.Reviews) != 0 {
		t.Fatalf("reviews: average %v, count %d, items %v", got.AverageRating, got.ReviewCount, got.Reviews)
	}
	if got.Mutual == nil || got.Mutual.CanTeachYou[0].Name != "Photoshop" || got.Mutual.WantsToLearn[0].Name != "Гітара" ||
		!slices.Equal(got.Mutual.CommonFormats, []string{"ONLINE"}) {
		t.Fatalf("mutual = %+v", got.Mutual)
	}
	if got := e.studentProfile(t, olha, markoID); got.Mutual != nil {
		t.Fatalf("one-sided interest is marked mutual: %+v", got.Mutual)
	}

	expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/auth/register",
		registerBody("pending@students.example.test", testPassword), nil), http.StatusCreated)
	pending := login(t, e.handler, "pending@students.example.test", testPassword)
	for name, id := range map[string]string{
		"unverified student": e.meID(t, pending),
		"the viewer":         e.meID(t, olha),
		"unknown ID":         "40000000-0000-0000-0000-000000000999",
		"not a UUID":         "andrii",
	} {
		t.Run(name, func(t *testing.T) {
			assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/students/"+id, "", olha), http.StatusNotFound, "student_not_found")
		})
	}
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/students/"+andriiID, "", pending), http.StatusForbidden, "email_not_verified")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/students/"+andriiID, "", nil), http.StatusUnauthorized, "unauthenticated")
}
