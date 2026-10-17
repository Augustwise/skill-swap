# Sprint 3: profile and skills (FR-02–FR-03)

Laboratory 1, section 7.2 assigns profile editing and separate teaching/learning
lists to sprint 3. This stack implements the signed-in user's own profile.

| Branch                             | Scope                                   | Base                               |
| ---------------------------------- | --------------------------------------- | ---------------------------------- |
| `codex/sprint3-profile-foundation` | API, validation, session loading, tests | `main`                             |
| `codex/sprint3-profile-page`       | Figma profile page and dashboard links  | `codex/sprint3-profile-foundation` |
| `codex/sprint3-profile-editing`    | Personal details and biography editing  | `codex/sprint3-profile-page`       |
| `codex/sprint3-profile-skills`     | Teaching and learning skill management  | `codex/sprint3-profile-editing`    |
| `codex/sprint3-profile-completion` | Lesson formats and acceptance checks    | `codex/sprint3-profile-skills`     |

Review in table order. Each PR compares against its parent and contains at most
500 added plus deleted lines. Retarget the next PR after merging its parent.

Design: Figma `Fer6NQRC3RZCPHPhqNF8QM`, own profile `48:2481`; skills `29:2191`.
Registration and onboarding persist through the Go API; the profile reads that
same data. No profile data or credentials are stored in browser storage.

The current API fixes the university at registration; it does not accept a
university change in `PATCH /me/profile`. The UI explains this read-only field.
University reassignment needs a separate backend contract change before all of
FR-02 can be claimed complete. Photos, nickname, birthday, phone, portfolio,
availability schedules and security settings from Figma are outside FR-02–FR-03.

Before every push: `npm test`, `npm run lint`, `npm run format:check`,
`npm run build` in `frontend/`, and `go test ./...` in `backend/`.
Database integration tests require a prepared local `TEST_DATABASE_URL`.

## Profile acceptance

- Register, verify the email in Mailpit and finish onboarding. Open `/profile`
  from the dashboard: names, university, faculty, course, city, bio and both skill
  lists must match the saved data. Reload and sign in again to verify persistence.
- Change personal fields, save, reload, then cancel another edit. Invalid names,
  a description over 600 Unicode characters and offline lessons without a city
  must show field explanations. At least one lesson format is required.
- Add a catalog skill to each list, change its level and remove it. A duplicate
  is unavailable in the same list; the same skill may appear in both lists.
- Test catalog failure/retry, an empty search, failed writes and an expired
  session. Drafts survive errors and section switches; duplicate submits are
  blocked. CSRF rejection refreshes the session once before retrying the write.
- Check 390px mobile and 1440px desktop layouts, keyboard focus, form labels,
  status announcements and the warning before leaving an unsaved form.

Automated coverage: `npm test` exercises validation, Unicode boundaries, CSRF,
request payloads, endpoint routing and failed writes. Headless Edge acceptance
checks use API fixtures for editing, skill CRUD, formats, recovery and responsive
layouts. They do not replace a live PostgreSQL/Mailpit registration-to-login run.
Review screenshots in `public/review/` contain fixture data only.

The final stack also passed a browser run against the real Go HTTP handlers,
using the existing `datatest.Memory` store and captured test email: registration,
verification, onboarding, profile editing, skill CRUD, logout and fresh login.
That run found and fixed the login destination for `/profile`. PostgreSQL and
Mailpit delivery remain environment-dependent checks; no configured database
was modified for these acceptance runs.

# Перевірка переходів до профілю

Після онбордингу відкривається `/main`. Кнопка «Заповнити профіль» і пункт
«Мій профіль» ведуть на `/profile`. Кнопка біля імені відкриває меню акаунта
на головній та у профілі. Меню закривається повторним натисканням, кліком поза ним,
клавішею Escape або виходом фокуса; пункти доступні клавішею Tab.

Для локальної перевірки потрібна гілка з усіма частинами профілю. Злиття stacked PR
у його батьківську гілку саме по собі не переносить зміни до `main`.
