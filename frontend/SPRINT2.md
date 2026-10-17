# Sprint 2: account access (FR-01)

Laboratory 1, section 7.2 assigns registration, login, server sessions, email
verification and password recovery through local Mailpit to sprint 2. Profile and
skill editing belong to sprint 3; discovery belongs to sprint 4.

## Review stack

Each branch is based on the previous branch. Review and merge from top to bottom;
retarget the next PR to `main` after its parent merges. Each PR stays below 500
added plus deleted lines, including documentation.

| Branch                             | Scope                                                        | Base                            |
| ---------------------------------- | ------------------------------------------------------------ | ------------------------------- |
| `codex/sprint2-auth-foundation`    | Shared API errors, timeout, validation and safe destinations | `main`                          |
| `codex/sprint2-registration`       | Catalog retry, duplicate email, accessible registration      | `codex/sprint2-auth-foundation` |
| `codex/sprint2-login-recovery`     | Login, session routing and password recovery                 | `codex/sprint2-registration`    |
| `codex/sprint2-password-reset`     | Password validation and invalid/expired reset links          | `codex/sprint2-login-recovery`  |
| `codex/sprint2-email-verification` | Verification, resend, session handling and acceptance checks | `codex/sprint2-password-reset`  |

## Design and integration

- Figma file: `Fer6NQRC3RZCPHPhqNF8QM`, page `2:4`, login `30:2`, registration
  `30:103`. Recovery and verification reuse the same brand panel and form styles.
- Password policy follows Go: at least 12 Unicode code points and at most 72 UTF-8
  bytes. The Figma placeholder saying 8 characters is superseded by the API policy.
- The Go API sets an HttpOnly session cookie. The frontend keeps CSRF tokens in
  memory and never stores passwords or session tokens in browser storage.
- Mailpit exercises local email flows; it does not prove ownership of a real
  university mailbox. Bcrypt, token expiry and session revocation are server rules.

## Checks before each push

Run `npm run lint`, `npm run format:check` and `npm run build` from `frontend/`.
Run `go test ./...` from `backend/` for integration regression coverage. Database
tests skip unless a prepared local `TEST_DATABASE_URL` is configured.

## Acceptance checks completed on October 18, 2026

Browser scenarios used the real Go HTTP/auth/profile services with disposable
in-memory repositories and local Mailpit. Fault injection and forced token expiry
were restricted to the temporary QA process. No application database was changed.

| Scenario                                         | Observed result                                                                        |
| ------------------------------------------------ | -------------------------------------------------------------------------------------- |
| Empty registration                               | Field descriptions explain errors; focus moves to the first invalid field.             |
| Short Unicode password                           | Six emoji are rejected; minimum length counts code points, maximum counts UTF-8 bytes. |
| University API unavailable                       | Submit stays disabled; retry loads the catalog when the API recovers.                  |
| Normalized duplicate email                       | Existing account is rejected and the email field receives focus.                       |
| SMTP unavailable on registration                 | Account creation succeeds; the screen explains how to resend after login.              |
| SMTP available                                   | Verification and recovery messages arrive in Mailpit.                                  |
| Incorrect login                                  | Credentials are rejected without leaving the form.                                     |
| Unverified login                                 | Browser opens verification; server session survives reload.                            |
| Verified login                                   | Browser opens onboarding; `/main` remains an allowlisted destination.                  |
| Logout                                           | Server session is revoked; opening `/main` requires login again.                       |
| Password confirmation mismatch                   | Password reset stays on the form with linked field errors.                             |
| Password reset                                   | New password works, old password fails and the old browser session ends.               |
| Reused reset link                                | Reset is rejected; the action opens `/login?mode=recovery`.                            |
| Missing/malformed reset token                    | No password form is rendered; recovery is offered directly.                            |
| Successful verification then failed session read | Retry preserves the successful result after the API recovers.                          |
| Reused/expired verification link                 | Link is rejected; signed-in users can request a fresh message.                         |
| Repeated verification message                    | Fresh CSRF is fetched; a 30-second button cooldown prevents rapid repeat clicks.       |
| Logout before verification                       | User can leave the unverified account without entering protected pages.                |
| Token-bearing pages                              | Consumed tokens leave the URL; metadata disables indexing and referrer disclosure.     |
| Desktop/mobile                                   | Checked at 1440 × 900 and 390 × 844; registration has no horizontal overflow.          |

All five branch checkpoints pass ESLint, Prettier, a production build and
`go test ./...`. Focused runtime assertions also cover API 204/429 responses,
non-JSON failures, cookie/CSRF options, Unicode limits and redirect allowlisting.
Screenshots are under `docs/sprint2/` at the repository root and linked in the PRs.

PostgreSQL integration tests were skipped because no separate local
`TEST_DATABASE_URL` was configured. Browser scenarios verify UI/API integration;
they do not replace a database-backed persistence test. Server auth tests cover
cookie persistence, token expiry/reuse, bcrypt validation and session revocation.

To repeat the full persistence check with PostgreSQL, start the normal local
stack from the root README, use a new disposable email matching a configured
university domain, complete registration in the browser, open the Mailpit link,
log in, reload, log out, request a reset, and verify that the previous password
and previously issued session no longer work.
