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
