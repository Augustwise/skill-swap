-- +goose Up
-- PostgreSQL 15+. This migration is the executable form of the sprint-1 ER model.

CREATE TYPE auth_provider AS ENUM ('LOCAL', 'UNIVERSITY_SSO');
CREATE TYPE user_role AS ENUM ('STUDENT', 'ADMIN');
CREATE TYPE account_status AS ENUM ('ACTIVE', 'SUSPENDED');
CREATE TYPE one_time_token_purpose AS ENUM ('EMAIL_VERIFICATION', 'PASSWORD_RESET');
CREATE TYPE confirmation_kind AS ENUM ('SCHEDULE', 'COMPLETION');
CREATE TYPE report_status AS ENUM ('OPEN', 'RESOLVED', 'DISMISSED');
CREATE TYPE verification_type AS ENUM ('UNIVERSITY_EMAIL', 'PHONE', 'TELEGRAM', 'PORTFOLIO');
CREATE TYPE verification_status AS ENUM ('PENDING', 'VERIFIED', 'REJECTED', 'EXPIRED');
CREATE TYPE skill_level AS ENUM ('BEGINNER', 'INTERMEDIATE', 'ADVANCED');
CREATE TYPE lesson_format AS ENUM ('ONLINE', 'CAMPUS', 'PARTNER_LOCATION');
CREATE TYPE request_scope AS ENUM ('ANYONE', 'SAME_UNIVERSITY');
CREATE TYPE match_status AS ENUM ('NEW', 'VIEWED', 'CONTACTED', 'ARCHIVED');
CREATE TYPE match_factor_type AS ENUM ('MUTUAL_SKILL', 'SKILL_LEVEL', 'LESSON_FORMAT', 'AVAILABILITY', 'SAME_UNIVERSITY', 'RATING');
CREATE TYPE exchange_request_status AS ENUM ('PENDING', 'COUNTERED', 'ACCEPTED', 'DECLINED', 'WITHDRAWN', 'EXPIRED');
CREATE TYPE exchange_status AS ENUM ('PLANNED', 'ACTIVE', 'PAUSED', 'COMPLETED', 'CANCELLED');
CREATE TYPE session_status AS ENUM ('DRAFT', 'PROPOSED', 'SCHEDULED', 'COMPLETED', 'CANCELLED');
CREATE TYPE schedule_proposal_status AS ENUM ('PENDING', 'ACCEPTED', 'DECLINED', 'SUPERSEDED');
CREATE TYPE conversation_type AS ENUM ('DIRECT', 'REQUEST', 'EXCHANGE');
CREATE TYPE message_type AS ENUM ('TEXT', 'FILE', 'SESSION_PROPOSAL', 'SYSTEM');
CREATE TYPE file_kind AS ENUM ('IMAGE', 'DOCUMENT', 'AUDIO', 'VIDEO', 'OTHER');
CREATE TYPE notification_event_type AS ENUM ('MUTUAL_MATCH', 'EXCHANGE_REQUEST', 'REQUEST_RESPONSE', 'CHAT_MESSAGE', 'SESSION_REMINDER', 'SESSION_UPDATED', 'REVIEW_RECEIVED', 'PRODUCT_NEWS');
CREATE TYPE calendar_provider AS ENUM ('GOOGLE', 'MICROSOFT', 'APPLE', 'ICS');
CREATE TYPE sync_status AS ENUM ('PENDING', 'SYNCED', 'FAILED', 'DELETED');

CREATE TABLE universities (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    name varchar(200) NOT NULL,
    short_name varchar(50),
    email_domain varchar(150) NOT NULL UNIQUE,
    city varchar(120),
    country_code char(2) NOT NULL DEFAULT 'UA',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE faculties (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    university_id uuid NOT NULL,
    name varchar(200) NOT NULL,
    code varchar(50),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    university_id uuid NOT NULL,
    faculty_id uuid,
    email varchar(320) NOT NULL UNIQUE,
    first_name varchar(100) NOT NULL,
    last_name varchar(100) NOT NULL,
    username varchar(50) UNIQUE,
    academic_year smallint,
    birth_date date,
    city varchar(120),
    bio varchar(600),
    avatar_url text,
    phone_e164 varchar(20),
    telegram_handle varchar(100),
    portfolio_url text,
    timezone varchar(64) NOT NULL DEFAULT 'Europe/Kyiv',
    locale varchar(10) NOT NULL DEFAULT 'uk-UA',
    role user_role NOT NULL DEFAULT 'STUDENT',
    account_status account_status NOT NULL DEFAULT 'ACTIVE',
    two_factor_enabled boolean NOT NULL DEFAULT false,
    terms_accepted_at timestamptz,
    last_active_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE TABLE auth_identities (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    provider auth_provider NOT NULL,
    provider_subject varchar(255) NOT NULL,
    password_hash text,
    password_changed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_login_at timestamptz
);

CREATE TABLE user_sessions (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    session_token_hash text NOT NULL UNIQUE,
    device_name varchar(150),
    user_agent text,
    ip_address inet,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);

CREATE TABLE one_time_tokens (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    purpose one_time_token_purpose NOT NULL,
    token_hash text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at timestamptz
);

CREATE TABLE user_verifications (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    type verification_type NOT NULL,
    identifier text NOT NULL,
    status verification_status NOT NULL DEFAULT 'PENDING',
    requested_at timestamptz NOT NULL DEFAULT now(),
    verified_at timestamptz,
    expires_at timestamptz
);

CREATE TABLE user_statistics (
    user_id uuid PRIMARY KEY,
    average_rating numeric(3,2) NOT NULL DEFAULT 0,
    review_count integer NOT NULL DEFAULT 0,
    completed_exchange_count integer NOT NULL DEFAULT 0,
    completed_session_count integer NOT NULL DEFAULT 0,
    taught_minutes integer NOT NULL DEFAULT 0,
    average_response_minutes integer,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_profile_settings (
    user_id uuid PRIMARY KEY,
    is_discoverable boolean NOT NULL DEFAULT true,
    accepts_requests boolean NOT NULL DEFAULT true,
    request_scope request_scope NOT NULL DEFAULT 'ANYONE',
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE skill_categories (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    name varchar(100) NOT NULL UNIQUE,
    slug varchar(100) NOT NULL UNIQUE,
    sort_order integer NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true
);

CREATE TABLE skills (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    category_id uuid NOT NULL,
    name varchar(150) NOT NULL,
    slug varchar(150) NOT NULL UNIQUE,
    description text,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_teaching_skills (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    skill_id uuid NOT NULL,
    level skill_level NOT NULL,
    description text,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_learning_skills (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    skill_id uuid NOT NULL,
    current_level skill_level NOT NULL DEFAULT 'BEGINNER',
    target_level skill_level,
    priority smallint NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_lesson_preferences (
    user_id uuid PRIMARY KEY,
    default_duration_minutes smallint NOT NULL DEFAULT 60,
    sessions_per_week_min smallint NOT NULL DEFAULT 1,
    sessions_per_week_max smallint NOT NULL DEFAULT 1,
    break_between_sessions_minutes smallint NOT NULL DEFAULT 15,
    communication_language varchar(80) NOT NULL DEFAULT 'Ukrainian',
    response_deadline_hours smallint NOT NULL DEFAULT 24,
    offers_trial_session boolean NOT NULL DEFAULT false,
    trial_duration_minutes smallint,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_lesson_formats (
    user_id uuid NOT NULL,
    format lesson_format NOT NULL,
    PRIMARY KEY (user_id, format)
);

CREATE TABLE user_availability_slots (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    day_of_week smallint NOT NULL,
    start_time time NOT NULL,
    end_time time NOT NULL,
    timezone varchar(64) NOT NULL DEFAULT 'Europe/Kyiv',
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE meeting_places (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    name varchar(180) NOT NULL,
    address text,
    notes text,
    latitude numeric(9,6),
    longitude numeric(9,6),
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_pause_periods (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    reason varchar(255),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE matches (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    user_a_id uuid NOT NULL,
    user_b_id uuid NOT NULL,
    score numeric(5,2) NOT NULL,
    status_for_a match_status NOT NULL DEFAULT 'NEW',
    status_for_b match_status NOT NULL DEFAULT 'NEW',
    calculated_at timestamptz NOT NULL DEFAULT now(),
    invalidated_at timestamptz
);

CREATE TABLE match_skill_links (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    match_id uuid NOT NULL,
    teacher_id uuid NOT NULL,
    learner_id uuid NOT NULL,
    teaching_skill_id uuid NOT NULL,
    learning_skill_id uuid NOT NULL,
    contribution numeric(5,2) NOT NULL DEFAULT 0
);

CREATE TABLE match_factors (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    match_id uuid NOT NULL,
    type match_factor_type NOT NULL,
    contribution numeric(5,2) NOT NULL,
    explanation varchar(255) NOT NULL
);

CREATE TABLE exchange_requests (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    match_id uuid,
    requester_id uuid NOT NULL,
    recipient_id uuid NOT NULL,
    requester_teaching_skill_id uuid NOT NULL,
    recipient_learning_skill_id uuid NOT NULL,
    recipient_teaching_skill_id uuid NOT NULL,
    requester_learning_skill_id uuid NOT NULL,
    format lesson_format NOT NULL,
    meeting_place_id uuid,
    total_sessions smallint NOT NULL,
    requester_sessions smallint NOT NULL,
    recipient_sessions smallint NOT NULL,
    requester_duration_minutes smallint NOT NULL,
    recipient_duration_minutes smallint NOT NULL,
    sessions_per_week smallint NOT NULL DEFAULT 1,
    message varchar(500),
    status exchange_request_status NOT NULL DEFAULT 'PENDING',
    parent_request_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    responded_at timestamptz
);

CREATE TABLE exchange_request_status_history (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    request_id uuid NOT NULL,
    status exchange_request_status NOT NULL,
    changed_by_user_id uuid,
    reason text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE exchanges (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    source_request_id uuid UNIQUE,
    user_a_id uuid NOT NULL,
    user_b_id uuid NOT NULL,
    status exchange_status NOT NULL DEFAULT 'PLANNED',
    total_sessions smallint NOT NULL,
    sessions_per_week smallint NOT NULL DEFAULT 1,
    format lesson_format NOT NULL,
    meeting_place_id uuid,
    online_platform varchar(100),
    default_meeting_url text,
    started_at timestamptz,
    completed_at timestamptz,
    cancelled_at timestamptz,
    cancellation_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE exchange_skill_commitments (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    exchange_id uuid NOT NULL,
    teacher_id uuid NOT NULL,
    learner_id uuid NOT NULL,
    teaching_skill_id uuid NOT NULL,
    learning_skill_id uuid NOT NULL,
    planned_sessions smallint NOT NULL,
    duration_minutes smallint NOT NULL,
    skill_name_snapshot varchar(150) NOT NULL,
    teaching_level_snapshot skill_level NOT NULL,
    learning_level_snapshot skill_level NOT NULL
);

CREATE TABLE exchange_status_history (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    exchange_id uuid NOT NULL,
    status exchange_status NOT NULL,
    changed_by_user_id uuid,
    reason text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    exchange_id uuid NOT NULL,
    plan_order smallint NOT NULL,
    title varchar(255) NOT NULL,
    description text,
    teacher_id uuid,
    learner_id uuid,
    status session_status NOT NULL DEFAULT 'DRAFT',
    scheduled_start timestamptz,
    scheduled_end timestamptz,
    format lesson_format,
    meeting_place_id uuid,
    meeting_url text,
    completed_at timestamptz,
    cancelled_at timestamptz,
    cancellation_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE session_confirmations (
    session_id uuid NOT NULL,
    user_id uuid NOT NULL,
    kind confirmation_kind NOT NULL,
    confirmed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, user_id, kind)
);

CREATE TABLE exchange_confirmations (
    exchange_id uuid NOT NULL,
    user_id uuid NOT NULL,
    confirmed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (exchange_id, user_id)
);

CREATE TABLE session_schedule_proposals (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    session_id uuid NOT NULL,
    proposed_by_user_id uuid NOT NULL,
    proposed_start timestamptz NOT NULL,
    proposed_end timestamptz NOT NULL,
    format lesson_format NOT NULL,
    meeting_place_id uuid,
    meeting_url text,
    status schedule_proposal_status NOT NULL DEFAULT 'PENDING',
    responded_by_user_id uuid,
    responded_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE session_ratings (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    session_id uuid NOT NULL,
    author_id uuid NOT NULL,
    recipient_id uuid NOT NULL,
    rating smallint NOT NULL,
    comment text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE conversations (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    type conversation_type NOT NULL,
    exchange_request_id uuid,
    exchange_id uuid,
    created_by_user_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_message_at timestamptz
);

CREATE TABLE conversation_participants (
    conversation_id uuid NOT NULL,
    user_id uuid NOT NULL,
    joined_at timestamptz NOT NULL DEFAULT now(),
    muted_until timestamptz,
    archived_at timestamptz,
    PRIMARY KEY (conversation_id, user_id)
);

CREATE TABLE messages (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    conversation_id uuid NOT NULL,
    author_id uuid,
    type message_type NOT NULL DEFAULT 'TEXT',
    body text,
    session_schedule_proposal_id uuid,
    reply_to_message_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    edited_at timestamptz,
    deleted_at timestamptz
);

CREATE TABLE message_receipts (
    message_id uuid NOT NULL,
    user_id uuid NOT NULL,
    delivered_at timestamptz,
    read_at timestamptz,
    PRIMARY KEY (message_id, user_id)
);

CREATE TABLE files (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    uploaded_by_user_id uuid NOT NULL,
    kind file_kind NOT NULL,
    original_name varchar(255) NOT NULL,
    storage_key text NOT NULL UNIQUE,
    mime_type varchar(150) NOT NULL,
    size_bytes bigint NOT NULL,
    duration_seconds integer,
    checksum_sha256 char(64),
    created_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE TABLE message_attachments (
    message_id uuid NOT NULL,
    file_id uuid NOT NULL,
    sort_order smallint NOT NULL DEFAULT 0,
    PRIMARY KEY (message_id, file_id)
);

CREATE TABLE exchange_materials (
    exchange_id uuid NOT NULL,
    file_id uuid NOT NULL,
    added_by_user_id uuid NOT NULL,
    session_id uuid,
    title varchar(255),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (exchange_id, file_id)
);

CREATE TABLE review_tags (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    label varchar(100) NOT NULL UNIQUE,
    is_active boolean NOT NULL DEFAULT true
);

CREATE TABLE exchange_reviews (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    exchange_id uuid NOT NULL,
    author_id uuid NOT NULL,
    recipient_id uuid NOT NULL,
    rating smallint NOT NULL,
    comment varchar(1000),
    willing_to_exchange_again boolean,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE exchange_review_tags (
    review_id uuid NOT NULL,
    tag_id uuid NOT NULL,
    PRIMARY KEY (review_id, tag_id)
);

CREATE TABLE notification_preferences (
    user_id uuid NOT NULL,
    event_type notification_event_type NOT NULL,
    is_enabled boolean NOT NULL DEFAULT true,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, event_type)
);

CREATE TABLE notifications (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    recipient_id uuid NOT NULL,
    actor_id uuid,
    event_type notification_event_type NOT NULL,
    event_key varchar(255) NOT NULL,
    title varchar(180) NOT NULL,
    body text,
    match_id uuid,
    exchange_request_id uuid,
    exchange_id uuid,
    session_id uuid,
    conversation_id uuid,
    message_id uuid,
    review_id uuid,
    payload jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    read_at timestamptz
);

CREATE TABLE user_blocks (
    blocker_id uuid NOT NULL,
    blocked_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id)
);

CREATE TABLE reports (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    reporter_id uuid NOT NULL,
    reported_user_id uuid NOT NULL,
    reason text NOT NULL,
    status report_status NOT NULL DEFAULT 'OPEN',
    created_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz
);

CREATE TABLE moderation_actions (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    report_id uuid,
    moderator_id uuid NOT NULL,
    subject_user_id uuid NOT NULL,
    action varchar(40) NOT NULL,
    reason text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE calendar_integrations (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    provider calendar_provider NOT NULL,
    external_account_id varchar(255),
    credential_reference text,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE calendar_event_links (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    integration_id uuid NOT NULL,
    session_id uuid NOT NULL,
    external_event_id varchar(255) NOT NULL,
    status sync_status NOT NULL DEFAULT 'PENDING',
    last_synced_at timestamptz,
    error_message text
);

ALTER TABLE faculties ADD CONSTRAINT fk_faculties_university_id FOREIGN KEY (university_id) REFERENCES universities (id);
ALTER TABLE users ADD CONSTRAINT fk_users_university_id FOREIGN KEY (university_id) REFERENCES universities (id);
ALTER TABLE users ADD CONSTRAINT fk_users_faculty_id FOREIGN KEY (faculty_id) REFERENCES faculties (id);
ALTER TABLE auth_identities ADD CONSTRAINT fk_auth_identities_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE user_sessions ADD CONSTRAINT fk_user_sessions_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE one_time_tokens ADD CONSTRAINT fk_one_time_tokens_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE user_verifications ADD CONSTRAINT fk_user_verifications_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE user_statistics ADD CONSTRAINT fk_user_statistics_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE user_profile_settings ADD CONSTRAINT fk_user_profile_settings_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE skills ADD CONSTRAINT fk_skills_category_id FOREIGN KEY (category_id) REFERENCES skill_categories (id);
ALTER TABLE user_teaching_skills ADD CONSTRAINT fk_user_teaching_skills_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE user_teaching_skills ADD CONSTRAINT fk_user_teaching_skills_skill_id FOREIGN KEY (skill_id) REFERENCES skills (id);
ALTER TABLE user_learning_skills ADD CONSTRAINT fk_user_learning_skills_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE user_learning_skills ADD CONSTRAINT fk_user_learning_skills_skill_id FOREIGN KEY (skill_id) REFERENCES skills (id);
ALTER TABLE user_lesson_preferences ADD CONSTRAINT fk_user_lesson_preferences_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE user_lesson_formats ADD CONSTRAINT fk_user_lesson_formats_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE user_availability_slots ADD CONSTRAINT fk_user_availability_slots_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE meeting_places ADD CONSTRAINT fk_meeting_places_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE user_pause_periods ADD CONSTRAINT fk_user_pause_periods_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE matches ADD CONSTRAINT fk_matches_user_a_id FOREIGN KEY (user_a_id) REFERENCES users (id);
ALTER TABLE matches ADD CONSTRAINT fk_matches_user_b_id FOREIGN KEY (user_b_id) REFERENCES users (id);
ALTER TABLE match_skill_links ADD CONSTRAINT fk_match_skill_links_match_id FOREIGN KEY (match_id) REFERENCES matches (id);
ALTER TABLE match_skill_links ADD CONSTRAINT fk_match_skill_links_teacher_id FOREIGN KEY (teacher_id) REFERENCES users (id);
ALTER TABLE match_skill_links ADD CONSTRAINT fk_match_skill_links_learner_id FOREIGN KEY (learner_id) REFERENCES users (id);
ALTER TABLE match_skill_links ADD CONSTRAINT fk_match_skill_links_teaching_skill_id FOREIGN KEY (teaching_skill_id) REFERENCES user_teaching_skills (id);
ALTER TABLE match_skill_links ADD CONSTRAINT fk_match_skill_links_learning_skill_id FOREIGN KEY (learning_skill_id) REFERENCES user_learning_skills (id);
ALTER TABLE match_factors ADD CONSTRAINT fk_match_factors_match_id FOREIGN KEY (match_id) REFERENCES matches (id);
ALTER TABLE exchange_requests ADD CONSTRAINT fk_exchange_requests_match_id FOREIGN KEY (match_id) REFERENCES matches (id);
ALTER TABLE exchange_requests ADD CONSTRAINT fk_exchange_requests_requester_id FOREIGN KEY (requester_id) REFERENCES users (id);
ALTER TABLE exchange_requests ADD CONSTRAINT fk_exchange_requests_recipient_id FOREIGN KEY (recipient_id) REFERENCES users (id);
ALTER TABLE exchange_requests ADD CONSTRAINT fk_exchange_requests_requester_teaching_skill_id FOREIGN KEY (requester_teaching_skill_id) REFERENCES user_teaching_skills (id);
ALTER TABLE exchange_requests ADD CONSTRAINT fk_exchange_requests_recipient_learning_skill_id FOREIGN KEY (recipient_learning_skill_id) REFERENCES user_learning_skills (id);
ALTER TABLE exchange_requests ADD CONSTRAINT fk_exchange_requests_recipient_teaching_skill_id FOREIGN KEY (recipient_teaching_skill_id) REFERENCES user_teaching_skills (id);
ALTER TABLE exchange_requests ADD CONSTRAINT fk_exchange_requests_requester_learning_skill_id FOREIGN KEY (requester_learning_skill_id) REFERENCES user_learning_skills (id);
ALTER TABLE exchange_requests ADD CONSTRAINT fk_exchange_requests_meeting_place_id FOREIGN KEY (meeting_place_id) REFERENCES meeting_places (id);
ALTER TABLE exchange_requests ADD CONSTRAINT fk_exchange_requests_parent_request_id FOREIGN KEY (parent_request_id) REFERENCES exchange_requests (id);
ALTER TABLE exchange_request_status_history ADD CONSTRAINT fk_exchange_request_status_history_request_id FOREIGN KEY (request_id) REFERENCES exchange_requests (id);
ALTER TABLE exchange_request_status_history ADD CONSTRAINT fk_exchange_request_status_history_changed_by_user_id FOREIGN KEY (changed_by_user_id) REFERENCES users (id);
ALTER TABLE exchanges ADD CONSTRAINT fk_exchanges_source_request_id FOREIGN KEY (source_request_id) REFERENCES exchange_requests (id);
ALTER TABLE exchanges ADD CONSTRAINT fk_exchanges_user_a_id FOREIGN KEY (user_a_id) REFERENCES users (id);
ALTER TABLE exchanges ADD CONSTRAINT fk_exchanges_user_b_id FOREIGN KEY (user_b_id) REFERENCES users (id);
ALTER TABLE exchanges ADD CONSTRAINT fk_exchanges_meeting_place_id FOREIGN KEY (meeting_place_id) REFERENCES meeting_places (id);
ALTER TABLE exchange_skill_commitments ADD CONSTRAINT fk_exchange_skill_commitments_exchange_id FOREIGN KEY (exchange_id) REFERENCES exchanges (id);
ALTER TABLE exchange_skill_commitments ADD CONSTRAINT fk_exchange_skill_commitments_teacher_id FOREIGN KEY (teacher_id) REFERENCES users (id);
ALTER TABLE exchange_skill_commitments ADD CONSTRAINT fk_exchange_skill_commitments_learner_id FOREIGN KEY (learner_id) REFERENCES users (id);
ALTER TABLE exchange_skill_commitments ADD CONSTRAINT fk_exchange_skill_commitments_teaching_skill_id FOREIGN KEY (teaching_skill_id) REFERENCES user_teaching_skills (id);
ALTER TABLE exchange_skill_commitments ADD CONSTRAINT fk_exchange_skill_commitments_learning_skill_id FOREIGN KEY (learning_skill_id) REFERENCES user_learning_skills (id);
ALTER TABLE exchange_status_history ADD CONSTRAINT fk_exchange_status_history_exchange_id FOREIGN KEY (exchange_id) REFERENCES exchanges (id);
ALTER TABLE exchange_status_history ADD CONSTRAINT fk_exchange_status_history_changed_by_user_id FOREIGN KEY (changed_by_user_id) REFERENCES users (id);
ALTER TABLE sessions ADD CONSTRAINT fk_sessions_exchange_id FOREIGN KEY (exchange_id) REFERENCES exchanges (id);
ALTER TABLE sessions ADD CONSTRAINT fk_sessions_teacher_id FOREIGN KEY (teacher_id) REFERENCES users (id);
ALTER TABLE sessions ADD CONSTRAINT fk_sessions_learner_id FOREIGN KEY (learner_id) REFERENCES users (id);
ALTER TABLE sessions ADD CONSTRAINT fk_sessions_meeting_place_id FOREIGN KEY (meeting_place_id) REFERENCES meeting_places (id);
ALTER TABLE session_confirmations ADD CONSTRAINT fk_session_confirmations_session_id FOREIGN KEY (session_id) REFERENCES sessions (id);
ALTER TABLE session_confirmations ADD CONSTRAINT fk_session_confirmations_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE exchange_confirmations ADD CONSTRAINT fk_exchange_confirmations_exchange_id FOREIGN KEY (exchange_id) REFERENCES exchanges (id);
ALTER TABLE exchange_confirmations ADD CONSTRAINT fk_exchange_confirmations_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE session_schedule_proposals ADD CONSTRAINT fk_session_schedule_proposals_session_id FOREIGN KEY (session_id) REFERENCES sessions (id);
ALTER TABLE session_schedule_proposals ADD CONSTRAINT fk_session_schedule_proposals_proposed_by_user_id FOREIGN KEY (proposed_by_user_id) REFERENCES users (id);
ALTER TABLE session_schedule_proposals ADD CONSTRAINT fk_session_schedule_proposals_meeting_place_id FOREIGN KEY (meeting_place_id) REFERENCES meeting_places (id);
ALTER TABLE session_schedule_proposals ADD CONSTRAINT fk_session_schedule_proposals_responded_by_user_id FOREIGN KEY (responded_by_user_id) REFERENCES users (id);
ALTER TABLE session_ratings ADD CONSTRAINT fk_session_ratings_session_id FOREIGN KEY (session_id) REFERENCES sessions (id);
ALTER TABLE session_ratings ADD CONSTRAINT fk_session_ratings_author_id FOREIGN KEY (author_id) REFERENCES users (id);
ALTER TABLE session_ratings ADD CONSTRAINT fk_session_ratings_recipient_id FOREIGN KEY (recipient_id) REFERENCES users (id);
ALTER TABLE conversations ADD CONSTRAINT fk_conversations_exchange_request_id FOREIGN KEY (exchange_request_id) REFERENCES exchange_requests (id);
ALTER TABLE conversations ADD CONSTRAINT fk_conversations_exchange_id FOREIGN KEY (exchange_id) REFERENCES exchanges (id);
ALTER TABLE conversations ADD CONSTRAINT fk_conversations_created_by_user_id FOREIGN KEY (created_by_user_id) REFERENCES users (id);
ALTER TABLE conversation_participants ADD CONSTRAINT fk_conversation_participants_conversation_id FOREIGN KEY (conversation_id) REFERENCES conversations (id);
ALTER TABLE conversation_participants ADD CONSTRAINT fk_conversation_participants_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE messages ADD CONSTRAINT fk_messages_conversation_id FOREIGN KEY (conversation_id) REFERENCES conversations (id);
ALTER TABLE messages ADD CONSTRAINT fk_messages_author_id FOREIGN KEY (author_id) REFERENCES users (id);
ALTER TABLE messages ADD CONSTRAINT fk_messages_session_schedule_proposal_id FOREIGN KEY (session_schedule_proposal_id) REFERENCES session_schedule_proposals (id);
ALTER TABLE messages ADD CONSTRAINT fk_messages_reply_to_message_id FOREIGN KEY (reply_to_message_id) REFERENCES messages (id);
ALTER TABLE message_receipts ADD CONSTRAINT fk_message_receipts_message_id FOREIGN KEY (message_id) REFERENCES messages (id);
ALTER TABLE message_receipts ADD CONSTRAINT fk_message_receipts_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE files ADD CONSTRAINT fk_files_uploaded_by_user_id FOREIGN KEY (uploaded_by_user_id) REFERENCES users (id);
ALTER TABLE message_attachments ADD CONSTRAINT fk_message_attachments_message_id FOREIGN KEY (message_id) REFERENCES messages (id);
ALTER TABLE message_attachments ADD CONSTRAINT fk_message_attachments_file_id FOREIGN KEY (file_id) REFERENCES files (id);
ALTER TABLE exchange_materials ADD CONSTRAINT fk_exchange_materials_exchange_id FOREIGN KEY (exchange_id) REFERENCES exchanges (id);
ALTER TABLE exchange_materials ADD CONSTRAINT fk_exchange_materials_file_id FOREIGN KEY (file_id) REFERENCES files (id);
ALTER TABLE exchange_materials ADD CONSTRAINT fk_exchange_materials_added_by_user_id FOREIGN KEY (added_by_user_id) REFERENCES users (id);
ALTER TABLE exchange_materials ADD CONSTRAINT fk_exchange_materials_session_id FOREIGN KEY (session_id) REFERENCES sessions (id);
ALTER TABLE exchange_reviews ADD CONSTRAINT fk_exchange_reviews_exchange_id FOREIGN KEY (exchange_id) REFERENCES exchanges (id);
ALTER TABLE exchange_reviews ADD CONSTRAINT fk_exchange_reviews_author_id FOREIGN KEY (author_id) REFERENCES users (id);
ALTER TABLE exchange_reviews ADD CONSTRAINT fk_exchange_reviews_recipient_id FOREIGN KEY (recipient_id) REFERENCES users (id);
ALTER TABLE exchange_review_tags ADD CONSTRAINT fk_exchange_review_tags_review_id FOREIGN KEY (review_id) REFERENCES exchange_reviews (id);
ALTER TABLE exchange_review_tags ADD CONSTRAINT fk_exchange_review_tags_tag_id FOREIGN KEY (tag_id) REFERENCES review_tags (id);
ALTER TABLE notification_preferences ADD CONSTRAINT fk_notification_preferences_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE notifications ADD CONSTRAINT fk_notifications_recipient_id FOREIGN KEY (recipient_id) REFERENCES users (id);
ALTER TABLE notifications ADD CONSTRAINT fk_notifications_actor_id FOREIGN KEY (actor_id) REFERENCES users (id);
ALTER TABLE notifications ADD CONSTRAINT fk_notifications_match_id FOREIGN KEY (match_id) REFERENCES matches (id);
ALTER TABLE notifications ADD CONSTRAINT fk_notifications_exchange_request_id FOREIGN KEY (exchange_request_id) REFERENCES exchange_requests (id);
ALTER TABLE notifications ADD CONSTRAINT fk_notifications_exchange_id FOREIGN KEY (exchange_id) REFERENCES exchanges (id);
ALTER TABLE notifications ADD CONSTRAINT fk_notifications_session_id FOREIGN KEY (session_id) REFERENCES sessions (id);
ALTER TABLE notifications ADD CONSTRAINT fk_notifications_conversation_id FOREIGN KEY (conversation_id) REFERENCES conversations (id);
ALTER TABLE notifications ADD CONSTRAINT fk_notifications_message_id FOREIGN KEY (message_id) REFERENCES messages (id);
ALTER TABLE notifications ADD CONSTRAINT fk_notifications_review_id FOREIGN KEY (review_id) REFERENCES exchange_reviews (id);
ALTER TABLE user_blocks ADD CONSTRAINT fk_user_blocks_blocker_id FOREIGN KEY (blocker_id) REFERENCES users (id);
ALTER TABLE user_blocks ADD CONSTRAINT fk_user_blocks_blocked_id FOREIGN KEY (blocked_id) REFERENCES users (id);
ALTER TABLE reports ADD CONSTRAINT fk_reports_reporter_id FOREIGN KEY (reporter_id) REFERENCES users (id);
ALTER TABLE reports ADD CONSTRAINT fk_reports_reported_user_id FOREIGN KEY (reported_user_id) REFERENCES users (id);
ALTER TABLE moderation_actions ADD CONSTRAINT fk_moderation_actions_report_id FOREIGN KEY (report_id) REFERENCES reports (id);
ALTER TABLE moderation_actions ADD CONSTRAINT fk_moderation_actions_moderator_id FOREIGN KEY (moderator_id) REFERENCES users (id);
ALTER TABLE moderation_actions ADD CONSTRAINT fk_moderation_actions_subject_user_id FOREIGN KEY (subject_user_id) REFERENCES users (id);
ALTER TABLE calendar_integrations ADD CONSTRAINT fk_calendar_integrations_user_id FOREIGN KEY (user_id) REFERENCES users (id);
ALTER TABLE calendar_event_links ADD CONSTRAINT fk_calendar_event_links_integration_id FOREIGN KEY (integration_id) REFERENCES calendar_integrations (id);
ALTER TABLE calendar_event_links ADD CONSTRAINT fk_calendar_event_links_session_id FOREIGN KEY (session_id) REFERENCES sessions (id);

CREATE INDEX idx_universities_name ON universities (name);
CREATE UNIQUE INDEX idx_faculties_university_id_name ON faculties (university_id, name);
CREATE INDEX idx_users_university_id_faculty_id_academic_year ON users (university_id, faculty_id, academic_year);
CREATE INDEX idx_users_last_active_at ON users (last_active_at);
CREATE UNIQUE INDEX idx_auth_identities_provider_provider_subject ON auth_identities (provider, provider_subject);
CREATE UNIQUE INDEX idx_auth_identities_user_id_provider ON auth_identities (user_id, provider);
CREATE INDEX idx_user_sessions_user_id_revoked_at ON user_sessions (user_id, revoked_at);
CREATE INDEX idx_one_time_tokens_user_id_purpose_expires_at ON one_time_tokens (user_id, purpose, expires_at);
CREATE UNIQUE INDEX idx_user_verifications_user_id_type_identifier ON user_verifications (user_id, type, identifier);
CREATE INDEX idx_user_verifications_type_status ON user_verifications (type, status);
CREATE UNIQUE INDEX idx_skills_category_id_name ON skills (category_id, name);
CREATE INDEX idx_skills_name ON skills (name);
CREATE UNIQUE INDEX idx_user_teaching_skills_user_id_skill_id ON user_teaching_skills (user_id, skill_id);
CREATE INDEX idx_user_teaching_skills_skill_id_level_is_active ON user_teaching_skills (skill_id, level, is_active);
CREATE UNIQUE INDEX idx_user_learning_skills_user_id_skill_id ON user_learning_skills (user_id, skill_id);
CREATE UNIQUE INDEX idx_user_learning_skills_user_id_priority ON user_learning_skills (user_id, priority);
CREATE INDEX idx_user_learning_skills_skill_id_current_level_is_active ON user_learning_skills (skill_id, current_level, is_active);
CREATE INDEX idx_user_lesson_formats_format_user_id ON user_lesson_formats (format, user_id);
CREATE UNIQUE INDEX idx_user_availability_slots_user_id_day_of_week_start__d67929dd ON user_availability_slots (user_id, day_of_week, start_time, end_time);
CREATE INDEX idx_user_availability_slots_day_of_week_start_time_end_time ON user_availability_slots (day_of_week, start_time, end_time);
CREATE INDEX idx_user_pause_periods_user_id_starts_at_ends_at ON user_pause_periods (user_id, starts_at, ends_at);
CREATE UNIQUE INDEX idx_matches_user_a_id_user_b_id ON matches (user_a_id, user_b_id);
CREATE INDEX idx_matches_user_a_id_status_for_a_score ON matches (user_a_id, status_for_a, score);
CREATE INDEX idx_matches_user_b_id_status_for_b_score ON matches (user_b_id, status_for_b, score);
CREATE UNIQUE INDEX idx_match_skill_links_match_id_teacher_id_learner_id_t_5e8304d1 ON match_skill_links (match_id, teacher_id, learner_id, teaching_skill_id, learning_skill_id);
CREATE UNIQUE INDEX idx_match_factors_match_id_type_explanation ON match_factors (match_id, type, explanation);
CREATE INDEX idx_exchange_requests_recipient_id_status_created_at ON exchange_requests (recipient_id, status, created_at);
CREATE INDEX idx_exchange_requests_requester_id_status_created_at ON exchange_requests (requester_id, status, created_at);
CREATE INDEX idx_exchange_requests_expires_at ON exchange_requests (expires_at);
CREATE INDEX idx_exchange_request_status_history_request_id_created_at ON exchange_request_status_history (request_id, created_at);
CREATE INDEX idx_exchanges_user_a_id_status_updated_at ON exchanges (user_a_id, status, updated_at);
CREATE INDEX idx_exchanges_user_b_id_status_updated_at ON exchanges (user_b_id, status, updated_at);
CREATE UNIQUE INDEX idx_exchange_skill_commitments_exchange_id_teacher_id__fa30c2e3 ON exchange_skill_commitments (exchange_id, teacher_id, learner_id, teaching_skill_id, learning_skill_id);
CREATE INDEX idx_exchange_status_history_exchange_id_created_at ON exchange_status_history (exchange_id, created_at);
CREATE UNIQUE INDEX idx_sessions_exchange_id_plan_order ON sessions (exchange_id, plan_order);
CREATE INDEX idx_sessions_scheduled_start_status ON sessions (scheduled_start, status);
CREATE INDEX idx_sessions_teacher_id_scheduled_start ON sessions (teacher_id, scheduled_start);
CREATE INDEX idx_sessions_learner_id_scheduled_start ON sessions (learner_id, scheduled_start);
CREATE INDEX idx_session_schedule_proposals_session_id_status_created_at ON session_schedule_proposals (session_id, status, created_at);
CREATE UNIQUE INDEX idx_session_ratings_session_id_author_id ON session_ratings (session_id, author_id);
CREATE INDEX idx_session_ratings_recipient_id_created_at ON session_ratings (recipient_id, created_at);
CREATE UNIQUE INDEX idx_conversations_exchange_request_id ON conversations (exchange_request_id);
CREATE UNIQUE INDEX idx_conversations_exchange_id ON conversations (exchange_id);
CREATE INDEX idx_conversations_last_message_at ON conversations (last_message_at);
CREATE INDEX idx_conversation_participants_user_id_archived_at ON conversation_participants (user_id, archived_at);
CREATE INDEX idx_messages_conversation_id_created_at ON messages (conversation_id, created_at);
CREATE INDEX idx_messages_author_id_created_at ON messages (author_id, created_at);
CREATE INDEX idx_message_receipts_user_id_read_at ON message_receipts (user_id, read_at);
CREATE INDEX idx_exchange_materials_session_id_created_at ON exchange_materials (session_id, created_at);
CREATE UNIQUE INDEX idx_exchange_reviews_exchange_id_author_id ON exchange_reviews (exchange_id, author_id);
CREATE INDEX idx_exchange_reviews_recipient_id_created_at ON exchange_reviews (recipient_id, created_at);
CREATE UNIQUE INDEX idx_notifications_recipient_id_event_key ON notifications (recipient_id, event_key);
CREATE INDEX idx_notifications_recipient_id_read_at_created_at ON notifications (recipient_id, read_at, created_at);
CREATE INDEX idx_notifications_event_type_created_at ON notifications (event_type, created_at);
CREATE INDEX idx_reports_status_created_at ON reports (status, created_at);
CREATE INDEX idx_moderation_actions_subject_user_id_created_at ON moderation_actions (subject_user_id, created_at);
CREATE UNIQUE INDEX idx_calendar_integrations_user_id_provider_external_account_id ON calendar_integrations (user_id, provider, external_account_id);
CREATE UNIQUE INDEX idx_calendar_event_links_integration_id_session_id ON calendar_event_links (integration_id, session_id);
CREATE UNIQUE INDEX idx_calendar_event_links_integration_id_external_event_id ON calendar_event_links (integration_id, external_event_id);

ALTER TABLE users ADD CONSTRAINT ck_users_1 CHECK (academic_year IS NULL OR academic_year BETWEEN 1 AND 8);
ALTER TABLE user_sessions ADD CONSTRAINT ck_user_sessions_1 CHECK (expires_at > created_at);
ALTER TABLE one_time_tokens ADD CONSTRAINT ck_one_time_tokens_1 CHECK (expires_at > created_at);
ALTER TABLE user_learning_skills ADD CONSTRAINT ck_user_learning_skills_1 CHECK (priority BETWEEN 1 AND 5);
ALTER TABLE user_availability_slots ADD CONSTRAINT ck_user_availability_slots_1 CHECK (day_of_week BETWEEN 1 AND 7);
ALTER TABLE user_availability_slots ADD CONSTRAINT ck_user_availability_slots_2 CHECK (end_time > start_time);
ALTER TABLE user_pause_periods ADD CONSTRAINT ck_user_pause_periods_1 CHECK (ends_at > starts_at);
ALTER TABLE matches ADD CONSTRAINT ck_matches_1 CHECK (user_a_id < user_b_id);
ALTER TABLE matches ADD CONSTRAINT ck_matches_2 CHECK (score BETWEEN 0 AND 100);
ALTER TABLE exchange_requests ADD CONSTRAINT ck_exchange_requests_1 CHECK (requester_id <> recipient_id);
ALTER TABLE exchange_requests ADD CONSTRAINT ck_exchange_requests_2 CHECK (requester_sessions > 0);
ALTER TABLE exchange_requests ADD CONSTRAINT ck_exchange_requests_3 CHECK (recipient_sessions > 0);
ALTER TABLE exchange_requests ADD CONSTRAINT ck_exchange_requests_4 CHECK (total_sessions = requester_sessions + recipient_sessions);
ALTER TABLE exchange_requests ADD CONSTRAINT ck_exchange_requests_5 CHECK (requester_duration_minutes BETWEEN 15 AND 240);
ALTER TABLE exchange_requests ADD CONSTRAINT ck_exchange_requests_6 CHECK (recipient_duration_minutes BETWEEN 15 AND 240);
ALTER TABLE exchange_requests ADD CONSTRAINT ck_exchange_requests_7 CHECK (requester_sessions * requester_duration_minutes = recipient_sessions * recipient_duration_minutes);
ALTER TABLE exchange_requests ADD CONSTRAINT ck_exchange_requests_8 CHECK (sessions_per_week > 0);
ALTER TABLE exchange_requests ADD CONSTRAINT ck_exchange_requests_9 CHECK (expires_at > created_at);
ALTER TABLE exchanges ADD CONSTRAINT ck_exchanges_1 CHECK (user_a_id <> user_b_id);
ALTER TABLE exchanges ADD CONSTRAINT ck_exchanges_2 CHECK (total_sessions > 0);
ALTER TABLE exchanges ADD CONSTRAINT ck_exchanges_3 CHECK (sessions_per_week > 0);
ALTER TABLE exchange_skill_commitments ADD CONSTRAINT ck_exchange_skill_commitments_1 CHECK (teacher_id <> learner_id);
ALTER TABLE exchange_skill_commitments ADD CONSTRAINT ck_exchange_skill_commitments_2 CHECK (planned_sessions > 0);
ALTER TABLE exchange_skill_commitments ADD CONSTRAINT ck_exchange_skill_commitments_3 CHECK (duration_minutes BETWEEN 15 AND 240);
ALTER TABLE sessions ADD CONSTRAINT ck_sessions_1 CHECK (plan_order > 0);
ALTER TABLE sessions ADD CONSTRAINT ck_sessions_2 CHECK ((scheduled_start IS NULL AND scheduled_end IS NULL) OR (scheduled_start IS NOT NULL AND scheduled_end > scheduled_start));
ALTER TABLE session_schedule_proposals ADD CONSTRAINT ck_session_schedule_proposals_1 CHECK (proposed_end > proposed_start);
ALTER TABLE session_ratings ADD CONSTRAINT ck_session_ratings_1 CHECK (rating BETWEEN 1 AND 5);
ALTER TABLE session_ratings ADD CONSTRAINT ck_session_ratings_2 CHECK (author_id <> recipient_id);
ALTER TABLE exchange_reviews ADD CONSTRAINT ck_exchange_reviews_1 CHECK (rating BETWEEN 1 AND 5);
ALTER TABLE exchange_reviews ADD CONSTRAINT ck_exchange_reviews_2 CHECK (author_id <> recipient_id);
ALTER TABLE user_blocks ADD CONSTRAINT ck_user_blocks_1 CHECK (blocker_id <> blocked_id);
ALTER TABLE reports ADD CONSTRAINT ck_reports_1 CHECK (reporter_id <> reported_user_id);
ALTER TABLE files ADD CONSTRAINT ck_files_1 CHECK (size_bytes >= 0);

-- +goose Down
DROP TABLE IF EXISTS calendar_event_links CASCADE;
DROP TABLE IF EXISTS calendar_integrations CASCADE;
DROP TABLE IF EXISTS moderation_actions CASCADE;
DROP TABLE IF EXISTS reports CASCADE;
DROP TABLE IF EXISTS user_blocks CASCADE;
DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS notification_preferences CASCADE;
DROP TABLE IF EXISTS exchange_review_tags CASCADE;
DROP TABLE IF EXISTS exchange_reviews CASCADE;
DROP TABLE IF EXISTS review_tags CASCADE;
DROP TABLE IF EXISTS exchange_materials CASCADE;
DROP TABLE IF EXISTS message_attachments CASCADE;
DROP TABLE IF EXISTS files CASCADE;
DROP TABLE IF EXISTS message_receipts CASCADE;
DROP TABLE IF EXISTS messages CASCADE;
DROP TABLE IF EXISTS conversation_participants CASCADE;
DROP TABLE IF EXISTS conversations CASCADE;
DROP TABLE IF EXISTS session_ratings CASCADE;
DROP TABLE IF EXISTS session_schedule_proposals CASCADE;
DROP TABLE IF EXISTS exchange_confirmations CASCADE;
DROP TABLE IF EXISTS session_confirmations CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS exchange_status_history CASCADE;
DROP TABLE IF EXISTS exchange_skill_commitments CASCADE;
DROP TABLE IF EXISTS exchanges CASCADE;
DROP TABLE IF EXISTS exchange_request_status_history CASCADE;
DROP TABLE IF EXISTS exchange_requests CASCADE;
DROP TABLE IF EXISTS match_factors CASCADE;
DROP TABLE IF EXISTS match_skill_links CASCADE;
DROP TABLE IF EXISTS matches CASCADE;
DROP TABLE IF EXISTS user_pause_periods CASCADE;
DROP TABLE IF EXISTS meeting_places CASCADE;
DROP TABLE IF EXISTS user_availability_slots CASCADE;
DROP TABLE IF EXISTS user_lesson_formats CASCADE;
DROP TABLE IF EXISTS user_lesson_preferences CASCADE;
DROP TABLE IF EXISTS user_learning_skills CASCADE;
DROP TABLE IF EXISTS user_teaching_skills CASCADE;
DROP TABLE IF EXISTS skills CASCADE;
DROP TABLE IF EXISTS skill_categories CASCADE;
DROP TABLE IF EXISTS user_profile_settings CASCADE;
DROP TABLE IF EXISTS user_statistics CASCADE;
DROP TABLE IF EXISTS user_verifications CASCADE;
DROP TABLE IF EXISTS one_time_tokens CASCADE;
DROP TABLE IF EXISTS user_sessions CASCADE;
DROP TABLE IF EXISTS auth_identities CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS faculties CASCADE;
DROP TABLE IF EXISTS universities CASCADE;
DROP TYPE IF EXISTS sync_status;
DROP TYPE IF EXISTS calendar_provider;
DROP TYPE IF EXISTS notification_event_type;
DROP TYPE IF EXISTS file_kind;
DROP TYPE IF EXISTS message_type;
DROP TYPE IF EXISTS conversation_type;
DROP TYPE IF EXISTS schedule_proposal_status;
DROP TYPE IF EXISTS session_status;
DROP TYPE IF EXISTS exchange_status;
DROP TYPE IF EXISTS exchange_request_status;
DROP TYPE IF EXISTS match_factor_type;
DROP TYPE IF EXISTS match_status;
DROP TYPE IF EXISTS request_scope;
DROP TYPE IF EXISTS lesson_format;
DROP TYPE IF EXISTS skill_level;
DROP TYPE IF EXISTS verification_status;
DROP TYPE IF EXISTS verification_type;
DROP TYPE IF EXISTS report_status;
DROP TYPE IF EXISTS confirmation_kind;
DROP TYPE IF EXISTS one_time_token_purpose;
DROP TYPE IF EXISTS account_status;
DROP TYPE IF EXISTS user_role;
DROP TYPE IF EXISTS auth_provider;
