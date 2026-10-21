package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/expect"
	"skillswap/backend/internal/mailer/mailertest"
)

// mvpEnv runs the MVP scenario on any store. The store-specific steps are functions:
// moving a lesson into the past and giving a user the administrator role.
type mvpEnv struct {
	*discoveryEnv
	prefix      string
	users       []string
	moveSession func(t *testing.T, sessionID string, startsAt, endsAt time.Time)
	makeAdmin   func(t *testing.T, userID string)
}

func (env *mvpEnv) email(name string) string {
	return env.prefix + "." + name + "@students.example.test"
}

// newStudent registers, verifies and fills the profile of a student, and remembers the ID.
func (env *mvpEnv) newStudent(t *testing.T, name, teach, learn string) (*session, string) {
	t.Helper()
	s := env.student(t, env.email(name), `["ONLINE"]`, "Київ", teach, learn)
	id := env.meID(t, s)
	env.users = append(env.users, id)
	return s, id
}

// findStudent reports whether the student is on any page of /students or /me/matches.
func (env *mvpEnv) findStudent(t *testing.T, s *session, path, studentID string) bool {
	t.Helper()
	for page := 1; ; page++ {
		result := sendAndDecode[pageOf[struct {
			Student studentSummary `json:"student"`
		}]](t, env.discoveryEnv, http.MethodGet, path+"page="+strconv.Itoa(page), "", s, http.StatusOK)
		for _, item := range result.Items {
			if item.Student.ID == studentID {
				return true
			}
		}
		if page*result.PageSize >= result.Total {
			return false
		}
	}
}

// proposeAndAccept lets the teacher propose a one-hour online lesson and the learner accept it.
func (env *mvpEnv) proposeAndAccept(t *testing.T, exchangeID, teacherID string, teacher, learner *session, startsAt time.Time) string {
	t.Helper()
	body := fmt.Sprintf(`{"title":"Перше заняття","teacherId":%q,"startsAt":%q,"endsAt":%q,
		"format":"ONLINE","meetingUrl":"https://meet.example.test/lesson"}`,
		teacherID, startsAt.Format(time.RFC3339), startsAt.Add(time.Hour).Format(time.RFC3339))
	proposed := sendAndDecode[struct {
		Session sessionResponse `json:"session"`
	}](t, env.discoveryEnv, http.MethodPost, "/api/v1/exchanges/"+exchangeID+"/sessions", body, teacher, http.StatusCreated).Session
	accepted := sendAndDecode[struct {
		Session sessionResponse `json:"session"`
	}](t, env.discoveryEnv, http.MethodPost, "/api/v1/sessions/"+proposed.ID+"/accept", "", learner, http.StatusOK).Session
	expect.Equal(t, "lesson status after acceptance", accepted.Status, data.SessionScheduled)
	return proposed.ID
}

// runMVPScenario goes through the main use cases of the MVP: two students find each other,
// agree an exchange, hold and confirm both lessons, review each other, and then a third
// student is reported, blocked and suspended, a profile is hidden and an account deleted.
func runMVPScenario(t *testing.T, env *mvpEnv) {
	// Registration with email verification; an unverified student cannot send requests.
	olha, olhaID := env.newStudent(t, "olha", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	andrii, andriiID := env.newStudent(t, "andrii", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	expectStatus(t, send(t, env.handler, http.MethodPost, "/api/v1/auth/register", registerBody(env.email("viktor"), testPassword), nil),
		http.StatusCreated)
	viktor := login(t, env.handler, env.email("viktor"), testPassword)
	env.users = append(env.users, env.meID(t, viktor))
	assertProblem(t, send(t, env.handler, http.MethodPost, requestsPath, mvpRequestBody(andriiID), viktor),
		http.StatusForbidden, "email_not_verified")

	// The profile and both skill lists are kept after signing in again.
	olha = login(t, env.handler, env.email("olha"), testPassword)
	profile := sendAndDecode[struct {
		Profile profileResponse `json:"profile"`
	}](t, env.discoveryEnv, http.MethodGet, "/api/v1/me/profile", "", olha, http.StatusOK).Profile
	expect.Equal(t, "city", profile.City, "Київ")
	expect.Equal(t, "taught skill", profile.TeachingSkills[0].SkillID, datatest.GuitarSkillID)
	expect.Equal(t, "wanted skill", profile.LearningSkills[0].SkillID, datatest.PhotoshopSkillID)
	expect.Equal(t, "eligible for matching", profile.EligibleForMatching, true)

	// Olha finds Andrii by the skill and among mutual matches.
	expect.Equal(t, "Andrii found by Photoshop", env.findStudent(t, olha, "/api/v1/students?q=Photoshop&", andriiID), true)
	expect.Equal(t, "Andrii in mutual matches", env.findStudent(t, olha, "/api/v1/me/matches?", andriiID), true)
	candidate := env.studentProfile(t, olha, andriiID)
	expect.Equal(t, "candidate is a mutual match", candidate.Mutual != nil, true)
	expect.Equal(t, "candidate reviews", candidate.ReviewCount, 0)
	expect.Equal(t, "candidate rating is set", candidate.AverageRating != nil, false)

	// Olha asks for one lesson each way; a repeated acceptance gives the same exchange.
	request := sendAndDecode[struct {
		Request exchangeRequest `json:"request"`
	}](t, env.discoveryEnv, http.MethodPost, requestsPath, mvpRequestBody(andriiID), olha, http.StatusCreated).Request
	assertProblem(t, send(t, env.handler, http.MethodPost, requestsPath, mvpRequestBody(andriiID), olha),
		http.StatusConflict, "duplicate_request")
	acceptPath := requestsPath + "/" + request.ID + "/accept"
	type acceptance struct {
		Exchange exchangeResponse `json:"exchange"`
	}
	exchange := sendAndDecode[acceptance](t, env.discoveryEnv, http.MethodPost, acceptPath, "", andrii, http.StatusOK).Exchange
	expect.Equal(t, "exchange status", exchange.Status, data.ExchangeActive)
	expect.Equal(t, "total sessions", exchange.TotalSessions, 2)
	expect.Equal(t, "exchange after a repeated acceptance",
		sendAndDecode[acceptance](t, env.discoveryEnv, http.MethodPost, acceptPath, "", andrii, http.StatusOK).Exchange.ID, exchange.ID)
	exchangePath := "/api/v1/exchanges/" + exchange.ID

	// A private chat for the exchange; a third student cannot read it.
	marko, markoID := env.newStudent(t, "marko", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	chat := getConversation(t, env.discoveryEnv, exchangePath+"/conversation", olha)
	messagesPath := "/api/v1/conversations/" + chat.ID + "/messages"
	expectStatus(t, send(t, env.handler, http.MethodPost, messagesPath, `{"body":"Привіт! Коли тобі зручно?"}`, olha), http.StatusCreated)
	expectStatus(t, send(t, env.handler, http.MethodPost, messagesPath, `{"body":"Завтра о десятій"}`, andrii), http.StatusCreated)
	assertProblem(t, send(t, env.handler, http.MethodPost, messagesPath, `{"body":"   "}`, olha), http.StatusUnprocessableEntity, "validation_failed")
	messages := sendAndDecode[messagePageResponse](t, env.discoveryEnv, http.MethodGet, messagesPath, "", andrii, http.StatusOK)
	expect.Equal(t, "messages in the chat", messages.Total, 2)
	assertProblem(t, send(t, env.handler, http.MethodGet, "/api/v1/conversations/"+chat.ID, "", marko), http.StatusNotFound, "conversation_not_found")
	assertProblem(t, send(t, env.handler, http.MethodGet, messagesPath, "", marko), http.StatusNotFound, "conversation_not_found")

	// Andrii was notified about the request and the message; reading them clears the counter.
	notifications := sendAndDecode[notificationPageResponse](t, env.discoveryEnv, http.MethodGet, "/api/v1/me/notifications", "", andrii, http.StatusOK)
	events := map[string]bool{}
	for _, n := range notifications.Items {
		events[n.EventType] = true
	}
	expect.Equal(t, "request notification", events[data.EventExchangeRequest], true)
	expect.Equal(t, "message notification", events[data.EventChatMessage], true)
	expect.Equal(t, "unread notifications", notifications.UnreadCount > 0, true)
	expectStatus(t, send(t, env.handler, http.MethodPost, "/api/v1/me/notifications/read-all", "", andrii), http.StatusOK)
	expect.Equal(t, "unread after reading",
		sendAndDecode[notificationPageResponse](t, env.discoveryEnv, http.MethodGet, "/api/v1/me/notifications", "", andrii, http.StatusOK).UnreadCount, 0)

	// One lesson in each direction, both in the schedule of each participant only.
	start := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Minute)
	guitarLesson := env.proposeAndAccept(t, exchange.ID, olhaID, olha, andrii, start)
	photoshopLesson := env.proposeAndAccept(t, exchange.ID, andriiID, andrii, olha, start.Add(2*time.Hour))
	schedule := sendAndDecode[schedulePageResponse](t, env.discoveryEnv, http.MethodGet, "/api/v1/me/schedule", "", olha, http.StatusOK)
	expect.Equal(t, "lessons in Olha's schedule", schedule.Total, 2)
	expect.Equal(t, "first lesson", schedule.Items[0].ID, guitarLesson)
	expect.Equal(t, "lessons in Marko's schedule",
		sendAndDecode[schedulePageResponse](t, env.discoveryEnv, http.MethodGet, "/api/v1/me/schedule", "", marko, http.StatusOK).Total, 0)

	// After the lessons ended, a lesson counts only when both confirm; then both end the exchange.
	ended := time.Now().UTC().Add(-time.Hour).Truncate(time.Minute)
	env.moveSession(t, guitarLesson, ended.Add(-4*time.Hour), ended.Add(-3*time.Hour))
	env.moveSession(t, photoshopLesson, ended.Add(-time.Hour), ended)
	first := sendAndDecode[confirmationResponse](t, env.discoveryEnv, http.MethodPost, "/api/v1/sessions/"+guitarLesson+"/confirm", "", olha, http.StatusOK)
	expect.Equal(t, "lesson after one confirmation", first.Session.Status, data.SessionScheduled)
	expect.Equal(t, "progress after one confirmation", first.Exchange.Progress.CompletedSessions, 0)
	assertProblem(t, send(t, env.handler, http.MethodPost, exchangePath+"/complete", "", olha), http.StatusConflict, "plan_not_completed")
	for _, lesson := range []string{guitarLesson, photoshopLesson} {
		for _, s := range []*session{andrii, olha} {
			expectStatus(t, send(t, env.handler, http.MethodPost, "/api/v1/sessions/"+lesson+"/confirm", "", s), http.StatusOK)
		}
	}
	progress := getExchangeDetails(t, env.discoveryEnv, exchange.ID, olha).Exchange.Progress
	expect.Equal(t, "completed lessons", progress.CompletedSessions, 2)
	expect.Equal(t, "plan completed", progress.PlanCompleted, true)
	expectStatus(t, send(t, env.handler, http.MethodPost, exchangePath+"/complete", "", olha), http.StatusOK)
	completed := sendAndDecode[exchangeEnvelope](t, env.discoveryEnv, http.MethodPost, exchangePath+"/complete", "", andrii, http.StatusOK).Exchange
	expect.Equal(t, "exchange status after both confirmations", completed.Status, data.ExchangeCompleted)

	// One review each; the profile shows the rating.
	reviewsPath := exchangePath + "/reviews"
	expectStatus(t, send(t, env.handler, http.MethodPost, reviewsPath, `{"rating":5,"comment":"Дуже зрозуміло пояснює"}`, olha), http.StatusCreated)
	assertProblem(t, send(t, env.handler, http.MethodPost, reviewsPath, `{"rating":4}`, olha), http.StatusConflict, "review_already_exists")
	expectStatus(t, send(t, env.handler, http.MethodPost, reviewsPath, `{"rating":4}`, andrii), http.StatusCreated)
	assertProblem(t, send(t, env.handler, http.MethodPost, reviewsPath, `{"rating":1}`, marko), http.StatusNotFound, "exchange_not_found")
	rated := env.studentProfile(t, olha, andriiID)
	expect.Equal(t, "Andrii's reviews", rated.ReviewCount, 1)
	expect.Equal(t, "Andrii's rating", valueOf(rated.AverageRating), 5.0)

	// Andrii reports and blocks Marko, who can no longer send requests to Andrii; the
	// administrator suspends Marko, and the decision has an author and a reason.
	report := sendAndDecode[struct {
		Report reportResponse `json:"report"`
	}](t, env.discoveryEnv, http.MethodPost, "/api/v1/reports", `{"reportedUserId":"`+markoID+`","reason":"Спам у повідомленнях"}`, andrii, http.StatusCreated).Report
	expectStatus(t, send(t, env.handler, http.MethodPost, "/api/v1/students/"+markoID+"/block", "", andrii), http.StatusOK)
	assertProblem(t, send(t, env.handler, http.MethodPost, requestsPath, mvpRequestBody(andriiID), marko), http.StatusNotFound, "student_not_found")
	admin, adminID := env.newStudent(t, "admin", "", "")
	env.makeAdmin(t, adminID)
	resolved := sendAndDecode[struct {
		Report adminReportResponse `json:"report"`
	}](t, env.discoveryEnv, http.MethodPost, "/api/v1/admin/reports/"+report.ID+"/resolve",
		`{"decision":"SUSPEND","reason":"Скаргу підтверджено"}`, admin, http.StatusOK).Report
	expect.Equal(t, "decision author", resolved.Decision.Moderator.ID, adminID)
	expect.Equal(t, "decision reason", resolved.Decision.Reason, "Скаргу підтверджено")
	assertProblem(t, send(t, env.handler, http.MethodGet, "/api/v1/auth/me", "", marko), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, env.handler, http.MethodPost, "/api/v1/auth/login", loginBody(env.email("marko"), testPassword), nil),
		http.StatusForbidden, "account_suspended")

	// A hidden profile disappears for others but keeps the exchange; a deleted account
	// cannot sign in, and its history stays without personal data.
	expectStatus(t, send(t, env.handler, http.MethodPatch, "/api/v1/me/privacy", `{"isDiscoverable":false}`, olha), http.StatusOK)
	assertProblem(t, send(t, env.handler, http.MethodGet, "/api/v1/students/"+olhaID, "", andrii), http.StatusNotFound, "student_not_found")
	expect.Equal(t, "exchange of a hidden student", getExchangeDetails(t, env.discoveryEnv, exchange.ID, andrii).Exchange.Status, data.ExchangeCompleted)
	expectStatus(t, send(t, env.handler, http.MethodDelete, "/api/v1/me/account", `{"password":"`+testPassword+`"}`, andrii), http.StatusNoContent)
	assertProblem(t, send(t, env.handler, http.MethodGet, "/api/v1/auth/me", "", andrii), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, env.handler, http.MethodPost, "/api/v1/auth/login", loginBody(env.email("andrii"), testPassword), nil),
		http.StatusUnauthorized, "invalid_credentials")
	partner := getExchangeDetails(t, env.discoveryEnv, exchange.ID, olha).Exchange.Partner
	expect.Equal(t, "deleted partner's name", partner.FirstName+" "+partner.LastName, "Видалений користувач")
	var bodies []string
	for _, m := range sendAndDecode[messagePageResponse](t, env.discoveryEnv, http.MethodGet, messagesPath, "", olha, http.StatusOK).Items {
		bodies = append(bodies, m.Body)
	}
	expect.Slice(t, "messages after deletion", bodies, []string{"Привіт! Коли тобі зручно?", "Повідомлення видалено"})
}

func mvpRequestBody(recipientID string) string {
	return fmt.Sprintf(`{"recipientId":%q,"teachSkillId":%q,"learnSkillId":%q,"format":"ONLINE",
		"teachSessions":1,"teachDurationMinutes":60,"learnSessions":1,"learnDurationMinutes":60,
		"message":"Давай обміняємося: гітара на Photoshop"}`, recipientID, datatest.GuitarSkillID, datatest.PhotoshopSkillID)
}

func TestMVPScenario(t *testing.T) {
	e := newDiscoveryEnv()
	runMVPScenario(t, &mvpEnv{
		discoveryEnv: e,
		prefix:       "mvp",
		moveSession: func(_ *testing.T, sessionID string, startsAt, endsAt time.Time) {
			e.store.SetSessionTime(sessionID, startsAt, endsAt)
		},
		makeAdmin: func(_ *testing.T, userID string) { e.store.SetAdmin(userID) },
	})
}

// The same scenario through the real database. Every row it creates is deleted at the end.
// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded local database.
func TestMVPScenarioAgainstLocalPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for the local PostgreSQL integration test")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	mail := &mailertest.Recorder{}
	exec := func(t *testing.T, query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(context.Background(), query, args...); err != nil {
			t.Fatal(err)
		}
	}
	env := &mvpEnv{
		discoveryEnv: &discoveryEnv{handler: postgresHandler(pool, mail), mail: mail},
		prefix:       fmt.Sprintf("mvp-%d", time.Now().UnixNano()),
		moveSession: func(t *testing.T, sessionID string, startsAt, endsAt time.Time) {
			exec(t, `UPDATE sessions SET scheduled_start = $2, scheduled_end = $3 WHERE id = $1`, sessionID, startsAt, endsAt)
		},
		makeAdmin: func(t *testing.T, userID string) {
			exec(t, `UPDATE users SET role = 'ADMIN' WHERE id = $1`, userID)
		},
	}
	t.Cleanup(func() { deleteUsers(t, pool, env.users, env.prefix) })
	runMVPScenario(t, env)
}

// deleteUsers removes the users and everything that refers to them, children first.
func deleteUsers(t *testing.T, pool *pgxpool.Pool, users []string, emailPrefix string) {
	const (
		requests      = `SELECT id FROM exchange_requests WHERE requester_id = ANY($1) OR recipient_id = ANY($1)`
		exchanges     = `SELECT id FROM exchanges WHERE user_a_id = ANY($1) OR user_b_id = ANY($1)`
		sessions      = `SELECT id FROM sessions WHERE exchange_id IN (` + exchanges + `)`
		conversations = `SELECT id FROM conversations WHERE exchange_request_id IN (` + requests + `) OR exchange_id IN (` + exchanges + `)`
		messages      = `SELECT id FROM messages WHERE conversation_id IN (` + conversations + `)`
	)
	for _, statement := range []string{
		`DELETE FROM notifications WHERE recipient_id = ANY($1) OR actor_id = ANY($1)`,
		`DELETE FROM message_receipts WHERE user_id = ANY($1) OR message_id IN (` + messages + `)`,
		`DELETE FROM messages WHERE id IN (` + messages + `)`,
		`DELETE FROM conversation_participants WHERE conversation_id IN (` + conversations + `)`,
		`DELETE FROM conversations WHERE id IN (` + conversations + `)`,
		`DELETE FROM exchange_reviews WHERE exchange_id IN (` + exchanges + `)`,
		`DELETE FROM session_confirmations WHERE session_id IN (` + sessions + `)`,
		`DELETE FROM session_schedule_proposals WHERE session_id IN (` + sessions + `)`,
		`DELETE FROM sessions WHERE id IN (` + sessions + `)`,
		`DELETE FROM exchange_confirmations WHERE exchange_id IN (` + exchanges + `)`,
		`DELETE FROM exchange_status_history WHERE exchange_id IN (` + exchanges + `)`,
		`DELETE FROM exchange_skill_commitments WHERE exchange_id IN (` + exchanges + `)`,
		`DELETE FROM exchanges WHERE id IN (` + exchanges + `)`,
		`DELETE FROM exchange_request_status_history WHERE request_id IN (` + requests + `)`,
		`DELETE FROM exchange_requests WHERE id IN (` + requests + `)`,
		`DELETE FROM meeting_places WHERE user_id = ANY($1)`,
		`DELETE FROM moderation_actions WHERE moderator_id = ANY($1) OR subject_user_id = ANY($1)`,
		`DELETE FROM reports WHERE reporter_id = ANY($1) OR reported_user_id = ANY($1)`,
		`DELETE FROM user_blocks WHERE blocker_id = ANY($1) OR blocked_id = ANY($1)`,
		`DELETE FROM user_teaching_skills WHERE user_id = ANY($1)`,
		`DELETE FROM user_learning_skills WHERE user_id = ANY($1)`,
		`DELETE FROM user_lesson_formats WHERE user_id = ANY($1)`,
		`DELETE FROM user_profile_settings WHERE user_id = ANY($1)`,
		`DELETE FROM user_statistics WHERE user_id = ANY($1)`,
		`DELETE FROM user_verifications WHERE user_id = ANY($1)`,
		`DELETE FROM user_sessions WHERE user_id = ANY($1)`,
		`DELETE FROM one_time_tokens WHERE user_id = ANY($1)`,
		`DELETE FROM auth_identities WHERE user_id = ANY($1)`,
		`DELETE FROM users WHERE id = ANY($1)`,
	} {
		if _, err := pool.Exec(context.Background(), statement, users); err != nil {
			t.Errorf("clean up: %v", err)
		}
	}
	if _, err := pool.Exec(context.Background(), `DELETE FROM login_throttles WHERE email LIKE $1`, emailPrefix+".%"); err != nil {
		t.Errorf("clean up: %v", err)
	}
}
