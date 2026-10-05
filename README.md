# Skill Swap

Skill Swap is a student platform for exchanging skills without payment. A student can
list what they teach and what they want to learn, then find a suitable learning partner.
The project uses a Next.js web client, a modular Go API, and PostgreSQL.

Repository: [Augustwise/skill-swap](https://github.com/Augustwise/skill-swap).

## Implementation scope

This checkout (`feat/SCRUM-6-profile-backend`, code commit `b349455`) contains the
application foundation, authentication, reference catalogs, and the profile and skill-list
API (FR-01–FR-03). The web client includes the landing page, registration, sign-in,
password reset, email verification, and an initial onboarding interface. In this checkout,
onboarding skill selection is still local UI state; the profile API is implemented separately.
Later frontend integration is available on newer branches and `main`.

Matching, exchange requests, chat, lesson scheduling, reviews, and moderation remain
planned application modules. An existing database table or UI placeholder does not mean
that the corresponding end-to-end feature is complete.

## Project structure

```text
skill-swap/
├── frontend/
│   ├── app/                 # Next.js routes and root layout
│   │   ├── components/      # React forms and reusable UI
│   │   └── styles/          # Global and section CSS
│   ├── public/              # Icons and static design assets
│   └── package.json         # Scripts and frontend dependencies
├── backend/
│   ├── cmd/api/             # API entry point and dependency wiring
│   ├── cmd/db/              # Connection checks, migrations, and demo seeding
│   ├── cmd/mailtest/        # Local SMTP smoke test
│   ├── internal/api/        # HTTP routes, DTOs, middleware, and errors
│   ├── internal/auth/       # Registration, sessions, and verification
│   ├── internal/core/       # Application facade and IApplication contract
│   ├── internal/profile/    # Profile and skill-list business rules
│   ├── internal/data/       # Repository interfaces and PostgreSQL adapter
│   ├── internal/mailer/     # IMailer contract and SMTP adapter
│   ├── internal/config/     # Configuration loading and validation
│   ├── internal/validate/   # Shared input validation
│   ├── migrations/          # Versioned Goose SQL migrations
│   ├── seeds/               # Repeatable demo data
│   └── docs/                # OpenAPI specification
├── scripts/dev.mjs          # Local process runner
├── .gitignore               # Local configuration and generated-file exclusions
└── README.md
```

Go tests are stored beside the code as `*_test.go`. In-memory test doubles are in
`internal/data/datatest` and `internal/mailer/mailertest`; a separate top-level `tests/`
directory is not required. Detailed setup and API documentation are in
[`backend/README.md`](backend/README.md) and
[`backend/docs/openapi.yaml`](backend/docs/openapi.yaml).

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
| `core.Application` / `IApplication` | Application entry point; `Profile`, `UpdateProfile`, `AddSkill`, and catalog queries. |
| `profile.Service` | Profile validation, teaching/learning lists, and `EligibleForMatching`. |
| `data.Postgres` | SQL repositories and `WithinTx` transaction boundaries. |
| `mailer.SMTP` / `IMailer` | Sending verification and reset messages through `Send`. |
| React form components | `RegistrationForm`, `LoginForm`, `ResetPasswordForm`, and `Onboarding`. |

### SOLID in the implementation

- **Single responsibility:** HTTP handling, authentication, profile rules, storage,
  and email delivery live in separate packages.
- **Open/closed:** services can receive another storage or mail implementation through
  their existing interfaces without rewriting their business rules.
- **Liskov substitution:** PostgreSQL and in-memory repositories implement the same
  contracts, allowing service tests to substitute a test double. The substitutes must
  preserve the contract's results and error semantics.
- **Interface segregation:** `IAuthData`, `ICatalogData`, `IProfileData`, `ITransaction`,
  and `IMailer` expose separate responsibilities.
- **Dependency inversion:** service constructors receive interfaces; concrete
  infrastructure is wired at the application entry point.

## Team roles and contributions

| Team member | Main role | Contribution to the current foundation |
| --- | --- | --- |
| Mykola Sheiko (Sheiko M. P.) | Backend developer | Go API, PostgreSQL schema and migrations, authentication, profile and skill-list services, backend tests, and local development tooling. |
| Oleksandr Nesmiian (Nesmiian O. S.) | Frontend developer | Next.js/React interface, registration and login forms, onboarding screens, and integration with authentication and email verification. |
| Both team members | Planning and quality assurance | Requirements, architecture, task estimation, integration checks, and mutual code review as the agreed workflow. |

The division follows Laboratories 1–3. Git history records backend work under
`Mykola Sheiko` and frontend work under `Oleksandr`; ownership does not prohibit
either participant from helping with the other part of the application.

## Git collaboration workflow

1. Create a task-specific branch from an up-to-date `main`; include the task key
   where available, for example `feat/SCRUM-6-profile-backend`.
2. Keep commits focused and use scoped subjects such as `feat(backend): ...` or
   `feat(frontend): ...`.
3. Open a pull request with a description, setup notes, checks, and screenshots
   for visible UI changes.
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

Back up an existing database before applying schema changes. The seed is intended for
development, not production data.

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
npm run lint
npm run format:check
npm run build
npm start -- --hostname 127.0.0.1
```

There is currently no frontend unit-test script. The build checks TypeScript and creates
the production bundle. Lint and formatting are separate checks and should both pass
before merging. Root and frontend `.gitignore` files exclude secrets, dependencies,
build outputs, logs, and local tools.
