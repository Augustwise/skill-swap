# Repository Guidelines

## Project Structure & Module Organization

`frontend/` contains the Next.js app: routes and layout live in `app/`, reusable UI in `app/components/`, section styles in `app/styles/`, and static assets in `public/`. `backend/` contains the Go API entry point in `cmd/api/`, database tooling in `cmd/db/`, and application code in `internal/api/` and `internal/config/`. Database changes belong in `backend/migrations/`; demo data lives in `backend/seeds/`. Read `frontend/AGENTS.md` before editing frontend code because it contains version-specific Next.js guidance.

## Build, Test, and Development Commands

Run frontend commands from `frontend/` after `npm ci`:

- `npm run dev` starts the app at `http://localhost:3000`.
- `npm run build` creates a production build; `npm start` serves it.
- `npm run lint` checks ESLint rules; `npm run format:check` checks Prettier formatting.

Run backend commands from `backend/` with Go 1.26 and a configured PostgreSQL URL:

- `go run ./cmd/api` starts the local API on `127.0.0.1:8080` by default.
- `go run ./cmd/db check|status|up|seed` checks the connection, inspects or applies migrations, or loads demo data.
- `go test ./...` runs Go tests.

## Coding Style & Naming Conventions

Use TypeScript with strict type checking for frontend changes. Follow the existing two-space indentation and Prettier settings: 100-character lines, double quotes, semicolons, and trailing commas. Name React components and their files in PascalCase (for example, `Logo.tsx`); keep section CSS in `app/styles/`. Format Go files with `gofmt`, use tabs as it emits, and keep package and file names lowercase.

## Testing Guidelines

Go tests use the standard `testing` package and `*_test.go` files beside the code they cover. Add focused tests for changed API or configuration behavior. The catalog integration test runs only when `TEST_DATABASE_URL` points to a prepared local PostgreSQL database; otherwise it skips. There is currently no frontend test script, so run lint, formatting checks, and a production build for frontend changes.

## Commit & Pull Request Guidelines

Recent feature and refactor commits use scoped subjects such as `feat(backend): ...` and `refactor(styles): ...`; use that pattern for new commits. Keep each commit focused. In pull requests, explain the change, list the checks run, note database or environment setup, and include screenshots for visible UI changes.

## Configuration & Security

Copy `backend/.env.example` to a local environment file and set `DATABASE_URL`; do not commit credentials. Keep local API and frontend origins on loopback addresses. Remote database URLs require verified TLS settings (`sslmode=verify-full` and `sslrootcert`).
