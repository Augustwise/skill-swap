package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"skillswap/backend/internal/expect"
)

func TestE2EAccountLifecycle(t *testing.T) {
	s := newStack(t)
	for _, pass := range []string{strings.Repeat("я", 11), strings.Repeat("a", 73), strings.Repeat("я", 36) + "a"} {
		s.request(nil, "POST", "/auth/register", map[string]string{
			"email": "invalid@students.example.test", "password": pass, "firstName": "Олена", "lastName": "Коваль",
		}, 422).invalid("password")
	}
	s.request(nil, "POST", "/auth/register", map[string]string{
		"email": "invalid@gmail.com", "password": password, "firstName": "Олена", "lastName": "Коваль",
	}, 422).invalid("email")
	for index, pass := range []string{strings.Repeat("я", 12), strings.Repeat("я", 36)} {
		email := "boundary" + strconv.Itoa(index) + "@students.example.test"
		s.request(nil, "POST", "/auth/register", map[string]string{
			"email": email, "password": pass, "firstName": "Олена", "lastName": "Коваль",
		}, 201)
		s.login(email, pass)
	}

	email := "account@students.example.test"
	registration := map[string]string{"email": " Account@Students.Example.Test ", "password": password, "firstName": "Олена", "lastName": "Коваль"}
	s.request(nil, "POST", "/auth/register", registration, 201)
	s.request(nil, "POST", "/auth/register", registration, 409).problem("email_taken")
	b := s.login(email, password)
	s.request(b, "POST", "/exchange-requests", requestBody(b.id), 403).problem("email_not_verified")
	oldToken := s.mail.token(t, email)
	s.sql(`UPDATE one_time_tokens SET created_at = now() - interval '2 days', expires_at = now() - interval '1 second' WHERE user_id = $1`, b.id)
	s.request(nil, "POST", "/auth/verify-email", map[string]string{"token": oldToken}, 400).problem("invalid_token")
	s.request(b, "POST", "/auth/resend-verification", nil, 202)
	token := s.mail.token(t, email)
	s.secrets = append(s.secrets, oldToken, token)
	s.request(nil, "POST", "/auth/verify-email", map[string]string{"token": oldToken}, 400).problem("invalid_token")
	s.request(nil, "POST", "/auth/verify-email", map[string]string{"token": token}, 200)
	s.request(nil, "POST", "/auth/verify-email", map[string]string{"token": token}, 400).problem("invalid_token")
	expect.Equal(t, "verified account", s.request(b, "GET", "/auth/me", nil, 200).flag("user", "emailVerified"), true)

	expect.Equal(t, "HttpOnly cookie", b.cookie.HttpOnly, true)
	expect.Equal(t, "SameSite cookie", b.cookie.SameSite, http.SameSiteLaxMode)
	expect.Equal(t, "cookie path", b.cookie.Path, "/")
	expect.Equal(t, "local HTTP cookie", b.cookie.Secure, false)
	var hash, otherHash string
	expect.NoError(t, "read password hash", s.pool.QueryRow(t.Context(), `SELECT password_hash FROM auth_identities WHERE user_id = $1`, b.id).Scan(&hash))
	cost, err := bcrypt.Cost([]byte(hash))
	expect.NoError(t, "parse bcrypt cost", err)
	expect.Equal(t, "bcrypt cost", cost, 12)
	expect.NoError(t, "stored password works", bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)))
	other := s.student("salt", "", "")
	expect.NoError(t, "read second password hash", s.pool.QueryRow(t.Context(), `SELECT password_hash FROM auth_identities WHERE user_id = $1`, other.id).Scan(&otherHash))
	expect.Equal(t, "individual password salts", hash != otherHash, true)
	expect.Equal(t, "raw session is not stored", s.count(`SELECT count(*) FROM user_sessions WHERE session_token_hash = $1`, b.cookie.Value), 0)
	digest := sha256.Sum256([]byte(b.cookie.Value))
	expect.Equal(t, "session is stored as SHA-256", s.count(`SELECT count(*) FROM user_sessions WHERE session_token_hash = $1`, hex.EncodeToString(digest[:])), 1)
	expect.Equal(t, "raw link token is not stored", s.count(`SELECT count(*) FROM one_time_tokens WHERE token_hash = $1`, token), 0)
	digest = sha256.Sum256([]byte(token))
	expect.Equal(t, "one-time link is stored as SHA-256", s.count(`SELECT count(*) FROM one_time_tokens WHERE token_hash = $1`, hex.EncodeToString(digest[:])), 1)

	for _, headers := range []map[string]string{{"X-CSRF-Token": ""}, {"X-CSRF-Token": "wrong"}, {"Origin": ""}, {"Origin": "https://foreign.example"}} {
		r, err := s.raw(b, "PATCH", "/me/profile", `{"firstName":"Змінено"}`, headers)
		expect.NoError(t, "rejected write", err)
		expect.Equal(t, "forbidden write", r.status, 403)
	}
	expect.Equal(t, "rejected writes did not change profile", s.request(b, "GET", "/me/profile", nil, 200).str("profile", "firstName"), "Олена")
	for range 5 {
		s.request(nil, "POST", "/auth/login", map[string]string{"email": strings.ToUpper(email), "password": "incorrect passphrase"}, 401).problem("invalid_credentials")
	}
	locked := s.request(nil, "POST", "/auth/login", map[string]string{"email": email, "password": password}, 429)
	locked.problem("login_locked")
	retry, err := strconv.Atoi(locked.header.Get("Retry-After"))
	expect.NoError(t, "parse retry duration", err)
	expect.Equal(t, "40 minute lock", retry >= 2390 && retry <= 2401, true)
	s.sql(`UPDATE login_throttles SET locked_until = now() - interval '1 second', window_started_at = now() - interval '41 minutes' WHERE email = $1`, email)
	second := s.login(email, password)

	s.request(nil, "POST", "/auth/forgot-password", map[string]string{"email": email}, 202)
	reset := s.mail.token(t, email)
	s.secrets = append(s.secrets, reset)
	s.sql(`UPDATE one_time_tokens SET created_at = now() - interval '2 days', expires_at = now() - interval '1 second' WHERE user_id = $1 AND used_at IS NULL`, b.id)
	s.request(nil, "POST", "/auth/reset-password", map[string]string{"token": reset, "password": password}, 400).problem("invalid_token")
	s.request(nil, "POST", "/auth/forgot-password", map[string]string{"email": email}, 202)
	reset = s.mail.token(t, email)
	s.secrets = append(s.secrets, reset)
	newPassword := "Reset password 2026!"
	s.request(nil, "POST", "/auth/reset-password", map[string]string{"token": reset, "password": newPassword}, 200)
	s.request(nil, "POST", "/auth/reset-password", map[string]string{"token": reset, "password": newPassword}, 400).problem("invalid_token")
	for _, old := range []*browser{b, second} {
		s.request(old, "GET", "/auth/me", nil, 401).problem("unauthenticated")
	}
	s.request(nil, "POST", "/auth/login", map[string]string{"email": email, "password": password}, 401).problem("invalid_credentials")
	b = s.login(email, newPassword)
	second = s.login(email, newPassword)
	changed := "Changed password 2026!"
	s.request(b, "POST", "/auth/change-password", map[string]string{"currentPassword": newPassword, "newPassword": changed}, 200)
	s.secrets = append(s.secrets, changed)
	s.request(b, "GET", "/auth/me", nil, 200)
	s.request(second, "GET", "/auth/me", nil, 401).problem("unauthenticated")
	s.request(b, "POST", "/auth/logout", nil, 204)
	s.request(b, "GET", "/auth/me", nil, 401).problem("unauthenticated")
}

func TestE2EProfilesAndDiscovery(t *testing.T) {
	s := newStack(t)
	a := s.student("guitarist", guitar, photoshop)
	b := s.student("designer", photoshop, guitar)
	oneWay := s.student("oneway", photoshop, "")
	patch := map[string]any{
		"firstName": "Марія", "lastName": "Наскрізна", "universityId": university,
		"facultyId": "50000000-0000-0000-0000-000000000001", "course": 3,
		"city": "Київ", "bio": strings.Repeat("я", 600), "formats": []string{"ONLINE", "OFFLINE"},
	}
	s.request(a, "PATCH", "/me/profile", patch, 200)
	s.request(a, "PATCH", "/me/profile", map[string]string{"bio": strings.Repeat("я", 601)}, 422).invalid("bio")
	s.request(a, "PATCH", "/me/profile", map[string]string{"city": ""}, 422).invalid("city")
	s.request(a, "PATCH", "/me/profile", map[string]string{"firstName": ""}, 422).invalid("firstName")
	s.request(a, "PATCH", "/me/profile", map[string]string{"universityId": "invalid"}, 422).invalid("universityId")
	s.request(a, "POST", "/auth/logout", nil, 204)
	a = s.login(a.email, password)
	p := s.request(a, "GET", "/me/profile", nil, 200)
	expect.Equal(t, "profile persisted", p.str("profile", "firstName"), "Марія")
	expect.Equal(t, "bio persisted", p.str("profile", "bio"), strings.Repeat("я", 600))
	expect.Equal(t, "course persisted", p.num("profile", "course"), 3)
	expect.Equal(t, "search by Ukrainian name", s.contains(b, "/students?q="+url.QueryEscape("Наскрізна")+"&", a.id), true)
	s.request(a, "PATCH", "/me/profile", map[string]any{"formats": []string{}}, 200)
	expect.Equal(t, "no format disables matching", s.request(a, "GET", "/me/matches", nil, 200).flag("eligibleForMatching"), false)
	s.request(a, "PATCH", "/me/profile", map[string]any{"formats": []string{"ONLINE", "OFFLINE"}}, 200)
	for _, list := range []string{"teaching", "learning"} {
		skill := guitar
		if list == "learning" {
			skill = photoshop
		}
		path := "/me/" + list + "-skills"
		s.request(a, "POST", path, map[string]string{"skillId": skill, "level": "BEGINNER"}, 409).problem("skill_already_added")
		s.request(a, "PATCH", path+"/"+skill, map[string]string{"level": "INTERMEDIATE"}, 200)
		s.request(a, "DELETE", path+"/"+skill, nil, 200)
		expect.Equal(t, "missing list disables matching", s.request(a, "GET", "/me/matches", nil, 200).flag("eligibleForMatching"), false)
		s.request(a, "POST", path, map[string]string{"skillId": skill, "level": "ADVANCED"}, 201)
	}
	expect.Equal(t, "mutual student found", s.contains(a, "/me/matches?", b.id), true)
	expect.Equal(t, "one-sided student excluded", s.contains(a, "/me/matches?", oneWay.id), false)
	s.request(b, "DELETE", "/me/learning-skills/"+guitar, nil, 200)
	expect.Equal(t, "next read uses changed skills", s.contains(a, "/me/matches?", b.id), false)
	s.request(b, "POST", "/me/learning-skills", map[string]string{"skillId": guitar, "level": "BEGINNER"}, 201)
	query := "/students?q=Photoshop&categoryId=20000000-0000-0000-0000-000000000002&level=ADVANCED&format=ONLINE&mutual=true&"
	expect.Equal(t, "combined filters find partner", s.contains(a, query, b.id), true)
	for _, item := range s.request(a, "GET", query+"page=1", nil, 200).items("items") {
		expect.Equal(t, "every card is mutual", item.flag("mutual"), true)
		found := false
		for _, skill := range item.items("teachingSkills") {
			if skill.str("skillId") == photoshop {
				found = true
			}
		}
		expect.Equal(t, "every result teaches Photoshop", found, true)
	}
	empty := s.request(a, "GET", "/students?q=NoSuchSkill938291", nil, 200)
	expect.Equal(t, "empty search count", empty.num("total"), 0)
	expect.Equal(t, "empty search array", len(empty.items("items")), 0)
	malicious := "'; DROP TABLE users; --"
	s.request(a, "GET", "/students?q="+url.QueryEscape(malicious), nil, 200)
	s.request(a, "GET", "/me/profile", nil, 200)
	s.request(b, "PATCH", "/students/"+a.id, patch, 405).problem("method_not_allowed")
	public := s.request(b, "GET", "/students/"+a.id, nil, 200)
	for _, secret := range []string{"email", "password", "passwordHash", "csrfToken", "messages", "meetingPlace"} {
		expect.Equal(t, "public profile excludes "+secret, strings.Contains(encode(public.body), `"`+secret+`"`), false)
	}
	s.command(suite.db, nil, "seed-load")
	first := s.request(a, "GET", "/students?page=1", nil, 200)
	second := s.request(a, "GET", "/students?page=2", nil, 200)
	expect.Equal(t, "page size", first.num("pageSize"), 20)
	expect.Equal(t, "first page length", len(first.items("items")), 20)
	expect.Equal(t, "second page length", len(second.items("items")), 20)
	seen := map[string]bool{}
	for _, item := range first.items("items") {
		seen[item.str("student", "id")] = true
	}
	for _, item := range second.items("items") {
		expect.Equal(t, "pages do not overlap", seen[item.str("student", "id")], false)
	}
}

func (s *stack) contains(b *browser, path, id string) bool {
	s.t.Helper()
	for page := 1; ; page++ {
		r := s.request(b, "GET", path+"page="+strconv.Itoa(page), nil, 200)
		for _, item := range r.items("items") {
			if item.str("student", "id") == id {
				return true
			}
		}
		if float64(page)*r.num("pageSize") >= r.num("total") {
			return false
		}
	}
}

func future() time.Time { return time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute) }
