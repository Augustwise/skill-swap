-- +goose Up
-- Sprint 10 reuses user_blocks, reports, moderation_actions, users.account_status, users.deleted_at
-- and user_profile_settings from 00001. Blocks already have PRIMARY KEY (blocker_id, blocked_id)
-- and blocker_id <> blocked_id; reports already forbid reporting oneself.

-- A report has a reason of 1 to 1000 characters and resolved_at exactly when it is decided.
ALTER TABLE reports ADD CONSTRAINT ck_reports_reason CHECK (
    btrim(reason) <> '' AND char_length(reason) <= 1000
);

ALTER TABLE reports ADD CONSTRAINT ck_reports_resolved CHECK (
    (status = 'OPEN') = (resolved_at IS NULL)
);

-- One open report per reporter and reported user; a new one is possible after the decision.
CREATE UNIQUE INDEX idx_reports_open_pair ON reports (reporter_id, reported_user_id)
    WHERE status = 'OPEN';

CREATE INDEX idx_reports_reporter_id_created_at ON reports (reporter_id, created_at);

-- The moderation log also records catalog changes, which have a skill instead of a user.
ALTER TABLE moderation_actions ALTER COLUMN subject_user_id DROP NOT NULL;
ALTER TABLE moderation_actions ADD COLUMN skill_id uuid;
ALTER TABLE moderation_actions ADD CONSTRAINT fk_moderation_actions_skill_id
    FOREIGN KEY (skill_id) REFERENCES skills (id);

ALTER TABLE moderation_actions ADD CONSTRAINT ck_moderation_actions_subject CHECK (
    subject_user_id IS NOT NULL OR skill_id IS NOT NULL
);

ALTER TABLE moderation_actions ADD CONSTRAINT ck_moderation_actions_action CHECK (
    action IN ('DISMISS_REPORT', 'RESOLVE_REPORT', 'SUSPEND_ACCOUNT', 'RESTORE_ACCOUNT', 'CREATE_SKILL', 'UPDATE_SKILL')
);

ALTER TABLE moderation_actions ADD CONSTRAINT ck_moderation_actions_reason CHECK (
    btrim(reason) <> '' AND char_length(reason) <= 1000
);

CREATE INDEX idx_moderation_actions_created_at ON moderation_actions (created_at);

-- Catalog entries edited by an administrator keep a readable name and a URL-safe slug.
ALTER TABLE skills ADD CONSTRAINT ck_skills_name CHECK (btrim(name) <> '');

ALTER TABLE skills ADD CONSTRAINT ck_skills_slug CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$');

-- +goose Down
ALTER TABLE skills DROP CONSTRAINT IF EXISTS ck_skills_slug;
ALTER TABLE skills DROP CONSTRAINT IF EXISTS ck_skills_name;
DROP INDEX IF EXISTS idx_moderation_actions_created_at;
ALTER TABLE moderation_actions DROP CONSTRAINT IF EXISTS ck_moderation_actions_reason;
ALTER TABLE moderation_actions DROP CONSTRAINT IF EXISTS ck_moderation_actions_action;
ALTER TABLE moderation_actions DROP CONSTRAINT IF EXISTS ck_moderation_actions_subject;
ALTER TABLE moderation_actions DROP CONSTRAINT IF EXISTS fk_moderation_actions_skill_id;
ALTER TABLE moderation_actions DROP COLUMN IF EXISTS skill_id;
-- Fails while catalog actions without a user exist; delete them first.
ALTER TABLE moderation_actions ALTER COLUMN subject_user_id SET NOT NULL;
DROP INDEX IF EXISTS idx_reports_reporter_id_created_at;
DROP INDEX IF EXISTS idx_reports_open_pair;
ALTER TABLE reports DROP CONSTRAINT IF EXISTS ck_reports_resolved;
ALTER TABLE reports DROP CONSTRAINT IF EXISTS ck_reports_reason;
