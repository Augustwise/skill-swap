package e2e

import (
	"strings"
	"testing"
	"time"

	"skillswap/backend/internal/expect"
)

func TestE2EModerationAndPrivacy(t *testing.T) {
	s := newStack(t)
	a := s.student("private-a", guitar, photoshop)
	b := s.student("private-b", photoshop, guitar)
	c := s.student("private-c", guitar, photoshop)
	admin := s.student("moderator", "", "")
	s.sql(`UPDATE users SET role = 'ADMIN' WHERE id = $1`, admin.id)
	_, ex := s.exchange(a, b)
	lesson := s.lesson(a, b, ex, future())
	chat := s.request(a, "GET", "/exchanges/"+ex+"/conversation", nil, 200).str("conversation", "id")
	chatPath := "/conversations/" + chat + "/messages"
	s.request(a, "POST", chatPath, map[string]string{"body": "Приватне ім'я та контакт автора"}, 201)
	s.request(b, "POST", chatPath, map[string]string{"body": "Історія партнера"}, 201)
	s.request(a, "PATCH", "/me/privacy", map[string]bool{"isDiscoverable": false}, 200)
	expect.Equal(t, "hidden profile excluded from search", s.contains(c, "/students?", a.id), false)
	expect.Equal(t, "hidden profile excluded from matches", s.contains(b, "/me/matches?", a.id), false)
	s.request(c, "GET", "/students/"+a.id, nil, 404).problem("student_not_found")
	s.request(c, "POST", "/exchange-requests", requestBody(a.id), 404).problem("student_not_found")
	expect.Equal(t, "hiding keeps active exchange", s.request(b, "GET", "/exchanges/"+ex, nil, 200).str("exchange", "status"), "ACTIVE")
	s.request(a, "POST", chatPath, map[string]string{"body": "Обмін продовжується"}, 201)
	s.request(a, "PATCH", "/me/privacy", map[string]bool{"isDiscoverable": true}, 200)
	for _, operation := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/admin/reports", nil}, {"GET", "/admin/moderation-actions", nil},
		{"POST", "/admin/users/" + b.id + "/suspend", map[string]string{"reason": "Без прав"}},
		{"POST", "/admin/skills", map[string]string{"name": "Без прав"}},
	} {
		s.request(a, operation.method, operation.path, operation.body, 403).problem("admin_required")
	}
	reportID := s.request(a, "POST", "/reports", map[string]string{"reportedUserId": b.id, "reason": "Образливі повідомлення"}, 201).str("report", "id")
	s.request(a, "GET", "/admin/reports/"+reportID, nil, 403).problem("admin_required")
	s.request(admin, "GET", "/admin/reports/"+reportID, nil, 200)
	s.request(a, "POST", "/students/"+b.id+"/block", nil, 200)
	expect.Equal(t, "block cancels shared exchange", s.request(a, "GET", "/exchanges/"+ex, nil, 200).str("exchange", "status"), "CANCELLED")
	expect.Equal(t, "block cancels future lesson", s.request(b, "GET", "/sessions/"+lesson, nil, 200).str("session", "status"), "CANCELLED")
	for _, participant := range []*browser{a, b} {
		s.request(participant, "POST", chatPath, map[string]string{"body": "Заблоковане"}, 409).problem("conversation_blocked")
	}
	s.request(a, "POST", "/exchange-requests", requestBody(b.id), 404).problem("student_not_found")
	s.request(a, "GET", chatPath, nil, 200)
	decision := s.request(admin, "POST", "/admin/reports/"+reportID+"/resolve", map[string]string{"decision": "SUSPEND", "reason": "Порушення підтверджено"}, 200)
	expect.Equal(t, "decision author", decision.str("report", "decision", "moderator", "id"), admin.id)
	expect.Equal(t, "decision reason", decision.str("report", "decision", "reason"), "Порушення підтверджено")
	_, err := time.Parse(time.RFC3339, decision.str("report", "decision", "createdAt"))
	expect.NoError(t, "decision timestamp", err)
	s.request(b, "GET", "/auth/me", nil, 401).problem("unauthenticated")
	s.request(nil, "POST", "/auth/login", map[string]string{"email": b.email, "password": password}, 403).problem("account_suspended")
	s.request(admin, "POST", "/admin/users/"+b.id+"/restore", map[string]string{"reason": "Обмеження знято"}, 200)
	b = s.login(b.email, password)
	s.request(a, "DELETE", "/students/"+b.id+"/block", nil, 204)
	expect.Equal(t, "unblocking does not restore exchange", s.request(a, "GET", "/exchanges/"+ex, nil, 200).str("exchange", "status"), "CANCELLED")
	skill := s.request(admin, "POST", "/admin/skills", map[string]string{
		"categoryId": "20000000-0000-0000-0000-000000000002", "name": "E2E дизайн", "slug": "e2e-design", "description": "Каталог",
	}, 201).str("skill", "id")
	s.request(admin, "PATCH", "/admin/skills/"+skill, map[string]any{"name": "E2E новий дизайн", "isActive": false}, 200)
	s.request(a, "POST", "/me/teaching-skills", map[string]string{"skillId": skill, "level": "BEGINNER"}, 404).problem("skill_not_found")
	ownActions := 0
	for _, action := range s.request(admin, "GET", "/admin/moderation-actions", nil, 200).items("items") {
		if action.str("moderator", "id") != admin.id {
			continue
		}
		ownActions++
		expect.Equal(t, "audit author", action.str("moderator", "id"), admin.id)
		expect.Equal(t, "audit reason", action.str("reason") != "", true)
		expect.Equal(t, "audit date", action.str("createdAt") != "", true)
	}
	expect.Equal(t, "account and catalog actions were audited", ownActions, 4)
	_, active := s.exchange(a, b)
	futureLesson := s.lesson(a, b, active, future())
	s.sql(`UPDATE users SET phone_e164 = '+380501234567', telegram_handle = 'private-e2e', portfolio_url = 'https://example.test/private-contact' WHERE id = $1`, a.id)
	oldSession := s.login(a.email, password)
	s.request(nil, "POST", "/auth/forgot-password", map[string]string{"email": a.email}, 202)
	resetToken := s.mail.token(t, a.email)
	s.secrets = append(s.secrets, resetToken)
	s.request(a, "DELETE", "/me/account", map[string]string{"password": "wrong password!"}, 422).invalid("password")
	s.request(a, "GET", "/auth/me", nil, 200)
	s.request(a, "DELETE", "/me/account", map[string]string{"password": password}, 204)
	for _, session := range []*browser{a, oldSession} {
		s.request(session, "GET", "/auth/me", nil, 401).problem("unauthenticated")
	}
	s.request(nil, "POST", "/auth/login", map[string]string{"email": a.email, "password": password}, 401).problem("invalid_credentials")
	s.request(nil, "POST", "/auth/reset-password", map[string]string{"token": resetToken, "password": password}, 400).problem("invalid_token")
	deleted := s.request(b, "GET", "/exchanges/"+active, nil, 200)
	expect.Equal(t, "deletion cancels active exchange", deleted.str("exchange", "status"), "CANCELLED")
	expect.Equal(t, "deleted author's name", deleted.str("exchange", "partner", "firstName")+" "+deleted.str("exchange", "partner", "lastName"), "Видалений користувач")
	expect.Equal(t, "deletion cancels future lesson", s.request(b, "GET", "/sessions/"+futureLesson, nil, 200).str("session", "status"), "CANCELLED")
	history := s.request(b, "GET", chatPath, nil, 200).items("items")
	expect.Equal(t, "conversation history retained", len(history), 3)
	redacted := 0
	for _, item := range history {
		if item.str("author", "id") == a.id {
			expect.Equal(t, "deleted message redacted", item.str("body"), "Повідомлення видалено")
			redacted++
		} else {
			expect.Equal(t, "partner message retained", item.str("body"), "Історія партнера")
		}
	}
	expect.Equal(t, "all deleted author's messages redacted", redacted, 2)
	expect.Equal(t, "identifying profile data cleared", s.count(`SELECT count(*) FROM users WHERE id = $1 AND email = 'deleted-' || id || '@deleted.invalid' AND city IS NULL AND bio IS NULL AND faculty_id IS NULL AND academic_year IS NULL AND phone_e164 IS NULL AND telegram_handle IS NULL AND portfolio_url IS NULL AND deleted_at IS NOT NULL`, a.id), 1)
	expect.Equal(t, "password identity removed", s.count(`SELECT count(*) FROM auth_identities WHERE user_id = $1`, a.id), 0)
}

func TestE2EPrivateOperationsAndInputValidation(t *testing.T) {
	s := newStack(t)
	a := s.student("security-a", guitar, photoshop)
	b := s.student("security-b", photoshop, guitar)
	_, ex := s.exchange(a, b)
	lesson := s.lesson(a, b, ex, future())
	chat := s.request(a, "GET", "/exchanges/"+ex+"/conversation", nil, 200).str("conversation", "id")
	operations := []struct {
		method, path string
		body         any
	}{
		{"GET", "/me/profile", nil}, {"PATCH", "/me/profile", map[string]string{"firstName": "Без сесії"}},
		{"POST", "/me/teaching-skills", map[string]string{"skillId": photoshop, "level": "BEGINNER"}},
		{"PATCH", "/me/teaching-skills/" + guitar, map[string]string{"level": "BEGINNER"}},
		{"DELETE", "/me/teaching-skills/" + guitar, nil}, {"GET", "/me/matches", nil}, {"GET", "/students", nil},
		{"POST", "/exchange-requests", requestBody(b.id)}, {"GET", "/exchanges/" + ex, nil},
		{"POST", "/sessions/" + lesson + "/cancel", map[string]string{"reason": "Без сесії"}},
		{"POST", "/sessions/" + lesson + "/confirm", nil}, {"POST", "/exchanges/" + ex + "/complete", nil},
		{"POST", "/exchanges/" + ex + "/reviews", map[string]int{"rating": 5}},
		{"GET", "/conversations/" + chat + "/messages", nil},
		{"POST", "/conversations/" + chat + "/messages", map[string]string{"body": "Без сесії"}},
		{"POST", "/me/notifications/read-all", nil}, {"GET", "/admin/reports", nil},
		{"POST", "/students/" + b.id + "/block", nil},
		{"POST", "/reports", map[string]string{"reportedUserId": b.id, "reason": "Без сесії"}},
		{"PATCH", "/me/privacy", map[string]bool{"isDiscoverable": false}},
		{"DELETE", "/me/account", map[string]string{"password": password}},
	}
	for _, operation := range operations {
		s.request(nil, operation.method, operation.path, operation.body, 401).problem("unauthenticated")
		if operation.method != "GET" {
			for _, headers := range []map[string]string{{"X-CSRF-Token": ""}, {"Origin": "http://foreign.example"}} {
				r, err := s.raw(a, operation.method, operation.path, encode(operation.body), headers)
				expect.NoError(t, "protected write", err)
				expect.Equal(t, "write requires CSRF and origin", r.status, 403)
			}
		}
	}
	for _, body := range []string{`{"firstName":`, `{"role":"ADMIN"}`, `{} {}`, strings.Repeat("x", 1<<20)} {
		r, err := s.raw(a, "PATCH", "/me/profile", body, nil)
		expect.NoError(t, "malformed input", err)
		expect.Equal(t, "bad JSON rejected", r.status, 400)
		r.problem("invalid_json")
	}
	r, err := s.raw(a, "PATCH", "/me/profile", "firstName=test", map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	expect.NoError(t, "non-JSON input", err)
	expect.Equal(t, "unsupported type rejected", r.status, 415)
	s.request(a, "GET", "/me/profile", nil, 200)
}
