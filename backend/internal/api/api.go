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
)

type API struct {
	db             *pgxpool.Pool
	frontendOrigin string
	logger         *slog.Logger
}

func New(db *pgxpool.Pool, frontendOrigin string, logger *slog.Logger) http.Handler {
	a := &API{db: db, frontendOrigin: frontendOrigin, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", a.health)
	mux.HandleFunc("GET /api/v1/ready", a.ready)
	mux.HandleFunc("GET /api/v1/universities", a.universities)
	mux.HandleFunc("GET /api/v1/skill-categories", a.categories)
	mux.HandleFunc("GET /api/v1/skills", a.skills)
	return a.withMiddleware(mux)
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	var migrated bool
	if err := a.db.QueryRow(ctx, `SELECT to_regclass('public.universities') IS NOT NULL`).Scan(&migrated); err != nil || !migrated {
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
	rows, err := a.db.Query(ctx, `SELECT id, name, COALESCE(short_name, ''), email_domain, COALESCE(city, '')
		FROM universities ORDER BY name, id`)
	if err != nil {
		a.queryError(w, err)
		return
	}
	defer rows.Close()
	result := make([]university, 0)
	for rows.Next() {
		var item university
		if err := rows.Scan(&item.ID, &item.Name, &item.ShortName, &item.EmailDomain, &item.City); err != nil {
			a.queryError(w, err)
			return
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		a.queryError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"items": result})
}

type category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (a *API) categories(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	rows, err := a.db.Query(ctx, `SELECT id, name, slug FROM skill_categories
		WHERE is_active = true ORDER BY sort_order, name, id`)
	if err != nil {
		a.queryError(w, err)
		return
	}
	defer rows.Close()
	result := make([]category, 0)
	for rows.Next() {
		var item category
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug); err != nil {
			a.queryError(w, err)
			return
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		a.queryError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"items": result})
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
	rows, err := a.db.Query(ctx, `SELECT s.id, s.category_id, s.name, s.slug FROM skills s
		JOIN skill_categories c ON c.id = s.category_id
		WHERE s.is_active = true AND c.is_active = true
		AND ($1 = '' OR strpos(lower(s.name), lower($1)) > 0 OR strpos(lower(s.slug), lower($1)) > 0)
		ORDER BY s.name, s.id LIMIT $2`, query, limit)
	if err != nil {
		a.queryError(w, err)
		return
	}
	defer rows.Close()
	result := make([]skill, 0)
	for rows.Next() {
		var item skill
		if err := rows.Scan(&item.ID, &item.CategoryID, &item.Name, &item.Slug); err != nil {
			a.queryError(w, err)
			return
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		a.queryError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"items": result})
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
