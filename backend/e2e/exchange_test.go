package e2e

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"skillswap/backend/internal/expect"
)

func requestBody(recipient string) map[string]any {
	return map[string]any{
		"recipientId": recipient, "teachSkillId": guitar, "learnSkillId": photoshop,
		"format": "ONLINE", "teachSessions": 1, "teachDurationMinutes": 60,
		"learnSessions": 1, "learnDurationMinutes": 60, "message": "Обміняймося навичками",
	}
}

func (s *stack) exchange(a, b *browser) (string, string) {
	s.t.Helper()
	r := s.request(a, "POST", "/exchange-requests", requestBody(b.id), 201)
	requestID := r.str("request", "id")
	ex := s.request(b, "POST", "/exchange-requests/"+requestID+"/accept", nil, 200)
	return requestID, ex.str("exchange", "id")
}

func lessonBody(teacher string, start time.Time) map[string]any {
	return map[string]any{
		"title": "Практичне заняття", "teacherId": teacher, "startsAt": start.Format(time.RFC3339),
		"endsAt": start.Add(time.Hour).Format(time.RFC3339), "format": "ONLINE", "meetingUrl": "https://meet.example.test/private-lesson",
	}
}

func (s *stack) lesson(a, b *browser, ex string, start time.Time) string {
	s.t.Helper()
	r := s.request(a, "POST", "/exchanges/"+ex+"/sessions", lessonBody(a.id, start), 201)
	id := r.str("session", "id")
	s.request(b, "POST", "/sessions/"+id+"/accept", nil, 200)
	return id
}

func (s *stack) finish(a, b *browser, ex string) {
	s.t.Helper()
	for i, pair := range [][2]*browser{{a, b}, {b, a}} {
		id := s.lesson(pair[0], pair[1], ex, future().Add(time.Duration(i)*2*time.Hour))
		s.sql(`UPDATE sessions SET scheduled_start = now() - interval '2 hours', scheduled_end = now() - interval '1 hour' WHERE id = $1`, id)
		s.request(a, "POST", "/sessions/"+id+"/confirm", nil, 200)
		s.request(b, "POST", "/sessions/"+id+"/confirm", nil, 200)
	}
	s.request(a, "POST", "/exchanges/"+ex+"/complete", nil, 200)
	s.request(b, "POST", "/exchanges/"+ex+"/complete", nil, 200)
}

func TestE2ERequestDecisions(t *testing.T) {
	s := newStack(t)
	a := s.student("requester", guitar, photoshop)
	b := s.student("recipient", photoshop, guitar)
	stranger := s.student("stranger", photoshop, guitar)
	s.request(a, "POST", "/exchange-requests", requestBody(a.id), 422).invalid("recipientId")
	bad := requestBody(b.id)
	bad["message"] = strings.Repeat("я", 501)
	s.request(a, "POST", "/exchange-requests", bad, 422).invalid("message")
	bad = requestBody(b.id)
	bad["teachSkillId"] = "30000000-0000-0000-0000-000000000003"
	s.request(a, "POST", "/exchange-requests", bad, 422).invalid("teachSkillId")
	for _, decision := range []string{"decline", "withdraw"} {
		r := s.request(a, "POST", "/exchange-requests", requestBody(b.id), 201)
		path := "/exchange-requests/" + r.str("request", "id")
		s.request(a, "POST", "/exchange-requests", requestBody(b.id), 409).problem("duplicate_request")
		for _, entry := range []struct {
			user      *browser
			direction string
		}{{a, "outgoing"}, {b, "incoming"}} {
			list := s.request(entry.user, "GET", "/me/exchange-requests?direction="+entry.direction+"&status=PENDING", nil, 200)
			expect.Equal(t, "request in participant list", list.num("total"), 1)
			expect.Equal(t, "listed request", list.items("items")[0].str("id"), r.str("request", "id"))
		}
		for _, action := range []string{"accept", "decline", "withdraw"} {
			s.request(stranger, "POST", path+"/"+action, nil, 404).problem("request_not_found")
		}
		actor := b
		if decision == "withdraw" {
			actor = a
		}
		s.request(actor, "POST", path+"/"+decision, nil, 200)
		s.request(b, "POST", path+"/accept", nil, 409).problem("request_not_pending")
		expect.Equal(t, "request history retained", len(s.request(a, "GET", path, nil, 200).items("history")), 2)
	}
	requestID, ex := s.exchange(a, b)
	accepted := s.request(b, "POST", "/exchange-requests/"+requestID+"/accept", nil, 200)
	expect.Equal(t, "repeated acceptance returns same exchange", accepted.str("exchange", "id"), ex)
	s.request(a, "DELETE", "/me/teaching-skills/"+guitar, nil, 200)
	details := s.request(a, "GET", "/exchanges/"+ex, nil, 200)
	expect.Equal(t, "accepted skill snapshot", details.str("exchange", "requesterTeaches", "skill", "skillId"), guitar)
	expect.Equal(t, "accepted lesson plan", len(details.items("sessions")), 2)
	for _, lesson := range details.items("sessions") {
		expect.Equal(t, "initial lesson status", lesson.str("status"), "DRAFT")
	}
}

func TestE2EChatAndNotifications(t *testing.T) {
	s := newStack(t)
	a := s.student("sender", guitar, photoshop)
	b := s.student("reader", photoshop, guitar)
	c := s.student("outsider", "", "")
	requestID, ex := s.exchange(a, b)
	conversation := s.request(a, "GET", "/exchange-requests/"+requestID+"/conversation", nil, 200).str("conversation", "id")
	expect.Equal(t, "chat preserved when accepted", s.request(b, "GET", "/exchanges/"+ex+"/conversation", nil, 200).str("conversation", "id"), conversation)
	path := "/conversations/" + conversation
	for _, body := range []string{" ", strings.Repeat("я", 2001)} {
		s.request(a, "POST", path+"/messages", map[string]string{"body": body}, 422).invalid("body")
	}
	text := `<script>alert('xss')</script> https://example.test/lesson '; DROP TABLE messages; --`
	message := s.request(a, "POST", path+"/messages", map[string]string{"body": text}, 201)
	expect.Equal(t, "message author", message.str("message", "author", "id"), a.id)
	expect.Equal(t, "message timestamp present", message.str("message", "createdAt") != "", true)
	s.request(a, "POST", path+"/messages", map[string]string{"body": strings.Repeat("я", 2000)}, 201)
	for _, endpoint := range []string{path, path + "/messages"} {
		s.request(c, "GET", endpoint, nil, 404).problem("conversation_not_found")
	}
	s.request(c, "POST", path+"/messages", map[string]string{"body": "Сторонній"}, 404).problem("conversation_not_found")
	s.request(b, "POST", "/auth/logout", nil, 204)
	b = s.login(b.email, password)
	history := s.request(b, "GET", path+"/messages", nil, 200)
	expect.Equal(t, "chat polling interval", history.num("pollIntervalSeconds"), 5)
	expect.Equal(t, "chat survives login", history.num("total"), 2)
	expect.Equal(t, "text remains literal", history.items("items")[0].str("body"), text)
	expect.Equal(t, "private content not cached", history.header.Get("Cache-Control"), "no-store")
	expect.Equal(t, "MIME sniffing disabled", history.header.Get("X-Content-Type-Options"), "nosniff")
	polled := s.request(b, "GET", path+"/messages?after="+message.str("message", "id"), nil, 200)
	expect.Equal(t, "poll returns only later messages", len(polled.items("items")), 1)
	s.request(b, "POST", path+"/read", nil, 200)
	expect.Equal(t, "read chat clears count", s.request(b, "GET", path, nil, 200).num("conversation", "unreadCount"), 0)
	_, secondEx := s.exchange(a, cWithSkills(s, c))
	s.lesson(a, c, secondEx, future())
	// This separate conversation must not affect the reader's event list.
	for _, event := range s.request(b, "GET", "/me/notifications", nil, 200).items("items") {
		expect.Equal(t, "event belongs to the intended exchange", event.value("exchangeId") == secondEx, false)
	}
	notifications := s.request(b, "GET", "/me/notifications", nil, 200)
	var requestEvent reply
	messageEvents := 0
	for _, event := range notifications.items("items") {
		if event.str("eventType") == "EXCHANGE_REQUEST" {
			requestEvent = event
		}
		if event.str("eventType") == "CHAT_MESSAGE" {
			messageEvents++
		}
	}
	expect.Equal(t, "one event per message", messageEvents, 2)
	expect.Equal(t, "request event target", requestEvent.str("target", "id"), requestID)
	id := requestEvent.str("id")
	s.request(c, "POST", "/me/notifications/"+id+"/read", nil, 404).problem("notification_not_found")
	s.request(a, "POST", path+"/messages", map[string]string{"body": "Нове непрочитане"}, 201)
	notifications = s.request(b, "GET", "/me/notifications", nil, 200)
	for _, event := range notifications.items("items") {
		if !event.flag("isRead") {
			id = event.str("id")
			break
		}
	}
	before := notifications.num("unreadCount")
	expect.Equal(t, "an unread event exists", before > 0, true)
	s.request(b, "POST", "/me/notifications/"+id+"/read", nil, 200)
	s.request(b, "POST", "/me/notifications/"+id+"/read", nil, 200)
	expect.Equal(t, "reading is idempotent", s.request(b, "GET", "/me/notifications", nil, 200).num("unreadCount"), before-1)
	s.request(b, "POST", "/me/notifications/read-all", nil, 200)
	expect.Equal(t, "all read clears counter", s.request(b, "GET", "/me/notifications", nil, 200).num("unreadCount"), 0)
}

func cWithSkills(s *stack, b *browser) *browser {
	s.request(b, "POST", "/me/teaching-skills", map[string]string{"skillId": photoshop, "level": "ADVANCED"}, 201)
	s.request(b, "POST", "/me/learning-skills", map[string]string{"skillId": guitar, "level": "BEGINNER"}, 201)
	return b
}

func TestE2ESchedulingCompletionAndReviews(t *testing.T) {
	s := newStack(t)
	a := s.student("teacher", guitar, photoshop)
	b := s.student("learner", photoshop, guitar)
	c := s.student("third", photoshop, guitar)
	_, ex := s.exchange(a, b)
	_, otherEx := s.exchange(a, c)
	exPath := "/exchanges/" + ex
	noReviews := s.request(c, "GET", "/students/"+b.id, nil, 200)
	expect.Equal(t, "no reviews yet", noReviews.num("student", "reviewCount"), 0)
	expect.Equal(t, "no fabricated rating", noReviews.value("student", "averageRating"), nil)
	invalidEnd := lessonBody(a.id, future())
	invalidEnd["endsAt"] = future().Add(-time.Hour).Format(time.RFC3339)
	for _, body := range []map[string]any{lessonBody(a.id, time.Now().Add(-time.Hour)), invalidEnd} {
		s.request(a, "POST", exPath+"/sessions", body, 422).problem("validation_failed")
	}
	for _, key := range []string{"startsAt", "endsAt"} {
		body := lessonBody(a.id, future())
		body[key] = future().Format("2006-01-02T15:04:05")
		s.request(a, "POST", exPath+"/sessions", body, 422).invalid(key)
	}
	s.request(a, "POST", exPath+"/complete", nil, 409).problem("plan_not_completed")
	s.request(a, "POST", exPath+"/reviews", map[string]int{"rating": 5}, 409).problem("exchange_not_completed")
	start := future().In(time.FixedZone("Kyiv", 2*60*60))
	proposal := s.request(a, "POST", exPath+"/sessions", lessonBody(a.id, start), 201)
	first := proposal.str("session", "id")
	firstPath := "/sessions/" + first
	s.sql(`UPDATE session_schedule_proposals SET proposed_start = now() - interval '2 hours', proposed_end = now() - interval '1 hour' WHERE session_id = $1 AND status = 'PENDING'`, first)
	s.request(b, "POST", firstPath+"/accept", nil, 409).problem("session_in_past")
	s.sql(`UPDATE session_schedule_proposals SET proposed_start = $2, proposed_end = $3 WHERE session_id = $1 AND status = 'PENDING'`, first, start, start.Add(time.Hour))
	s.request(b, "POST", firstPath+"/accept", nil, 200)
	s.request(a, "POST", firstPath+"/confirm", nil, 409).problem("session_not_finished")
	s.request(c, "GET", firstPath, nil, 404).problem("session_not_found")
	s.request(c, "GET", exPath, nil, 404).problem("exchange_not_found")
	for _, action := range []string{"accept", "decline", "cancel", "confirm"} {
		s.request(c, "POST", firstPath+"/"+action, nil, 404).problem("session_not_found")
	}
	s.request(c, "POST", firstPath+"/proposals", lessonBody(a.id, future()), 404).problem("session_not_found")
	detail := s.request(b, "GET", firstPath, nil, 200)
	parsed, err := time.Parse(time.RFC3339, detail.str("session", "startsAt"))
	expect.NoError(t, "parse zoned time", err)
	expect.Equal(t, "same stored instant", parsed.Equal(start), true)
	conflict := lessonBody(a.id, start.Add(30*time.Minute))
	s.request(a, "POST", "/exchanges/"+otherEx+"/sessions", conflict, 409).problem("session_conflict")
	adjacent := s.lesson(a, c, otherEx, start.Add(time.Hour))
	expect.Equal(t, "adjacent interval accepted", adjacent != "", true)
	s.request(a, "POST", firstPath+"/proposals", lessonBody(a.id, start.Add(4*time.Hour)), 201)
	expect.Equal(t, "rescheduling needs consent", s.request(b, "GET", firstPath, nil, 200).str("session", "status"), "PROPOSED")
	s.request(b, "POST", firstPath+"/decline", nil, 200)
	s.request(a, "POST", firstPath+"/proposals", lessonBody(a.id, start.Add(4*time.Hour)), 201)
	s.request(b, "POST", firstPath+"/accept", nil, 200)
	second := s.lesson(b, a, ex, start.Add(6*time.Hour))
	for _, participant := range []*browser{a, b} {
		items := s.request(participant, "GET", "/me/schedule?upcoming=true", nil, 200).items("items")
		var previous time.Time
		for _, item := range items {
			instant, err := time.Parse(time.RFC3339, item.str("startsAt"))
			expect.NoError(t, "parse schedule time", err)
			expect.Equal(t, "chronological schedule", instant.Before(previous), false)
			expect.Equal(t, "own meetings only", item.str("teacher", "id") == participant.id || item.str("learner", "id") == participant.id, true)
			previous = instant
		}
	}
	s.request(b, "POST", "/sessions/"+second+"/cancel", map[string]string{"reason": "Змінився розклад"}, 200)
	expect.Equal(t, "cancellation does not count", s.request(a, "GET", exPath, nil, 200).num("exchange", "progress", "completedSessions"), 0)
	second = s.lesson(b, a, ex, start.Add(6*time.Hour))
	s.sql(`UPDATE sessions SET scheduled_start = now() - interval '2 hours', scheduled_end = now() - interval '1 hour' WHERE id = ANY($1::uuid[])`, []string{first, second})
	confirmed := s.request(a, "POST", firstPath+"/confirm", nil, 200)
	expect.Equal(t, "unilateral confirmation does not count", confirmed.num("exchange", "progress", "completedSessions"), 0)
	for _, id := range []string{first, second} {
		s.request(a, "POST", "/sessions/"+id+"/confirm", nil, 200)
		s.request(b, "POST", "/sessions/"+id+"/confirm", nil, 200)
	}
	progress := s.request(a, "GET", exPath, nil, 200)
	expect.Equal(t, "both directions completed", progress.flag("exchange", "progress", "planCompleted"), true)
	expect.Equal(t, "full progress", progress.num("exchange", "progress", "percent"), 100)
	expect.Equal(t, "one final confirmation stays active", s.request(a, "POST", exPath+"/complete", nil, 200).str("exchange", "status"), "ACTIVE")
	expect.Equal(t, "both confirmations finish exchange", s.request(b, "POST", exPath+"/complete", nil, 200).str("exchange", "status"), "COMPLETED")
	s.request(c, "POST", exPath+"/reviews", map[string]int{"rating": 5}, 404).problem("exchange_not_found")
	for _, rating := range []int{0, 6} {
		s.request(a, "POST", exPath+"/reviews", map[string]int{"rating": rating}, 422).invalid("rating")
	}
	s.request(a, "POST", exPath+"/reviews", map[string]any{"rating": 5, "comment": strings.Repeat("я", 1001)}, 422).invalid("comment")
	s.request(a, "POST", exPath+"/reviews", map[string]any{"rating": 5, "recipientId": a.id}, 400).problem("invalid_json")
	s.request(a, "POST", exPath+"/reviews", map[string]any{"rating": 5, "comment": strings.Repeat("я", 1000)}, 201)
	s.request(a, "POST", exPath+"/reviews", map[string]int{"rating": 4}, 409).problem("review_already_exists")
	reverse := s.request(b, "POST", exPath+"/reviews", map[string]int{"rating": 4}, 201)
	expect.Equal(t, "review author is session user", reverse.str("review", "author", "id"), b.id)
	rated := s.request(c, "GET", "/students/"+b.id, nil, 200)
	expect.Equal(t, "rating", rated.num("student", "averageRating"), 5)
	expect.Equal(t, "review count", rated.num("student", "reviewCount"), 1)
	_, another := s.exchange(a, b)
	s.finish(a, b, another)
	s.request(a, "POST", "/exchanges/"+another+"/reviews", map[string]int{"rating": 4}, 201)
	rated = s.request(c, "GET", "/students/"+b.id, nil, 200)
	expect.Equal(t, "aggregate rating keeps one decimal", rated.num("student", "averageRating"), 4.5)
	expect.Equal(t, "aggregate review count", rated.num("student", "reviewCount"), 2)
	s.request(a, "POST", "/exchanges/"+otherEx+"/cancel", map[string]string{"reason": "Немає часу"}, 200)
	s.request(a, "POST", "/exchanges/"+otherEx+"/reviews", map[string]int{"rating": 5}, 409).problem("exchange_not_completed")
	expect.Equal(t, "cancelling exchange cancels future meeting", s.request(c, "GET", "/sessions/"+adjacent, nil, 200).str("session", "status"), "CANCELLED")
}

func TestE2EOfflinePlanningAndPrivacy(t *testing.T) {
	s := newStack(t)
	a := s.student("offline-a", guitar, photoshop)
	b := s.student("offline-b", photoshop, guitar)
	c := s.student("offline-outsider", "", "")
	body := requestBody(b.id)
	body["format"] = "OFFLINE"
	s.request(b, "PATCH", "/me/profile", map[string]string{"city": "Львів"}, 200)
	s.request(a, "POST", "/exchange-requests", body, 422).invalid("format")
	s.request(b, "PATCH", "/me/profile", map[string]string{"city": "Київ"}, 200)
	request := s.request(a, "POST", "/exchange-requests", body, 201).str("request", "id")
	ex := s.request(b, "POST", "/exchange-requests/"+request+"/accept", nil, 200).str("exchange", "id")
	body = lessonBody(a.id, future())
	body["format"] = "OFFLINE"
	delete(body, "meetingUrl")
	s.request(a, "POST", "/exchanges/"+ex+"/sessions", body, 422).invalid("meetingPlace")
	body["meetingPlace"] = "Бібліотека, кімната 401"
	lesson := s.request(a, "POST", "/exchanges/"+ex+"/sessions", body, 201).str("session", "id")
	s.request(b, "POST", "/sessions/"+lesson+"/accept", nil, 200)
	for _, viewer := range []*browser{a, b} {
		details := s.request(viewer, "GET", "/sessions/"+lesson, nil, 200)
		expect.Equal(t, "private offline meeting place", details.str("session", "meetingPlace"), "Бібліотека, кімната 401")
	}
	s.request(c, "GET", "/sessions/"+lesson, nil, 404).problem("session_not_found")
	public := encode(s.request(c, "GET", "/students/"+a.id, nil, 200).body)
	expect.Equal(t, "public profile excludes exact meeting place", strings.Contains(public, "кімната 401"), false)
}

func (s *stack) concurrently(browsers []*browser, path string, body any) []reply {
	s.t.Helper()
	start := make(chan struct{})
	results := make([]reply, len(browsers))
	errors := make([]error, len(browsers))
	var group sync.WaitGroup
	for i, b := range browsers {
		group.Go(func() { <-start; results[i], errors[i] = s.raw(b, "POST", path, encode(body), nil) })
	}
	close(start)
	group.Wait()
	for _, err := range errors {
		expect.NoError(s.t, "concurrent HTTP request", err)
	}
	return results
}

func TestE2EAtomicityAndConcurrentRetries(t *testing.T) {
	s := newStack(t)
	a := s.student("atomic-a", guitar, photoshop)
	b := s.student("atomic-b", photoshop, guitar)
	r := s.request(a, "POST", "/exchange-requests", requestBody(b.id), 201)
	requestID := r.str("request", "id")
	path := "/exchange-requests/" + requestID + "/accept"
	// Fail at the final notification insert, after exchange and lesson creation.
	s.sql(`CREATE FUNCTION fail_acceptance() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
		IF NEW.event_type = 'REQUEST_RESPONSE' THEN RAISE EXCEPTION 'injected acceptance failure'; END IF;
		RETURN NEW; END $$`)
	s.sql(`CREATE TRIGGER fail_acceptance BEFORE INSERT ON notifications FOR EACH ROW EXECUTE FUNCTION fail_acceptance()`)
	s.request(b, "POST", path, nil, 503).problem("database_unavailable")
	expect.Equal(t, "request status rolled back", s.request(a, "GET", "/exchange-requests/"+requestID, nil, 200).str("request", "status"), "PENDING")
	expect.Equal(t, "no partial exchange", s.count(`SELECT count(*) FROM exchanges WHERE source_request_id = $1`, requestID), 0)
	expect.Equal(t, "no partial commitments", s.count(`SELECT count(*) FROM exchange_skill_commitments WHERE teacher_id = $1 OR learner_id = $1`, a.id), 0)
	expect.Equal(t, "no partial lesson plan", s.count(`SELECT count(*) FROM sessions WHERE teacher_id = $1 OR learner_id = $1`, a.id), 0)
	expect.Equal(t, "no response event", s.count(`SELECT count(*) FROM notifications WHERE event_type = 'REQUEST_RESPONSE' AND exchange_request_id = $1`, requestID), 0)
	s.sql(`DROP TRIGGER fail_acceptance ON notifications`)
	s.sql(`DROP FUNCTION fail_acceptance()`)
	accepted := s.concurrently([]*browser{b, b}, path, nil)
	for _, response := range accepted {
		expect.Equal(t, "acceptance status", response.status, 200)
	}
	ex := accepted[0].str("exchange", "id")
	expect.Equal(t, "both retries return one exchange", accepted[1].str("exchange", "id"), ex)
	expect.Equal(t, "one persisted exchange", s.count(`SELECT count(*) FROM exchanges WHERE source_request_id = $1`, requestID), 1)
	expect.Equal(t, "one pair of commitments", s.count(`SELECT count(*) FROM exchange_skill_commitments WHERE exchange_id = $1`, ex), 2)
	expect.Equal(t, "one lesson plan", s.count(`SELECT count(*) FROM sessions WHERE exchange_id = $1`, ex), 2)
	expect.Equal(t, "one response notification", s.count(`SELECT count(*) FROM notifications WHERE event_type = 'REQUEST_RESPONSE' AND exchange_id = $1`, ex), 1)
	for i, pair := range [][2]*browser{{a, b}, {b, a}} {
		id := s.lesson(pair[0], pair[1], ex, future().Add(time.Duration(i)*2*time.Hour))
		s.sql(`UPDATE sessions SET scheduled_start = now() - interval '2 hours', scheduled_end = now() - interval '1 hour' WHERE id = $1`, id)
		responses := s.concurrently([]*browser{a, b, a, b}, "/sessions/"+id+"/confirm", nil)
		for _, response := range responses {
			expect.Equal(t, "concurrent confirmation status", response.status, 200)
		}
		expect.Equal(t, "one confirmation per participant", s.count(`SELECT count(*) FROM session_confirmations WHERE session_id = $1 AND kind = 'COMPLETION'`, id), 2)
		expect.Equal(t, "one completed lesson", s.count(`SELECT count(*) FROM sessions WHERE id = $1 AND status = 'COMPLETED'`, id), 1)
	}
	for _, response := range s.concurrently([]*browser{a, b}, "/exchanges/"+ex+"/complete", nil) {
		expect.Equal(t, "concurrent completion status", response.status, 200)
	}
	statuses := []int{}
	for _, response := range s.concurrently([]*browser{a, a}, "/exchanges/"+ex+"/reviews", map[string]int{"rating": 5}) {
		statuses = append(statuses, response.status)
	}
	slices.Sort(statuses)
	expect.Slice(t, "one review wins the race", statuses, []int{201, 409})
	expect.Equal(t, "one persisted review", s.count(`SELECT count(*) FROM exchange_reviews WHERE exchange_id = $1 AND author_id = $2`, ex, a.id), 1)
	expect.Equal(t, "one review notification", s.count(`SELECT count(*) FROM notifications WHERE event_type = 'REVIEW_RECEIVED' AND exchange_id = $1`, ex), 1)
}
