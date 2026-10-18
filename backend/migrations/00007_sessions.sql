-- +goose Up
-- Sprint 7 (FR-09, FR-10, NFR-05, NFR-09): lesson scheduling, time agreement, active
-- exchanges, and personal schedule.
-- Already in 00001 and reused as is: sessions with UNIQUE (exchange_id, plan_order) and
-- indexes on (scheduled_start, status), (teacher_id, scheduled_start), and
-- (learner_id, scheduled_start); session_schedule_proposals; session_confirmations with
-- PRIMARY KEY (session_id, user_id, kind); and meeting_places.

ALTER TABLE meeting_places ADD CONSTRAINT ck_meeting_places_name CHECK (
    btrim(name) <> ''
);

ALTER TABLE sessions ADD CONSTRAINT ck_sessions_title CHECK (
    btrim(title) <> ''
);

ALTER TABLE sessions ADD CONSTRAINT ck_sessions_participants CHECK (
    teacher_id IS NOT NULL AND learner_id IS NOT NULL AND teacher_id <> learner_id
);

ALTER TABLE sessions ADD CONSTRAINT ck_sessions_duration CHECK (
    scheduled_start IS NULL OR (
        scheduled_end >= scheduled_start + interval '15 minutes'
        AND scheduled_end <= scheduled_start + interval '240 minutes'
    )
);

ALTER TABLE sessions ADD CONSTRAINT ck_sessions_schedule_state CHECK (
    (status = 'DRAFT' AND scheduled_start IS NULL AND scheduled_end IS NULL AND meeting_place_id IS NULL AND meeting_url IS NULL)
    OR (status IN ('PROPOSED', 'SCHEDULED', 'COMPLETED') AND scheduled_start IS NOT NULL AND scheduled_end IS NOT NULL)
    OR status = 'CANCELLED'
);

ALTER TABLE sessions ADD CONSTRAINT ck_sessions_location CHECK (
    status IN ('DRAFT', 'CANCELLED')
    OR (format = 'ONLINE' AND meeting_url IS NOT NULL AND btrim(meeting_url) <> '' AND meeting_place_id IS NULL)
    OR (format = 'OFFLINE' AND meeting_place_id IS NOT NULL AND meeting_url IS NULL)
);

ALTER TABLE session_schedule_proposals ADD CONSTRAINT ck_session_schedule_proposals_duration CHECK (
    proposed_end >= proposed_start + interval '15 minutes'
    AND proposed_end <= proposed_start + interval '240 minutes'
);

ALTER TABLE session_schedule_proposals ADD CONSTRAINT ck_session_schedule_proposals_location CHECK (
    (format = 'ONLINE' AND meeting_url IS NOT NULL AND btrim(meeting_url) <> '' AND meeting_place_id IS NULL)
    OR (format = 'OFFLINE' AND meeting_place_id IS NOT NULL AND meeting_url IS NULL)
);

-- At most one pending schedule proposal per session at a time.
CREATE UNIQUE INDEX idx_session_schedule_proposals_one_pending
    ON session_schedule_proposals (session_id)
    WHERE status = 'PENDING';

-- +goose Down
DROP INDEX IF EXISTS idx_session_schedule_proposals_one_pending;
ALTER TABLE session_schedule_proposals DROP CONSTRAINT IF EXISTS ck_session_schedule_proposals_location;
ALTER TABLE session_schedule_proposals DROP CONSTRAINT IF EXISTS ck_session_schedule_proposals_duration;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS ck_sessions_location;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS ck_sessions_schedule_state;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS ck_sessions_duration;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS ck_sessions_participants;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS ck_sessions_title;
ALTER TABLE meeting_places DROP CONSTRAINT IF EXISTS ck_meeting_places_name;
