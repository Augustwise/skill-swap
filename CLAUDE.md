# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Skill Swap is a student skill-exchange platform: a Next.js web client (`frontend/`), a Go HTTP API (`backend/`) and PostgreSQL. The UI is in Ukrainian. `README.md` (root), `backend/README.md` and `backend/AGENTS.md` hold the detailed setup, API walkthroughs and conventions; `backend/docs/openapi.yaml` is the API contract. `frontend/CLAUDE.md` imports `frontend/AGENTS.md`: this Next.js version (16) has breaking changes, so read the relevant guide in `frontend/node_modules/next/dist/docs/` before writing Next.js code.

## Commands

Backend (run from `backend/`; Go 1.26, PostgreSQL 15+, config only from `backend/.env`, see `backend/README.md` for the template):

```bash
go test ./...                          # unit tests; DB integration tests skip without TEST_DATABASE_URL
go test ./internal/exchange -run TestName -v   # single test / package
go vet ./... && go build ./...
go run ./cmd/db check|status|up|seed   # connectivity, migration state, apply Goose migrations, demo data
go run ./cmd/db backup|counts          # pg_dump to .local/backups (keeps 7), row counts per table
go run ./cmd/db restore FILE           # into a separate local RESTORE_DATABASE_URL, then compares counts
go run ./cmd/db seed-load && go run ./cmd/loadtest   # 1000 students + search/matches load test (separate DB)
go run ./cmd/api                       # API on 127.0.0.1:8080
```

Integration tests (`-run AgainstLocalPostgres` in `internal/api`, `-run AgainstPostgres` in `internal/session`) need `TEST_DATABASE_URL` pointing at a separate, migrated and seeded **local** database (without `seed-load`). `internal/api/mvp_test.go` runs the whole FR-01–FR-15 scenario in memory and, with that variable, against PostgreSQL. Some of them commit data (concurrency tests) and clean up after themselves. The `.env` `DATABASE_URL` may be a shared remote training DB: ask before running `seed` or `up` against it.

Frontend (run from `frontend/`):

```bash
npm ci
npm run dev                  # http://localhost:3000 (must be localhost: API CORS origin)
npm test                     # node --test tests/*.test.mjs
node --test tests/exchange.test.mjs   # single test file
npm run lint && npm run format:check  # `npm run format` fixes
npm run build                # includes the TypeScript check
npm run test:browser         # Playwright against `next start` on port 3100 (build first)
```

Whole stack from the repo root: `node scripts/dev.mjs [--migrate] [--seed] [--no-mail] [--no-api] [--no-web]` starts Mailpit (`backend/.local/mailpit`), builds and runs the API and starts Next.js, logging to `logs/`.

## Architecture

**Request flow:** browser -> Next.js (`next.config.ts` rewrites `/api/v1/*` to `127.0.0.1:8080`, so the HttpOnly `skillswap_session` cookie stays same-origin) -> `internal/api` handlers -> `core.IApplication` / `auth.Service` -> domain services -> `internal/data` repositories -> PostgreSQL.

**Backend layering (strict):** handlers (`internal/api`: routes in `api.go` on `http.ServeMux`, `respond`/`problem` helpers, `middleware.go` for origin/CSRF) only parse requests, take the user from the session and call services; they never run SQL or send mail. Services (`auth`, `profile`, `discovery`, `exchange`, `session`, `communication`) are aggregated by `internal/core` and must not import `net/http`. They depend on small interfaces (`IProfileData`, `IExchangeData`, `ITransaction`, `IMailer`, ...) implemented by `internal/data` (pgx v5) and `internal/mailer`; `cmd/api/main.go` wires the concrete pieces. In-memory doubles live in `internal/data/datatest` and `internal/mailer/mailertest`.

**Cross-cutting backend rules:**
- Multi-step operations use `ITransaction.WithinTx`; repository calls with that context join the same transaction, and emails are sent only after COMMIT.
- Pair operations (messaging, scheduling, confirmations) lock both users in canonical UUID order before touching the row, to avoid deadlocks and make concurrent actions apply serially.
- Exchange flow: request (PENDING) -> accept creates an ACTIVE exchange with copied skill commitments and `totalSessions` DRAFT lesson slots -> sessions are proposed/accepted/scheduled (overlap check on half-open intervals) -> both sides confirm each session -> both confirm exchange completion, or cancel with a reason. Notifications and chat conversations are created inside the same transactions (idempotent via `event_key`).
- Writes require the session cookie, the `X-CSRF-Token` header and the frontend `Origin`. Errors are always `{ "error": { "code", "message" } }`, with `fields` for 422 `validation_failed`. Non-participants get 404, not 403.
- Schema changes: add a new numbered Goose file in `migrations/` (never edit an applied one); keep `seeds/demo.sql` idempotent. Demo students sign in with `demo.<name>@students.example.test`.
- Tests use `internal/expect` (`Equal`, `Slice`, `Error`, `NoError`) with one value per line instead of long boolean conditions.

**Frontend:** App Router. `app/<route>/page.tsx` are thin server entries; the work is in `app/components/`. Each backend area has a `*Api.ts` client module (DTO types, requests, validation mirroring server limits, error mapping) used by PascalCase page components; all requests go through `authRequest` in `AuthApi.ts` (CSRF token from `/auth/me`, 15 s timeout, typed `AuthError`). Private pages are wrapped in `AppGate` (session + verified email) and `AppShell` (navigation, notification bell). Business rules stay on the server; filters/tabs/page numbers live in the URL, and `?next=`/`?from=` return paths go through `safeAppPath`. Unit tests in `frontend/tests/*.test.mjs` run on `node:test` against the `.ts` modules via `tests/load-source.mjs`; styles are per-section CSS files in `app/styles/` (some sections are split into subfolders).

## Working conventions

- Commit subjects are scoped, e.g. `feat(backend): ...`, `feat(frontend): ...`; work is done on task branches and merged through reviewed PRs.
- Keep code comments sparse and free of requirement IDs (FR-xx/LR-xx); put explanations in the README or PR description.
- Run `gofmt` on changed Go files and `prettier` (via `npm run format`) on frontend files.
