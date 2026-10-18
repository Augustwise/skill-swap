# Skill Swap web client

Next.js (App Router), React and TypeScript client for Skill Swap. It talks only to the
Go API: `next.config.ts` rewrites `/api/v1/*` to `http://127.0.0.1:8080/api/v1/*`, so
the browser keeps one origin and the HttpOnly session cookie. Read `AGENTS.md` before
changing code: this Next.js version differs from older releases.

## Run

```bash
npm ci
npm run dev
```

Open <http://localhost:3000> (the API's `FRONTEND_ORIGIN` must match this address). Start
the API and Mailpit as described in the root `README.md`.

| Script                 | Purpose                                    |
| ---------------------- | ------------------------------------------ |
| `npm run dev`          | Development server on port 3000            |
| `npm test`             | Unit tests in `tests/` with `node:test`    |
| `npm run lint`         | ESLint                                     |
| `npm run format:check` | Prettier check (`npm run format` fixes it) |
| `npm run build`        | Production build with the TypeScript check |
| `npm start`            | Serves the production build                |

## Routes

| Route                   | Sprint | Page                                               |
| ----------------------- | ------ | -------------------------------------------------- |
| `/`, `/terms`           | —      | Landing page and data-handling terms               |
| `/register`, `/login`   | 2      | Registration, login and password recovery          |
| `/verify-email`         | 2      | Email verification and resend                      |
| `/reset-password`       | 2      | New password from a one-time link                  |
| `/onboarding`           | 3      | First profile, skill lists and lesson formats      |
| `/profile`              | 3      | Own profile, skills and formats                    |
| `/main`                 | 3–4    | Dashboard with recommended mutual matches          |
| `/search`               | 4      | Student search with filters                        |
| `/matches`              | 4      | Mutual matches with explanations                   |
| `/students/[studentId]` | 4      | Candidate profile with skills, formats and reviews |

Sprint notes: [`SPRINT2.md`](SPRINT2.md), [`SPRINT3.md`](SPRINT3.md),
[`SPRINT4.md`](SPRINT4.md).

## Structure

```text
app/
├── <route>/page.tsx      # Server entry: metadata, Suspense, route params
├── components/
│   ├── *Api.ts           # HTTP client modules: DTO types, requests, validation
│   ├── AppShell.tsx      # Shared sidebar, top bar search and account menu
│   ├── AppGate.tsx       # Signed-in, verified-email gate for private pages
│   └── *.tsx             # Page and UI components (PascalCase)
└── styles/               # Global and per-section CSS
public/figma/             # Icons exported from the Figma design
tests/                    # node:test unit tests for the *.ts modules
```

Private pages render `AppGate`, which loads `GET /auth/me` and `GET /me/profile`, sends
unverified users to `/verify-email`, and offers login or retry otherwise. Inside it,
`AppShell` provides the common layout.

## Conventions

- Requests go through `authRequest` in `AuthApi.ts`: same-origin credentials, no cache,
  a 15-second timeout and typed `AuthError` failures. Writes send the CSRF token from
  `/auth/me`; tokens and passwords are never stored in browser storage.
- Business rules stay on the server. Client checks only give earlier, translated
  feedback and mirror the server limits.
- Page state that a user may share or restore (filters, page numbers) lives in the URL.
  Return paths in `?next=` and `?from=` pass through `safeAppPath`.
- The interface is Ukrainian, works from 360 to 1920 px without horizontal scrolling,
  and keeps visible focus, labels and text error messages (NFR-10).
- No design fixtures replace user data: empty lists show an empty state.
