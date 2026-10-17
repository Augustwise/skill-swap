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
