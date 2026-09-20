package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"skillswap/backend/docs"
)

type API struct {
	store          catalogStore
	frontendOrigin string
	logger         *slog.Logger
}

func New(db *pgxpool.Pool, frontendOrigin string, logger *slog.Logger) http.Handler {
	return newHandler(newPostgresCatalogStore(db), frontendOrigin, logger)
}

func newHandler(store catalogStore, frontendOrigin string, logger *slog.Logger) http.Handler {
	a := &API{store: store, frontendOrigin: frontendOrigin, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", getOnly(a.health))
	mux.HandleFunc("/api/v1/ready", getOnly(a.ready))
	mux.HandleFunc("/api/v1/universities", getOnly(a.universities))
	mux.HandleFunc("/api/v1/skill-categories", getOnly(a.categories))
	mux.HandleFunc("/api/v1/skills", getOnly(a.skills))
	mux.HandleFunc("/api/v1/openapi.yaml", getOnly(openAPI))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		problem(w, http.StatusNotFound, "not_found", "Resource was not found")
	})
	return a.withMiddleware(mux)
}

func openAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(docs.OpenAPI)
}

func getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			problem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method is not allowed")
			return
		}
		next(w, r)
	}
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.store.Ready(ctx); err != nil {
		problem(w, http.StatusServiceUnavailable, "database_unavailable", "Database is unavailable")
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": "ready"})
}

type university struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ShortName   string `json:"shortName"`
	EmailDomain string `json:"emailDomain"`
	City        string `json:"city"`
}

func (a *API) universities(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	items, err := a.store.Universities(ctx)
	if err != nil {
		a.queryError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"items": items})
}

type skillCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (a *API) categories(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	items, err := a.store.SkillCategories(ctx)
	if err != nil {
		a.queryError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"items": items})
}

type skill struct {
	ID         string `json:"id"`
	CategoryID string `json:"categoryId"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
}

func (a *API) skills(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) > 150 {
		problem(w, http.StatusBadRequest, "invalid_query", "q must be 150 characters or fewer")
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			problem(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	items, err := a.store.Skills(ctx, query, limit)
	if err != nil {
		a.queryError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) queryError(w http.ResponseWriter, err error) {
	if !errors.Is(err, context.Canceled) {
		a.logger.Error("database query failed", "error", err)
	}
	problem(w, http.StatusServiceUnavailable, "database_unavailable", "Database is unavailable")
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func problem(w http.ResponseWriter, status int, code, message string) {
	respond(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
