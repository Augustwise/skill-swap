# Skill Swap backend

## Requirements

- Go 1.26, as declared in `go.mod`;
- PostgreSQL 15 or newer;
- Mailpit for local email testing. Docker is not required because Mailpit is
  available as a standalone Windows executable.

Run all commands below from the `backend/` directory unless a section says
otherwise.

## Local configuration

Create the local environment file:

```bash
cp .env.example .env.local
```

Use the following values as a starting point:

```dotenv
DATABASE_URL=postgres://postgres:postgres@localhost:5432/skillswap?sslmode=disable
API_ADDR=127.0.0.1:8080
FRONTEND_ORIGIN=http://localhost:3000
SMTP_ADDR=127.0.0.1:1025
SMTP_USERNAME=
SMTP_PASSWORD=
MAIL_FROM=no-reply@students.example.test
```

`SMTP_USERNAME` and `SMTP_PASSWORD` must either both be empty, as they are for
the default Mailpit setup, or both be set. The adapter automatically enables
STARTTLS when the SMTP server advertises it.

The backend loads the first available file in this order: `.env.local`, then
`.env`. Existing environment variables take precedence over values from these
files. Do not commit local environment files or credentials.

Remote PostgreSQL URLs must use verified TLS and include both
`sslmode=verify-full` and `sslrootcert`.

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

The catalog integration test is skipped unless `TEST_DATABASE_URL` is set. To
run it, use a separate local database that already has the migrations and demo
seed applied:

```bash
export TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/skillswap_test?sslmode=disable'
export DATABASE_URL="$TEST_DATABASE_URL"

go run ./cmd/db up
go run ./cmd/db seed
go test ./internal/api -run TestCatalogAgainstLocalPostgres -v
```
