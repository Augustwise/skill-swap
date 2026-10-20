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

Each PR targets the preceding branch; PR 1 targets `main`. Merge in order.
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
