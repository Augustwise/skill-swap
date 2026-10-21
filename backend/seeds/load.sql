-- 1000 verified students for the search and matches load test.
-- Run after seeds/demo.sql: it uses the demo university, catalog skills and demo password.
-- Every insert skips rows that already exist, so the seed can be run again.
INSERT INTO users (id, university_id, email, first_name, last_name, academic_year, city, bio, terms_accepted_at)
SELECT
  format('70000000-0000-0000-0000-%s', lpad(n::text, 12, '0'))::uuid,
  '10000000-0000-0000-0000-000000000001',
  format('load.%s@students.example.test', lpad(n::text, 4, '0')),
  (ARRAY['Олена', 'Ігор', 'Марія', 'Богдан', 'Юлія', 'Назар', 'Ірина', 'Максим', 'Дарина', 'Роман'])[1 + n % 10],
  (ARRAY['Шевченко', 'Бойко', 'Ткаченко', 'Кравченко', 'Олійник', 'Лисенко', 'Мороз', 'Павленко'])[1 + n % 8],
  1 + n % 4,
  CASE WHEN n % 2 = 0 THEN 'Київ' ELSE 'Львів' END,
  'Тестовий профіль для перевірки навантаження.',
  now()
FROM generate_series(1, 1000) AS n
ON CONFLICT (id) DO NOTHING;

INSERT INTO auth_identities (user_id, provider, provider_subject, password_hash, password_changed_at)
SELECT id, 'LOCAL', id::text, '$2a$12$jP.A1C2mRlD84H6bscBd0uD4vSUL7nckKTSQAkn3qWrriVWcmr8h6', now()
FROM users WHERE email LIKE 'load.%@students.example.test'
ON CONFLICT DO NOTHING;

INSERT INTO user_verifications (user_id, type, identifier, status, verified_at)
SELECT id, 'UNIVERSITY_EMAIL', email, 'VERIFIED', now()
FROM users WHERE email LIKE 'load.%@students.example.test'
ON CONFLICT DO NOTHING;

-- Everyone studies online; every third student also meets offline.
INSERT INTO user_lesson_formats (user_id, format)
SELECT format('70000000-0000-0000-0000-%s', lpad(n::text, 12, '0'))::uuid, 'ONLINE'::lesson_format
FROM generate_series(1, 1000) AS n
UNION ALL
SELECT format('70000000-0000-0000-0000-%s', lpad(n::text, 12, '0'))::uuid, 'OFFLINE'::lesson_format
FROM generate_series(1, 1000) AS n WHERE n % 3 = 0
ON CONFLICT DO NOTHING;

-- Two taught (+0, +5) and two wanted (+2, +3) skills out of the 14 demo skills. The lists
-- never share a skill, and students whose numbers differ by 2 or 3 (mod 14) match mutually,
-- so everyone has about 280 mutual matches.
INSERT INTO user_teaching_skills (user_id, skill_id, level)
SELECT
  format('70000000-0000-0000-0000-%s', lpad(n::text, 12, '0'))::uuid,
  format('30000000-0000-0000-0000-%s', lpad((1 + (n + shift) % 14)::text, 12, '0'))::uuid,
  (ARRAY['BEGINNER', 'INTERMEDIATE', 'ADVANCED'])[1 + (n + shift) % 3]::skill_level
FROM generate_series(1, 1000) AS n, unnest(ARRAY[0, 5]) AS shift
ON CONFLICT (user_id, skill_id) DO NOTHING;

INSERT INTO user_learning_skills (user_id, skill_id, current_level)
SELECT
  format('70000000-0000-0000-0000-%s', lpad(n::text, 12, '0'))::uuid,
  format('30000000-0000-0000-0000-%s', lpad((1 + (n + shift) % 14)::text, 12, '0'))::uuid,
  'BEGINNER'
FROM generate_series(1, 1000) AS n, unnest(ARRAY[2, 3]) AS shift
ON CONFLICT (user_id, skill_id) DO NOTHING;
