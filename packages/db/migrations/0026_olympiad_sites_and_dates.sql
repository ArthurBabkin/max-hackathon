-- Сайты, организаторы и сроки трёх олимпиад, которых не было в датасете C.
--
-- У олимпиады РГГУ, «Звезды» и Санкт-Петербургской астрономической в базе не
-- было ни сайта, ни этапов: карточка без блока «Об олимпиаде», в трекер со
-- сроками не добавить. Теперь они в C (сверено 28.09.2026):
--
-- * РГГУ (rsuh.ru): регистрация 1 декабря 2026 — 11 января 2027, отбор заочно
--   13–27 января, заключительный по обществознанию — 21 февраля 2027, очно в
--   РГГУ; 9–11 классы.
-- * «Звезда» (zv.susu.ru, организатор — ЮУрГУ по Положению): отбор по
--   естественным наукам и технике и технологиям — интернет-тур 9 ноября —
--   9 декабря 2026 или очно на площадках вузов-партнёров 9 ноября —
--   20 декабря; финал не объявлен — демо. Классы 6–11 и 7–11.
-- * Санкт-Петербургская астрономическая (school.astro.spbu.ru, организатор —
--   Алфёровский университет по Положению): сроки 2026/27 не опубликованы.
--   Примерные даты — сезон 2025/26 через 52 недели (туры по воскресеньям):
--   заочный отбор 15 декабря — 19 января, теоретический тур 7 февраля,
--   практический 14 марта; все — демо. 5–11 классы.
--
-- В сиде 0003 это уже есть — здесь то же самое для накатанных баз. Триггер
-- 0006 запишет события изменения сроков по четырём профилям: кто следит за
-- этими олимпиадами, узнает, что даты появились.
--
-- Этапы: удаляется 0, добавляется или меняется 12. Источников новых 2.
-- Олимпиад меняется 3, профилей 4.
--
-- Сгенерировано datasets/parser/diff_seed.py --tables stages.

-- +goose Up

INSERT INTO sources (id, kind, title, url, verified_at) VALUES
  ('src-206e814604fe', 'site', 'Олимпиада РГГУ для школьников — сроки этапов', 'https://www.rsuh.ru/education/cdo/olimpiada-rggu-dlya-shkolnikov.php', '2026-09-28'::date),
  ('src-8e02759eb9ca', 'site', 'Многопрофильная инженерная олимпиада «Звезда» — сроки этапов', 'https://zv.susu.ru/index.php/mnogoprofilnaya-inzhenernaya-olimpiada-zvezda/otborochnyj-etap', '2026-09-28'::date)
ON CONFLICT (id) DO UPDATE SET
  kind = EXCLUDED.kind,
  title = EXCLUDED.title,
  url = EXCLUDED.url,
  verified_at = EXCLUDED.verified_at;

INSERT INTO olympiads (id, name, organizer, kind, official_url, format, final_city, final_region_code) VALUES
  ('p669-36', 'Многопрофильная инженерная олимпиада «Звезда»', 'ЮУрГУ', 'perechen', 'https://zv.susu.ru/', 'Онлайн-отбор, очный финал', NULL, NULL),
  ('p669-48', 'Олимпиада РГГУ для школьников', 'РГГУ', 'perechen', 'https://www.rsuh.ru/education/cdo/olimpiada-rggu-dlya-shkolnikov.php', 'Онлайн-отбор, очный финал', 'Москва', '77'),
  ('p669-74', 'Санкт-Петербургская астрономическая олимпиада', 'Алфёровский университет', 'perechen', 'http://school.astro.spbu.ru/?q=olymp', 'Онлайн-отбор, очный финал', NULL, NULL)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  organizer = EXCLUDED.organizer,
  kind = EXCLUDED.kind,
  official_url = EXCLUDED.official_url,
  format = EXCLUDED.format,
  final_city = EXCLUDED.final_city,
  final_region_code = EXCLUDED.final_region_code;

INSERT INTO olympiad_profiles (id, olympiad_id, subject_code, profile_slug, profile_name, level, school_year, grades_from, grades_to, source_id) VALUES
  ('p669-36-estestvennye-nauki', 'p669-36', 'phys', 'estestvennye-nauki', 'естественные науки', 'II', '2026/27', 6, 11, 'src-cc212c88451d'),
  ('p669-36-tehnika-i-tehnologii', 'p669-36', 'phys', 'tehnika-i-tehnologii', 'техника и технологии', 'I', '2026/27', 7, 11, 'src-cc212c88451d'),
  ('p669-48-obschestvoznanie', 'p669-48', 'soc', 'obschestvoznanie', 'обществознание', 'II', '2026/27', 9, 11, 'src-cc212c88451d'),
  ('p669-74-astronomiya', 'p669-74', 'astro', 'astronomiya', 'астрономия', 'I', '2026/27', 5, 11, 'src-cc212c88451d')
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
  ('p669-36-estestvennye-nauki:qualifying:1', 'p669-36-estestvennye-nauki', 'qualifying', 'Отборочный этап, интернет-тур', '2026-11-09 00:00:00+03'::timestamptz, '2026-12-09 23:59:59+03'::timestamptz, '2026-12-09 23:59:59+03'::timestamptz, true, false, 'src-8e02759eb9ca'),
  ('p669-36-estestvennye-nauki:qualifying:2', 'p669-36-estestvennye-nauki', 'qualifying', 'Отборочный этап, очно на площадках вузов-партнёров', '2026-11-09 00:00:00+03'::timestamptz, '2026-12-20 23:59:59+03'::timestamptz, '2026-12-20 23:59:59+03'::timestamptz, false, false, 'src-8e02759eb9ca'),
  ('p669-36-estestvennye-nauki:final:1', 'p669-36-estestvennye-nauki', 'final', 'Заключительный этап', '2027-01-19 00:00:00+03'::timestamptz, '2027-01-21 23:59:59+03'::timestamptz, '2027-01-19 00:00:00+03'::timestamptz, false, true, NULL),
  ('p669-36-tehnika-i-tehnologii:qualifying:1', 'p669-36-tehnika-i-tehnologii', 'qualifying', 'Отборочный этап, интернет-тур', '2026-11-09 00:00:00+03'::timestamptz, '2026-12-09 23:59:59+03'::timestamptz, '2026-12-09 23:59:59+03'::timestamptz, true, false, 'src-8e02759eb9ca'),
  ('p669-36-tehnika-i-tehnologii:qualifying:2', 'p669-36-tehnika-i-tehnologii', 'qualifying', 'Отборочный этап, очно на площадках вузов-партнёров', '2026-11-09 00:00:00+03'::timestamptz, '2026-12-20 23:59:59+03'::timestamptz, '2026-12-20 23:59:59+03'::timestamptz, false, false, 'src-8e02759eb9ca'),
  ('p669-36-tehnika-i-tehnologii:final:1', 'p669-36-tehnika-i-tehnologii', 'final', 'Заключительный этап', '2027-01-19 00:00:00+03'::timestamptz, '2027-01-21 23:59:59+03'::timestamptz, '2027-01-19 00:00:00+03'::timestamptz, false, true, NULL),
  ('p669-48-obschestvoznanie:registration:1', 'p669-48-obschestvoznanie', 'registration', 'Регистрация', '2026-12-01 00:00:00+03'::timestamptz, '2027-01-11 23:59:59+03'::timestamptz, '2027-01-11 23:59:59+03'::timestamptz, true, false, 'src-206e814604fe'),
  ('p669-48-obschestvoznanie:qualifying:1', 'p669-48-obschestvoznanie', 'qualifying', 'Отборочный этап (заочный, дистанционный)', '2027-01-13 00:00:00+03'::timestamptz, '2027-01-27 23:59:59+03'::timestamptz, '2027-01-27 23:59:59+03'::timestamptz, true, false, 'src-206e814604fe'),
  ('p669-48-obschestvoznanie:final:1', 'p669-48-obschestvoznanie', 'final', 'Заключительный этап, очно в РГГУ (Москва)', '2027-02-21 00:00:00+03'::timestamptz, '2027-02-21 23:59:59+03'::timestamptz, '2027-02-21 00:00:00+03'::timestamptz, false, false, 'src-206e814604fe'),
  ('p669-74-astronomiya:qualifying:1', 'p669-74-astronomiya', 'qualifying', 'Отборочный этап (заочный)', '2026-12-15 00:00:00+03'::timestamptz, '2027-01-19 23:59:59+03'::timestamptz, '2027-01-19 23:59:59+03'::timestamptz, true, true, NULL),
  ('p669-74-astronomiya:final:1', 'p669-74-astronomiya', 'final', 'Заключительный этап, теоретический тур', '2027-02-07 00:00:00+03'::timestamptz, '2027-02-07 23:59:59+03'::timestamptz, '2027-02-07 00:00:00+03'::timestamptz, false, true, NULL),
  ('p669-74-astronomiya:final:2', 'p669-74-astronomiya', 'final', 'Заключительный этап, практический тур', '2027-03-14 00:00:00+03'::timestamptz, '2027-03-14 23:59:59+03'::timestamptz, '2027-03-14 00:00:00+03'::timestamptz, false, true, NULL)
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

-- +goose Down

DELETE FROM stages WHERE id IN (
  'p669-36-estestvennye-nauki:final:1',
  'p669-36-estestvennye-nauki:qualifying:1',
  'p669-36-estestvennye-nauki:qualifying:2',
  'p669-36-tehnika-i-tehnologii:final:1',
  'p669-36-tehnika-i-tehnologii:qualifying:1',
  'p669-36-tehnika-i-tehnologii:qualifying:2',
  'p669-48-obschestvoznanie:final:1',
  'p669-48-obschestvoznanie:qualifying:1',
  'p669-48-obschestvoznanie:registration:1',
  'p669-74-astronomiya:final:1',
  'p669-74-astronomiya:final:2',
  'p669-74-astronomiya:qualifying:1'
);

INSERT INTO olympiad_profiles (id, olympiad_id, subject_code, profile_slug, profile_name, level, school_year, grades_from, grades_to, source_id) VALUES
  ('p669-36-estestvennye-nauki', 'p669-36', 'phys', 'estestvennye-nauki', 'естественные науки', 'II', '2026/27', 8, 11, 'src-cc212c88451d'),
  ('p669-36-tehnika-i-tehnologii', 'p669-36', 'phys', 'tehnika-i-tehnologii', 'техника и технологии', 'I', '2026/27', 8, 11, 'src-cc212c88451d'),
  ('p669-48-obschestvoznanie', 'p669-48', 'soc', 'obschestvoznanie', 'обществознание', 'II', '2026/27', 8, 11, 'src-cc212c88451d'),
  ('p669-74-astronomiya', 'p669-74', 'astro', 'astronomiya', 'астрономия', 'I', '2026/27', 8, 11, 'src-cc212c88451d')
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

INSERT INTO olympiads (id, name, organizer, kind, official_url, format, final_city, final_region_code) VALUES
  ('p669-36', 'Многопрофильная инженерная олимпиада «Звезда»', NULL, 'perechen', NULL, NULL, NULL, NULL),
  ('p669-48', 'Олимпиада РГГУ для школьников', NULL, 'perechen', NULL, NULL, NULL, NULL),
  ('p669-74', 'Санкт-Петербургская астрономическая олимпиада', NULL, 'perechen', NULL, NULL, NULL, NULL)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  organizer = EXCLUDED.organizer,
  kind = EXCLUDED.kind,
  official_url = EXCLUDED.official_url,
  format = EXCLUDED.format,
  final_city = EXCLUDED.final_city,
  final_region_code = EXCLUDED.final_region_code;

DELETE FROM sources s WHERE s.id IN (
  'src-206e814604fe',
  'src-8e02759eb9ca')
  AND NOT EXISTS (SELECT 1 FROM benefits b WHERE b.source_id = s.id)
  AND NOT EXISTS (SELECT 1 FROM direction_benefits d WHERE d.source_id = s.id)
  AND NOT EXISTS (SELECT 1 FROM stages st WHERE st.source_id = s.id)
  AND NOT EXISTS (SELECT 1 FROM olympiad_profiles p WHERE p.source_id = s.id);
