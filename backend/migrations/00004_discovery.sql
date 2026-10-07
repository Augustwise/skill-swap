-- +goose Up
-- Sprint 4 (FR-04, FR-05, NFR-01): indexes for student search and mutual matches.
-- Already in 00001 and reused as is: (skill_id, level, is_active) on user_teaching_skills,
-- (skill_id, current_level, is_active) on user_learning_skills, UNIQUE (user_id, skill_id)
-- in both skill lists, (format, user_id) on user_lesson_formats and the user_blocks key.

-- FR-04: search matches a part of a skill name or a person's name ("photo" finds
-- "Photoshop"). A trigram index lets ILIKE '%...%' use an index instead of a full scan.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_skills_name_trgm ON skills USING gin (name gin_trgm_ops);

-- The search query must use the same expression, otherwise this index is not used.
CREATE INDEX idx_users_full_name_trgm ON users
    USING gin ((first_name || ' ' || last_name) gin_trgm_ops);

-- LR1 3.2: a blocked profile is hidden in both directions. The primary key covers
-- "whom did I block"; this index covers "who blocked me".
CREATE INDEX idx_user_blocks_blocked_id_blocker_id ON user_blocks (blocked_id, blocker_id);

-- +goose Down
DROP INDEX IF EXISTS idx_user_blocks_blocked_id_blocker_id;
DROP INDEX IF EXISTS idx_users_full_name_trgm;
DROP INDEX IF EXISTS idx_skills_name_trgm;
-- Fails while other objects still use pg_trgm, which keeps them working.
DROP EXTENSION IF EXISTS pg_trgm;
