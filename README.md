# Skill Swap

**Trade what you know for what you want to learn.** Skill Swap is a free platform for
university students to exchange skills: you teach a peer something you're good at, and
they teach you something in return. No money changes hands.

Built with **Next.js 16** · **Go 1.26** · **PostgreSQL 15+**. The interface is in Ukrainian.

## Features

- **Verified student accounts.** Registration with a university email, email
  verification and password reset.
- **Skill profiles.** Each student lists what they can teach and what they want to learn.
- **Search and mutual matches.** Find students by skill, or let the platform suggest
  people whose skills complement yours.
- **Exchange requests.** Propose an exchange; the other student accepts, declines or you
  withdraw it.
- **Lesson planning.** Agree on lesson times, see a personal schedule, and have both
  sides confirm each lesson and the finished exchange.
- **Chat and notifications.** Private messages between partners and in-app notifications.
- **Reviews and ratings** after a completed exchange.
- **Safety tools.** Blocking, reports, and an admin moderation panel.

## Quick start

### Prerequisites

- [Node.js](https://nodejs.org/) 20.9+ and npm
- [Go](https://go.dev/dl/) 1.26
- [PostgreSQL](https://www.postgresql.org/download/) 15+ with an empty `skillswap` database
- [Mailpit](https://mailpit.axllent.org/docs/install/) to catch local emails
  (see [backend/README.md](backend/README.md#run-mailpit-on-windows-without-docker)
  for a portable Windows setup)

### 1. Install dependencies

```bash
git clone https://github.com/Augustwise/skill-swap.git
cd skill-swap
(cd frontend && npm ci)
(cd backend && go mod download)
```

### 2. Configure the backend

Create `backend/.env`:

```dotenv
DATABASE_URL=postgres://postgres:postgres@localhost:5432/skillswap?sslmode=disable
API_ADDR=127.0.0.1:8080
FRONTEND_ORIGIN=http://localhost:3000
SMTP_ADDR=127.0.0.1:1025
SMTP_USERNAME=
SMTP_PASSWORD=
MAIL_FROM=no-reply@students.example.test
ALLOWED_EMAIL_DOMAINS=students.example.test
```

The file is git-ignored; never commit credentials.

### 3. Run everything

```bash
node scripts/dev.mjs --migrate --seed
```

This starts Mailpit, applies database migrations, loads demo data, then builds and runs
the API and the web app. Logs go to `logs/`; press Ctrl+C to stop. Drop `--migrate --seed`
on later runs, and use `--no-mail`, `--no-api` or `--no-web` to skip a service that's
already running.

| Service         | Address                                      |
| --------------- | -------------------------------------------- |
| Web app         | http://localhost:3000                        |
| API health      | http://127.0.0.1:8080/api/v1/health          |
| OpenAPI spec    | http://127.0.0.1:8080/api/v1/openapi.yaml    |
| Mailpit inbox   | http://127.0.0.1:8025                        |

> Open the app at `localhost:3000` exactly: it's the only origin the API accepts.

### Demo accounts

The demo seed creates students such as `demo.olha@students.example.test` and
`demo.andrii@students.example.test`, plus an administrator
`demo.admin@students.example.test`. All of them use the local-only password
`SkillSwapDemo2026`.

## How it works

```text
Browser ──► Next.js (frontend/) ──/api/v1/*──► Go API (backend/) ──► PostgreSQL
                                                     │
                                                     └──► SMTP (Mailpit locally)
```

The Next.js app proxies `/api/v1/*` to the Go API, so the session cookie stays
same-origin. All business rules live on the server; the frontend only renders data and
mirrors validation for quicker feedback.

The API is layered: HTTP handlers parse requests and call domain services (`auth`,
`profile`, `discovery`, `exchange`, `session`, `communication`, `trust`), which talk to
storage and email through small interfaces. That keeps the services free of HTTP and SQL
details and lets tests swap in in-memory implementations.

```text
skill-swap/
├── frontend/          Next.js app: routes in app/, page components and API clients in app/components/
├── backend/
│   ├── cmd/           Entry points: api, db (migrate/seed/backup), loadtest, mailtest
│   ├── internal/      HTTP layer, domain services, PostgreSQL and mail adapters
│   ├── migrations/    Goose SQL migrations
│   ├── seeds/         Demo and load-test data
│   └── docs/          OpenAPI contract
└── scripts/dev.mjs    One-command local runner
```

## Development

### Tests and checks

```bash
# backend/
go test ./...
go vet ./...

# frontend/
npm test
npm run lint
npm run format:check
npm run build
```

Database integration tests run only when `TEST_DATABASE_URL` points at a separate local
database; see [backend/README.md](backend/README.md#tests-and-build-verification).

### Database commands

Run from `backend/`:

```bash
go run ./cmd/db status    # migration state
go run ./cmd/db up        # apply migrations
go run ./cmd/db seed      # load demo data (idempotent)
go run ./cmd/db backup    # pg_dump to .local/backups, keeps the last 7
```

### Contributing

1. Branch off an up-to-date `main`, e.g. `feat/SCRUM-6-profile-backend`.
2. Use scoped commit subjects: `feat(backend): ...`, `fix(frontend): ...`.
3. Open a pull request and get a review from the other team member before merging.

## Documentation

- [backend/README.md](backend/README.md): configuration details, every API area with
  example requests, load testing, backup and restore.
- [backend/docs/openapi.yaml](backend/docs/openapi.yaml): the API contract.

## Team

| Member             | Role                                                         |
| ------------------ | ------------------------------------------------------------ |
| Mykola Sheiko      | Backend: Go API, database, domain services, tests, tooling   |
| Oleksandr Nesmiian | Frontend: Next.js interface, auth and onboarding flows       |
