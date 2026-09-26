package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"skillswap/backend/internal/auth"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/mailer/mailertest"
)

const testPassword = "correct horse battery"

var linkToken = regexp.MustCompile(`token=([A-Za-z0-9_-]{43})`)

type session struct {
	cookie *http.Cookie
	csrf   string
}

func newAuthHandler() (http.Handler, *datatest.Memory, *mailertest.Recorder) {
	store := datatest.NewMemory()
	mail := &mailertest.Recorder{}
	access := auth.NewService(store, store, mail, testAuthConfig, discardLogger())
	return New(&fakeApp{}, access, &fakeApp{}, testSettings, discardLogger()), store, mail
}

func send(t *testing.T, handler http.Handler, method, path, body string, s *session) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Origin", "http://localhost:3000")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if s != nil {
		if s.cookie != nil {
			req.AddCookie(s.cookie)
		}
		if s.csrf != "" {
			req.Header.Set(csrfHeaderName, s.csrf)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func registerBody(email, password string) string {
	return fmt.Sprintf(`{"email":%q,"password":%q,"firstName":"Олена","lastName":"Коваль"}`, email, password)
}

func loginBody(email, password string) string {
	return fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
}

func tokenFromMail(t *testing.T, mail *mailertest.Recorder) string {
	t.Helper()
	match := linkToken.FindStringSubmatch(mail.Last().TextBody)
	if match == nil {
		t.Fatalf("no token link in email %q", mail.Last().TextBody)
	}
	return match[1]
}

func login(t *testing.T, handler http.Handler, email, password string) *session {
	t.Helper()
	response := send(t, handler, http.MethodPost, "/api/v1/auth/login", loginBody(email, password), nil)
	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		User      user   `json:"user"`
		CSRFToken string `json:"csrfToken"`
	}
	decodeJSON(t, response, &body)
	cookie := sessionCookie(response)
	if cookie == nil || body.CSRFToken == "" {
		t.Fatalf("login did not return a session: %s", response.Body.String())
	}
	return &session{cookie: cookie, csrf: body.CSRFToken}
}

func sessionCookie(response *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			return cookie
		}
	}
	return nil
}

func TestRegisterSendsVerificationEmail(t *testing.T) {
	handler, _, mail := newAuthHandler()
	response := send(t, handler, http.MethodPost, "/api/v1/auth/register",
		registerBody("  Olena.Koval@Students.Example.Test ", testPassword), nil)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		User                  user `json:"user"`
		VerificationEmailSent bool `json:"verificationEmailSent"`
	}
	decodeJSON(t, response, &body)
	if body.User.Email != "olena.koval@students.example.test" || body.User.EmailVerified || !body.VerificationEmailSent {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "password") {
		t.Fatal("response must not contain password data")
	}
	if mail.Count() != 1 || mail.Last().To[0] != "olena.koval@students.example.test" {
		t.Fatalf("unexpected mail: %+v", mail.Last())
	}
	if !strings.Contains(mail.Last().TextBody, "http://localhost:3000/onboarding?token=") {
		t.Fatalf("email has no verification link: %q", mail.Last().TextBody)
	}

	response = send(t, handler, http.MethodPost, "/api/v1/auth/register",
		registerBody("OLENA.KOVAL@students.example.test", testPassword), nil)
	assertProblem(t, response, http.StatusConflict, "email_taken")
}

func TestRegisterStillSucceedsWhenMailIsDown(t *testing.T) {
	handler, _, mail := newAuthHandler()
	mail.Err = errors.New("smtp down")
	response := send(t, handler, http.MethodPost, "/api/v1/auth/register",
		registerBody("student@students.example.test", testPassword), nil)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"verificationEmailSent":false`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestRegisterValidation(t *testing.T) {
	handler, _, _ := newAuthHandler()
	tests := []struct {
		name  string
		body  string
		field string
	}{
		{"invalid email", registerBody("not-an-email", testPassword), "email"},
		{"foreign domain", registerBody("student@gmail.com", testPassword), "email"},
		{"short password", registerBody("a@students.example.test", "short"), "password"},
		{"73-byte password", registerBody("a@students.example.test", strings.Repeat("a", 73)), "password"},
		{"missing names", `{"email":"a@students.example.test","password":"correct horse battery","firstName":" ","lastName":""}`, "firstName"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := send(t, handler, http.MethodPost, "/api/v1/auth/register", test.body, nil)
			assertProblem(t, response, http.StatusUnprocessableEntity, "validation_failed")
			var body struct {
				Error struct {
					Fields map[string]string `json:"fields"`
				} `json:"error"`
			}
			decodeJSON(t, response, &body)
			if body.Error.Fields[test.field] == "" {
				t.Fatalf("missing %s field error: %s", test.field, response.Body.String())
			}
		})
	}
}

func TestAuthRequestFormat(t *testing.T) {
	handler, _, _ := newAuthHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginBody("a@students.example.test", testPassword)))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	assertProblem(t, response, http.StatusForbidden, "origin_forbidden")

	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("email=a"))
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	assertProblem(t, response, http.StatusUnsupportedMediaType, "unsupported_media_type")

	response = send(t, handler, http.MethodPost, "/api/v1/auth/login", `{"email":"a","role":"ADMIN"}`, nil)
	assertProblem(t, response, http.StatusBadRequest, "invalid_json")

	response = send(t, handler, http.MethodGet, "/api/v1/auth/login", "", nil)
	assertProblem(t, response, http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestVerifyEmailLinkWorksOnce(t *testing.T) {
	handler, _, mail := newAuthHandler()
	send(t, handler, http.MethodPost, "/api/v1/auth/register", registerBody("a@students.example.test", testPassword), nil)
	token := tokenFromMail(t, mail)

	response := send(t, handler, http.MethodPost, "/api/v1/auth/verify-email", fmt.Sprintf(`{"token":%q}`, token), nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	response = send(t, handler, http.MethodPost, "/api/v1/auth/verify-email", fmt.Sprintf(`{"token":%q}`, token), nil)
	assertProblem(t, response, http.StatusBadRequest, "invalid_token")

	response = send(t, handler, http.MethodPost, "/api/v1/auth/verify-email", `{"token":"garbage"}`, nil)
	assertProblem(t, response, http.StatusBadRequest, "invalid_token")

	s := login(t, handler, "a@students.example.test", testPassword)
	response = send(t, handler, http.MethodGet, "/api/v1/auth/me", "", s)
	if !strings.Contains(response.Body.String(), `"emailVerified":true`) {
		t.Fatalf("user is not verified: %s", response.Body.String())
	}
}

func TestExpiredVerificationLinkFails(t *testing.T) {
	handler, store, mail := newAuthHandler()
	send(t, handler, http.MethodPost, "/api/v1/auth/register", registerBody("a@students.example.test", testPassword), nil)
	store.ExpireTokens()
	response := send(t, handler, http.MethodPost, "/api/v1/auth/verify-email", fmt.Sprintf(`{"token":%q}`, tokenFromMail(t, mail)), nil)
	assertProblem(t, response, http.StatusBadRequest, "invalid_token")
}

func TestLoginSessionAndLogout(t *testing.T) {
	handler, _, _ := newAuthHandler()
	send(t, handler, http.MethodPost, "/api/v1/auth/register", registerBody("a@students.example.test", testPassword), nil)

	response := send(t, handler, http.MethodPost, "/api/v1/auth/login", loginBody("a@students.example.test", "wrong password!!"), nil)
	assertProblem(t, response, http.StatusUnauthorized, "invalid_credentials")
	response = send(t, handler, http.MethodPost, "/api/v1/auth/login", loginBody("nobody@students.example.test", testPassword), nil)
	assertProblem(t, response, http.StatusUnauthorized, "invalid_credentials")

	response = send(t, handler, http.MethodPost, "/api/v1/auth/login", loginBody(" A@Students.Example.Test", testPassword), nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	cookie := sessionCookie(response)
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Secure {
		t.Fatalf("unexpected cookie: %+v", cookie)
	}
	s := login(t, handler, "a@students.example.test", testPassword)

	response = send(t, handler, http.MethodGet, "/api/v1/auth/me", "", s)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"email":"a@students.example.test"`) {
		t.Fatalf("me status = %d, body = %s", response.Code, response.Body.String())
	}
	response = send(t, handler, http.MethodGet, "/api/v1/auth/me", "", nil)
	assertProblem(t, response, http.StatusUnauthorized, "unauthenticated")

	response = send(t, handler, http.MethodPost, "/api/v1/auth/logout", "", &session{cookie: s.cookie})
	assertProblem(t, response, http.StatusForbidden, "csrf_invalid")
	response = send(t, handler, http.MethodPost, "/api/v1/auth/logout", "", &session{cookie: s.cookie, csrf: "wrong"})
	assertProblem(t, response, http.StatusForbidden, "csrf_invalid")

	response = send(t, handler, http.MethodPost, "/api/v1/auth/logout", "", s)
	if response.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, body = %s", response.Code, response.Body.String())
	}
	if cleared := sessionCookie(response); cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("logout did not clear cookie: %+v", cleared)
	}
	response = send(t, handler, http.MethodGet, "/api/v1/auth/me", "", s)
	assertProblem(t, response, http.StatusUnauthorized, "unauthenticated")
}

func TestLoginRememberMeControlsCookiePersistence(t *testing.T) {
	handler, _, _ := newAuthHandler()
	send(t, handler, http.MethodPost, "/api/v1/auth/register", registerBody("a@students.example.test", testPassword), nil)

	for _, test := range []struct {
		name       string
		rememberMe bool
	}{
		{"browser session", false},
		{"persistent", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"email":"a@students.example.test","password":%q,"rememberMe":%t}`, testPassword, test.rememberMe)
			response := send(t, handler, http.MethodPost, "/api/v1/auth/login", body, nil)
			if response.Code != http.StatusOK {
				t.Fatalf("login status = %d, body = %s", response.Code, response.Body.String())
			}
			cookie := sessionCookie(response)
			if cookie == nil {
				t.Fatal("session cookie missing")
			}
			if test.rememberMe {
				if cookie.MaxAge != int(auth.SessionTTL.Seconds()) || cookie.Expires.IsZero() {
					t.Fatalf("expected persistent cookie, got %+v", cookie)
				}
			} else if cookie.MaxAge != 0 || !cookie.Expires.IsZero() {
				t.Fatalf("expected browser-session cookie, got %+v", cookie)
			}
		})
	}
}

func TestLoginLockoutAfterFiveFailures(t *testing.T) {
	handler, _, _ := newAuthHandler()
	send(t, handler, http.MethodPost, "/api/v1/auth/register", registerBody("a@students.example.test", testPassword), nil)
	for attempt := 1; attempt <= 5; attempt++ {
		response := send(t, handler, http.MethodPost, "/api/v1/auth/login", loginBody("a@students.example.test", "wrong password!!"), nil)
		assertProblem(t, response, http.StatusUnauthorized, "invalid_credentials")
	}
	response := send(t, handler, http.MethodPost, "/api/v1/auth/login", loginBody("a@students.example.test", testPassword), nil)
	assertProblem(t, response, http.StatusTooManyRequests, "login_locked")
	retryAfter, err := strconv.Atoi(response.Header().Get("Retry-After"))

	if err != nil || retryAfter < 2*60*60-60 || retryAfter > 2*60*60+1 {
		t.Fatalf("Retry-After = %q", response.Header().Get("Retry-After"))
	}
}

func TestResendVerification(t *testing.T) {
	handler, _, mail := newAuthHandler()
	send(t, handler, http.MethodPost, "/api/v1/auth/register", registerBody("a@students.example.test", testPassword), nil)
	firstToken := tokenFromMail(t, mail)
	s := login(t, handler, "a@students.example.test", testPassword)

	response := send(t, handler, http.MethodPost, "/api/v1/auth/resend-verification", "", s)
	if response.Code != http.StatusAccepted || mail.Count() != 2 {
		t.Fatalf("status = %d, mails = %d", response.Code, mail.Count())
	}
	response = send(t, handler, http.MethodPost, "/api/v1/auth/verify-email", fmt.Sprintf(`{"token":%q}`, firstToken), nil)
	assertProblem(t, response, http.StatusBadRequest, "invalid_token")

	response = send(t, handler, http.MethodPost, "/api/v1/auth/verify-email", fmt.Sprintf(`{"token":%q}`, tokenFromMail(t, mail)), nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	response = send(t, handler, http.MethodPost, "/api/v1/auth/resend-verification", "", s)
	assertProblem(t, response, http.StatusConflict, "email_already_verified")
}

func TestPasswordReset(t *testing.T) {
	handler, _, mail := newAuthHandler()
	send(t, handler, http.MethodPost, "/api/v1/auth/register", registerBody("a@students.example.test", testPassword), nil)
	oldSession := login(t, handler, "a@students.example.test", testPassword)

	response := send(t, handler, http.MethodPost, "/api/v1/auth/forgot-password", `{"email":"nobody@students.example.test"}`, nil)
	if response.Code != http.StatusAccepted || mail.Count() != 1 {
		t.Fatalf("unknown email: status = %d, mails = %d", response.Code, mail.Count())
	}
	response = send(t, handler, http.MethodPost, "/api/v1/auth/forgot-password", `{"email":"A@students.example.test"}`, nil)
	if response.Code != http.StatusAccepted || mail.Count() != 2 {
		t.Fatalf("known email: status = %d, mails = %d", response.Code, mail.Count())
	}
	if !strings.Contains(mail.Last().TextBody, "http://localhost:3000/reset-password?token=") {
		t.Fatalf("email has no reset link: %q", mail.Last().TextBody)
	}
	token := tokenFromMail(t, mail)

	response = send(t, handler, http.MethodPost, "/api/v1/auth/reset-password", fmt.Sprintf(`{"token":%q,"password":"short"}`, token), nil)
	assertProblem(t, response, http.StatusUnprocessableEntity, "validation_failed")

	const newPassword = "a brand new passphrase"
	body := fmt.Sprintf(`{"token":%q,"password":%q}`, token, newPassword)
	response = send(t, handler, http.MethodPost, "/api/v1/auth/reset-password", body, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	response = send(t, handler, http.MethodPost, "/api/v1/auth/reset-password", body, nil)
	assertProblem(t, response, http.StatusBadRequest, "invalid_token")

	response = send(t, handler, http.MethodGet, "/api/v1/auth/me", "", oldSession)
	assertProblem(t, response, http.StatusUnauthorized, "unauthenticated")
	response = send(t, handler, http.MethodPost, "/api/v1/auth/login", loginBody("a@students.example.test", testPassword), nil)
	assertProblem(t, response, http.StatusUnauthorized, "invalid_credentials")
	login(t, handler, "a@students.example.test", newPassword)
}

func TestAuthAgainstLocalPostgres(t *testing.T) {
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
	email := fmt.Sprintf("it-%d@students.example.test", time.Now().UnixNano())

	response := send(t, handler, http.MethodPost, "/api/v1/auth/register", registerBody(email, testPassword), nil)
	if response.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", response.Code, response.Body.String())
	}
	response = send(t, handler, http.MethodPost, "/api/v1/auth/register", registerBody(strings.ToUpper(email), testPassword), nil)
	assertProblem(t, response, http.StatusConflict, "email_taken")

	verifyBody := fmt.Sprintf(`{"token":%q}`, tokenFromMail(t, mail))
	if response = send(t, handler, http.MethodPost, "/api/v1/auth/verify-email", verifyBody, nil); response.Code != http.StatusOK {
		t.Fatalf("verify status = %d, body = %s", response.Code, response.Body.String())
	}
	response = send(t, handler, http.MethodPost, "/api/v1/auth/verify-email", verifyBody, nil)
	assertProblem(t, response, http.StatusBadRequest, "invalid_token")

	s := login(t, handler, email, testPassword)
	response = send(t, handler, http.MethodGet, "/api/v1/auth/me", "", s)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"emailVerified":true`) {
		t.Fatalf("me status = %d, body = %s", response.Code, response.Body.String())
	}

	send(t, handler, http.MethodPost, "/api/v1/auth/forgot-password", fmt.Sprintf(`{"email":%q}`, email), nil)
	const newPassword = "a brand new passphrase"
	resetBody := fmt.Sprintf(`{"token":%q,"password":%q}`, tokenFromMail(t, mail), newPassword)
	if response = send(t, handler, http.MethodPost, "/api/v1/auth/reset-password", resetBody, nil); response.Code != http.StatusOK {
		t.Fatalf("reset status = %d, body = %s", response.Code, response.Body.String())
	}
	response = send(t, handler, http.MethodGet, "/api/v1/auth/me", "", s)
	assertProblem(t, response, http.StatusUnauthorized, "unauthenticated")

	for attempt := 1; attempt <= 5; attempt++ {
		response = send(t, handler, http.MethodPost, "/api/v1/auth/login", loginBody(email, "wrong password!!"), nil)
		assertProblem(t, response, http.StatusUnauthorized, "invalid_credentials")
	}
	response = send(t, handler, http.MethodPost, "/api/v1/auth/login", loginBody(email, newPassword), nil)
	assertProblem(t, response, http.StatusTooManyRequests, "login_locked")
	if _, err := pool.Exec(context.Background(), `DELETE FROM login_throttles WHERE email = $1`, email); err != nil {
		t.Fatal(err)
	}

	s = login(t, handler, email, newPassword)
	if response = send(t, handler, http.MethodPost, "/api/v1/auth/logout", "", s); response.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, body = %s", response.Code, response.Body.String())
	}
	response = send(t, handler, http.MethodGet, "/api/v1/auth/me", "", s)
	assertProblem(t, response, http.StatusUnauthorized, "unauthenticated")
}
