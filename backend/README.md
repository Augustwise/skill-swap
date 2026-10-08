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
for 40 minutes (NFR-08; `login_throttles`, migration `00002_auth.sql`).

### Code structure (LR2 component diagram)

| Diagram component | Package |
| ----------------- | ------- |
| HTTP handlers: routes, DTOs, errors | `internal/api` |
| Access and authentication | `internal/auth` |
| Domain core and `IApplication` | `internal/core` (modules such as `internal/profile`) |
| Shared input checks (names, text length, UUID) | `internal/validate` |
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
  POST, PATCH and DELETE that needs a session). On page load call `/auth/me` to restore it.
- Verification emails open `/verify-email?token=...`, where the frontend confirms
  the email through `/auth/verify-email` and then removes the token from the URL.
  Older `/onboarding?token=...` links redirect to the verification page.
  Signed-in users continue to onboarding after verification; otherwise they must sign in.
  Password-reset emails open `/reset-password?token=...` and use
  `/auth/reset-password`.
- Errors always have the shape `{ "error": { "code", "message" } }`; `validation_failed`
  (422) also has `fields`, a map from field name to message for form hints.
- `user.emailVerified` is `false` until a valid link is opened. Unverified users can sign in
  only to check verification status, resend the email, or sign out. The frontend sends them
  to `/verify-email` and blocks onboarding and the dashboard. All profile and skill-list
  endpoints under `/me` return 403 `email_not_verified` until verification succeeds.

Example session with curl (the `Origin` header is required on POST):

```bash
curl -i -c cookies.txt -H "Origin: http://localhost:3000" -H "Content-Type: application/json" \
  -d '{"email":"student@students.example.test","password":"correct horse battery"}' \
  http://127.0.0.1:8080/api/v1/auth/login
curl -b cookies.txt http://127.0.0.1:8080/api/v1/auth/me
```

## Profile and skill lists (Sprint 3, FR-02–FR-03)

`ProfileService` (`internal/profile`) validates and saves the profile and the two skill
lists. All `/me` routes take the user from the session cookie, so a user can only read
and change their own profile; changes also need `X-CSRF-Token` and the frontend `Origin`.
Every successful change returns the whole updated `{ "profile": ... }`.

| Method | Path | Session | Purpose |
| ------ | ---- | ------- | ------- |
| GET | `/api/v1/universities/{universityId}/faculties` | — | Faculties for the profile form |
| GET | `/api/v1/me/profile` | cookie | Profile, both lists and `eligibleForMatching` |
| PATCH | `/api/v1/me/profile` | cookie + CSRF | Change only the sent fields; `null` clears a value |
| POST | `/api/v1/me/teaching-skills` | cookie + CSRF | Add `{ "skillId", "level" }` to "I teach" (201) |
| PATCH | `/api/v1/me/teaching-skills/{skillId}` | cookie + CSRF | Change the level `{ "level" }` |
| DELETE | `/api/v1/me/teaching-skills/{skillId}` | cookie + CSRF | Remove the skill from "I teach" |
| POST, PATCH, DELETE | `/api/v1/me/learning-skills[/{skillId}]` | cookie + CSRF | The same for "I learn" |

Rules checked on the server (NFR-09):

- `firstName` and `lastName` are required, up to 100 characters; the university comes from
  the email domain and cannot be changed;
- `facultyId` must belong to the user's university; `course` is 1–6; `city` is up to
  100 characters; `bio` is up to 600 characters (characters, not bytes);
- `formats` contains `ONLINE` and/or `OFFLINE`; `OFFLINE` requires a `city`;
- `level` is `BEGINNER`, `INTERMEDIATE` or `ADVANCED` (a self-assessment in "I teach", the
  current level in "I learn"); a skill must be active in the catalog and can be in each
  list once (409 `skill_already_added`);
- `eligibleForMatching` is true when both lists are filled, a format is chosen, and
  offline lessons have a city.

A removed skill is kept as an inactive row (`is_active = false`), so exchanges created in
later sprints keep their references; adding it again reactivates the row.

Migration `00003_profile.sql` adds the `OFFLINE` lesson format, limits the course to 1–6,
and makes the learning-goal priority optional (priorities belong to FR-16). Before
applying it to a shared database, make a backup outside the PostgreSQL data directory
(NFR-04), for example:

```bash
pg_dump --format=custom --file=../backups/skillswap-before-00003.dump "$DATABASE_URL"
go run ./cmd/db up
go run ./cmd/db seed
```

The demo seed adds faculties of the demo university and the skills shown in onboarding.

## Search and mutual matches (FR-04, FR-05)

Migration `00004_discovery.sql` enables the `pg_trgm` extension and adds indexes for
searching by a part of a skill name or a person's name, plus a reverse index on
`user_blocks` so a block hides profiles in both directions. Back up a shared database
before applying it (NFR-04):

```bash
pg_dump --format=custom --file=../backups/skillswap-before-00004.dump "$DATABASE_URL"
go run ./cmd/db up
go run ./cmd/db seed
```

The seed also adds 11 demo students (`demo.<name>@students.example.test`) who sign in
with the local-only password `SkillSwapDemo2026`. Sign in as `demo.olha` to check the
acceptance cases:

| Student | Case as seen by Olha |
| --- | --- |
| `demo.andrii` | guitar ↔ Photoshop online: a mutual match |
| `demo.taras` | the same pair, offline only, both in Kyiv: a mutual match |
| `demo.marko` | teaches Photoshop but wants English: one-sided, not mutual |
| `demo.kateryna` | teaches Photoshop, no learning skills: found by search only |
| `demo.nataliia` | hidden profile: never shown |
| `demo.viktor` | unverified email: never shown |
| `demo.oleh` | blocked by Olha: never shown to her |

`demo.sofiia` shares skills with Andrii and Taras but only the offline format in a
different city, so she has no mutual matches. `demo.iryna` has two matches with a
different number of skill pairs (Dmytro 3, Marko 2) to check the ordering.

`GET /api/v1/me/matches?page=1` returns the signed-in user's mutual matches, 20 per page.
It needs a session with a verified email, like the profile endpoints. Two students match
when each teaches at least one catalog skill the other wants and they share a format;
offline counts only in the same city (LR1 3.2). Each item explains the match:
`canTeachYou` (their skills you want), `wantsToLearn` (your skills they want), and
`commonFormats`. Students with more matching skills come first, then by name. The list
is computed on every request, so skill changes apply at once. An incomplete own profile
returns an empty list with `eligibleForMatching: false`; a page past the end returns an
empty list with the real `total`.

`GET /api/v1/students` searches students, 20 per page. All filters are optional and
apply together:

| Parameter | Meaning |
| --- | --- |
| `q` | part of an offered skill name or of the full name, case insensitive |
| `categoryId`, `level` | an offered skill in this category and with this level; with a skill-name `q` it must be the same skill |
| `format` | `ONLINE` or `OFFLINE`, one of the student's formats |
| `mutual=true` | only mutual matches |
| `page` | 1–500 |

Only students who offer at least one skill are listed. Each item has the student card,
all offered skills with levels, formats, and `mutual`; mutual matches come first. With
the demo data, Olha's search for `Photoshop` returns Andrii, Taras, Kateryna and Marko,
while `Photoshop` with `level=INTERMEDIATE` returns only Marko. No results is an empty
`items` array with `total: 0`; invalid filters return 422 `validation_failed`.

`GET /api/v1/students/{userId}` opens another student's profile: university, faculty,
course, city, description, formats, both skill lists with levels, `averageRating`,
`reviewCount` and up to 20 newest `reviews`. Reviews come from finished exchanges, which
later sprints add, so the demo students have none yet. `mutual` explains the match
(`canTeachYou`, `wantsToLearn`, `commonFormats`) or is null for one-sided interest. A
hidden, blocked, unverified or unknown student, and the user's own ID, return 404
`student_not_found`.

Discovery never shows the user themselves, hidden profiles, suspended or deleted
accounts, unverified emails, or blocks in either direction. The PostgreSQL tests in
`internal/data/discovery_test.go` check these rules on the demo students; the test that
changes skills runs inside a transaction that is rolled back.

## Exchange requests (FR-06, FR-07)

Migration `00005_exchange_requests.sql` makes `exchange_requests.expires_at` optional:
in the MVP a request waits until the recipient accepts or declines it or the author
withdraws it. A partial unique index allows only one pending request for the same pair
of skills between two students, in either direction. While Olha's "guitar ↔ Photoshop"
request to Andrii is pending, Andrii cannot send the same pair back and should accept
hers instead. Back up a shared database before applying it (NFR-04):

```bash
pg_dump --format=custom --file=../backups/skillswap-before-00005.dump "$DATABASE_URL"
go run ./cmd/db up
go run ./cmd/db seed
```

The seed adds request cases to the demo students:

| Case | Where to see it |
| --- | --- |
| `demo.taras` → `demo.olha`: pending, Photoshop ↔ guitar, offline, 2 × 60 min each way | Olha's incoming and Taras's sent requests |
| `demo.dmytro` does not accept new requests | a request from `demo.iryna` to Dmytro is refused |
| `demo.andrii` has no requests yet | Olha sends him "guitar ↔ Photoshop" by hand |

Running the seed again does not reset a demo request that was already answered.

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
