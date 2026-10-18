# Sprint 4: search, mutual matches and candidate profile (FR-04–FR-05)

Laboratory 1, section 7.2 assigns student search with filters, mutual matches and the
candidate profile to sprint 4. The Go endpoints were merged earlier (PR #10); this branch,
`feat/sprint4-frontend-discovery`, adds the web client for them.

## Pages

| Route                   | Purpose                                                                 | API                  |
| ----------------------- | ----------------------------------------------------------------------- | -------------------- |
| `/search`               | Search by an offered skill or a name; category, level, format, mutual   | `GET /students`      |
| `/matches`              | Mutual matches with the reason: who teaches what, and the common format | `GET /me/matches`    |
| `/students/[studentId]` | Candidate profile: skills with levels, bio, formats, rating and reviews | `GET /students/{id}` |
| `/main` (updated)       | First two mutual matches and a global search box in the top bar         | `GET /me/matches`    |

All pages need a session with a verified email. Results come 20 per page; the page
number and every filter live in the URL (`/search?q=Photoshop&level=ADVANCED&page=2`),
so a result list can be bookmarked, shared, refreshed and restored with the Back button.

## Architecture

The client follows the Laboratory 2 layers: pages call only the Go HTTP API through the
`/api/v1/*` rewrite and never reach the database. MatchingService rules stay on the
server; the client renders its DTOs.

| Module                                     | Responsibility                                                               |
| ------------------------------------------ | ---------------------------------------------------------------------------- |
| `DiscoveryApi.ts`                          | DTO types, URL filter parsing and serialization, requests, error mapping     |
| `DiscoveryFormat.ts`                       | Ukrainian labels, plural forms, rating with one decimal, dates, initials     |
| `AppRoutes.ts`                             | Allow-list for `?next=` and `?from=` return paths, profile links             |
| `useApiQuery.ts`                           | Loads data per key; a new key aborts the previous request                    |
| `AppGate.tsx`, `AppMessage.tsx`            | Session and verified-email gate with loading, login and retry states         |
| `AppShell.tsx`                             | One sidebar, top bar search, account menu and logout for all signed-in pages |
| `StudentSearch.tsx`, `SearchFilters.tsx`   | Search page and filter form                                                  |
| `MutualMatches.tsx`                        | Matches page, eligibility hint                                               |
| `CandidateProfile.tsx`                     | Candidate profile and the match explanation                                  |
| `DiscoveryCards.tsx`, `DiscoveryError.tsx` | Cards, skill chips, pagination, empty and error states                       |

`Dashboard` and `ProfileShell` now use `AppShell`, so the navigation is defined once.
On phones the sidebar is hidden; the account menu links to search and matches.

## Decisions

- **No compatibility percentage.** LR1 3.2 forbids a score in the MVP, so the former
  "94% збіг" design card was removed. A mutual match shows the badge "Взаємний збіг",
  the skills in both directions and the common format.
- **Server is the source of truth.** Mutual matches and the `mutual` flag come from the
  API. For a one-sided candidate the profile also explains what is missing (no skill
  you want, no skill you teach, no common format); it compares catalog IDs and the city
  exactly like the server, but does not decide the match.
- **Filters apply together.** Selects and the checkbox apply at once; the text query
  applies on "Знайти" or Enter. Invalid values in a hand-edited URL are dropped, and
  the text is trimmed to 100 characters like on the server.
- **Safe navigation.** The candidate profile returns to the list it was opened from
  (`?from=`), and login returns to the requested page (`?next=`). Both accept only known
  in-app paths, so a crafted link cannot redirect outside the site.
- **States.** Every list has loading, empty, error-with-retry, expired-session and
  "page past the end" states. A hidden, blocked or missing student shows "Профіль
  недоступний" (the API returns 404 without saying why).
- **Out of scope.** "Запропонувати обмін" opens the "Незабаром" dialog: sending
  exchange requests (FR-06) is the next frontend step. Reviews are listed when they
  exist; creating them belongs to FR-12.

## Checks

Sign in as `demo.olha@students.example.test` with the demo password from
`backend/README.md` after `go run ./cmd/db seed`:

| Case                                               | Expected                                                     |
| -------------------------------------------------- | ------------------------------------------------------------ |
| `/search?q=Photoshop`                              | Andrii and Taras (mutual, first), Kateryna, Marko            |
| `q=Photoshop`, level "Просунутий", format "Онлайн" | Andrii and Kateryna only                                     |
| "Лише взаємні збіги"                               | Andrii and Taras                                             |
| A query with no results                            | "За запитом «…» нікого не знайдено." and a reset link        |
| `/matches`                                         | Andrii and Taras with "Навчить тебе" / "Хоче вивчити у тебе" |
| Marko's profile                                    | "Чому це не взаємний збіг" with the missing direction        |
| Nataliia's profile (`…0008`, hidden)               | "Профіль недоступний"                                        |
| Widths 360, 768, 1440 px                           | No horizontal scrolling; keyboard focus is visible           |

Before every push, from `frontend/`: `npm test`, `npm run lint`, `npm run format:check`
and `npm run build`. `npm test` covers URL filters, request paths, error mapping,
eligibility, the match explanation, Ukrainian plurals and return-path validation.
