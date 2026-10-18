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
The teacher comes from the selected lesson in the agreed plan. Choose a lesson in the
other direction to plan with the other teacher. Draft planning requires an active exchange.
Before a decision, reload the proposal and reject a replaced proposal in the client;
the existing API does not offer an atomic proposal-version precondition.
