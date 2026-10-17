-- +goose Up
-- Sprint 6 (FR-08, FR-13, NFR-02, NFR-09): private text chat and in-app notifications.
-- Already in 00001 and reused as is: conversations with UNIQUE (exchange_request_id)
-- and UNIQUE (exchange_id), conversation_participants with PRIMARY KEY (conversation_id, user_id),
-- messages with (conversation_id, created_at), message_receipts and notifications with
-- UNIQUE (recipient_id, event_key) and (recipient_id, read_at, created_at).

-- Every request or exchange conversation must point to its source object.
ALTER TABLE conversations ADD CONSTRAINT ck_conversations_context CHECK (
    (type = 'REQUEST' AND exchange_request_id IS NOT NULL)
    OR (type = 'EXCHANGE' AND exchange_id IS NOT NULL)
    OR type = 'DIRECT'
);

-- FR-08: text messages are plain text from 1 to 2000 characters and always have an author.
ALTER TABLE messages ADD CONSTRAINT ck_messages_text_body CHECK (
    type <> 'TEXT' OR (body IS NOT NULL AND btrim(body) <> '' AND char_length(body) <= 2000)
);

ALTER TABLE messages ADD CONSTRAINT ck_messages_author CHECK (
    type = 'SYSTEM' OR author_id IS NOT NULL
);

-- FR-13, ARCH-08: notifications need a non-empty title and idempotency key.
ALTER TABLE notifications ADD CONSTRAINT ck_notifications_title CHECK (
    btrim(title) <> '' AND btrim(event_key) <> ''
);

-- +goose Down
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS ck_notifications_title;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS ck_messages_author;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS ck_messages_text_body;
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS ck_conversations_context;
