# Sprint 10: safety, privacy and moderation

LR1 section 7.2 assigns FR-14 and FR-15 to sprint 10 (14–20 November 2026).
This delivery implements the frontend against the existing Go API and requires
no database migrations.

## Student experience

- `/settings` opens privacy controls loaded from the server: profile visibility,
  accepting new requests and restricting requests to the same university. Hiding
  a profile explains that existing exchanges, lessons and chats stay available.
  A failed save retains the selected values; success uses the server response.
- `/settings?tab=security` changes the password with confirmation, the existing
  12-character / 72-byte UTF-8 rules and translated server validation. Passwords
  are sent only in JSON request bodies and cleared on success. The current
  session stays active while the server revokes the other sessions.
- Deleting an account requires a separate dialog, acknowledgement and the current
  password. The dialog explains anonymization, session revocation, cancellation
  and replacement of messages. Cancel receives initial focus; pending submission
  disables closing and prevents repeated writes. Success returns to login.
- Student profiles, request details, exchange details and chats offer reporting
  and personal blocking. Reports require a trimmed reason of 1–1000 Unicode code
  points. Failures retain the draft; an already-open report links to its status.
- Blocking explains cancellation of shared active exchanges and future lessons,
  closure of pending requests and loss of new messages. After success, affected
  details reload from the server; a blocked profile opens the block list. Chat
  uses the server's `canSend` and `blockedByMe` values and supports unblocking.
  Existing interaction history stays readable.
- `/settings?tab=blocks` lists personal blocks and allows confirmed unblocking.
  `/settings?tab=reports` lists sent reports and their status. Both lists use
  server pagination. Unblocking never restores cancelled exchanges.

## Administrator experience

- The verified session's role controls moderation navigation. Students opening
  an administrator URL see an access message and issue no administrator reads.
  Every operation remains protected by the backend's role check.
- `/admin?status=OPEN` lists reports with a URL status filter and pagination.
  Choosing all statuses retains an explicit empty status in the URL.
- `/admin/reports/{id}` shows the reporter, reported account, reason, status,
  decision and the last 20 moderation actions. Administrators can dismiss a
  report, confirm a violation, or confirm and suspend the account, with a reason.
  Concurrent decisions preserve the draft and offer a data refresh.
- Account controls suspend or restore the reported user after confirmation and
  a reason. Suspension explains session revocation and cancellation. Self-action
  is unavailable; stale roles and backend rejections are explained in Ukrainian.
- `/admin?tab=history` shows the moderation log, including author, time, reason
  and account or catalog skill. `userId` filters the log and exposes account
  controls, including restoration after an earlier suspension.
- `/admin?tab=skills` lists active and hidden catalog entries with URL search
  and pagination. The editor adds skills and changes their name, category, slug,
  description and visibility. Category choices come from `/skill-categories`;
  conflicts retain the draft. The server records catalog changes in its log.

## Implementation and design

`SafetyApi.ts` and `AdminApi.ts` define the DTOs, API calls, validation, URL
helpers and translated errors. All private requests use `authRequest` and
`writeRequest`, including CSRF refresh for PATCH and DELETE. Business rules,
status transitions, account anonymization and moderation logging stay on the
server. User text renders as plain React text.

Routes are thin server entries using Next.js 16 asynchronous route props. The
UI reuses `AppGate`, `AppShell`, the native `AppDialog`, pagination, cards,
typography, purple actions, existing Figma icons and the light `#F7F8FC` canvas.
The supplied Figma file `Fer6NQRC3RZCPHPhqNF8QM` required sign-in during this
delivery. New safety and moderation screens therefore follow the established
repository design; direct comparison with unavailable Figma frames is pending.

## Verification

Run from `frontend/`:

```sh
npm test
npm run lint
npm run format:check
npm run build
npm run test:browser
```

Unit tests cover Unicode and password limits, identifier validation, safe URLs,
pagination, API paths and methods, exact request bodies, CSRF renewal, duplicate
conflicts and translated validation. Browser scenarios cover persistence,
failed reads and writes, retained drafts, focus, deletion acknowledgement,
pending protection, blocks from all four entry points, chat unblocking, role
checks, moderator decisions, concurrent conflicts, restoration, catalog changes
and responsive layouts at 360, 768, 1440 and 1920 px. Screenshots are generated
under the ignored `frontend/test-results/` directory.

Delivery checks passed: 75 frontend unit tests, 62 browser tests (25 safety and
moderation cases plus 37 existing regressions), ESLint with no warnings,
Prettier, the production TypeScript build and `go test -count=1 ./...`.
Successful blocking remains acknowledged if its subsequent detail reload fails.
Dialog screenshots also cover deletion, reporting, moderation and catalog
editing at 360 and 1440 px, with explicit keyboard focus wrapping.

Browser tests use isolated API fixtures and do not modify PostgreSQL. Backend
unit tests run with `go test ./...`; database tests skip when a separate local
`TEST_DATABASE_URL` is absent. A live demonstration with two student accounts,
an administrator and a local PostgreSQL database, followed by review from the
other team member, remains part of final sprint acceptance under LR1.

## Troubleshooting an existing installation

Reports, report decisions and the action log need backend migration 10. A schema
still at version 7 fails with `column ma.skill_id does not exist`. Back up the
configured database, inspect `go run ./cmd/db status` and apply pending migrations
with `go run ./cmd/db up` from `backend/`. Do not reseed an existing database to
fix this error. The API now checks its required migrations before listening.

If a dev server returns 404 for existing dynamic routes after changing branches,
stop it and regenerate its ignored `frontend/.next/dev` cache before restarting.
Keep one dev server per checkout. PostgreSQL integration tests use the stable
`backend/seeds/integration.sql` fixture through the local-only `seed-test` command;
the demonstration dataset is maintained independently.
