# Sprint 9: reviews and ratings

LR1 section 7.2 assigns FR-12 to sprint 9 (7–13 November 2026). This delivery
implements the frontend against the existing Go API; no database migration is needed.

## Delivered behaviour

- A completed exchange offers **Залишити відгук** when the server's `canReview`
  allows it. Active, cancelled and inaccessible exchanges never offer this form.
- Each participant rates their partner once, from 1 to 5. A comment is optional
  and limited to 1000 Unicode code points after trimming, matching server validation.
  The recipient is determined by the server, never supplied by the form.
- The dialog supports keyboard star selection, field errors, a character counter,
  focus restoration and a disabled state during submission. It explains that a
  submitted review cannot be edited. Failed writes preserve the draft; duplicate
  conflicts reload the stored review instead of creating another one.
- The exchange displays each participant's review separately and supports refresh.
  A successful POST is shown immediately even if the following read fails.
- Student profiles show the aggregate rating to one decimal place and the total
  review count. Received reviews have server pagination, 20 per page, newest first.
  An empty rating displays **Ще немає відгуків**.
- The signed-in profile has a **Відгуки про мене** tab and a rating card.
  Tabs use `?tab=…`; review pagination uses `?reviewsPage=…`. Links retain the
  candidate profile's `from` return path, and login links retain the current page.
- Completed exchange lists lead to writing or reading reviews. `REVIEW_RECEIVED`
  notifications use the existing mark-read flow, show a star and open the exchange.
- Comments render as plain React text, including strings containing HTML.

## Design

Reference: Figma file `Fer6NQRC3RZCPHPhqNF8QM`, Screens page, student profile
frame `22:1303`. The implementation reuses the application's light `#F7F8FC`
canvas, white cards, purple actions, existing avatars and the Figma star asset
(`public/figma/main/icon-star.svg`, gold `#F5A524`). The rating dialog and extra
review states follow these existing components and tokens.

## API

| Purpose                                 | Endpoint                            |
| --------------------------------------- | ----------------------------------- |
| Read both reviews and eligibility       | `GET /exchanges/{id}/reviews`       |
| Create a review of the partner          | `POST /exchanges/{id}/reviews`      |
| Read another student's received reviews | `GET /students/{id}/reviews?page=…` |
| Read your received reviews              | `GET /me/reviews?page=…`            |

`ReviewApi.ts` uses `authRequest` / `writeRequest`: same-origin cookies,
15-second timeout, CSRF and one stale-token refresh. The server enforces
membership, completed status, self-review prevention, uniqueness and rating
aggregation. The client never fabricates a rating or increments a count locally.

## Verification

Run from `frontend/`:

```sh
npm test
npm run lint
npm run format:check
npm run build
npm run test:browser
```

Install the browser once with `npx playwright install chromium`.

Unit coverage includes integer ratings, optional text, Unicode length and control
characters, safe pagination URLs, API paths, session/CSRF behaviour, duplicate
conflicts, translated errors and review notification targets. Browser coverage
includes persistence, keyboard controls, pending protection, retained drafts,
server validation, expired sessions, forbidden exchange states, duplicate writes,
plain-text HTML rendering, profile tabs, pagination and retry.

Responsive checks and screenshots cover 360, 768, 1440 and 1920 px. Screenshots
are generated under the ignored `frontend/test-results/` directory. Existing
sprint 7–8 browser scenarios run with the new tests.

The delivery checks comprise 66 unit tests and 37 browser tests (21 review cases
and 16 regressions), plus ESLint, Prettier and the production TypeScript build.

`go test ./...` also verifies the backend review restrictions and the in-memory
MVP scenario. Browser tests use isolated API fixtures and do not modify PostgreSQL.
Database integration tests require a separate local `TEST_DATABASE_URL`; without
it those tests skip. A live two-account PostgreSQL demonstration and review by
another team member remain acceptance steps outside this automated frontend run.
