package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"skillswap/backend/internal/auth"
	"skillswap/backend/internal/data"
)

const (
	sessionCookieName = "skillswap_session"
	csrfHeaderName    = "X-CSRF-Token"
	maxBodyBytes      = 16 << 10
	maxUserAgent      = 512
	authTimeout       = 5 * time.Second
)

func (a *API) postOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			problem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method is not allowed")
			return
		}
		if r.Header.Get("Origin") != a.settings.FrontendOrigin {
			problem(w, http.StatusForbidden, "origin_forbidden", "Origin is not allowed")
			return
		}
		next(w, r)
	}
}

type userHandler func(w http.ResponseWriter, r *http.Request, current data.User, sessionToken string)

func (a *API) withUser(next userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			token = cookie.Value
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		current, err := a.access.Authenticate(ctx, token)
		if err != nil {
			if errors.Is(err, auth.ErrUnauthenticated) && token != "" {
				clearSessionCookie(w)
			}
			a.authError(w, err)
			return
		}
		if r.Method != http.MethodGet && !auth.ValidCSRF(token, r.Header.Get(csrfHeaderName)) {
			problem(w, http.StatusForbidden, "csrf_invalid", "CSRF token is missing or invalid")
			return
		}
		next(w, r, current, token)
	}
}

type registerRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var body registerRequest
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), authTimeout)
	defer cancel()
	created, sent, err := a.access.Register(ctx, auth.RegisterInput{
		Email: body.Email, Password: body.Password, FirstName: body.FirstName, LastName: body.LastName,
	})
	if err != nil {
		a.authError(w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]any{"user": toUser(created), "verificationEmailSent": sent})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if !decodeBody(w, r, &body) {
		return
	}
	previous := ""
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		previous = cookie.Value
	}
	ctx, cancel := context.WithTimeout(r.Context(), authTimeout)
	defer cancel()
	session, err := a.access.Login(ctx, auth.LoginInput{
		Email:           body.Email,
		Password:        body.Password,
		UserAgent:       userAgent(r),
		IP:              clientIP(r),
		PreviousSession: previous,
	})
	if err != nil {
		a.authError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.Token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		MaxAge:   int(auth.SessionTTL.Seconds()),
		HttpOnly: true,

		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})
	respond(w, http.StatusOK, map[string]any{"user": toUser(session.User), "csrfToken": session.CSRFToken})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request, _ data.User, sessionToken string) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := a.access.Logout(ctx, sessionToken); err != nil {
		a.authError(w, err)
		return
	}
	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) me(w http.ResponseWriter, _ *http.Request, current data.User, sessionToken string) {
	respond(w, http.StatusOK, map[string]any{"user": toUser(current), "csrfToken": auth.CSRFToken(sessionToken)})
}

type tokenRequest struct {
	Token string `json:"token"`
}

func (a *API) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var body tokenRequest
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := a.access.VerifyEmail(ctx, body.Token); err != nil {
		a.authError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "verified"})
}

func (a *API) resendVerification(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := a.access.ResendVerification(ctx, current); err != nil {
		a.authError(w, err)
		return
	}
	respond(w, http.StatusAccepted, map[string]string{"status": "sent"})
}

type emailRequest struct {
	Email string `json:"email"`
}

func (a *API) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var body emailRequest
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := a.access.ForgotPassword(ctx, body.Email); err != nil {
		a.authError(w, err)
		return
	}
	respond(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (a *API) resetPassword(w http.ResponseWriter, r *http.Request) {
	var body resetPasswordRequest
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), authTimeout)
	defer cancel()
	if err := a.access.ResetPassword(ctx, body.Token, body.Password); err != nil {
		a.authError(w, err)
		return
	}
	clearSessionCookie(w)
	respond(w, http.StatusOK, map[string]string{"status": "password_reset"})
}

func (a *API) authError(w http.ResponseWriter, err error) {
	var invalid *auth.ValidationError
	var locked *auth.LockedError
	switch {
	case errors.As(err, &invalid):
		validationProblem(w, invalid.Fields)
	case errors.As(err, &locked):
		w.Header().Set("Retry-After", strconv.Itoa(int(time.Until(locked.Until).Seconds())+1))
		problem(w, http.StatusTooManyRequests, "login_locked", "Too many failed attempts. Try again later")
	case errors.Is(err, auth.ErrEmailTaken):
		problem(w, http.StatusConflict, "email_taken", "An account with this email already exists")
	case errors.Is(err, auth.ErrInvalidCredentials):
		problem(w, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect")
	case errors.Is(err, auth.ErrAccountSuspended):
		problem(w, http.StatusForbidden, "account_suspended", "This account is suspended")
	case errors.Is(err, auth.ErrUnauthenticated):
		problem(w, http.StatusUnauthorized, "unauthenticated", "Sign in to continue")
	case errors.Is(err, auth.ErrInvalidToken):
		problem(w, http.StatusBadRequest, "invalid_token", "The link is invalid, already used, or expired")
	case errors.Is(err, auth.ErrAlreadyVerified):
		problem(w, http.StatusConflict, "email_already_verified", "Email is already verified")
	case errors.Is(err, auth.ErrMailUnavailable):
		problem(w, http.StatusServiceUnavailable, "mail_unavailable", "Email could not be sent. Try again later")
	default:
		a.queryError(w, err)
	}
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func decodeBody(w http.ResponseWriter, r *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		problem(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		problem(w, http.StatusBadRequest, "invalid_json", "Request body must be a valid JSON object")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		problem(w, http.StatusBadRequest, "invalid_json", "Request body must contain a single JSON object")
		return false
	}
	return true
}

func userAgent(r *http.Request) string {
	agent := r.UserAgent()
	if len(agent) > maxUserAgent {
		agent = agent[:maxUserAgent]
	}
	return strings.ToValidUTF8(agent, "")
}

func clientIP(r *http.Request) netip.Addr {
	addrPort, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	return addrPort.Addr()
}
