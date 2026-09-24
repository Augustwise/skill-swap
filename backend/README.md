# Skill Swap backend

## Requirements

- Go 1.26, as declared in `go.mod`;
- PostgreSQL 15 or newer;
- Mailpit for local email testing. Docker is not required because Mailpit is
  available as a standalone Windows executable.

Run all commands below from the `backend/` directory unless a section says
otherwise.

## Local configuration

Keep all backend settings in a single `backend/.env` file. Create it using
the following values as a starting point:

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

`ALLOWED_EMAIL_DOMAINS` is a comma-separated list of university email domains
accepted at registration. Each domain must also exist in `universities.email_domain`
(the demo seed adds `students.example.test`).

`SMTP_USERNAME` and `SMTP_PASSWORD` must either both be empty, as they are for
the default Mailpit setup, or both be set. The adapter automatically enables
STARTTLS when the SMTP server advertises it.

The backend reads only `.env`. Existing process environment variables take
precedence, allowing test and deployment overrides. The `.env` file is ignored
by Git; do not commit credentials.

Remote PostgreSQL URLs must use verified TLS and include both
`sslmode=verify-full` and `sslrootcert`.

For the network database, use a complete URL, not just the server hostname:

```dotenv
DATABASE_URL="postgres://USER:URL_ENCODED_PASSWORD@HOST:5432/DBNAME?sslmode=verify-full&sslrootcert=C:/Projects/skill-swap/backend/.local/certs/eu-central-1-bundle.pem&connect_timeout=10"
```

Replace the placeholders with the database credentials and name. Percent-encode
special characters in the username and password. The certificate path must point
to the CA bundle for your database server. Mailpit remains independent of the
network database: SMTP uses port 1025, its web UI uses 8025, and the API uses 8080.

## PostgreSQL migrations and demo data

Create an empty `skillswap` database, then check the connection, inspect and
apply the migrations, and load the repeatable demo data:

```bash
go run ./cmd/db check
go run ./cmd/db status
go run ./cmd/db up
go run ./cmd/db seed
```

The `up` command reads Goose migrations from `migrations/`. The `seed` command
loads `seeds/demo.sql`.

## Run the API

```bash
go run ./cmd/api
```

Local development addresses:

- API base URL: `http://127.0.0.1:8080/api/v1`;
- health endpoint: `http://127.0.0.1:8080/api/v1/health`;
- readiness endpoint: `http://127.0.0.1:8080/api/v1/ready`;
- OpenAPI contract: `docs/openapi.yaml` and
  `http://127.0.0.1:8080/api/v1/openapi.yaml`;
- allowed frontend origin: `http://localhost:3000`;
- Mailpit SMTP server: `127.0.0.1:1025`;
- Mailpit web interface: `http://127.0.0.1:8025`.

## Authentication (Sprint 2, FR-01)

The API uses opaque server-side sessions: `POST /auth/login` sets the HttpOnly
`skillswap_session` cookie (SameSite=Lax, Path=/, 7 days) and stores only its SHA-256
hash in `user_sessions`. Passwords are bcrypt hashes with cost 12; a password needs
at least 12 characters and at most 72 UTF-8 bytes. Email verification and password
reset links are single-use tokens stored as hashes in `one_time_tokens` (24 hours and
1 hour). After 5 failed sign-ins for one email within 15 minutes, sign-in is locked
for 2 hours (`login_throttles`, migration `00002_auth.sql`).

### Code structure (LR2 component diagram)

| Diagram component | Package |
| ----------------- | ------- |
| HTTP handlers: routes, DTOs, errors | `internal/api` |
| Access and authentication | `internal/auth` |
| Domain core and `IApplication` | `internal/core` (modules such as `internal/profile`) |
| Data access: `IData` / `ITransaction`, pgx v5, pgxpool | `internal/data` |
| Mail adapter `IMailer` | `internal/mailer` |

Handlers only check the request format, take the user from the session, and call
`internal/auth` or `core.IApplication`. Services group repository calls with
`ITransaction.WithinTx`; every repository method called with that context joins the same
transaction, and emails are sent only after COMMIT. `internal/data/datatest` and
`internal/mailer/mailertest` provide in-memory doubles for unit tests.

| Method | Path | Session | Purpose |
| ------ | ---- | ------- | ------- |
| POST | `/api/v1/auth/register` | — | Create an account and email a verification link |
| POST | `/api/v1/auth/verify-email` | — | Confirm the email with `{ "token" }` |
| POST | `/api/v1/auth/login` | — | Sign in; returns `{ user, csrfToken }` and sets the cookie |
| GET | `/api/v1/auth/me` | cookie | Current `{ user, csrfToken }` or 401 |
| POST | `/api/v1/auth/logout` | cookie + CSRF | Revoke the session |
| POST | `/api/v1/auth/resend-verification` | cookie + CSRF | Send a new verification link |
| POST | `/api/v1/auth/forgot-password` | — | Email a reset link (always 202) |
| POST | `/api/v1/auth/reset-password` | — | Set a new password with `{ "token", "password" }` |

The full contract, including request fields and error codes, is in `docs/openapi.yaml`.

### Connecting the frontend

- The Next.js app proxies `/api/v1/*` to `http://127.0.0.1:8080` (see
  `frontend/next.config.ts`). Call relative URLs such as `fetch("/api/v1/auth/me")` so
  the session cookie belongs to `http://localhost:3000`, and open the app at
  `http://localhost:3000` because that is the allowed `Origin`.
- Send JSON bodies with `Content-Type: application/json`.
- Keep `csrfToken` from `/auth/login` or `/auth/me` in memory and send it as the
  `X-CSRF-Token` header on `/auth/logout` and `/auth/resend-verification` (and on every
  later POST that needs a session). On page load call `/auth/me` to restore it.
- Email links open frontend pages `/verify-email?token=...` and
  `/reset-password?token=...`; these pages send the `token` query value to
  `/auth/verify-email` or `/auth/reset-password`.
- Errors always have the shape `{ "error": { "code", "message" } }`; `validation_failed`
  (422) also has `fields`, a map from field name to message for form hints.
- `user.emailVerified` is `false` until the link is opened. Unverified users can sign in,
  but later features (exchange requests) must require verification.

Example session with curl (the `Origin` header is required on POST):

```bash
curl -i -c cookies.txt -H "Origin: http://localhost:3000" -H "Content-Type: application/json" \
  -d '{"email":"student@students.example.test","password":"correct horse battery"}' \
  http://127.0.0.1:8080/api/v1/auth/login
curl -b cookies.txt http://127.0.0.1:8080/api/v1/auth/me
```

## Run Mailpit on Windows without Docker

Mailpit is distributed as a single portable executable. These commands are for
Git Bash on a standard 64-bit Intel or AMD Windows computer.

Download and extract the latest official release into the ignored local tools
directory:

```bash
cd /c/Projects/skill-swap/backend
mkdir -p .local/mailpit

curl -L \
  https://github.com/axllent/mailpit/releases/latest/download/mailpit-windows-amd64.zip \
  -o .local/mailpit/mailpit.zip

unzip -o .local/mailpit/mailpit.zip -d .local/mailpit
```

For Windows on ARM, download `mailpit-windows-arm64.zip` instead. Release files
and alternative installation methods are listed in the
[official Mailpit installation documentation](https://mailpit.axllent.org/docs/install/).

Start Mailpit and bind both listeners to the local machine only:

```bash
MP_UI_BIND_ADDR=127.0.0.1:8025 \
MP_SMTP_BIND_ADDR=127.0.0.1:1025 \
./.local/mailpit/mailpit.exe
```

Keep that terminal open. In a second Git Bash terminal, send a test email with
the backend development command:

```bash
cd /c/Projects/skill-swap/backend
go run ./cmd/mailtest student@example.test
```

The command should print:

```text
Test email sent to student@example.test via 127.0.0.1:1025
```

Open `http://127.0.0.1:8025` in a browser to inspect the captured email. The
recipient address can be fictional: Mailpit captures the message locally and
does not deliver it to the public internet. PostgreSQL and the API do not need
to be running for this SMTP check.

## Tests and build verification

The API, configuration, and SMTP adapter unit tests do not require external
services:

```bash
go test ./...
go vet ./...
go build ./...
```

The catalog and auth integration tests are skipped unless `TEST_DATABASE_URL` is set. To
run it, use a separate local database that already has the migrations and demo
seed applied:

```bash
export TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/skillswap_test?sslmode=disable'
export DATABASE_URL="$TEST_DATABASE_URL"

go run ./cmd/db up
go run ./cmd/db seed
go test ./internal/api -run AgainstLocalPostgres -v
```
