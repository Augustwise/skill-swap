package api

import (
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"
)

func (a *API) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Add("Vary", "Origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if origin := r.Header.Get("Origin"); origin != "" {
			if origin != a.settings.FrontendOrigin {
				problem(w, http.StatusForbidden, "origin_forbidden", "Origin is not allowed")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
		}
		if r.Method == http.MethodOptions {
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		wrapped := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(wrapped, r)
		a.logger.Log(r.Context(), slog.LevelInfo, "http request", "method", r.Method, "path", r.URL.Path, "status", wrapped.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (a *API) methods(handlers map[string]http.HandlerFunc) http.HandlerFunc {
	allow := strings.Join(slices.Sorted(maps.Keys(handlers)), ", ")
	return func(w http.ResponseWriter, r *http.Request) {
		next, ok := handlers[r.Method]
		if !ok {
			w.Header().Set("Allow", allow)
			problem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method is not allowed")
			return
		}
		if r.Method != http.MethodGet && r.Header.Get("Origin") != a.settings.FrontendOrigin {
			problem(w, http.StatusForbidden, "origin_forbidden", "Origin is not allowed")
			return
		}
		next(w, r)
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}
