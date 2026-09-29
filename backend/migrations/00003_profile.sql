-- +goose Up
-- Sprint 3 (FR-02, FR-03): profile fields and the "I teach" / "I learn" lists.
-- Already in 00001 and reused as is: users.faculty_id -> faculties, users.academic_year
-- (the course), users.city, users.bio varchar(600) (the limit counts characters),
-- the skill_level enum, and UNIQUE (user_id, skill_id) in both skill lists.

-- FR-02: a student chooses online and/or offline lessons. Offline means meeting in person
-- in the student's city (LR1, 3.2). CAMPUS and PARTNER_LOCATION are kept for FR-17.
ALTER TYPE lesson_format ADD VALUE IF NOT EXISTS 'OFFLINE';

-- FR-02: the course is 1-6 (00001 allowed up to 8).
ALTER TABLE users DROP CONSTRAINT ck_users_1;
ALTER TABLE users ADD CONSTRAINT ck_users_academic_year
    CHECK (academic_year IS NULL OR academic_year BETWEEN 1 AND 6);

-- FR-03: goal priorities belong to FR-16 and are not part of the MVP, so a learning
-- skill may have no priority. UNIQUE (user_id, priority) still applies when it is set.
ALTER TABLE user_learning_skills ALTER COLUMN priority DROP NOT NULL;

-- +goose Down
-- Fails while learning skills without a priority exist; give them priorities first.
ALTER TABLE user_learning_skills ALTER COLUMN priority SET NOT NULL;
ALTER TABLE users DROP CONSTRAINT IF EXISTS ck_users_academic_year;
ALTER TABLE users ADD CONSTRAINT ck_users_1
    CHECK (academic_year IS NULL OR academic_year BETWEEN 1 AND 8);
-- PostgreSQL cannot remove an enum value, so 'OFFLINE' stays in lesson_format.
