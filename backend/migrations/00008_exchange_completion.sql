-- +goose Up
-- Completion confirmations reuse session_confirmations (kind COMPLETION) and
-- exchange_confirmations from 00001: their primary keys keep one confirmation per participant.

ALTER TABLE sessions ADD CONSTRAINT ck_sessions_completed CHECK (
    status <> 'COMPLETED' OR completed_at IS NOT NULL
);

ALTER TABLE exchanges ADD CONSTRAINT ck_exchanges_completed CHECK (
    status <> 'COMPLETED' OR completed_at IS NOT NULL
);

ALTER TABLE exchanges ADD CONSTRAINT ck_exchanges_cancelled CHECK (
    status <> 'CANCELLED' OR (
        cancelled_at IS NOT NULL
        AND btrim(cancellation_reason) <> ''
        AND char_length(cancellation_reason) <= 500
    )
);

-- +goose Down
ALTER TABLE exchanges DROP CONSTRAINT IF EXISTS ck_exchanges_cancelled;
ALTER TABLE exchanges DROP CONSTRAINT IF EXISTS ck_exchanges_completed;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS ck_sessions_completed;
