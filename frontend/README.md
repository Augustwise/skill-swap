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

| Route                        | Sprint | Page                                                      |
| ---------------------------- | ------ | --------------------------------------------------------- |
| `/`, `/terms`                | —      | Landing page and data-handling terms                      |
| `/register`, `/login`        | 2      | Registration, login and password recovery                 |
| `/verify-email`              | 2      | Email verification and resend                             |
| `/reset-password`            | 2      | New password from a one-time link                         |
| `/onboarding`                | 3      | First profile, skill lists and lesson formats             |
| `/profile`                   | 3      | Own profile, skills and formats                           |
| `/main`                      | 3–5    | Dashboard: mutual matches, new requests, active exchanges |
| `/search`                    | 4      | Student search with filters                               |
| `/matches`                   | 4      | Mutual matches with explanations                          |
| `/students/[studentId]`      | 4–5    | Candidate profile and the exchange proposal dialog        |
| `/requests`                  | 5      | Incoming and sent exchange requests with decisions        |
| `/requests/[requestId]`      | 5–6    | Request terms, decision, history and the request's chat   |
| `/exchanges/[exchangeId]`    | 5–6    | Started exchange: agreed terms, progress and its chat     |
| `/messages`                  | 6      | Conversations list; chats of requests and exchanges       |
| `/messages/[conversationId]` | 6      | Private chat polled every 5 seconds                       |
| `/notifications`             | 6      | All or unread notifications, mark one or all as read      |

Sprint notes: [`SPRINT2.md`](SPRINT2.md), [`SPRINT3.md`](SPRINT3.md),
[`SPRINT4.md`](SPRINT4.md), [`SPRINT5.md`](SPRINT5.md), [`SPRINT6.md`](SPRINT6.md).

## Structure

```text
app/
├── <route>/page.tsx      # Server entry: metadata, Suspense, route params
├── components/
│   ├── *Api.ts           # HTTP client modules: DTO types, requests, validation
│   ├── AppShell.tsx      # Shared sidebar, top bar, notification bell, navigation counters
│   ├── AppGate.tsx       # Signed-in, verified-email gate for private pages
│   ├── AppDialog.tsx     # Native modal dialog for proposals and confirmations
│   ├── AppNavigation.ts  # Navigation items and page titles shared with skeletons
│   ├── Skeleton.tsx      # Page-shaped loading skeletons
│   └── *.tsx             # Page and UI components (PascalCase)
└── styles/               # Global and per-section CSS
public/figma/             # Icons and status dots exported from the Figma design
tests/                    # node:test unit tests for the *.ts modules
```

Private pages render `AppGate`, which loads `GET /auth/me` and `GET /me/profile`, sends
unverified users to `/verify-email`, and offers login or retry otherwise. Inside it,
`AppShell` provides the common layout. While the session loads, `AppGate` shows
`AppSkeleton`: the real sidebar and page title with a shimmering outline of the page.

## Conventions

- Requests go through `authRequest` in `AuthApi.ts`: same-origin credentials, no cache,
  a 15-second timeout and typed `AuthError` failures. Writes send the CSRF token from
  `/auth/me`; tokens and passwords are never stored in browser storage.
- Business rules stay on the server. Client checks only give earlier, translated
  feedback and mirror the server limits.
- Page state that a user may share or restore (filters, tabs, page numbers) lives in the
  URL. Return paths in `?next=` and `?from=` pass through `safeAppPath`.
- Actions that change state disable their buttons while sending and rely on the server's
  idempotent answers; final decisions (decline, withdraw) ask for confirmation. After a
  conflict the page reloads the object instead of guessing its new state.
- The interface is Ukrainian, works from 360 to 1920 px without horizontal scrolling,
  and keeps visible focus, labels and text error messages (NFR-10).
- No design fixtures replace user data: empty lists show an empty state.
- Loading states are skeletons shaped like the content they replace (cards, panels, the
  detail columns, form fields), never a blank page or a "loading" card. They are hidden
  from screen readers, which hear one "Завантажуємо …" status instead; the shimmer stops
  with `prefers-reduced-motion`.
