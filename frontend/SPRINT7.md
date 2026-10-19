# Sprint 7: lessons, time agreement, exchanges and schedule (FR-09–FR-10)

LR1 section 7.2 assigns FR-09–FR-10 to sprint 7 (24–30 October 2026).
Lesson completion and exchange completion/cancellation are sprint 8 (FR-11).
The frontend uses the existing Go API through `/api/v1/*`, with cookie sessions and CSRF.

## Ten parts, five stacked pull requests

| PR / branch (`codex/sprint7-…`) | Parts                                          | Deliverable                                                        |
| ------------------------------- | ---------------------------------------------- | ------------------------------------------------------------------ |
| 1 `session-api`                 | 1. API contracts; 2. validation and time zones | Typed lessons, proposals, errors, safe links, validation and tests |
| 2 `session-planning`            | 3. lesson planning; 4. time agreement          | Lesson page, planning form, accept/decline                         |
| 3 `session-changes`             | 5. rescheduling; 6. cancellation/history       | New agreement, cancellation reason, proposal history               |
| 4 `active-exchanges`            | 7. active exchanges; 8. exchange lessons       | Paginated exchange list, lesson plan, nearest meeting              |
| 5 `schedule`                    | 9. chronological schedule; 10. integration/QA  | Filters, chat and notifications, responsive verification           |

Each branch starts from the preceding branch; each PR targets its predecessor
(PR 1 targets `main`). Review and merge in order. Each diff is limited to 500 changed lines.
Run `npm test`, `npm run lint`, `npm run format:check`, and `npm run build` before each push.

## Design

Figma file `Fer6NQRC3RZCPHPhqNF8QM`, screen `27:1935` (07 · Обмін — деталі):
light `#F7F8FC` canvas, white cards, purple primary buttons, numbered lesson plan,
partner/conditions/progress cards, status labels and meeting actions. Reuse existing
application components, typography and tokens. New schedule screens follow this system.

## Time and meeting links

Planning inputs display the browser's named time zone. Send ISO timestamps in UTC,
including `Z`; never send a zone-less datetime to the API. Reject dates normalized by
JavaScript and clock times skipped during daylight saving changes. The schedule displays
the time zone returned by the API. Meeting links accept only HTTP(S) and open separately.
The server remains authoritative for membership, blocked users, conflicts and status changes.

## Lesson changes

`/sessions/[sessionId]` supports new time proposals for proposed/scheduled lessons,
accept/decline, cancellation with a reason, and the full proposal history. A new proposal
requires the partner's agreement again; cancelled lessons retain their history.
Draft planning requires an active exchange. Select either participant as the teacher;
the server swaps a free draft slot to preserve the agreed plan in both directions.
The teacher stays fixed after the first proposal.
Before a decision, reload the proposal and reject a replaced proposal in the client;
the existing API does not offer an atomic proposal-version precondition.

## Exchanges

`/exchanges?page=2` lists only active exchanges through `GET /me/exchanges?status=ACTIVE`.
Cards reuse the dashboard's partner, skill directions and server progress. Each links to
the complete conditions and lesson plan; the original request remains available there.
`GET /exchanges/{id}/sessions` supplies every slot, including drafts and cancelled lessons.
Select a numbered lesson to propose/agree/change its time. The nearest meeting links to
its lesson and opens only a validated HTTP(S) meeting URL. Empty, missing, loading and
connection-error states use shared application components.

## Schedule and integration

`/schedule` uses `GET /me/schedule` in server chronological order, with status, upcoming
and page filters in the URL. Completed/cancelled tabs include past dates by default.
The schedule uses the API's named time zone; lesson details and planning show the browser
time zone. The sidebar calendar, account menu and dashboard link to the schedule.
The dashboard loads its next three agreed lessons from the same API. Session notifications
open the exact lesson; an active exchange chat links to its lesson plan.

## Verification

Before every push: frontend unit tests, ESLint, Prettier and production build.
Final frontend suite: 50 tests. `go test ./...` also passes. PostgreSQL integration tests
require `TEST_DATABASE_URL`; it was not configured for this run.

Browser checks use isolated API fixtures, without modifying the user's database:

| Scenario                          | Expected result                                                       |
| --------------------------------- | --------------------------------------------------------------------- |
| Empty times or unsafe link        | Validation prevents the POST                                          |
| Plan → partner accepts            | Lesson becomes scheduled with both time confirmations                 |
| Reschedule → partner declines     | A new agreement is required; proposal history is retained             |
| Cancel with reason                | Cancelled state and reason persist; server progress remains unchanged |
| Empty, missing, failed API, retry | Explanations, recovery and correct navigation                         |
| Widths 360, 768 and 1440 px       | No horizontal overflow; usable forms and lesson cards                 |

Visual QA follows Figma's exchange plan and existing application tokens. Live two-account
acceptance against a prepared PostgreSQL database remains an integration-environment check.
