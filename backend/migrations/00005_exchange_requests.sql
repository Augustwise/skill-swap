-- +goose Up
-- Sprint 5 (FR-06, FR-07): exchange requests.
-- Already in 00001 and reused as is: exchange_requests with its checks (not to oneself,
-- total = both directions, 15-240 minutes, equal total time), the status history tables,
-- exchanges with UNIQUE (source_request_id) and exchange_skill_commitments with snapshots.

-- A request waits until the recipient answers or the author withdraws it; the MVP has no
-- expiry, so EXPIRED is not used.
ALTER TABLE exchange_requests ALTER COLUMN expires_at DROP NOT NULL;

-- One pending request per pair of skills, in either direction. A user_teaching_skills
-- row belongs to one student, so the two teaching rows identify both students and both
-- skills, and B -> A with the same skills collides with A -> B.
CREATE UNIQUE INDEX idx_exchange_requests_pending_skill_pair ON exchange_requests (
    LEAST(requester_teaching_skill_id, recipient_teaching_skill_id),
    GREATEST(requester_teaching_skill_id, recipient_teaching_skill_id)
) WHERE status = 'PENDING';

-- +goose Down
DROP INDEX IF EXISTS idx_exchange_requests_pending_skill_pair;
-- Fails while requests without expires_at exist; set it for them first.
ALTER TABLE exchange_requests ALTER COLUMN expires_at SET NOT NULL;
