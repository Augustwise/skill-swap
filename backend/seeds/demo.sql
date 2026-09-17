-- Local, repeatable reference data for the sprint-1 presentation.
-- This is deliberately separate from migrations: real university data can replace it.
INSERT INTO universities (id, name, short_name, email_domain, city)
VALUES ('10000000-0000-0000-0000-000000000001', 'Демонстраційний університет', 'Demo Uni', 'students.example.test', 'Київ')
ON CONFLICT (id) DO NOTHING;

INSERT INTO skill_categories (id, name, slug, sort_order)
VALUES
  ('20000000-0000-0000-0000-000000000001', 'Музика', 'music', 10),
  ('20000000-0000-0000-0000-000000000002', 'Дизайн', 'design', 20),
  ('20000000-0000-0000-0000-000000000003', 'Програмування', 'programming', 30)
ON CONFLICT (id) DO NOTHING;

INSERT INTO skills (id, category_id, name, slug, description)
VALUES
  ('30000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', 'Гітара', 'guitar', 'Базова та просунута гра на гітарі'),
  ('30000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000002', 'Photoshop', 'photoshop', 'Обробка зображень'),
  ('30000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000002', 'Figma', 'figma', 'Дизайн інтерфейсів'),
  ('30000000-0000-0000-0000-000000000004', '20000000-0000-0000-0000-000000000003', 'Go', 'go', 'Основи мови Go')
ON CONFLICT (id) DO NOTHING;
