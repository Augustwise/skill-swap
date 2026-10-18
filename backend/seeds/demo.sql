-- Local, repeatable reference data for sprint presentations.
-- This is deliberately separate from migrations: real university data can replace it.
-- Every insert skips rows that already exist, so the seed can be run again.
INSERT INTO universities (id, name, short_name, email_domain, city)
VALUES
  ('10000000-0000-0000-0000-000000000001', 'Демонстраційний університет', 'Demo Uni', 'students.example.test', 'Київ'),
  ('10000000-0000-0000-0000-000000000002', 'Другий демонстраційний університет', 'Demo Uni 2', 'students2.example.test', 'Львів')
ON CONFLICT (id) DO NOTHING;

INSERT INTO faculties (id, university_id, name, code)
VALUES
  ('50000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', 'Факультет інформатики та обчислювальної техніки', 'ФІОТ'),
  ('50000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001', 'Факультет прикладної математики', 'ФПМ'),
  ('50000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000001', 'Факультет лінгвістики', 'ФЛ'),
  ('50000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000001', 'Факультет дизайну', 'ФД'),
  ('50000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000002', 'Факультет економіки', 'ФЕ')
ON CONFLICT (id) DO NOTHING;

INSERT INTO skill_categories (id, name, slug, sort_order)
VALUES
  ('20000000-0000-0000-0000-000000000001', 'Музика', 'music', 10),
  ('20000000-0000-0000-0000-000000000002', 'Дизайн', 'design', 20),
  ('20000000-0000-0000-0000-000000000003', 'Програмування', 'programming', 30),
  ('20000000-0000-0000-0000-000000000004', 'Мови', 'languages', 40)
ON CONFLICT (id) DO NOTHING;

INSERT INTO skills (id, category_id, name, slug, description)
VALUES
  ('30000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', 'Гітара', 'guitar', 'Базова та просунута гра на гітарі'),
  ('30000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000002', 'Photoshop', 'photoshop', 'Обробка зображень'),
  ('30000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000002', 'Figma', 'figma', 'Дизайн інтерфейсів'),
  ('30000000-0000-0000-0000-000000000004', '20000000-0000-0000-0000-000000000003', 'Go', 'go', 'Основи мови Go'),
  ('30000000-0000-0000-0000-000000000005', '20000000-0000-0000-0000-000000000002', 'Illustrator', 'illustrator', 'Векторна графіка'),
  ('30000000-0000-0000-0000-000000000006', '20000000-0000-0000-0000-000000000002', 'UI/UX-дизайн', 'ui-ux-design', 'Проєктування інтерфейсів і досвіду користувача'),
  ('30000000-0000-0000-0000-000000000007', '20000000-0000-0000-0000-000000000002', 'Web-дизайн', 'web-design', 'Дизайн вебсайтів'),
  ('30000000-0000-0000-0000-000000000008', '20000000-0000-0000-0000-000000000002', 'Motion-дизайн', 'motion-design', 'Анімація інтерфейсів і графіки'),
  ('30000000-0000-0000-0000-000000000009', '20000000-0000-0000-0000-000000000002', '3D · Blender', 'blender-3d', 'Тривимірне моделювання в Blender'),
  ('30000000-0000-0000-0000-000000000010', '20000000-0000-0000-0000-000000000002', 'Ілюстрація', 'illustration', 'Цифрова та традиційна ілюстрація'),
  ('30000000-0000-0000-0000-000000000011', '20000000-0000-0000-0000-000000000002', 'Типографіка', 'typography', 'Робота зі шрифтами'),
  ('30000000-0000-0000-0000-000000000012', '20000000-0000-0000-0000-000000000002', 'Брендинг', 'branding', 'Айдентика та фірмовий стиль'),
  ('30000000-0000-0000-0000-000000000013', '20000000-0000-0000-0000-000000000003', 'JavaScript', 'javascript', 'Основи мови JavaScript'),
  ('30000000-0000-0000-0000-000000000014', '20000000-0000-0000-0000-000000000004', 'Англійська', 'english', 'Розмовна англійська мова')
ON CONFLICT (id) DO NOTHING;

-- Sprint 4 (FR-04, FR-05): demo students for search and mutual matches.
INSERT INTO users (id, university_id, faculty_id, email, first_name, last_name, academic_year, city, bio, terms_accepted_at)
VALUES
  ('60000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000003',
   'demo.olha@students.example.test', 'Ольга', 'Гнатюк', 2, 'Київ', 'Граю на гітарі вісім років, хочу навчитися обробляти фото.', now()),
  ('60000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000004',
   'demo.andrii@students.example.test', 'Андрій', 'Коваль', 3, 'Київ', 'Дизайнер, працюю в Photoshop та Illustrator. Мрію про гітару.', now()),
  ('60000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000004',
   'demo.marko@students.example.test', 'Марко', 'Савчук', 4, 'Львів', 'Ретушую портрети, готуюся до IELTS.', now()),
  ('60000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000003',
   'demo.iryna@students.example.test', 'Ірина', 'Бондар', 3, 'Київ', 'Викладаю англійську, малюю інтерфейси у Figma.', now()),
  ('60000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000001',
   'demo.dmytro@students.example.test', 'Дмитро', 'Мельник', 4, 'Київ', 'Бекенд на Go, трохи фронтенду.', now()),
  ('60000000-0000-0000-0000-000000000006', '10000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000002',
   'demo.sofiia@students.example.test', 'Софія', 'Ткаченко', 1, 'Львів', 'Займаюся лише наживо, у Львові.', now()),
  ('60000000-0000-0000-0000-000000000007', '10000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000004',
   'demo.taras@students.example.test', 'Тарас', 'Лисенко', 2, 'Київ', 'Зустрічаюся лише офлайн у Києві.', now()),
  ('60000000-0000-0000-0000-000000000008', '10000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000004',
   'demo.nataliia@students.example.test', 'Наталія', 'Романенко', 3, 'Київ', 'Профіль прихований від пошуку.', now()),
  ('60000000-0000-0000-0000-000000000009', '10000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000004',
   'demo.viktor@students.example.test', 'Віктор', 'Павленко', 1, 'Київ', 'Ще не підтвердив пошту.', now()),
  ('60000000-0000-0000-0000-000000000010', '10000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000004',
   'demo.oleh@students.example.test', 'Олег', 'Кравець', 2, 'Київ', 'Заблокований Ольгою.', now()),
  ('60000000-0000-0000-0000-000000000011', '10000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000004',
   'demo.kateryna@students.example.test', 'Катерина', 'Шевчук', 4, 'Київ', 'Навчаю Photoshop, поки нічого не вивчаю.', now())
ON CONFLICT (id) DO NOTHING;

INSERT INTO auth_identities (user_id, provider, provider_subject, password_hash, password_changed_at)
SELECT id, 'LOCAL', id::text, '$2a$12$jP.A1C2mRlD84H6bscBd0uD4vSUL7nckKTSQAkn3qWrriVWcmr8h6', now()
FROM users WHERE id BETWEEN '60000000-0000-0000-0000-000000000001' AND '60000000-0000-0000-0000-000000000011'
ON CONFLICT DO NOTHING;

-- Viktor (09) keeps a pending verification; everyone else has a verified email.
INSERT INTO user_verifications (user_id, type, identifier, status, verified_at)
SELECT id, 'UNIVERSITY_EMAIL', email,
  CASE WHEN id = '60000000-0000-0000-0000-000000000009' THEN 'PENDING' ELSE 'VERIFIED' END::verification_status,
  CASE WHEN id = '60000000-0000-0000-0000-000000000009' THEN NULL ELSE now() END
FROM users WHERE id BETWEEN '60000000-0000-0000-0000-000000000001' AND '60000000-0000-0000-0000-000000000011'
ON CONFLICT DO NOTHING;

INSERT INTO user_lesson_formats (user_id, format)
VALUES
  ('60000000-0000-0000-0000-000000000001', 'ONLINE'),
  ('60000000-0000-0000-0000-000000000001', 'OFFLINE'),
  ('60000000-0000-0000-0000-000000000002', 'ONLINE'),
  ('60000000-0000-0000-0000-000000000002', 'OFFLINE'),
  ('60000000-0000-0000-0000-000000000003', 'ONLINE'),
  ('60000000-0000-0000-0000-000000000004', 'ONLINE'),
  ('60000000-0000-0000-0000-000000000005', 'ONLINE'),
  ('60000000-0000-0000-0000-000000000006', 'OFFLINE'),
  ('60000000-0000-0000-0000-000000000007', 'OFFLINE'),
  ('60000000-0000-0000-0000-000000000008', 'ONLINE'),
  ('60000000-0000-0000-0000-000000000009', 'ONLINE'),
  ('60000000-0000-0000-0000-000000000010', 'ONLINE'),
  ('60000000-0000-0000-0000-000000000011', 'ONLINE')
ON CONFLICT DO NOTHING;

-- Skills: 01 guitar, 02 Photoshop, 03 Figma, 04 Go, 05 Illustrator, 13 JavaScript, 14 English.
INSERT INTO user_teaching_skills (user_id, skill_id, level)
VALUES
  ('60000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', 'ADVANCED'),
  ('60000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000002', 'ADVANCED'),
  ('60000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000005', 'INTERMEDIATE'),
  ('60000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000002', 'INTERMEDIATE'),
  ('60000000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000014', 'ADVANCED'),
  ('60000000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000003', 'INTERMEDIATE'),
  ('60000000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000004', 'ADVANCED'),
  ('60000000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000013', 'INTERMEDIATE'),
  ('60000000-0000-0000-0000-000000000006', '30000000-0000-0000-0000-000000000001', 'INTERMEDIATE'),
  ('60000000-0000-0000-0000-000000000007', '30000000-0000-0000-0000-000000000002', 'BEGINNER'),
  ('60000000-0000-0000-0000-000000000008', '30000000-0000-0000-0000-000000000002', 'ADVANCED'),
  ('60000000-0000-0000-0000-000000000009', '30000000-0000-0000-0000-000000000002', 'INTERMEDIATE'),
  ('60000000-0000-0000-0000-000000000010', '30000000-0000-0000-0000-000000000002', 'INTERMEDIATE'),
  ('60000000-0000-0000-0000-000000000011', '30000000-0000-0000-0000-000000000002', 'ADVANCED')
ON CONFLICT (user_id, skill_id) DO NOTHING;

INSERT INTO user_learning_skills (user_id, skill_id, current_level)
VALUES
  ('60000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000002', 'BEGINNER'),
  ('60000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000001', 'BEGINNER'),
  ('60000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000014', 'INTERMEDIATE'),
  ('60000000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000002', 'BEGINNER'),
  ('60000000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000004', 'BEGINNER'),
  ('60000000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000003', 'BEGINNER'),
  ('60000000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000014', 'INTERMEDIATE'),
  ('60000000-0000-0000-0000-000000000006', '30000000-0000-0000-0000-000000000002', 'BEGINNER'),
  ('60000000-0000-0000-0000-000000000007', '30000000-0000-0000-0000-000000000001', 'BEGINNER'),
  ('60000000-0000-0000-0000-000000000008', '30000000-0000-0000-0000-000000000001', 'BEGINNER'),
  ('60000000-0000-0000-0000-000000000009', '30000000-0000-0000-0000-000000000001', 'BEGINNER'),
  ('60000000-0000-0000-0000-000000000010', '30000000-0000-0000-0000-000000000001', 'BEGINNER')
ON CONFLICT (user_id, skill_id) DO NOTHING;

INSERT INTO user_profile_settings (user_id, is_discoverable)
VALUES ('60000000-0000-0000-0000-000000000008', false)
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO user_blocks (blocker_id, blocked_id)
VALUES ('60000000-0000-0000-0000-000000000001', '60000000-0000-0000-0000-000000000010')
ON CONFLICT DO NOTHING;

-- Sprint 5 (FR-06, FR-07): Dmytro does not accept new requests, and Olha has one
-- incoming request from Taras.
INSERT INTO user_profile_settings (user_id, accepts_requests)
VALUES ('60000000-0000-0000-0000-000000000005', false)
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO exchange_requests (id, requester_id, recipient_id,
  requester_teaching_skill_id, recipient_learning_skill_id, recipient_teaching_skill_id, requester_learning_skill_id,
  format, total_sessions, requester_sessions, recipient_sessions,
  requester_duration_minutes, recipient_duration_minutes, message)
SELECT '70000000-0000-0000-0000-000000000001', taras_teaches.user_id, olha_teaches.user_id,
  taras_teaches.id, olha_learns.id, olha_teaches.id, taras_learns.id,
  'OFFLINE', 4, 2, 2, 60, 60, 'Привіт! Покажу Photoshop на живих прикладах, а ти мене навчиш гітари. Зустрінемося в Києві?'
FROM user_teaching_skills taras_teaches
JOIN user_learning_skills olha_learns
  ON olha_learns.user_id = '60000000-0000-0000-0000-000000000001' AND olha_learns.skill_id = '30000000-0000-0000-0000-000000000002'
JOIN user_teaching_skills olha_teaches
  ON olha_teaches.user_id = '60000000-0000-0000-0000-000000000001' AND olha_teaches.skill_id = '30000000-0000-0000-0000-000000000001'
JOIN user_learning_skills taras_learns
  ON taras_learns.user_id = '60000000-0000-0000-0000-000000000007' AND taras_learns.skill_id = '30000000-0000-0000-0000-000000000001'
WHERE taras_teaches.user_id = '60000000-0000-0000-0000-000000000007' AND taras_teaches.skill_id = '30000000-0000-0000-0000-000000000002'
ON CONFLICT DO NOTHING;

INSERT INTO exchange_request_status_history (id, request_id, status, changed_by_user_id, created_at)
SELECT '71000000-0000-0000-0000-000000000001', id, 'PENDING', requester_id, created_at
FROM exchange_requests WHERE id = '70000000-0000-0000-0000-000000000001'
ON CONFLICT (id) DO NOTHING;

-- Sprint 6 (FR-08, FR-13): private chat and notifications for Taras's request to Olha.
INSERT INTO conversations (id, type, exchange_request_id, exchange_id, created_by_user_id, created_at, last_message_at)
SELECT '72000000-0000-0000-0000-000000000001',
  CASE WHEN e.id IS NOT NULL THEN 'EXCHANGE'::conversation_type ELSE 'REQUEST'::conversation_type END,
  r.id, e.id, r.requester_id, r.created_at, r.created_at
FROM exchange_requests r
LEFT JOIN exchanges e ON e.source_request_id = r.id
WHERE r.id = '70000000-0000-0000-0000-000000000001'
ON CONFLICT DO NOTHING;

INSERT INTO conversation_participants (conversation_id, user_id, joined_at)
SELECT c.id, p.user_id, c.created_at
FROM conversations c
JOIN exchange_requests r ON r.id = c.exchange_request_id
CROSS JOIN LATERAL (VALUES (r.requester_id), (r.recipient_id)) AS p(user_id)
WHERE c.id = '72000000-0000-0000-0000-000000000001'
ON CONFLICT (conversation_id, user_id) DO NOTHING;

INSERT INTO messages (id, conversation_id, author_id, type, body, created_at)
SELECT '73000000-0000-0000-0000-000000000001', c.id, r.requester_id, 'TEXT',
  'Привіт! Можемо зустрітися біля корпусу в будні після пар і домовитися про перше заняття.',
  c.created_at
FROM conversations c
JOIN exchange_requests r ON r.id = c.exchange_request_id
WHERE c.id = '72000000-0000-0000-0000-000000000001'
ON CONFLICT (id) DO NOTHING;

INSERT INTO notifications (id, recipient_id, actor_id, event_type, event_key, title, body,
  exchange_request_id, conversation_id, created_at)
SELECT '74000000-0000-0000-0000-000000000001', r.recipient_id, r.requester_id,
  'EXCHANGE_REQUEST', 'exchange-request:70000000-0000-0000-0000-000000000001',
  'Новий запит на обмін',
  'Тарас Лисенко пропонує обмін: Photoshop ↔ Гітара.',
  r.id, c.id, r.created_at
FROM exchange_requests r
JOIN conversations c ON c.exchange_request_id = r.id
WHERE r.id = '70000000-0000-0000-0000-000000000001'
ON CONFLICT (recipient_id, event_key) DO NOTHING;

INSERT INTO notifications (id, recipient_id, actor_id, event_type, event_key, title, body,
  exchange_request_id, conversation_id, message_id, created_at)
SELECT '74000000-0000-0000-0000-000000000002', r.recipient_id, r.requester_id,
  'CHAT_MESSAGE', 'chat-message:73000000-0000-0000-0000-000000000001',
  'Нове повідомлення',
  'Тарас Лисенко: Привіт! Можемо зустрітися біля корпусу в будні після пар і домовитися про перше заняття.',
  r.id, c.id, m.id, m.created_at
FROM exchange_requests r
JOIN conversations c ON c.exchange_request_id = r.id
JOIN messages m ON m.id = '73000000-0000-0000-0000-000000000001'
WHERE r.id = '70000000-0000-0000-0000-000000000001'
ON CONFLICT (recipient_id, event_key) DO NOTHING;

-- Sprint 7 (FR-09, FR-10): active exchange and lesson schedule between Iryna and Marko.
INSERT INTO exchange_requests (id, requester_id, recipient_id,
  requester_teaching_skill_id, recipient_learning_skill_id, recipient_teaching_skill_id, requester_learning_skill_id,
  format, total_sessions, requester_sessions, recipient_sessions,
  requester_duration_minutes, recipient_duration_minutes, message, status, created_at, responded_at)
SELECT '70000000-0000-0000-0000-000000000002', iryna_teaches.user_id, marko_teaches.user_id,
  iryna_teaches.id, marko_learns.id, marko_teaches.id, iryna_learns.id,
  'ONLINE', 4, 2, 2, 60, 60,
  'Привіт! Можу допомогти з розмовною англійською для IELTS в обмін на ретуш у Photoshop.',
  'ACCEPTED', now() - interval '2 days', now() - interval '1 day'
FROM user_teaching_skills iryna_teaches
JOIN user_learning_skills marko_learns
  ON marko_learns.user_id = '60000000-0000-0000-0000-000000000003' AND marko_learns.skill_id = '30000000-0000-0000-0000-000000000014'
JOIN user_teaching_skills marko_teaches
  ON marko_teaches.user_id = '60000000-0000-0000-0000-000000000003' AND marko_teaches.skill_id = '30000000-0000-0000-0000-000000000002'
JOIN user_learning_skills iryna_learns
  ON iryna_learns.user_id = '60000000-0000-0000-0000-000000000004' AND iryna_learns.skill_id = '30000000-0000-0000-0000-000000000002'
WHERE iryna_teaches.user_id = '60000000-0000-0000-0000-000000000004' AND iryna_teaches.skill_id = '30000000-0000-0000-0000-000000000014'
ON CONFLICT DO NOTHING;

INSERT INTO exchange_request_status_history (id, request_id, status, changed_by_user_id, created_at)
SELECT '71000000-0000-0000-0000-000000000002', id, 'PENDING', requester_id, created_at
FROM exchange_requests WHERE id = '70000000-0000-0000-0000-000000000002'
ON CONFLICT (id) DO NOTHING;

INSERT INTO exchange_request_status_history (id, request_id, status, changed_by_user_id, created_at)
SELECT '71000000-0000-0000-0000-000000000003', id, 'ACCEPTED', recipient_id, COALESCE(responded_at, now())
FROM exchange_requests WHERE id = '70000000-0000-0000-0000-000000000002'
ON CONFLICT (id) DO NOTHING;

INSERT INTO exchanges (id, source_request_id, user_a_id, user_b_id, status, total_sessions, format, started_at, created_at, updated_at)
SELECT '80000000-0000-0000-0000-000000000001', id, requester_id, recipient_id, 'ACTIVE', total_sessions, format,
  COALESCE(responded_at, now()), COALESCE(responded_at, now()), COALESCE(responded_at, now())
FROM exchange_requests WHERE id = '70000000-0000-0000-0000-000000000002'
ON CONFLICT DO NOTHING;

INSERT INTO exchange_skill_commitments (id, exchange_id, teacher_id, learner_id, teaching_skill_id,
  learning_skill_id, planned_sessions, duration_minutes, skill_name_snapshot,
  teaching_level_snapshot, learning_level_snapshot)
SELECT x.commitment_id::uuid, e.id, t.user_id, l.user_id, t.id, l.id,
  x.sessions, x.duration, s.name, t.level, l.current_level
FROM exchanges e
JOIN exchange_requests r ON r.id = e.source_request_id
CROSS JOIN LATERAL (VALUES
  ('80200000-0000-0000-0000-000000000001', r.requester_teaching_skill_id, r.recipient_learning_skill_id, r.requester_sessions, r.requester_duration_minutes),
  ('80200000-0000-0000-0000-000000000002', r.recipient_teaching_skill_id, r.requester_learning_skill_id, r.recipient_sessions, r.recipient_duration_minutes)
) AS x (commitment_id, teaching_id, learning_id, sessions, duration)
JOIN user_teaching_skills t ON t.id = x.teaching_id
JOIN user_learning_skills l ON l.id = x.learning_id
JOIN skills s ON s.id = t.skill_id
WHERE e.id = '80000000-0000-0000-0000-000000000001'
ON CONFLICT DO NOTHING;

INSERT INTO exchange_status_history (id, exchange_id, status, changed_by_user_id, created_at)
SELECT '80100000-0000-0000-0000-000000000001', e.id, 'ACTIVE', e.user_b_id, e.created_at
FROM exchanges e WHERE e.id = '80000000-0000-0000-0000-000000000001'
ON CONFLICT (id) DO NOTHING;

INSERT INTO conversations (id, type, exchange_request_id, exchange_id, created_by_user_id, created_at, last_message_at)
SELECT '72000000-0000-0000-0000-000000000002', 'EXCHANGE', r.id, e.id, r.requester_id, r.created_at, e.created_at
FROM exchanges e
JOIN exchange_requests r ON r.id = e.source_request_id
WHERE e.id = '80000000-0000-0000-0000-000000000001'
ON CONFLICT DO NOTHING;

INSERT INTO conversation_participants (conversation_id, user_id, joined_at)
SELECT c.id, p.user_id, c.created_at
FROM conversations c
JOIN exchanges e ON e.id = c.exchange_id
CROSS JOIN LATERAL (VALUES (e.user_a_id), (e.user_b_id)) AS p(user_id)
WHERE c.id = '72000000-0000-0000-0000-000000000002'
ON CONFLICT (conversation_id, user_id) DO NOTHING;

-- Four planned sessions for Iryna <-> Marko (only inserted if the exchange has no sessions yet).
INSERT INTO sessions (id, exchange_id, plan_order, title, description, teacher_id, learner_id,
  status, scheduled_start, scheduled_end, format, meeting_url, created_at, updated_at)
SELECT s.session_id::uuid, e.id, s.plan_order, s.title, s.description, s.teacher_id::uuid, s.learner_id::uuid,
  s.status::session_status, s.scheduled_start, s.scheduled_end, 'ONLINE', s.meeting_url, e.created_at, e.created_at
FROM exchanges e
CROSS JOIN LATERAL (VALUES
  ('81000000-0000-0000-0000-000000000001', 1::smallint, 'Англійська — розмовна практика для IELTS',
   'Розбираємо частину Speaking та типові запитання.',
   '60000000-0000-0000-0000-000000000004', '60000000-0000-0000-0000-000000000003',
   'SCHEDULED', date_trunc('hour', now()) + interval '2 days', date_trunc('hour', now()) + interval '2 days 1 hour',
   'https://meet.example.test/skillswap-english-1'),
  ('81000000-0000-0000-0000-000000000002', 2::smallint, 'Англійська — заняття 2',
   NULL,
   '60000000-0000-0000-0000-000000000004', '60000000-0000-0000-0000-000000000003',
   'DRAFT', NULL::timestamptz, NULL::timestamptz, NULL::text),
  ('81000000-0000-0000-0000-000000000003', 3::smallint, 'Photoshop — ретуш і робота з масками',
   'Практика недеструктивної ретуші портретів.',
   '60000000-0000-0000-0000-000000000003', '60000000-0000-0000-0000-000000000004',
   'PROPOSED', date_trunc('hour', now()) + interval '4 days', date_trunc('hour', now()) + interval '4 days 1 hour',
   'https://meet.example.test/skillswap-photoshop-1'),
  ('81000000-0000-0000-0000-000000000004', 4::smallint, 'Photoshop — заняття 2',
   NULL,
   '60000000-0000-0000-0000-000000000003', '60000000-0000-0000-0000-000000000004',
   'DRAFT', NULL::timestamptz, NULL::timestamptz, NULL::text)
) AS s (session_id, plan_order, title, description, teacher_id, learner_id, status, scheduled_start, scheduled_end, meeting_url)
WHERE e.id = '80000000-0000-0000-0000-000000000001'
  AND NOT EXISTS (SELECT 1 FROM sessions existing WHERE existing.exchange_id = e.id)
ON CONFLICT DO NOTHING;

INSERT INTO session_schedule_proposals (id, session_id, proposed_by_user_id, proposed_start, proposed_end,
  format, meeting_url, status, responded_by_user_id, responded_at, created_at)
SELECT '82000000-0000-0000-0000-000000000001', s.id, s.teacher_id, s.scheduled_start, s.scheduled_end,
  s.format, s.meeting_url, 'ACCEPTED', s.learner_id, s.created_at, s.created_at
FROM sessions s
WHERE s.id = '81000000-0000-0000-0000-000000000001' AND s.status = 'SCHEDULED'
ON CONFLICT (id) DO NOTHING;

INSERT INTO session_confirmations (session_id, user_id, kind, confirmed_at)
SELECT s.id, p.user_id, 'SCHEDULE', s.created_at
FROM sessions s
CROSS JOIN LATERAL (VALUES (s.teacher_id), (s.learner_id)) AS p(user_id)
WHERE s.id = '81000000-0000-0000-0000-000000000001' AND s.status = 'SCHEDULED'
ON CONFLICT (session_id, user_id, kind) DO NOTHING;

INSERT INTO session_schedule_proposals (id, session_id, proposed_by_user_id, proposed_start, proposed_end,
  format, meeting_url, status, created_at)
SELECT '82000000-0000-0000-0000-000000000002', s.id, s.teacher_id, s.scheduled_start, s.scheduled_end,
  s.format, s.meeting_url, 'PENDING', s.created_at
FROM sessions s
WHERE s.id = '81000000-0000-0000-0000-000000000003' AND s.status = 'PROPOSED'
ON CONFLICT (id) DO NOTHING;

INSERT INTO session_confirmations (session_id, user_id, kind, confirmed_at)
SELECT s.id, s.teacher_id, 'SCHEDULE', s.created_at
FROM sessions s
WHERE s.id = '81000000-0000-0000-0000-000000000003' AND s.status = 'PROPOSED'
ON CONFLICT (session_id, user_id, kind) DO NOTHING;

INSERT INTO notifications (id, recipient_id, actor_id, event_type, event_key, title, body,
  exchange_id, session_id, conversation_id, created_at)
SELECT '74000000-0000-0000-0000-000000000003', s.learner_id, s.teacher_id,
  'SESSION_UPDATED', 'session-proposal:82000000-0000-0000-0000-000000000002',
  'Нова пропозиція заняття',
  'Марко Савчук пропонує заняття «Photoshop — ретуш і робота з масками».',
  s.exchange_id, s.id, c.id, s.created_at
FROM sessions s
JOIN conversations c ON c.exchange_id = s.exchange_id
WHERE s.id = '81000000-0000-0000-0000-000000000003' AND s.status = 'PROPOSED'
ON CONFLICT (recipient_id, event_key) DO NOTHING;

-- Sprint 8: Iryna and Dmytro have one Figma lesson completed and one Go lesson that only
-- Dmytro has confirmed, so Iryna can confirm it and both can finish the exchange.
INSERT INTO exchange_requests (id, requester_id, recipient_id,
  requester_teaching_skill_id, recipient_learning_skill_id, recipient_teaching_skill_id, requester_learning_skill_id,
  format, total_sessions, requester_sessions, recipient_sessions,
  requester_duration_minutes, recipient_duration_minutes, message, status, created_at, responded_at)
SELECT '70000000-0000-0000-0000-000000000003', iryna_teaches.user_id, dmytro_teaches.user_id,
  iryna_teaches.id, dmytro_learns.id, dmytro_teaches.id, iryna_learns.id,
  'ONLINE', 2, 1, 1, 60, 60,
  'Привіт! Покажу основи Figma, а ти мені — Go.',
  'ACCEPTED', now() - interval '10 days', now() - interval '9 days'
FROM user_teaching_skills iryna_teaches
JOIN user_learning_skills dmytro_learns
  ON dmytro_learns.user_id = '60000000-0000-0000-0000-000000000005' AND dmytro_learns.skill_id = '30000000-0000-0000-0000-000000000003'
JOIN user_teaching_skills dmytro_teaches
  ON dmytro_teaches.user_id = '60000000-0000-0000-0000-000000000005' AND dmytro_teaches.skill_id = '30000000-0000-0000-0000-000000000004'
JOIN user_learning_skills iryna_learns
  ON iryna_learns.user_id = '60000000-0000-0000-0000-000000000004' AND iryna_learns.skill_id = '30000000-0000-0000-0000-000000000004'
WHERE iryna_teaches.user_id = '60000000-0000-0000-0000-000000000004' AND iryna_teaches.skill_id = '30000000-0000-0000-0000-000000000003'
ON CONFLICT DO NOTHING;

INSERT INTO exchange_request_status_history (id, request_id, status, changed_by_user_id, created_at)
SELECT h.id::uuid, r.id, h.status::exchange_request_status, h.user_id, h.created_at
FROM exchange_requests r
CROSS JOIN LATERAL (VALUES
  ('71000000-0000-0000-0000-000000000004', 'PENDING', r.requester_id, r.created_at),
  ('71000000-0000-0000-0000-000000000005', 'ACCEPTED', r.recipient_id, r.responded_at)
) AS h (id, status, user_id, created_at)
WHERE r.id = '70000000-0000-0000-0000-000000000003'
ON CONFLICT (id) DO NOTHING;

INSERT INTO exchanges (id, source_request_id, user_a_id, user_b_id, status, total_sessions, format, started_at, created_at, updated_at)
SELECT '80000000-0000-0000-0000-000000000002', id, requester_id, recipient_id, 'ACTIVE', total_sessions, format,
  responded_at, responded_at, responded_at
FROM exchange_requests WHERE id = '70000000-0000-0000-0000-000000000003'
ON CONFLICT DO NOTHING;

INSERT INTO exchange_skill_commitments (id, exchange_id, teacher_id, learner_id, teaching_skill_id,
  learning_skill_id, planned_sessions, duration_minutes, skill_name_snapshot,
  teaching_level_snapshot, learning_level_snapshot)
SELECT x.commitment_id::uuid, e.id, t.user_id, l.user_id, t.id, l.id,
  x.sessions, x.duration, s.name, t.level, l.current_level
FROM exchanges e
JOIN exchange_requests r ON r.id = e.source_request_id
CROSS JOIN LATERAL (VALUES
  ('80200000-0000-0000-0000-000000000003', r.requester_teaching_skill_id, r.recipient_learning_skill_id, r.requester_sessions, r.requester_duration_minutes),
  ('80200000-0000-0000-0000-000000000004', r.recipient_teaching_skill_id, r.requester_learning_skill_id, r.recipient_sessions, r.recipient_duration_minutes)
) AS x (commitment_id, teaching_id, learning_id, sessions, duration)
JOIN user_teaching_skills t ON t.id = x.teaching_id
JOIN user_learning_skills l ON l.id = x.learning_id
JOIN skills s ON s.id = t.skill_id
WHERE e.id = '80000000-0000-0000-0000-000000000002'
ON CONFLICT DO NOTHING;

INSERT INTO exchange_status_history (id, exchange_id, status, changed_by_user_id, created_at)
SELECT '80100000-0000-0000-0000-000000000002', e.id, 'ACTIVE', e.user_b_id, e.created_at
FROM exchanges e WHERE e.id = '80000000-0000-0000-0000-000000000002'
ON CONFLICT (id) DO NOTHING;

INSERT INTO conversations (id, type, exchange_request_id, exchange_id, created_by_user_id, created_at, last_message_at)
SELECT '72000000-0000-0000-0000-000000000003', 'EXCHANGE', r.id, e.id, r.requester_id, r.created_at, e.created_at
FROM exchanges e
JOIN exchange_requests r ON r.id = e.source_request_id
WHERE e.id = '80000000-0000-0000-0000-000000000002'
ON CONFLICT DO NOTHING;

INSERT INTO conversation_participants (conversation_id, user_id, joined_at)
SELECT c.id, p.user_id, c.created_at
FROM conversations c
JOIN exchanges e ON e.id = c.exchange_id
CROSS JOIN LATERAL (VALUES (e.user_a_id), (e.user_b_id)) AS p(user_id)
WHERE c.id = '72000000-0000-0000-0000-000000000003'
ON CONFLICT (conversation_id, user_id) DO NOTHING;

INSERT INTO sessions (id, exchange_id, plan_order, title, teacher_id, learner_id,
  status, scheduled_start, scheduled_end, format, meeting_url, completed_at, created_at, updated_at)
SELECT s.session_id::uuid, e.id, s.plan_order, s.title, s.teacher_id::uuid, s.learner_id::uuid,
  s.status::session_status, s.scheduled_start, s.scheduled_start + interval '1 hour', 'ONLINE', s.meeting_url,
  CASE WHEN s.status = 'COMPLETED' THEN s.scheduled_start + interval '2 hours' END, e.created_at, e.created_at
FROM exchanges e
CROSS JOIN LATERAL (VALUES
  ('81000000-0000-0000-0000-000000000005', 1::smallint, 'Figma — автолейаути та компоненти',
   '60000000-0000-0000-0000-000000000004', '60000000-0000-0000-0000-000000000005',
   'COMPLETED', date_trunc('hour', now()) - interval '5 days', 'https://meet.example.test/skillswap-figma-1'),
  ('81000000-0000-0000-0000-000000000006', 2::smallint, 'Go — горутини та канали',
   '60000000-0000-0000-0000-000000000005', '60000000-0000-0000-0000-000000000004',
   'SCHEDULED', date_trunc('hour', now()) - interval '1 day', 'https://meet.example.test/skillswap-go-1')
) AS s (session_id, plan_order, title, teacher_id, learner_id, status, scheduled_start, meeting_url)
WHERE e.id = '80000000-0000-0000-0000-000000000002'
  AND NOT EXISTS (SELECT 1 FROM sessions existing WHERE existing.exchange_id = e.id)
ON CONFLICT DO NOTHING;

INSERT INTO session_schedule_proposals (id, session_id, proposed_by_user_id, proposed_start, proposed_end,
  format, meeting_url, status, responded_by_user_id, responded_at, created_at)
SELECT p.proposal_id::uuid, s.id, s.teacher_id, s.scheduled_start, s.scheduled_end,
  s.format, s.meeting_url, 'ACCEPTED', s.learner_id, s.created_at, s.created_at
FROM sessions s
JOIN (VALUES
  ('82000000-0000-0000-0000-000000000003', '81000000-0000-0000-0000-000000000005'),
  ('82000000-0000-0000-0000-000000000004', '81000000-0000-0000-0000-000000000006')
) AS p (proposal_id, session_id) ON p.session_id::uuid = s.id
WHERE s.status IN ('SCHEDULED', 'COMPLETED')
ON CONFLICT (id) DO NOTHING;

-- Both lessons have both SCHEDULE confirmations; only the first has both COMPLETION ones.
INSERT INTO session_confirmations (session_id, user_id, kind, confirmed_at)
SELECT s.id, c.user_id, c.kind::confirmation_kind, s.scheduled_end
FROM sessions s
CROSS JOIN LATERAL (VALUES
  (s.teacher_id, 'SCHEDULE'),
  (s.learner_id, 'SCHEDULE'),
  (s.teacher_id, 'COMPLETION'),
  (s.learner_id, CASE WHEN s.status = 'COMPLETED' THEN 'COMPLETION' END)
) AS c (user_id, kind)
WHERE s.id IN ('81000000-0000-0000-0000-000000000005', '81000000-0000-0000-0000-000000000006')
  AND s.status IN ('SCHEDULED', 'COMPLETED')
  AND c.kind IS NOT NULL
ON CONFLICT (session_id, user_id, kind) DO NOTHING;

INSERT INTO notifications (id, recipient_id, actor_id, event_type, event_key, title, body,
  exchange_id, session_id, conversation_id, created_at)
SELECT '74000000-0000-0000-0000-000000000004', s.learner_id, s.teacher_id,
  'SESSION_UPDATED', 'session-completion:82000000-0000-0000-0000-000000000004:' || s.teacher_id,
  'Підтвердіть проведення заняття',
  'Дмитро Мельник підтвердив проведення заняття «Go — горутини та канали». Підтвердіть і ви, щоб його зарахувати.',
  s.exchange_id, s.id, c.id, s.scheduled_end
FROM sessions s
JOIN conversations c ON c.exchange_id = s.exchange_id
WHERE s.id = '81000000-0000-0000-0000-000000000006' AND s.status = 'SCHEDULED'
ON CONFLICT (recipient_id, event_key) DO NOTHING;

