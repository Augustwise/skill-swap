package api

import (
	"fmt"
	"net/http"
	"testing"

	"skillswap/backend/internal/auth"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/discovery"
	"skillswap/backend/internal/expect"
	"skillswap/backend/internal/mailer/mailertest"
)

// discoveryEnv is the API on an in-memory store, with the mail recorder needed to
// verify new accounts.
type discoveryEnv struct {
	handler http.Handler
	mail    *mailertest.Recorder
	store   *datatest.Memory
}

func newDiscoveryEnv() *discoveryEnv {
	store := datatest.NewMemory()
	mail := &mailertest.Recorder{}
	access := auth.NewService(store, store, mail, testAuthConfig, discardLogger())
	app := memoryApp(store)
	return &discoveryEnv{handler: New(app, access, &fakeApp{}, testSettings, discardLogger()), mail: mail, store: store}
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
	return sendAndDecode[matchesResponse](t, e, http.MethodGet, "/api/v1/me/matches"+query, "", s, http.StatusOK)
}

// meID returns the signed-in user's ID.
func (e *discoveryEnv) meID(t *testing.T, s *session) string {
	t.Helper()
	return sendAndDecode[struct {
		User user `json:"user"`
	}](t, e, http.MethodGet, "/api/v1/auth/me", "", s, http.StatusOK).User.ID
}

func TestMutualMatchesRequireVerifiedSession(t *testing.T) {
	e := newDiscoveryEnv()
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/matches", "", nil), http.StatusUnauthorized, "unauthenticated")

	expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/auth/register",
		registerBody("pending@students.example.test", testPassword), nil), http.StatusCreated)
	pending := login(t, e.handler, "pending@students.example.test", testPassword)
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/matches", "", pending), http.StatusForbidden, "email_not_verified")

	response := send(t, e.handler, http.MethodPost, "/api/v1/me/matches", "", pending)
	expectStatus(t, response, http.StatusMethodNotAllowed)
	expect.Equal(t, "Allow", response.Header().Get("Allow"), http.MethodGet)
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
	expect.Equal(t, "page", got.Page, 1)
	expect.Equal(t, "page size", got.PageSize, discovery.PageSize)
	expect.Equal(t, "Olha's match total", got.Total, 1)
	expect.Equal(t, "Olha eligible for matching", got.EligibleForMatching, true)
	expect.Equal(t, "Olha's matches", len(got.Items), 1)
	match := got.Items[0]
	expect.Equal(t, "matched student", match.Student.ID, andriiID)
	expect.Equal(t, "university name is set", match.Student.University.Name != "", true)
	expect.Equal(t, "matched student's city", match.Student.City, "Київ")
	expect.Equal(t, "skills Andrii can teach", len(match.CanTeachYou), 1)
	expect.Equal(t, "skill Andrii can teach", match.CanTeachYou[0].Name, "Photoshop")
	expect.Equal(t, "Andrii's Photoshop level", match.CanTeachYou[0].Level, "ADVANCED")
	expect.Equal(t, "skills Andrii wants to learn", len(match.WantsToLearn), 1)
	expect.Equal(t, "skill Andrii wants to learn", match.WantsToLearn[0].Name, "Гітара")
	expect.Equal(t, "Andrii's guitar level", match.WantsToLearn[0].Level, "BEGINNER")
	// Offline is not usable: Olha chose only online lessons.
	expect.Slice(t, "common formats", match.CommonFormats, []string{"ONLINE"})
	andriiMatches := e.matches(t, andrii, "")
	expect.Equal(t, "Andrii's match total", andriiMatches.Total, 1)
	expect.Equal(t, "Andrii's matches", len(andriiMatches.Items), 1)

	// Andrii stops teaching Photoshop: the next request no longer shows the match.
	expectStatus(t, send(t, e.handler, http.MethodDelete, "/api/v1/me/teaching-skills/"+datatest.PhotoshopSkillID, "", andrii), http.StatusOK)
	after := e.matches(t, olha, "")
	expect.Equal(t, "Olha's match total after the change", after.Total, 0)
	expect.Equal(t, "Olha's matches after the change", len(after.Items), 0)
	expect.Equal(t, "Olha eligible after the change", after.EligibleForMatching, true)
}

// A user whose own profile is incomplete gets an empty list with the reason.
func TestMutualMatchesForIncompleteProfile(t *testing.T) {
	e := newDiscoveryEnv()
	e.student(t, "andrii@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	olha := e.student(t, "olha@students.example.test", `["ONLINE"]`, "Київ", datatest.GuitarSkillID, "")

	got := e.matches(t, olha, "")
	expect.Equal(t, "eligible for matching", got.EligibleForMatching, false)
	expect.Equal(t, "match total", got.Total, 0)
	expect.Equal(t, "items are an array", got.Items != nil, true)
	expect.Equal(t, "matches", len(got.Items), 0)
}

func TestMutualMatchesPages(t *testing.T) {
	e := newDiscoveryEnv()
	olha := e.student(t, "olha@students.example.test", `["ONLINE"]`, "Київ", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	e.student(t, "andrii@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)

	secondPage := e.matches(t, olha, "?page=2")
	expect.Equal(t, "page", secondPage.Page, 2)
	expect.Equal(t, "total on page 2", secondPage.Total, 1)
	expect.Equal(t, "matches on page 2", len(secondPage.Items), 0)
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
	return sendAndDecode[searchResponse](t, e, http.MethodGet, "/api/v1/students"+query, "", s, http.StatusOK)
}

func TestSearchStudentsRequiresVerifiedSession(t *testing.T) {
	e := newDiscoveryEnv()
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/students?q=Photoshop", "", nil), http.StatusUnauthorized, "unauthenticated")
	expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/auth/register",
		registerBody("pending@students.example.test", testPassword), nil), http.StatusCreated)
	pending := login(t, e.handler, "pending@students.example.test", testPassword)
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/students", "", pending), http.StatusForbidden, "email_not_verified")
}

// "Photoshop" finds only students who offer it, filters apply together, and an
// empty result is a normal empty page.
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
			expect.Slice(t, "students found", names(got), test.want)
			expect.Equal(t, "total", got.Total, len(test.want))
			expect.Equal(t, "page", got.Page, 1)
			expect.Equal(t, "page size", got.PageSize, discovery.PageSize)
		})
	}

	got := e.search(t, olha, "?q=Photoshop")
	andriiCard, markoCard := got.Items[0], got.Items[1]
	expect.Equal(t, "Andrii is a mutual match", andriiCard.Mutual, true)
	expect.Equal(t, "Marko is a mutual match", markoCard.Mutual, false)
	expect.Equal(t, "Marko's teaching skills", len(markoCard.TeachingSkills), 1)
	expect.Equal(t, "Marko's Photoshop level", markoCard.TeachingSkills[0].Level, "BEGINNER")
	expect.Slice(t, "Marko's formats", markoCard.Formats, []string{"ONLINE"})
	expect.Equal(t, "Marko's city", markoCard.Student.City, "Львів")
	expect.Equal(t, "items are an array", got.Items != nil, true)
	expect.Equal(t, "items of an empty result are an array", e.search(t, olha, "?q=Скрипка").Items != nil, true)
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
	return sendAndDecode[struct {
		Student studentProfileResponse `json:"student"`
	}](t, e, http.MethodGet, "/api/v1/students/"+id, "", s, http.StatusOK).Student
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
	expect.Equal(t, "student", got.ID, andriiID)
	expect.Equal(t, "bio", got.Bio, "Дизайнер")
	expect.Equal(t, "course", valueOf(got.Course), 3)
	expect.Equal(t, "faculty is set", got.Faculty != nil, true)
	expect.Equal(t, "university name is set", got.University.Name != "", true)
	expect.Equal(t, "city", got.City, "Київ")
	expect.Slice(t, "formats", got.Formats, []string{"ONLINE"})
	expect.Equal(t, "teaching skills", len(got.TeachingSkills), 1)
	expect.Equal(t, "teaching skill level", got.TeachingSkills[0].Level, "ADVANCED")
	expect.Equal(t, "learning skills", len(got.LearningSkills), 1)
	expect.Equal(t, "average rating is set", got.AverageRating != nil, false)
	expect.Equal(t, "review count", got.ReviewCount, 0)
	expect.Equal(t, "reviews are an array", got.Reviews != nil, true)
	expect.Equal(t, "reviews", len(got.Reviews), 0)
	expect.Equal(t, "mutual is set", got.Mutual != nil, true)
	expect.Equal(t, "skill Andrii can teach", got.Mutual.CanTeachYou[0].Name, "Photoshop")
	expect.Equal(t, "skill Andrii wants to learn", got.Mutual.WantsToLearn[0].Name, "Гітара")
	expect.Slice(t, "common formats", got.Mutual.CommonFormats, []string{"ONLINE"})
	expect.Equal(t, "one-sided interest marked mutual", e.studentProfile(t, olha, markoID).Mutual != nil, false)

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
