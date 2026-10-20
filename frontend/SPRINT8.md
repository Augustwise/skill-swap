# Sprint 8: confirmations, progress and exchange lifecycle (FR-11)

LR1 section 7.2 assigns FR-11 to sprint 8 (31 October–6 November 2026).
Reviews and ratings (FR-12) belong to sprint 9.

## Five stacked pull requests

| Branch (`codex/sprint8-…`) | Deliverable                                                                    |
| -------------------------- | ------------------------------------------------------------------------------ |
| `completion-api`           | Typed API, server eligibility flags, state messages, validation and unit tests |
| `session-confirmation`     | Mutual lesson confirmation after the agreed end time                           |
| `exchange-completion`      | Directional progress and mutual final completion                               |
| `exchange-cancellation`    | Cancellation reason, preserved history and archived exchanges                  |
| `integration`              | Browser acceptance tests, responsive checks and documentation                  |

Each PR targets the preceding branch; PR 1 targets `main`. Merge dependent PRs first, from the top of the stack down, and merge the main-targeting PR last.
Every diff is limited to 500 added/deleted lines. Before each commit/push run
`npm test`, `npm run lint`, `npm run format:check` and `npm run build`.

## Design and API

Figma file `Fer6NQRC3RZCPHPhqNF8QM`, frame `27:1935` (07 · Обмін — деталі):
light `#F7F8FC` canvas, white cards, purple primary actions, numbered lesson plan,
partner and progress cards. Additional states reuse the application tokens and dialogs.

The existing Go API is authoritative for time, membership, blocks, progress and eligibility.
The client uses cookie sessions and CSRF through `/api/v1/*`; it does not infer completion
from a local click or increment progress before both confirmations are persisted.

| Action                               | Endpoint                            |
| ------------------------------------ | ----------------------------------- |
| Confirm a finished lesson            | `POST /sessions/{id}/confirm`       |
| Confirm final completion             | `POST /exchanges/{id}/complete`     |
| Cancel with a reason                 | `POST /exchanges/{id}/cancel`       |
| Read status history                  | `GET /exchanges/{id}`               |
| Active/completed/cancelled exchanges | `GET /me/exchanges?status=…&page=…` |

## Behaviour and verification

The lesson page shows each participant's persisted confirmation separately. The button
appears only when `canConfirmCompletion` is true; after the agreed end time use
"Оновити дані" to refresh eligibility. One confirmation keeps the lesson scheduled and
does not increase progress. Both confirmations count the lesson in its teaching direction.

The exchange page shows progress in both directions, lessons awaiting confirmation,
and the final confirmation panel. `planCompleted` and `canConfirmCompletion` come from
the server. One final confirmation leaves the exchange active; both complete it.
The list keeps active, completed and cancelled filters and pagination in the URL.

Cancellation requires 1–500 Unicode characters. The dialog explains its consequences,
preserves the reason after a failed request, and prevents closing/submitting while saving.
The server cancels unfinished lessons while retaining completed lessons, chat, terms and
status history. The history shows the actor, date and reason. Terminal exchanges have no
completion/cancellation actions. Conflicts reload data; an expired session offers login
with the current page and return path retained.

Before the final push: 58 unit tests, ESLint, Prettier, production build and 16 Playwright
tests (12 FR-11 cases plus four sprint 7 regressions). Browser cases cover both confirmation
orders, future lessons, incomplete plans, pending buttons, persisted progress, conflicts,
session expiry, invalid cancellation, network retry, retained history and archive navigation.
Visual checks at 360, 768 and 1440 px include lesson panels and cancellation dialogs;
screenshots are saved to the ignored `test-results/` directory.

Run `npm run build`, then `npm run test:browser`. Install Chromium once with
`npx playwright install chromium`, or set `$env:PLAYWRIGHT_CHANNEL="chrome"` to use installed
Chrome. The browser suite uses isolated API fixtures and does not write to PostgreSQL.
`go test ./...` passes; database integration tests require `TEST_DATABASE_URL`, which was
not configured. Live acceptance with two accounts against PostgreSQL remains an
environment-level check; frontend behaviour is verified against the existing API contract.

## Restoring the merged stack

The original PRs #48-#51 were merged into their preceding branches after PR #47
was already merged into main, so those changes did not reach main. The restoration
stack starts from the current main and reapplies the original changes in four PRs.
Merge in this order: restore-tests, restore-cancellation, restore-completion,
restore-sessions. Keep the base branches until all dependent PRs are merged.
