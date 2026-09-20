# Repository Guidelines

## Backend Layout

This directory is the Go module `skillswap/backend`. `cmd/api/main.go` starts the HTTP server; `cmd/db/main.go` handles database checks, migrations, and demo seeding. Put request handlers and middleware in `internal/api/`, and environment parsing and validation in `internal/config/`. PostgreSQL schema changes live in numbered Goose files under `migrations/`; repeatable presentation data belongs in `seeds/demo.sql`.

## Development Commands

Run these commands from `backend/` with Go 1.26 and PostgreSQL available:

- `go test ./...` runs all unit tests and any enabled integration tests.
- `go build ./...` verifies that both commands and internal packages compile.
- `go run ./cmd/db check` verifies database connectivity; `status` shows migration state, `up` applies migrations, and `seed` inserts demo catalog data.
- `go run ./cmd/api` starts the API at `127.0.0.1:8080` unless `API_ADDR` sets another loopback address.

Keep all local backend settings in `.env` and set `DATABASE_URL` before starting the API or database command. This is the only configuration file loaded; existing process environment variables take precedence. Use the values documented in `README.md` when creating it.

## Go and API Conventions

Run `gofmt` on changed Go files. Use lowercase package names, exported names only for public interfaces, and `*_test.go` files beside the code they test. Register versioned routes in `internal/api/api.go` with `http.ServeMux`; keep cross-cutting origin and response-header behavior in `middleware.go`. Return JSON through the existing `respond` and `problem` helpers, and use request contexts with bounded database query timeouts. Keep SQL parameters bound through pgx rather than interpolating request input.

## Tests and Database Changes

Tests use Go's standard `testing` package and `httptest` for HTTP behavior. The catalog integration test runs when `TEST_DATABASE_URL` points to a migrated, demo-seeded local database; without it, the test skips. Add focused tests for changed endpoint, middleware, or configuration behavior. Create a new numbered Goose migration for schema changes instead of rewriting an applied migration, and keep demo inserts idempotent.

## Review and Security

Use focused commit subjects such as `feat(backend): add ...`. In pull requests, describe API or schema changes, list the commands run, and explain any new environment variables or migration steps. Never commit local environment files or credentials. `API_ADDR` must use a loopback IP; remote PostgreSQL URLs require `sslmode=verify-full` and `sslrootcert`.
