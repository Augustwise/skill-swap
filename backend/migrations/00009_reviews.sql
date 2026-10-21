-- +goose Up
-- Sprint 9 reviews reuse exchange_reviews from 00001: it already has UNIQUE (exchange_id, author_id),
-- a rating from 1 to 5 and author_id <> recipient_id. The service checks that the exchange is
-- COMPLETED and that the recipient is the author's partner.

-- A review text is optional, but when it is stored it is not blank and has at most 1000 characters.
ALTER TABLE exchange_reviews ADD CONSTRAINT ck_exchange_reviews_comment CHECK (
    comment IS NULL OR (btrim(comment) <> '' AND char_length(comment) <= 1000)
);

-- +goose Down
ALTER TABLE exchange_reviews DROP CONSTRAINT IF EXISTS ck_exchange_reviews_comment;
