package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	return New(nil, "http://localhost:3000", slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestHealthAndOrigin(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	request.Header.Set("Origin", "https://example.org")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("foreign origin status = %d", response.Code)
	}
}

func TestCatalogAgainstLocalPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for the local PostgreSQL integration test")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	handler := New(pool, "http://localhost:3000", slog.New(slog.NewTextHandler(io.Discard, nil)))

	for _, test := range []struct {
		path  string
		count int
	}{
		{path: "/api/v1/skills?q=Photoshop", count: 1},
		{path: "/api/v1/skills?q=%25", count: 0},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var body struct {
				Items []skill `json:"items"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Items) != test.count {
				t.Fatalf("got %d skills, want %d", len(body.Items), test.count)
			}
		})
	}
}
