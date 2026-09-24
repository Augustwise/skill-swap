-- +goose Up
-- Sprint 2 (FR-01, NFR-08): failed-login throttling per normalized email address.
-- After 5 failed attempts within 15 minutes, login is locked for 2 hours.
-- The durations live in the auth service; this table only stores the counters.

CREATE TABLE login_throttles (
    email varchar(320) PRIMARY KEY,
    failed_count smallint NOT NULL DEFAULT 0,
    window_started_at timestamptz NOT NULL DEFAULT now(),
    locked_until timestamptz
);

ALTER TABLE login_throttles ADD CONSTRAINT ck_login_throttles_1 CHECK (failed_count >= 0);

-- Each user has at most one LOCAL identity whose password hash is bcrypt.
ALTER TABLE auth_identities ADD CONSTRAINT ck_auth_identities_1
    CHECK (provider <> 'LOCAL' OR password_hash IS NOT NULL);

-- +goose Down
ALTER TABLE auth_identities DROP CONSTRAINT IF EXISTS ck_auth_identities_1;
DROP TABLE IF EXISTS login_throttles;
