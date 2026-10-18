# Sprint 5: exchange requests and the started exchange (FR-06–FR-07)

Laboratory 1, section 7.2 assigns creating, accepting, declining and withdrawing exchange
requests, and creating the exchange, to sprint 5. The Go endpoints were merged earlier
(`feat/sprint5-*`); this branch, `feat/sprint5-frontend-requests`, adds the web client
for them.

## Pages

| Route                     | Purpose                                                                  | API                                                           |
| ------------------------- | ------------------------------------------------------------------------ | ------------------------------------------------------------- |
| `/students/[studentId]`   | "Запропонувати обмін" dialog: what you give ⇄ what you get, format, plan | `POST /exchange-requests`                                     |
| `/requests`               | Incoming and sent requests, status filter, accept / decline / withdraw   | `GET /me/exchange-requests`, decisions                        |
| `/requests/[requestId]`   | Request terms, both levels, message, decision and status history         | `GET /exchange-requests/{id}`, decisions                      |
| `/exchanges/[exchangeId]` | The started exchange: agreed terms (snapshot), progress, next lesson     | `GET /exchanges/{id}`                                         |
| `/main` (updated)         | "Нові запити" with accept / decline, "Активні обміни" and their count    | `GET /me/exchange-requests`, `GET /me/exchanges`              |
| Sidebar and account menu  | "Запити на обмін" link with the number of incoming requests waiting      | `GET /me/exchange-requests?direction=incoming&status=PENDING` |

All pages need a session with a verified email. The direction, status filter and page
of `/requests` live in the URL (`/requests?direction=outgoing&status=PENDING&page=2`),
so a list can be bookmarked and restored with the Back button. Results come 20 per page.

## Architecture

The client follows the Laboratory 2 layers: pages call only the Go HTTP API through the
`/api/v1/*` rewrite. ExchangeService rules (who may answer, which skills fit, equal total
time, duplicates, idempotent acceptance) stay on the server; the client only explains
them earlier and renders the DTOs.

| Module                             | Responsibility                                                                    |
| ---------------------------------- | --------------------------------------------------------------------------------- |
| `ExchangeApi.ts`                   | DTO types, URL filters, proposal options and validation, requests, error messages |
| `ExchangeFormat.ts`                | Status labels and tones, durations ("1 год 30 хв"), dates and relative time       |
| `ExchangeProposal.tsx`             | The proposal dialog and its success state                                         |
| `ExchangeRequests.tsx`             | `/requests`: tabs with counters, status filter, request cards, pagination         |
| `RequestOverview.tsx`              | `/requests/[requestId]`: terms, message, decision card and status history         |
| `ExchangeOverview.tsx`             | `/exchanges/[exchangeId]`: agreed terms, progress and the next scheduled lesson   |
| `RequestActions.tsx`               | Accept / decline / withdraw buttons, confirmation dialog, conflict handling       |
| `ExchangeCards.tsx`                | `StatusBadge`, skill tags, both directions of an exchange, partner heading, card  |
| `DashboardActivity.tsx`            | Dashboard panels with real requests and active exchanges                          |
| `AppDialog.tsx`                    | Shared native modal dialog; `DashboardModal` now builds on it                     |
| `AppShell.tsx` (updated)           | Requests link and counter; `useRequestsChanged` refreshes it after an action      |
| `Skeleton.tsx`, `AppNavigation.ts` | Loading skeletons for signed-in pages and onboarding; shared titles and nav       |

The acceptance follows the sequence diagram from Laboratory 2 (figure 3): the client
sends `POST /exchange-requests/{id}/accept` with the session cookie and CSRF token; on
200 it opens the exchange (`/exchanges/{exchangeId}?started=1`), where the student sees
the exchange card. The alternatives map to these client states:

| Server answer                      | Client behaviour                                                             |
| ---------------------------------- | ---------------------------------------------------------------------------- |
| 401                                | "Сесія завершилася" with a login link that returns to the same page          |
| 403 `csrf_invalid`                 | The token is refreshed from `/auth/me` and the action is retried once        |
| 403 `action_not_allowed`, 404, 409 | The message is shown and the request is reloaded to show its real state      |
| 409 `request_outdated`             | Explains that a student or skill is no longer available; author can withdraw |
| Repeated click, lost response      | Buttons are disabled while sending; a retry returns the same exchange (200)  |
| 503, no connection                 | The message stays next to the buttons; the user can try again                |

## Design

The Figma file `Fer6NQRC3RZCPHPhqNF8QM` has only the overview and component pages; the
"05 · Запити на обмін" and "12 · Модалка обміну" screens from the overview map are not in
it. The screens were designed in code from the library components and the existing app
style:

| Figma component        | Use                                                                       |
| ---------------------- | ------------------------------------------------------------------------- |
| `StatusBadge` (`6:36`) | Request and exchange statuses; "Відкликано" uses a neutral grey dot       |
| `Tab` (`8:64`)         | "Вхідні" / "Надіслані" with the number of requests waiting for an answer  |
| `AppSidebar` (`11:44`) | The counter badge on "Запити на обмін"                                    |
| `Button` (`5:50`)      | Primary, secondary and a red variant for decline / withdraw confirmations |
| `SkillTag` (`6:20`)    | Pink "Teach" tag for what you teach, blue "Learn" tag for what you learn  |
| `Input` (`7:22`)       | Selects and the message field of the proposal dialog                      |
| Icons                  | `handshake` for the started exchange, `swap` between the two directions   |

The status dots and the handshake icon are exported to `public/figma/requests/`.

## Decisions

- **A request needs a mutual match.** The server requires your skill in their "I learn"
  list, their skill in your "I learn" list and a common format, so the dialog offers only
  those options (from the match explanation of the candidate profile). Otherwise the
  profile explains why an exchange cannot be proposed.
- **Equal total time.** "Однакові умови в обидва боки" is on by default and copies your
  lessons to the other direction. When it is off, the summary shows both totals and the
  request is not sent until they match (`teachSessions × teachDurationMinutes =
learnSessions × learnDurationMinutes`). Durations go in 15-minute steps, 15–240 min,
  1–20 lessons each way.
- **Confirmation for final answers.** Declining and withdrawing ask for confirmation;
  accepting does not, because it is what the requester proposed and it is idempotent.
- **Changing terms.** A pending request is not edited: the author withdraws it and sends
  a new one, as Laboratory 1, section 3.3 describes.
- **The snapshot is shown as agreed.** The exchange page shows the skill names and levels
  copied on acceptance, not the current profile values.
- **Skeleton loading.** The former full-screen "Завантажуємо …" card is replaced on every
  signed-in page and in onboarding by a skeleton with the real sidebar, title and a
  content outline; lists, panels, detail pages and skill catalogs load the same way.
- **No invented numbers.** The dashboard shows the real number of active exchanges; the
  other counters stay "—" until their features exist (FR-23).
- **Out of scope.** Chat and notifications (FR-08, FR-13) and lesson planning (FR-09–FR-10)
  have backend endpoints but their pages belong to the next sprints; the exchange page
  says so. "Активні обміни", "Повідомлення" and "Календар" in the sidebar still open the
  "Незабаром" dialog.

## Checks

Sign in with the demo password from `backend/README.md` after `go run ./cmd/db seed`. The
seed does not reset answered requests, so on a shared database create new requests
instead of answering Taras's demo request.

| Case                                                                | Expected                                                               |
| ------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `demo.olha`: `/requests`                                            | Taras's pending request, "Вхідні 1" and the sidebar counter 1          |
| Olha → Andrii's profile → "Запропонувати обмін", no format          | "Обери спільний формат занять." and focus on the format                |
| Same dialog, mirror off, Andrii 3 × 1 год                           | "Ти навчаєш 2 год, а Андрій — 3 год. Сумарний час має збігатися."      |
| Andrii 4 × 30 хв, online, send                                      | "Запит надіслано", the request in "Надіслані"                          |
| Send the same pair again                                            | "Запит на цю пару навичок уже очікує відповіді." with links            |
| "Відкликати" → confirm                                              | "Запит відкликано.", status "Відкликано", history has two entries      |
| `demo.andrii`: dashboard "Нові запити" → "Відхилити" → confirm      | "Запит відхилено.", the panel is empty                                 |
| A new request → recipient opens it → "Прийняти"                     | `/exchanges/{id}?started=1`: "Обмін розпочато!", terms, 0 of 4 lessons |
| The accepted request page                                           | "Прийнято", "Перейти до обміну", history "Запит прийнято…"             |
| Dashboard after acceptance                                          | "Активні обміни" 1, the exchange row with "0 з 4 заняття"              |
| `/requests/00000000-0000-0000-0000-000000000000`, `/exchanges/nope` | "Запит не знайдено." / "Обмін не знайдено."                            |
| Widths 360, 768, 1440 px                                            | No horizontal scrolling; keyboard focus is visible; dialogs scroll     |

Before every push, from `frontend/`: `npm test`, `npm run lint`, `npm run format:check`
and `npm run build`. `npm test` covers the URL filters, both sides of a request, the
available actions, proposal validation and equal time, Ukrainian error messages, request
bodies and paths, the CSRF retry, status labels, durations, relative dates and return paths.
