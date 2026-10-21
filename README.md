# Skill Swap

Skill Swap is a student platform for exchanging skills without payment. A student can
list what they teach and what they want to learn, then find a suitable learning partner.
The project uses a Next.js web client, a modular Go API, and PostgreSQL.

Repository: [Augustwise/skill-swap](https://github.com/Augustwise/skill-swap).

## Project structure

```text
skill-swap/
├── frontend/
│   ├── app/                 # Next.js routes (search, matches, students/[id], requests, exchanges/[id], ...)
│   │   ├── components/      # Pages, shared shell, and *Api.ts HTTP client modules
│   │   └── styles/          # Global and section CSS
│   ├── public/              # Icons and static design assets
│   ├── tests/               # node:test unit tests for the client modules
│   └── package.json         # Scripts and frontend dependencies
├── backend/
│   ├── cmd/api/                 # API entry point and dependency wiring
│   ├── cmd/db/                  # Connection checks, migrations, seeding, backup, and restore
│   ├── cmd/loadtest/            # Search and mutual-match load test
│   ├── cmd/mailtest/            # Local SMTP smoke test
│   ├── internal/api/            # HTTP routes, DTOs, middleware, and errors
│   ├── internal/auth/           # Registration, sessions, and verification
│   ├── internal/core/           # Application facade and IApplication contract
│   ├── internal/profile/        # Profile and skill-list business rules
│   ├── internal/discovery/      # Student search, candidate profile, and mutual matches
│   ├── internal/exchange/       # Exchange requests, decisions, completion, and cancellation
│   ├── internal/session/        # Lesson planning, time agreement, confirmations, and schedule
│   ├── internal/communication/  # Private text chat and in-app notifications
│   ├── internal/trust/          # Reviews, blocks, reports, moderation, and account deletion
│   ├── internal/data/           # Repository interfaces and PostgreSQL adapter
│   ├── internal/mailer/         # IMailer contract and SMTP adapter
│   ├── internal/config/         # Configuration loading and validation
│   ├── internal/validate/       # Shared input validation
│   ├── internal/expect/         # Readable test assertions
│   ├── migrations/              # Versioned Goose SQL migrations
│   ├── seeds/                   # Repeatable demo data and load-test students
│   └── docs/                    # OpenAPI specification
├── scripts/dev.mjs          # Local process runner
├── .gitignore               # Local configuration and generated-file exclusions
└── README.md
```

Go tests are stored beside the code as `*_test.go`. In-memory test doubles are in
`internal/data/datatest` and `internal/mailer/mailertest`; a separate top-level `tests/`
directory is not required. Detailed setup and API documentation are in
[`backend/README.md`](backend/README.md) and
[`backend/docs/openapi.yaml`](backend/docs/openapi.yaml). The web client, its routes and
conventions are described in [`frontend/README.md`](frontend/README.md); sprint notes are
in `frontend/SPRINT2.md`–`SPRINT6.md`.

## Architecture and main components

The system follows the layered architecture from Laboratory 2. Requests flow from
the Next.js client through HTTP handlers to application services and repositories.
`cmd/api` constructs and connects the services, PostgreSQL pool, and mail adapter.
Go structs, methods, and interfaces provide the responsibilities represented as
classes and interfaces in the design.

| Component | Responsibility and representative operations |
| --- | --- |
| `api.API` | HTTP routing, request parsing, responses, and access checks; `api.New` builds the handler. |
| `auth.Service` | Registration, login, email verification, password reset, and session management. |
| `core.Application` / `IApplication` | Application entry point; coordinates profile, discovery, exchange, session, and communication services. |
| `profile.Service` | Profile validation, teaching/learning lists, and `EligibleForMatching`. |
| `discovery.Service` | Student search with filters, candidate profiles, and `MutualMatches`. |
| `exchange.Service` | Exchange requests, decisions (`AcceptRequest`, `DeclineRequest`, `WithdrawRequest`), `Exchanges`, final confirmation (`CompleteExchange`), and `CancelExchange`. |
| `session.Service` | Lesson plan (`ExchangeSessions`), proposals (`ProposeSession`, `AcceptSessionProposal`, `DeclineSessionProposal`), cancellation, two-sided `ConfirmSessionCompletion`, and chronological `Schedule`. |
| `communication.Service` | Private conversations, chat messages, 5-second polling (`Messages`), and in-app `Notifications`. |
| `data.Postgres` | SQL repositories and `WithinTx` transaction boundaries. |
| `mailer.SMTP` / `IMailer` | Sending verification and reset messages through `Send`. |
| React form components | `RegistrationForm`, `LoginForm`, `ResetPasswordForm`, `Onboarding`, and `Profile`. |
| `AppShell` / `AppGate` | Shared signed-in layout (navigation with the incoming-requests counter, top bar search, account menu) and the verified-session gate. |
| Discovery pages | `StudentSearch`, `MutualMatches`, and `CandidateProfile` render FR-04/FR-05 through `DiscoveryApi`. |
| Exchange pages | `ExchangeProposal`, `ExchangeRequests`, `RequestOverview`, `RequestActions`, and `ExchangeOverview` render FR-06/FR-07 (sending, accepting, declining, withdrawing, the started exchange) through `ExchangeApi`. |
| `*Api.ts` client modules | `AuthApi`, `ProfileApi`, `DiscoveryApi`, and `ExchangeApi`: DTO types, requests, validation, and error mapping. |

The web client is the presentation layer from Laboratory 2. Pages use only the HTTP
API (through the `/api/v1/*` rewrite) and render its DTOs; matching, access, and state
rules stay in the Go services. Filters and page numbers are kept in the URL, and return
paths are limited to known in-app pages.

### SOLID in the implementation

- **Single responsibility:** HTTP handling, authentication, profile rules, discovery,
  exchanges, session scheduling, communication, storage, and email delivery live in
  separate packages.
- **Open/closed:** services can receive another storage or mail implementation through
  their existing interfaces without rewriting their business rules.
- **Liskov substitution:** PostgreSQL and in-memory repositories implement the same
  contracts, allowing service tests to substitute a test double. The substitutes must
  preserve the contract's results and error semantics.
- **Interface segregation:** `IAuthData`, `ICatalogData`, `IProfileData`,
  `IDiscoveryData`, `IExchangeData`, `ISessionData`, `ICommunicationData`,
  `ITransaction`, and `IMailer` expose separate responsibilities.
- **Dependency inversion:** service constructors receive interfaces; concrete
  infrastructure is wired at the application entry point.

## Team roles and contributions

| Team member | Main role | Contribution to the current foundation |
| --- | --- | --- |
| Mykola Sheiko (Sheiko M. P.) | Backend developer | Go API, PostgreSQL schema and migrations, authentication, profile, discovery, exchange, session, and communication services, backend tests, local development tooling, and the sprint 4–5 web pages for search, mutual matches, candidate profiles, and exchange requests. |
| Oleksandr Nesmiian (Nesmiian O. S.) | Frontend developer | Next.js/React interface, registration and login forms, onboarding screens, and integration with authentication and email verification. |
| Both team members | Planning and quality assurance | Requirements, architecture, task estimation, integration checks, and mutual code review as the agreed workflow. |

The division follows Laboratories 1–3. Git history records most backend work under
`Mykola Sheiko` and most frontend work under `Oleksandr`; ownership does not prohibit
either participant from helping with the other part of the application.

## Git collaboration workflow

1. Create a task-specific branch from an up-to-date `main`; include the task key
   where available, for example `feat/SCRUM-6-profile-backend`.
2. Keep commits focused and use scoped subjects such as `feat(backend): ...` or
   `feat(frontend): ...`.
3. Open a pull request with a description, setup notes, and checks.
4. Ask the other team member to review the changes, address feedback, and record
   approval before merging into `main`.

Existing examples include [`feat/registration`, PR #2](https://github.com/Augustwise/skill-swap/pull/2)
and [`feat/SCRUM-6-profile-backend`, PR #3](https://github.com/Augustwise/skill-swap/pull/3).
PR #3 confirms a branch-based merge by the other contributor. As checked on
October 5, 2026, it has no submitted GitHub review; the merge alone is not evidence
that the formal review step was completed.

## Requirements

- Node.js **20.9 or later** (the installed Next.js package requires `>=20.9.0`) and npm.
- Go **1.26**, as declared in `backend/go.mod`.
- PostgreSQL **15 or later** and an empty development database named `skillswap`.
- Mailpit for local verification and password-reset emails.

## Installation and local configuration

```bash
git clone https://github.com/Augustwise/skill-swap.git
cd skill-swap
cd frontend
npm ci
cd ../backend
go mod download
```

Create `backend/.env` using the example in [`backend/README.md`](backend/README.md).
Set `DATABASE_URL` for your database, `API_ADDR=127.0.0.1:8080`,
`FRONTEND_ORIGIN=http://localhost:3000`, and the Mailpit SMTP settings.
Keep credentials out of Git. Local development services must listen only on loopback;
remote PostgreSQL connections require `sslmode=verify-full` and `sslrootcert`.

Prepare the development database from `backend/`:

```bash
go run ./cmd/db check
go run ./cmd/db status
go run ./cmd/db up
go run ./cmd/db seed
```

Back up an existing database before applying schema changes with `go run ./cmd/db backup`
(it keeps the newest 7 copies in `backend/.local/backups`; `restore` loads one into a separate
local database). The seed is intended for development, not production data. The load test,
backups and the end-to-end scenario test are described in the backend README.

## Run the application

Install the portable Mailpit executable as described in the backend README. After
configuration and database preparation, start Mailpit and the API from the project root:

```bash
node scripts/dev.mjs --no-web
```

In a second terminal, run the frontend on loopback:

```bash
cd frontend
npm run dev -- --hostname 127.0.0.1
```

The runner starts Mailpit, builds and starts the Go API, and writes service logs to
`logs/`. Press Ctrl+C to stop its child processes. Use `--no-mail`,
`--no-api`, or `--no-web` when a service is already running. Optional `--migrate` and
`--seed` prepare the configured database before API startup.

Alternatively, start Mailpit separately, run `go run ./cmd/api` from `backend/`, and
run `npm run dev -- --hostname 127.0.0.1` from `frontend/` in another terminal.

| Service | Local address |
| --- | --- |
| Web application | <http://localhost:3000> |
| API health | <http://127.0.0.1:8080/api/v1/health> |
| Database readiness | <http://127.0.0.1:8080/api/v1/ready> |
| OpenAPI contract | <http://127.0.0.1:8080/api/v1/openapi.yaml> |
| Mailpit inbox | <http://127.0.0.1:8025> |

The frontend proxies `/api/v1/*` to the Go API. Open the web application using
`localhost:3000`, matching the configured frontend origin.

## Verification and production build

From `backend/`:

```bash
go test ./...
go vet ./...
go build ./...
```

Database integration tests require a separate, migrated and seeded **local** database
in `TEST_DATABASE_URL`; otherwise they skip. See the backend README for the commands.

From `frontend/`:

```bash
npm test
npm run lint
npm run format:check
npm run build
npm start -- --hostname 127.0.0.1
```

`npm test` runs `node:test` unit tests for the client modules (validation, request paths,
URL filters, error mapping, exchange-request rules, and safe return paths) without a browser. The build checks
TypeScript and creates the production bundle. Lint and formatting are separate checks
and should all pass before merging. Root and frontend `.gitignore` files exclude secrets, dependencies,
build outputs, logs, and local tools.
