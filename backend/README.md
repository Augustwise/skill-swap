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
| Domain core and `IApplication` | `internal/core` (`internal/profile`, `internal/discovery`, `internal/exchange`, `internal/session`, `internal/communication`, `internal/trust`) |
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

The web client shows the same cases at <http://localhost:3000/search>, `/matches`, and
`/students/{id}`; the frontend checklist is in [`frontend/SPRINT4.md`](../frontend/SPRINT4.md).

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

`POST /api/v1/exchange-requests` sends a request. It needs a session with a verified
email, the CSRF header and the frontend origin:

```json
{
  "recipientId": "60000000-0000-0000-0000-000000000002",
  "teachSkillId": "30000000-0000-0000-0000-000000000001",
  "learnSkillId": "30000000-0000-0000-0000-000000000002",
  "format": "ONLINE",
  "teachSessions": 2, "teachDurationMinutes": 60,
  "learnSessions": 2, "learnDurationMinutes": 60,
  "message": "Привіт!"
}
```

`teachSkillId` is the catalog skill you teach and `learnSkillId` the one the recipient
teaches you. The response is 201 with `{ "request" }`: both students, both directions
with the skill, the teacher's and learner's current levels, sessions and minutes, plus
`totalSessions`, `status: "PENDING"`, `message` (or null) and `exchangeId: null`. The
checks run in this order:

| Code | HTTP | When |
| --- | --- | --- |
| `validation_failed` | 422 | not a UUID, your own ID, the same skill both ways, a format other than `ONLINE`/`OFFLINE`, 1–20 sessions and 15–240 minutes in each direction, a different total time in the two directions, a message over 500 characters |
| `student_not_found` | 404 | the recipient is not visible in discovery, including blocks in either direction |
| `requests_closed` | 409 | the recipient does not accept requests |
| `same_university_only` | 409 | the recipient accepts requests only from their university |
| `validation_failed` | 422 | `teachSkillId` is not in your "I teach" and their "I learn" lists, `learnSkillId` is not in their "I teach" and your "I learn" lists, or the format is not in both profiles (offline also needs the same city) |
| `duplicate_request` | 409 | a request for the same pair of skills is pending in either direction |

With the demo data, Olha's request to Andrii succeeds, a second one returns
`duplicate_request`, and her guitar ↔ Photoshop request to Taras does too because his
request is pending. Iryna's request to Dmytro returns `requests_closed`.

Two read-only routes show the requests to their participants. Both need a session
with a verified email:

- `GET /api/v1/me/exchange-requests?direction=incoming|outgoing&status=&page=` lists
  the requests sent to you (`incoming`) or by you (`outgoing`), newest first, 20 per
  page, as `{ items, page, pageSize, total }`. `status` is optional: `PENDING`,
  `ACCEPTED`, `DECLINED` or `WITHDRAWN`. A missing or unknown `direction` or `status`
  returns 422 `validation_failed`.
- `GET /api/v1/exchange-requests/{requestId}` returns `{ request, history }`. Each
  history item has `status`, `changedBy` (`{ id, firstName, lastName }` or null) and
  `createdAt`, oldest first. Anyone except the requester and the recipient gets 404
  `request_not_found`, the same as for an unknown ID.

Participants keep seeing a request after the other student hides the profile or blocks
them, so the history is not lost. With the demo data, Olha's incoming list and Taras's
outgoing list show the Photoshop ↔ guitar request; Andrii gets 404 for it.

Two routes answer a pending request. Both need a session with a verified email, the
CSRF header and the frontend origin, and take no body:

- `POST /api/v1/exchange-requests/{requestId}/decline` — only the recipient.
- `POST /api/v1/exchange-requests/{requestId}/withdraw` — only the requester.

Each locks the request row, sets the new status and `respondedAt`, and adds one history
item with the student who made the change, all in one transaction. The response is 200
with `{ "request" }`. Repeating the same action on a request that already has that
status returns 200 with the request as it is and adds no history, so a retry after a
lost response is safe.

| Code | HTTP | When |
| --- | --- | --- |
| `request_not_found` | 404 | no request has this ID, or you do not take part in it |
| `action_not_allowed` | 403 | you take part in the request but in the other role, e.g. the requester declines |
| `request_not_pending` | 409 | the request was already accepted, declined or withdrawn in another way |

A declined or withdrawn request no longer blocks the pair of skills, so either student
can send a new request. With the demo data, Olha can decline Taras's request, and
declining it again returns 200; Taras then gets `request_not_pending` when he tries to
withdraw it.

`POST /api/v1/exchange-requests/{requestId}/accept` lets the recipient accept a pending
request. It has the same session, CSRF and origin rules and takes no body. In one
transaction it locks the request, checks that it can still be accepted, sets
`ACCEPTED`, `respondedAt` and a history item, and creates an `ACTIVE` exchange: the
requester is `user_a`, the recipient `user_b`, with the request's format and
`totalSessions`, two skill commitments and an `ACTIVE` exchange history item. Each
commitment copies the skill name, the teacher's level, the learner's current level and
the sessions and minutes, so later profile changes do not alter the exchange.

The response is 200 with `{ request, exchange }`. Accepting again, or two acceptances at
the same time, return the same exchange; `UNIQUE (source_request_id)` guards it in the
database too.

| Code | HTTP | When |
| --- | --- | --- |
| `request_not_found` | 404 | no request has this ID, or you do not take part in it |
| `action_not_allowed` | 403 | the requester tries to accept |
| `request_not_pending` | 409 | the request was declined or withdrawn |
| `request_outdated` | 409 | one student blocked the other, an account is suspended or deleted, or one of the four skills was removed from a profile; the request stays pending and its author can withdraw it |

`GET /api/v1/exchanges/{exchangeId}` returns `{ exchange, sessions }` with the terms from
those copies, `partner`, `progress`, `nextSession`, and the ordered lesson plan. Only the
two participants see it; anyone else, an unknown ID or a value that is not a UUID gets
404 `exchange_not_found`. The list of exchanges is `GET /api/v1/me/exchanges` (FR-10).

With the demo data, Olha can accept Taras's request and open the exchange. Do this only
on your own database: the seed does not reset an answered demo request.

The web client covers these routes at <http://localhost:3000/requests>,
`/requests/{id}`, `/exchanges/{id}` and the "Запропонувати обмін" dialog on
`/students/{id}`; the frontend checklist is in [`frontend/SPRINT5.md`](../frontend/SPRINT5.md).
`TestAcceptRequestConcurrentlyAgainstPostgres` commits its data, because two parallel
acceptances need separate transactions; it creates its own students and deletes them
afterwards.

## Private chat and notifications (Sprint 6, FR-08, FR-13)

Migration `00006_communication.sql` adds integrity checks for request and exchange
conversations, text message length (1–2000 characters), message authorship, and
notification titles and idempotency keys. Back up a shared database before applying it
(NFR-04):

```bash
pg_dump --format=custom --file=../backups/skillswap-before-00006.dump "$DATABASE_URL"
go run ./cmd/db up
go run ./cmd/db seed
```

The seed adds a private conversation (`72000000-0000-0000-0000-000000000001`), an
initial chat message from Taras (`73000000-0000-0000-0000-000000000001`), and two
unread notifications (`EXCHANGE_REQUEST` and `CHAT_MESSAGE`) for Olha on Taras's
demo request.

`CommunicationService` (`internal/communication`) manages private chats, messages, and
in-app notifications. `ExchangeService` calls `CommunicationService` inside the same
`WithinTx` transaction when a request is created, accepted, or declined:

- Creating a request opens its `REQUEST` conversation for both participants and records
  an `EXCHANGE_REQUEST` notification for the recipient (`event_key = exchange-request:{requestId}`).
- Accepting a request links the existing request conversation to the created exchange
  (`type = EXCHANGE`, preserving all messages sent while the request was pending) and
  records a `REQUEST_RESPONSE` notification for the requester
  (`event_key = request-response:{requestId}`).
- Declining a request records a `REQUEST_RESPONSE` notification for the requester.
- Sending a chat message locks both participants in canonical UUID order (LR2 5.2),
  checks membership in `conversation_participants` and absence of blocks in either
  direction, stores the message, updates `conversations.last_message_at`, and records a
  `CHAT_MESSAGE` notification (`event_key = chat-message:{messageId}`) for the other
  participant.

Every notification uses `ON CONFLICT (recipient_id, event_key) DO NOTHING` (ARCH-08),
so retries or concurrent acceptances never create duplicate notifications.

### Conversation and chat routes (FR-08, NFR-02)

| Method | Path | Session | Purpose |
| ------ | ---- | ------- | ------- |
| GET | `/api/v1/me/conversations?page=1` | cookie | List your conversations, newest activity first, 20 per page |
| GET | `/api/v1/exchange-requests/{requestId}/conversation` | cookie | Open the conversation of an exchange request |
| GET | `/api/v1/exchanges/{exchangeId}/conversation` | cookie | Open the conversation of an exchange |
| GET | `/api/v1/conversations/{conversationId}` | cookie | Open a conversation by ID |
| GET | `/api/v1/conversations/{conversationId}/messages?page=1&after=` | cookie | List messages chronologically or poll newer messages after `after` |
| POST | `/api/v1/conversations/{conversationId}/messages` | cookie + CSRF | Send `{ "body" }` (1–2000 characters, plain text and links) |
| POST | `/api/v1/conversations/{conversationId}/read` | cookie + CSRF | Mark incoming messages and chat notifications in this conversation as read |

Each `conversation` object includes `partner` (the other student relative to the
signed-in viewer), `requester`, `recipient`, `requesterTeaches`, `recipientTeaches`,
`lastMessage`, `lastMessageAt`, `unreadCount`, and `canSend`.

For an open chat, the frontend loads the initial history with
`GET /api/v1/conversations/{conversationId}/messages` and then polls
`GET /api/v1/conversations/{conversationId}/messages?after={lastMessageId}` every
5 seconds (`pollIntervalSeconds: 5`) so only new messages are transferred (LR2 7.1).

| Code | HTTP | When |
| --- | --- | --- |
| `conversation_not_found` | 404 | no conversation has this ID, or you are not in `conversation_participants` |
| `request_not_found` | 404 | no request has this ID, or you do not take part in it |
| `exchange_not_found` | 404 | no exchange has this ID, or you do not take part in it |
| `conversation_blocked` | 409 | one participant blocked the other, or an account is suspended or deleted; existing history stays readable |
| `validation_failed` | 422 | `body` is empty, over 2000 characters, or has control characters; or `after` is not a message in this conversation |

### Notification routes (FR-13)

| Method | Path | Session | Purpose |
| ------ | ---- | ------- | ------- |
| GET | `/api/v1/me/notifications?unreadOnly=false&page=1` | cookie | List notifications (newest first, 20 per page) and `unreadCount` |
| POST | `/api/v1/me/notifications/{notificationId}/read` | cookie + CSRF | Mark one notification as read; returns `{ notification, unreadCount }` |
| POST | `/api/v1/me/notifications/read-all` | cookie + CSRF | Mark all unread notifications as read; returns `{ markedCount, unreadCount }` |

Each `notification` has `eventType` (`EXCHANGE_REQUEST`, `REQUEST_RESPONSE`,
`CHAT_MESSAGE`, `SESSION_UPDATED`), `title`, `body`, `actor`, `isRead`, `readAt`,
`requestId`, `exchangeId`, `conversationId`, `messageId`, `sessionId`, and `target`
(`{ kind, id, requestId, exchangeId, conversationId, sessionId }`) so clicking a
notification in the UI can open the corresponding request, exchange, conversation, or
session directly.

## Lesson planning, active exchanges and schedule (Sprint 7, FR-09, FR-10)

Migration `00007_sessions.sql` adds integrity constraints for `meeting_places`, `sessions`
(non-empty title, distinct teacher and learner, 15–240 minute duration, schedule state, and
format-specific location/link rules), and `session_schedule_proposals` (duration,
format-specific location/link rules, and a partial unique index allowing at most one
`PENDING` proposal per session). Back up a shared database before applying it (NFR-04):

```bash
pg_dump --format=custom --file=../backups/skillswap-before-00007.dump "$DATABASE_URL"
go run ./cmd/db up
go run ./cmd/db seed
```

The seed adds an active exchange (`80000000-0000-0000-0000-000000000001`) between
`demo.iryna` and `demo.marko` (English ↔ Photoshop, online, 2 × 60 min each way,
`totalSessions: 4`) with four planned lesson slots (`81000000-0000-0000-0000-000000000001`
through `04`):

- Session 1 (`planOrder: 1`, Iryna teaches Marko English): `SCHEDULED` with an accepted
  proposal (`82000000-0000-0000-0000-000000000001`) and both participants' `SCHEDULE`
  confirmations;
- Session 2 (`planOrder: 2`, Iryna teaches Marko English): `DRAFT`;
- Session 3 (`planOrder: 3`, Marko teaches Iryna Photoshop): `PROPOSED` with a pending
  proposal from Marko (`82000000-0000-0000-0000-000000000002`) and an unread
  `SESSION_UPDATED` notification for Iryna;
- Session 4 (`planOrder: 4`, Marko teaches Iryna Photoshop): `DRAFT`.

Accepting an exchange request (`POST /api/v1/exchange-requests/{requestId}/accept`)
automatically creates the `totalSessions` initial `DRAFT` slots in `sessions` (LR2 5.1,
Figure 3).

### Active exchanges and schedule routes (FR-09, FR-10)

| Method | Path | Session | Purpose |
| ------ | ---- | ------- | ------- |
| GET | `/api/v1/me/exchanges?status=&page=1` | cookie | List your exchanges (`ACTIVE` first, 20 per page) with `partner`, `progress`, and `nextSession` |
| GET | `/api/v1/exchanges/{exchangeId}` | cookie | Open an exchange with its terms, `progress`, `nextSession`, and `sessions` |
| GET | `/api/v1/exchanges/{exchangeId}/sessions` | cookie | List all sessions of an exchange ordered by `planOrder` |
| POST | `/api/v1/exchanges/{exchangeId}/sessions` | cookie + CSRF | Propose a lesson on an active exchange (picks the next `DRAFT` slot for `teacherId` or targets `sessionId`) |
| GET | `/api/v1/sessions/{sessionId}` | cookie | Open a session and its chronological `proposals` history |
| POST | `/api/v1/sessions/{sessionId}/proposals` | cookie + CSRF | Propose or reschedule a specific session (`DRAFT`, `PROPOSED`, or `SCHEDULED`) |
| POST | `/api/v1/sessions/{sessionId}/accept` | cookie + CSRF | Accept the partner's pending schedule proposal (`PROPOSED` → `SCHEDULED`) |
| POST | `/api/v1/sessions/{sessionId}/decline` | cookie + CSRF | Decline the partner's pending schedule proposal (`PROPOSED` → `DRAFT`) |
| POST | `/api/v1/sessions/{sessionId}/cancel` | cookie + CSRF | Cancel a `PROPOSED` or `SCHEDULED` session with an optional `{ "reason" }` |
| GET | `/api/v1/me/schedule?status=SCHEDULED&upcoming=false&page=1` | cookie | List your meetings chronologically (`startsAt` ascending, 20 per page), `timezone`, and `pendingProposalsCount` |

Server rules for lesson planning and schedules (FR-09, FR-10, LR2 5.3, NFR-05, NFR-09):

- Only the two participants of an exchange can view or act on its sessions; anyone else
  receives 404 `exchange_not_found` or `session_not_found`, and `GET /api/v1/me/schedule`
  returns only the signed-in student's own lessons.
- `startsAt` and `endsAt` must be ISO-8601 / RFC3339 timestamps with timezone (`timestamptz`),
  in the future, with `endsAt > startsAt` and duration between 15 and 240 minutes.
- `ONLINE` proposals require a valid `http://` or `https://` `meetingUrl` (up to 500
  characters) and no `meetingPlace`; `OFFLINE` proposals require a `meetingPlace` (1–180
  characters, stored in `meeting_places`) and no `meetingUrl`.
- Proposing or accepting a session locks both participants in canonical UUID order
  (`LockUsers`, LR2 5.2–5.3) and checks half-open intervals `[startsAt, endsAt)` against
  all `SCHEDULED` sessions of both participants across all exchanges. Overlapping intervals
  return 409 `session_conflict`; back-to-back intervals (`[14:00, 15:00)` and
  `[15:00, 16:00)`) are allowed.
- Rescheduling an already `SCHEDULED` or `PROPOSED` session supersedes any pending
  proposal, clears previous `SCHEDULE` confirmations in `session_confirmations`, sets the
  session back to `PROPOSED` with only the proposer's confirmation, and notifies the
  partner (`SESSION_UPDATED`).
- Cancelling a `PROPOSED` or `SCHEDULED` session marks it `CANCELLED`, preserves it in the
  exchange history without increasing `progress.completedSessions`, and inserts a
  replacement `DRAFT` slot so the agreed plan can still be completed.

| Code | HTTP | When |
| --- | --- | --- |
| `exchange_not_found` | 404 | no exchange has this ID, or you do not take part in it |
| `session_not_found` | 404 | no session has this ID, or you do not take part in it |
| `action_not_allowed` | 403 | the proposer tries to accept or decline their own proposal |
| `exchange_not_active` | 409 | the exchange is already `COMPLETED` or `CANCELLED` |
| `exchange_blocked` | 409 | one participant blocked the other, or an account is suspended or deleted |
| `plan_limit_reached` | 409 | all planned sessions for the chosen teacher in this exchange are already proposed, scheduled, or completed |
| `session_not_schedulable` | 409 | the session is already `COMPLETED` or `CANCELLED` |
| `session_not_proposed` | 409 | the session has no pending proposal to accept or decline |
| `session_not_cancellable` | 409 | trying to cancel a `DRAFT` or `COMPLETED` session |
| `session_in_past` | 409 | the proposed start time has already passed before acceptance |
| `session_conflict` | 409 | either participant already has a `SCHEDULED` session overlapping `[startsAt, endsAt)` |
| `validation_failed` | 422 | invalid `title`, `description`, `teacherId`, `startsAt`, `endsAt`, `format`, `meetingUrl`, `meetingPlace`, `reason`, `status`, or `upcoming` |

## Lesson confirmations, exchange completion and cancellation (Sprint 8, FR-11)

Migration `00008_exchange_completion.sql` adds integrity checks: a `COMPLETED` session and
a `COMPLETED` exchange need `completed_at`, and a `CANCELLED` exchange needs `cancelled_at`
and a reason of 1–500 characters. Confirmations reuse `session_confirmations` (kind
`COMPLETION`) and `exchange_confirmations` from `00001`; their primary keys keep at most one
confirmation per participant, so a retry or two simultaneous requests never add a second
one (NFR-05). Back up a shared database before applying it (NFR-04):

```bash
pg_dump --format=custom --file=../backups/skillswap-before-00008.dump "$DATABASE_URL"
go run ./cmd/db up
go run ./cmd/db seed
```

The seed adds an active exchange (`80000000-0000-0000-0000-000000000002`) between
`demo.iryna` and `demo.dmytro` (Figma ↔ Go, online, 1 × 60 min each way,
`totalSessions: 2`):

- Session 1 (`81000000-0000-0000-0000-000000000005`, Iryna teaches Figma): `COMPLETED` five
  days ago and confirmed by both;
- Session 2 (`81000000-0000-0000-0000-000000000006`, Dmytro teaches Go): ended yesterday,
  still `SCHEDULED` because only Dmytro has confirmed it. Iryna has an unread
  `SESSION_UPDATED` notification asking her to confirm it.

Sign in as Iryna and confirm session 2: the progress becomes 2 of 2 with
`planCompleted: true`. Then Iryna and Dmytro each confirm the end of the exchange, and it
becomes `COMPLETED`. Cancellation can be shown on the Iryna ↔ Marko exchange from Sprint 7.
Running the seed again does not reset these confirmations.

| Method | Path | Session | Purpose |
| ------ | ---- | ------- | ------- |
| POST | `/api/v1/sessions/{sessionId}/confirm` | cookie + CSRF | Confirm that a `SCHEDULED` session took place; returns `{ session, exchange }` |
| POST | `/api/v1/exchanges/{exchangeId}/complete` | cookie + CSRF | Confirm the end of an exchange with the whole plan done; returns `{ exchange }` |
| POST | `/api/v1/exchanges/{exchangeId}/cancel` | cookie + CSRF | Cancel an active exchange with `{ "reason" }`; returns `{ exchange }` |
| GET | `/api/v1/exchanges/{exchangeId}` | cookie | Now also returns `history`: `status`, `changedBy`, `reason` and `createdAt`, oldest first |

Server rules (FR-11, NFR-05, NFR-08):

- Only the two participants can act; anyone else, an unknown ID or a value that is not a
  UUID gets 404 `session_not_found` or `exchange_not_found`.
- A session can be confirmed only after its `endsAt`. One confirmation does not count the
  session: it stays `SCHEDULED`, and the partner is notified. The second confirmation sets
  `COMPLETED` and `completedAt` in the same transaction, so the session is added to
  `progress.completedSessions` and to its direction's counter. Repeating a confirmation, or
  confirming a completed session, changes nothing.
- `progress.awaitingConfirmationSessions` counts `SCHEDULED` sessions that have already
  ended. If a lesson did not take place, either participant can reschedule or cancel it
  instead; both actions clear the session's confirmations, so a one-sided confirmation does
  not survive a new time.
- The exchange can be completed only when `progress.planCompleted` is true: every planned
  session in both directions is `COMPLETED`. The first call records the participant's
  confirmation and notifies the partner; the exchange stays `ACTIVE`. The partner's call sets
  `COMPLETED`, `completedAt` and a `COMPLETED` history item.
- Cancelling needs a reason of 1–500 characters. In one transaction the exchange becomes
  `CANCELLED` with `cancelledAt`, the reason and a history item; every `DRAFT`, `PROPOSED`
  and `SCHEDULED` session becomes `CANCELLED` with the same reason and pending proposals
  are superseded. Completed sessions, proposals and all history are kept. Repeating the call
  returns the cancelled exchange as it is.
- Confirmations need both accounts to be active and no block between the participants
  (`exchange_blocked`); cancelling stays available as a way out.
- Every action locks both participants in the same order as the other pair operations,
  then the session or exchange row. Two simultaneous confirmations are therefore applied one
  after the other and complete a session or an exchange exactly once.
- The partner gets a `SESSION_UPDATED` notification for each step (`session-completion:`,
  `session-completed:`, `exchange-completion:`, `exchange-completed:`, `exchange-cancelled:`
  event keys). Exchange notifications have no `sessionId`, so their `target` opens the
  exchange.

New response fields: sessions have `completionConfirmedByMe`,
`completionConfirmedByPartner` and `canConfirmCompletion`; exchanges have
`completionConfirmedByMe`, `completionConfirmedByPartner`, `canConfirmCompletion` and
`canCancel`; `progress` has `awaitingConfirmationSessions` and `planCompleted`.

| Code | HTTP | When |
| --- | --- | --- |
| `session_not_found` / `exchange_not_found` | 404 | no session or exchange has this ID, or you do not take part in it |
| `session_not_finished` | 409 | the session has not ended yet |
| `session_not_confirmable` | 409 | the session is `DRAFT`, `PROPOSED` or `CANCELLED` |
| `plan_not_completed` | 409 | not every planned session in both directions is completed |
| `exchange_not_active` | 409 | confirming in a completed or cancelled exchange, completing a cancelled one, or cancelling a completed one |
| `exchange_blocked` | 409 | one participant blocked the other, or an account is suspended or deleted |
| `validation_failed` | 422 | the cancellation `reason` is empty, longer than 500 characters or has control characters |

`TestCompletionConcurrentlyAgainstPostgres` and `TestCancelExchangeAgainstPostgres` in
`internal/session` commit their data, because concurrent confirmations need separate
transactions; they create their own students and delete them afterwards.

## Reviews and ratings (Sprint 9, FR-12)

`internal/trust` (TrustService in LR2) stores reviews in `exchange_reviews` from `00001`,
which already has `UNIQUE (exchange_id, author_id)`, a rating from 1 to 5 and
`author_id <> recipient_id`. Migration `00009_reviews.sql` adds one more check: a stored
comment is not blank and has at most 1000 characters. The rating is computed from
`exchange_reviews` on every read, so it is always up to date.

```bash
go run ./cmd/db up
go run ./cmd/db seed
```

The seed adds a completed exchange (`80000000-0000-0000-0000-000000000003`) between
`demo.iryna` and `demo.dmytro` (English ↔ Go, both lessons held and confirmed). Dmytro has
already rated Iryna 5 and Iryna has an unread `REVIEW_RECEIVED` notification. Sign in as
Iryna, open the exchange and leave a review for Dmytro: his profile then shows the rating.

| Method | Path | Session | Purpose |
| ------ | ---- | ------- | ------- |
| POST | `/api/v1/exchanges/{exchangeId}/reviews` | cookie + CSRF | Rate the partner with `{ "rating", "comment" }`; returns 201 `{ review, exchange }` |
| GET | `/api/v1/exchanges/{exchangeId}/reviews` | cookie | `{ myReview, partnerReview, canReview }`; each review is null until it is written |
| GET | `/api/v1/me/reviews?page=` | cookie | Reviews you received: `{ items, page, pageSize, total, averageRating, reviewCount }` |
| GET | `/api/v1/students/{userId}/reviews?page=` | cookie | The same for a visible student |

Server rules (FR-12, NFR-05, NFR-08):

- Only a participant of a `COMPLETED` exchange can write a review; the recipient is always
  the partner, so a review of oneself is impossible. Someone else's exchange, an unknown ID
  or a value that is not a UUID gets 404 `exchange_not_found`; an active or cancelled
  exchange gets 409 `exchange_not_completed`.
- `rating` is a whole number from 1 to 5; `comment` is optional plain text of up to 1000
  characters (trimmed; blank text is stored as no text). Other values get 422
  `validation_failed`; a fractional rating such as `4.5` is not valid JSON for the field and
  gets 400 `invalid_json`.
- Each participant reviews an exchange once. A second review, also from a parallel request
  stopped by the unique index, gets 409 `review_already_exists`; the first review is kept.
- The review and the partner's `REVIEW_RECEIVED` notification (`review-received:{reviewId}`)
  are stored in one transaction. The notification has `reviewId` and its `target` opens the
  exchange.
- `averageRating` is rounded to one decimal place (4.67 → 4.7) and is null while there are
  no reviews, so the client shows «Ще немає відгуків»; `reviewCount` counts all reviews.
  `GET /students/{userId}` keeps returning the 20 newest reviews; the list routes page
  through all of them, newest first.

New response fields: exchanges have `reviewedByMe`, `reviewedByPartner` and `canReview`
(true for a completed exchange the viewer has not reviewed); reviews have `exchangeId`;
notifications have `reviewId`.

| Code | HTTP | When |
| --- | --- | --- |
| `exchange_not_found` | 404 | no exchange has this ID, or you do not take part in it |
| `student_not_found` | 404 | the student is hidden, blocked, unverified or does not exist |
| `exchange_not_completed` | 409 | the exchange is active or cancelled |
| `review_already_exists` | 409 | you have already reviewed this exchange |
| `validation_failed` | 422 | `rating` is not 1–5, or `comment` is too long or has control characters |

## Privacy, blocks, reports and moderation (Sprint 10, FR-14, FR-15)

`internal/trust` (TrustService in LR2) also holds personal blocks, reports, the
administrator tools and account deletion; privacy settings belong to `internal/profile` and
the password change to `internal/auth`. Migration `00010_moderation.sql` reuses
`user_blocks`, `reports`, `moderation_actions`, `users.account_status`, `users.deleted_at` and
`user_profile_settings` from `00001` and adds checks: a report has a reason of 1 to 1000
characters and `resolved_at` exactly when it is decided, one report per reporter and
reported user stays open (`idx_reports_open_pair`), the moderation log has a known action
and a reason, and may point to a catalog skill instead of a user (`skill_id`).

```bash
go run ./cmd/db up
go run ./cmd/db seed
```

The seed adds the test administrator `demo.admin@students.example.test` (role `ADMIN`, the
same demo password), an open report from `demo.olha` about `demo.oleh` (whom she blocked) and
a report from `demo.andrii` about `demo.marko` that the administrator dismissed. Sign in as
the administrator: `/auth/me` returns `"role": "ADMIN"`, and the client can show the
moderation pages.

Student routes:

| Method | Path | Session | Purpose |
| ------ | ---- | ------- | ------- |
| GET | `/api/v1/me/privacy` | cookie | `{ privacy: { isDiscoverable, acceptsRequests, requestScope } }` |
| PATCH | `/api/v1/me/privacy` | cookie + CSRF | Change only the sent fields; returns the same shape |
| POST | `/api/v1/auth/change-password` | cookie + CSRF | `{ currentPassword, newPassword }`; signs out the other sessions |
| DELETE | `/api/v1/me/account` | cookie + CSRF | `{ password }`; anonymizes the account, 204 and clears the cookie |
| POST | `/api/v1/students/{userId}/block` | cookie + CSRF | Block; returns `{ block, cancelledExchanges, closedRequests }` |
| DELETE | `/api/v1/students/{userId}/block` | cookie + CSRF | Unblock (204) |
| GET | `/api/v1/me/blocks?page=` | cookie | Students you blocked: `{ items: [{ user, createdAt }], page, pageSize, total }` |
| POST | `/api/v1/reports` | cookie + CSRF | `{ reportedUserId, reason }`; returns 201 `{ report }` |
| GET | `/api/v1/me/reports?page=` | cookie | Your reports with `status` and `resolvedAt` |

Administrator routes (`withAdmin`: a verified session with `role = ADMIN`, otherwise 403
`admin_required`):

| Method | Path | Purpose |
| ------ | ---- | ------- |
| GET | `/api/v1/admin/reports?status=&page=` | Reports, newest first, with `reportedUser.accountStatus` and `decision` |
| GET | `/api/v1/admin/reports/{reportId}` | `{ report, history }`: up to 20 earlier actions about the reported user |
| POST | `/api/v1/admin/reports/{reportId}/resolve` | `{ decision: DISMISS \| RESOLVE \| SUSPEND, reason }` |
| POST | `/api/v1/admin/users/{userId}/suspend` | `{ reason }`; returns `{ user, action }` |
| POST | `/api/v1/admin/users/{userId}/restore` | `{ reason }`; returns `{ user, action }` |
| GET | `/api/v1/admin/moderation-actions?userId=&page=` | The log of all administrator actions, newest first |
| GET | `/api/v1/admin/skills?q=&page=` | Every catalog skill with `isActive`, 50 per page |
| POST | `/api/v1/admin/skills` | `{ categoryId, name, slug, description?, isActive? }`; returns 201 `{ skill }` |
| PATCH | `/api/v1/admin/skills/{skillId}` | Change only the sent fields; `isActive: false` hides the skill |

Server rules:

- **Privacy (FR-15).** A hidden profile (`isDiscoverable: false`) is missing from search,
  mutual matches and `/students/{userId}`, and nobody can send it a new request; current
  requests, exchanges and chats keep working. `acceptsRequests` and `requestScope`
  (`ANYONE`, `SAME_UNIVERSITY`) keep their Sprint 5 meaning. Email and meeting places were
  already private: profiles never contain the email, and sessions are shown only to the two
  participants.
- **Password change.** A wrong `currentPassword` gets 422 with `fields.currentPassword`, and
  wrong attempts count towards the sign-in lockout (429 `login_locked`). The current session
  stays; every other session is revoked.
- **Account deletion (FR-15).** After the password check, one transaction locks the user,
  anonymizes the account and ends its interactions: the name becomes
  «Видалений користувач», the email becomes `deleted-{id}@deleted.invalid`, the city,
  description, contacts, formats, skill lists, verification, sign-in identity, blocks and
  sessions are removed or revoked, the user's chat messages become «Повідомлення видалено»,
  request messages and notification texts written by the user are cleared, active exchanges
  are cancelled with the reason «Учасник видалив обліковий запис», own pending requests are
  withdrawn and incoming ones declined. Requests, exchanges, reviews and chats stay for the
  partners, now with the anonymous author.
- **Blocking (FR-14).** You can block a student you can see, one you share a request or an
  exchange with, or one you already blocked; anyone else, yourself excluded (422), gets 404
  `student_not_found`. Both users are locked in UUID order, as in every other pair
  operation, then the block is stored, shared active exchanges are cancelled with the
  reason «Один з учасників заблокував іншого» (the partner gets a `SESSION_UPDATED`
  notification) and pending requests between the pair are closed. After that neither user
  finds the other, new requests are impossible and the chat has `canSend: false`; the
  blocker sees `blockedByMe: true` on the conversation. Repeating the block changes nothing.
  Unblocking does not restore cancelled exchanges.
- **Reports (FR-14).** The same students as for blocking can be reported, so a partner can
  report even after being blocked. `reason` is 1–1000 characters. A second report about
  the same student while the first is `OPEN` gets 409 `report_already_open`. The author sees
  only the status of the report, not the administrator's decision.
- **Moderation (FR-14).** Every administrator action is stored in `moderation_actions` with
  the administrator, time and reason, and is listed by `/admin/moderation-actions`.
  `DISMISS` closes a report as `DISMISSED`, `RESOLVE` as `RESOLVED`, `SUSPEND` as `RESOLVED`
  and suspends the reported account. A decided report gets 409 `report_already_resolved`.
  Suspension sets `account_status = SUSPENDED`, revokes all sessions (sign-in returns 403
  `account_suspended`) and ends the account's exchanges and requests as above, recorded as
  system changes with the reason «Обліковий запис учасника заблоковано адміністратором».
  Restoring lets the user sign in again. Suspending a suspended account or restoring an
  active one returns `action: null` and logs nothing; an administrator cannot moderate their
  own account (409 `self_moderation`).
- **Catalog (FR-14).** Administrators add skills and rename, move, describe, hide or restore
  them. `slug` uses lowercase Latin letters, digits and single hyphens; a duplicate name in
  a category or a taken slug gets 409 `skill_already_exists`. A hidden skill disappears from
  `/skills`, search and matches and cannot be added to lists; agreed exchanges keep their
  skill names. Catalog changes are logged with the skill instead of a user.

| Code | HTTP | When |
| --- | --- | --- |
| `admin_required` | 403 | an `/admin` route is called by a student |
| `student_not_found` | 404 | the student is unknown to you, deleted, or the ID is not a UUID |
| `user_not_found` | 404 | the account to suspend or restore does not exist or was deleted |
| `report_not_found` | 404 | no report has this ID |
| `skill_not_found` | 404 | no catalog skill has this ID |
| `report_already_open` | 409 | you already have an open report about this student |
| `report_already_resolved` | 409 | the report has already been decided |
| `self_moderation` | 409 | an administrator tries to suspend or restore their own account |
| `skill_already_exists` | 409 | the name is taken in the category, or the slug is taken |
| `validation_failed` | 422 | a field is invalid, a password is wrong, or you target yourself (`fields.userId`) |
| `login_locked` | 429 | too many wrong passwords; see `Retry-After` |

## MVP integration, load test and backups (Sprint 11, NFR-01–NFR-05)

Sprint 11 adds no new routes: the frontend keeps using the contract above. It adds checks
of the whole MVP and the tools for the non-functional requirements.

Common API behaviour:

- **Unexpected failures (NFR-03).** A panic in a handler is logged with its stack and
  answered with 500 `internal_error` in the usual `{ "error": { "code", "message" } }`
  shape; the API keeps serving other requests. A transaction that was open at that moment
  is rolled back, so the client can show a generic error and repeat the action.
- **No caching.** Every response has `Cache-Control: no-store`, so the browser does not keep
  private pages such as chats or profiles in its cache.

| Code | HTTP | When |
| --- | --- | --- |
| `internal_error` | 500 | the server failed unexpectedly; see the API log |

### End-to-end scenario (FR-01–FR-15)

`internal/api/mvp_test.go` runs one scenario through the HTTP API, the way the browser
does: two students register and verify their email (an unverified third one cannot send
requests), fill in their profiles, find each other by skill and among mutual matches,
agree one lesson in each direction (a repeated acceptance gives the same exchange), chat
(a stranger gets 404), receive notifications, schedule both lessons, confirm them after
they end (one confirmation does not count), complete the exchange and review each other.
Then one of them reports and blocks a third student, the administrator suspends that
student, a profile is hidden and an account is deleted, leaving «Видалений користувач»
and «Повідомлення видалено» in the partner's history.

```bash
go test ./internal/api -run TestMVPScenario -v
```

`TestMVPScenario` uses the in-memory store and always runs.
`TestMVPScenarioAgainstLocalPostgres` runs the same steps on `TEST_DATABASE_URL` and then
deletes every row it created.

### Load test (NFR-01)

`seeds/load.sql` adds 1000 verified students `load.0001` … `load.1000@students.example.test`
(demo password) with two taught and two wanted skills, about 280 mutual matches each.
`cmd/loadtest` signs in 50 of them and repeats a mix of searches and mutual-match pages
for 10 minutes with a 1 second pause, then prints p50, p95 and the maximum per request
group. It passes when p95 is at most 2 s and fewer than 1% of the requests fail; otherwise
it exits with code 1.

Use a separate local database for this test, not `TEST_DATABASE_URL` and not the shared
training database: the extra students push the demo students off the first result pages
that the demo-data tests check.

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/skillswap_load?sslmode=disable'
go run ./cmd/db up
go run ./cmd/db seed
go run ./cmd/db seed-load
go run ./cmd/api                       # in another terminal, with the same DATABASE_URL
go run ./cmd/loadtest                  # 50 users, 10 minutes
go run ./cmd/loadtest -users 50 -duration 1m   # a short check
```

Other flags: `-base` (API URL, default `http://127.0.0.1:8080/api/v1`), `-pause`,
`-timeout`, `-password`. A short run on a development laptop (50 users, 30 s) gave p95 of
about 40 ms with no errors.

### Backup and restore (NFR-04)

```bash
go run ./cmd/db backup                 # pg_dump into .local/backups, keep the newest 7
go run ./cmd/db counts                 # row count of every table
RESTORE_DATABASE_URL='postgres://postgres:postgres@localhost:5432/skillswap_restore?sslmode=disable' \
  go run ./cmd/db restore .local/backups/skillswap-20261130-180000.dump
```

- `backup` runs `pg_dump --format=custom` for `DATABASE_URL` and saves
  `skillswap-YYYYMMDD-HHMMSS.dump` in `BACKUP_DIR` (default `backend/.local/backups`, ignored
  by Git and outside the PostgreSQL data directory). After a successful dump it deletes
  the older `skillswap-*.dump` files, keeping 7. Make a backup before `go run ./cmd/db up`
  and at the end of each working day with data.
- `restore` runs `pg_restore --clean --if-exists --single-transaction` into
  `RESTORE_DATABASE_URL`, which must be a local database other than `DATABASE_URL`, prints
  the time it took and compares the row count of every table with `DATABASE_URL`.
  Create the empty target database first (`createdb skillswap_restore`). To finish the
  check, run the tests or the scenario against the restored database:
  `TEST_DATABASE_URL=$RESTORE_DATABASE_URL go test ./...`.
- `pg_dump` and `pg_restore` come with PostgreSQL. If they are not in `PATH`, set `PG_BIN`,
  for example `PG_BIN='C:/Program Files/PostgreSQL/18/bin'`. The database password is passed
  to them through `PGPASSWORD`, not on the command line.

| Variable | Default | Used by |
| --- | --- | --- |
| `BACKUP_DIR` | `.local/backups` | `backup` |
| `RESTORE_DATABASE_URL` | — (required) | `restore` |
| `PG_BIN` | — (`PATH` is used) | `backup`, `restore` |

### Restart check (NFR-03, NFR-05)

1. Start the API and the frontend, sign in as two demo students and go through part of the
   scenario (send a request, accept it, send messages).
2. Stop the API (Ctrl+C: it finishes running requests within 10 s) and restart PostgreSQL.
3. Start the API again: `/api/v1/ready` returns `ready`, and the requests, exchanges and
   messages are still there; continue the scenario.

Atomicity and repeated actions (NFR-05) are covered by the concurrency tests in
`internal/exchange` and `internal/session` (`AgainstPostgres`): two simultaneous
acceptances or confirmations produce one exchange, one completed lesson and one
notification.

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
go test ./internal/session -run AgainstPostgres -v
go test ./internal/data -run AgainstPostgres -v
```

`TestMVPScenarioAgainstLocalPostgres` (part of the first command) runs the whole MVP
scenario on this database and removes its rows afterwards. Do not run
`go run ./cmd/db seed-load` on the test database: the demo-data tests expect only the demo
students on the first result pages.
