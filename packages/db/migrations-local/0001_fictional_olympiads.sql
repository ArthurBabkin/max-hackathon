-- Вымышленные олимпиады вне перечня — чтобы локально показать блок F16.
-- Накатываются только на локальный стенд (seed-demo) и в CI, в прод не попадают.
--
-- ФАЙЛ СГЕНЕРИРОВАН: datasets/parser/build_seed.py (make seed). Руками не править.

-- +goose Up

INSERT INTO olympiads (id, name, organizer, kind, official_url, format, final_city, final_region_code) VALUES
  ('other-biznes-start', 'Олимпиада по экономике «Бизнес-старт»', 'Вымышленный пример для демонстрации', 'other', NULL, 'Онлайн-отбор, очный финал', 'Москва', '77'),
  ('other-impuls', 'Городская олимпиада по физике «Импульс»', 'Вымышленный пример для демонстрации', 'other', NULL, 'Онлайн-отбор, очный финал', 'Казань', '16'),
  ('other-tyk', 'Турнир юных программистов Казани', 'Вымышленный пример для демонстрации', 'other', NULL, 'Онлайн-отбор, очный финал', 'Казань', '16')
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  organizer = EXCLUDED.organizer,
  kind = EXCLUDED.kind,
  official_url = EXCLUDED.official_url,
  format = EXCLUDED.format,
  final_city = EXCLUDED.final_city,
  final_region_code = EXCLUDED.final_region_code;

INSERT INTO olympiad_profiles (id, olympiad_id, subject_code, profile_slug, profile_name, level, school_year, grades_from, grades_to, source_id) VALUES
  ('other-biznes-start-econ', 'other-biznes-start', 'econ', 'econ', NULL, NULL, '2026/27', 8, 11, NULL),
  ('other-impuls-phys', 'other-impuls', 'phys', 'phys', NULL, NULL, '2026/27', 7, 11, NULL),
  ('other-tyk-inf', 'other-tyk', 'inf', 'inf', NULL, NULL, '2026/27', 7, 11, NULL)
ON CONFLICT (id) DO UPDATE SET
  olympiad_id = EXCLUDED.olympiad_id,
  subject_code = EXCLUDED.subject_code,
  profile_slug = EXCLUDED.profile_slug,
  profile_name = EXCLUDED.profile_name,
  level = EXCLUDED.level,
  school_year = EXCLUDED.school_year,
  grades_from = EXCLUDED.grades_from,
  grades_to = EXCLUDED.grades_to,
  source_id = EXCLUDED.source_id;

INSERT INTO stages (id, olympiad_profile_id, kind, title, starts_at, ends_at, deadline_at, is_online, is_demo, source_id) VALUES
  ('other-biznes-start-econ:registration:1', 'other-biznes-start-econ', 'registration', 'Регистрация', '2026-10-01 00:00:00+03'::timestamptz, '2026-10-21 23:59:59+03'::timestamptz, '2026-10-21 23:59:59+03'::timestamptz, true, true, NULL),
  ('other-biznes-start-econ:qualifying:1', 'other-biznes-start-econ', 'qualifying', 'Отборочный этап', '2026-10-28 00:00:00+03'::timestamptz, '2026-11-10 23:59:59+03'::timestamptz, '2026-11-10 23:59:59+03'::timestamptz, true, true, NULL),
  ('other-biznes-start-econ:final:1', 'other-biznes-start-econ', 'final', 'Заключительный этап', '2026-12-10 00:00:00+03'::timestamptz, '2026-12-12 23:59:59+03'::timestamptz, '2026-12-10 00:00:00+03'::timestamptz, false, true, NULL),
  ('other-impuls-phys:registration:1', 'other-impuls-phys', 'registration', 'Регистрация', '2026-10-05 00:00:00+03'::timestamptz, '2026-10-25 23:59:59+03'::timestamptz, '2026-10-25 23:59:59+03'::timestamptz, true, true, NULL),
  ('other-impuls-phys:qualifying:1', 'other-impuls-phys', 'qualifying', 'Отборочный этап', '2026-11-01 00:00:00+03'::timestamptz, '2026-11-14 23:59:59+03'::timestamptz, '2026-11-14 23:59:59+03'::timestamptz, true, true, NULL),
  ('other-impuls-phys:final:1', 'other-impuls-phys', 'final', 'Заключительный этап', '2026-12-14 00:00:00+03'::timestamptz, '2026-12-16 23:59:59+03'::timestamptz, '2026-12-14 00:00:00+03'::timestamptz, false, true, NULL),
  ('other-tyk-inf:registration:1', 'other-tyk-inf', 'registration', 'Регистрация', '2026-10-06 00:00:00+03'::timestamptz, '2026-10-26 23:59:59+03'::timestamptz, '2026-10-26 23:59:59+03'::timestamptz, true, true, NULL),
  ('other-tyk-inf:qualifying:1', 'other-tyk-inf', 'qualifying', 'Отборочный этап', '2026-11-02 00:00:00+03'::timestamptz, '2026-11-15 23:59:59+03'::timestamptz, '2026-11-15 23:59:59+03'::timestamptz, true, true, NULL),
  ('other-tyk-inf:final:1', 'other-tyk-inf', 'final', 'Заключительный этап', '2026-12-15 00:00:00+03'::timestamptz, '2026-12-17 23:59:59+03'::timestamptz, '2026-12-15 00:00:00+03'::timestamptz, false, true, NULL)
ON CONFLICT (id) DO UPDATE SET
  olympiad_profile_id = EXCLUDED.olympiad_profile_id,
  kind = EXCLUDED.kind,
  title = EXCLUDED.title,
  starts_at = EXCLUDED.starts_at,
  ends_at = EXCLUDED.ends_at,
  deadline_at = EXCLUDED.deadline_at,
  is_online = EXCLUDED.is_online,
  is_demo = EXCLUDED.is_demo,
  source_id = EXCLUDED.source_id;

INSERT INTO benefits (id, olympiad_profile_id, university_id, admission_year, benefit, extra_points, ege_min, diploma_grades, note, source_id) VALUES
  ('other-biznes-start-econ__hse__2026__extra_points', 'other-biznes-start-econ', 'hse', 2026, 'extra_points', 2, NULL, NULL, 'Баллы за индивидуальные достижения, не больше 10 в сумме', NULL),
  ('other-biznes-start-econ__kfu__2026__extra_points', 'other-biznes-start-econ', 'kfu', 2026, 'extra_points', 1, NULL, NULL, 'Баллы за индивидуальные достижения, не больше 10 в сумме', NULL),
  ('other-impuls-phys__kfu__2026__extra_points', 'other-impuls-phys', 'kfu', 2026, 'extra_points', 2, NULL, NULL, 'Баллы за индивидуальные достижения, не больше 10 в сумме', NULL),
  ('other-tyk-inf__innopolis__2026__extra_points', 'other-tyk-inf', 'innopolis', 2026, 'extra_points', 2, NULL, NULL, 'Баллы за индивидуальные достижения, не больше 10 в сумме', NULL),
  ('other-tyk-inf__kfu__2026__extra_points', 'other-tyk-inf', 'kfu', 2026, 'extra_points', 3, NULL, NULL, 'Баллы за индивидуальные достижения, не больше 10 в сумме', NULL)
ON CONFLICT (id) DO UPDATE SET
  olympiad_profile_id = EXCLUDED.olympiad_profile_id,
  university_id = EXCLUDED.university_id,
  admission_year = EXCLUDED.admission_year,
  benefit = EXCLUDED.benefit,
  extra_points = EXCLUDED.extra_points,
  ege_min = EXCLUDED.ege_min,
  diploma_grades = EXCLUDED.diploma_grades,
  note = EXCLUDED.note,
  source_id = EXCLUDED.source_id;

-- Вставка этапов и льгот рождает события изменения (0006) — о демо не уведомляем.
DELETE FROM content_changes WHERE notified_at IS NULL AND entity_id LIKE 'other-%';

-- +goose Down
DELETE FROM olympiads WHERE id IN ('other-biznes-start', 'other-impuls', 'other-tyk');
