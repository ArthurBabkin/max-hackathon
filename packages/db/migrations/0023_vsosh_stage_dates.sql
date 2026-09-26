-- Сроки ВсОШ 2026/27 вместо придуманных: в сиде даты этапов ВсОШ получались
-- хешем от профиля и помечались демо. Теперь:
-- • школьный этап по астрономии, физике, биологии, химии, математике и
--   информатике — окно по графику «Сириус.Курсов» на 2026/27
--   (siriusolymp.ru/school2026/about): регионы пишут тур в разные дни окна;
-- • остальные этапы и школьный этап прочих предметов — крайние сроки из
--   Порядка проведения ВсОШ (vserosolimp.edsoo.ru): школьный — до 1 ноября,
--   муниципальный — до 25 декабря, региональный — до 1 марта, заключительный —
--   до конца апреля. Точный день в регионе задаёт региональный орган, поэтому
--   это срок, а не дата начала.
-- Сверено 26.09.2026. В сиде 0003 это уже есть (build_seed.py, vsosh_stages) —
-- здесь то же самое для накатанных баз. Триггер 0006 запишет события
-- изменения сроков: кто следит за ВсОШ, получит уведомление, что даты стали
-- настоящими.
--
-- Этапы: удаляется 0, добавляется или меняется 36. Источников новых 2.
--
-- Сгенерировано datasets/parser/diff_seed.py --tables stages.

-- +goose Up

INSERT INTO sources (id, kind, title, url, verified_at) VALUES
  ('src-0be9bd40dfeb', 'site', 'ВсОШ: сроки этапов по Порядку проведения', 'https://vserosolimp.edsoo.ru/', '2026-09-26'::date),
  ('src-848dc7264ddc', 'site', 'ВсОШ: график школьного этапа на «Сириус.Курсах», 2026/27', 'https://siriusolymp.ru/school2026/about', '2026-09-26'::date)
ON CONFLICT (id) DO UPDATE SET
  kind = EXCLUDED.kind,
  title = EXCLUDED.title,
  url = EXCLUDED.url,
  verified_at = EXCLUDED.verified_at;

INSERT INTO stages (id, olympiad_profile_id, kind, title, starts_at, ends_at, deadline_at, is_online, is_demo, source_id) VALUES
  ('vsosh-astronomiya:school:1', 'vsosh-astronomiya', 'school', 'Школьный этап', '2026-09-22 00:00:00+03'::timestamptz, '2026-09-25 23:59:59+03'::timestamptz, '2026-09-25 23:59:59+03'::timestamptz, true, false, 'src-848dc7264ddc'),
  ('vsosh-astronomiya:municipal:1', 'vsosh-astronomiya', 'municipal', 'Муниципальный этап', NULL, '2026-12-25 23:59:59+03'::timestamptz, '2026-12-25 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-astronomiya:regional:1', 'vsosh-astronomiya', 'regional', 'Региональный этап', NULL, '2027-03-01 23:59:59+03'::timestamptz, '2027-03-01 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-astronomiya:final:1', 'vsosh-astronomiya', 'final', 'Заключительный этап', NULL, '2027-04-30 23:59:59+03'::timestamptz, '2027-04-30 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-biologiya:school:1', 'vsosh-biologiya', 'school', 'Школьный этап', '2026-10-06 00:00:00+03'::timestamptz, '2026-10-09 23:59:59+03'::timestamptz, '2026-10-09 23:59:59+03'::timestamptz, true, false, 'src-848dc7264ddc'),
  ('vsosh-biologiya:municipal:1', 'vsosh-biologiya', 'municipal', 'Муниципальный этап', NULL, '2026-12-25 23:59:59+03'::timestamptz, '2026-12-25 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-biologiya:regional:1', 'vsosh-biologiya', 'regional', 'Региональный этап', NULL, '2027-03-01 23:59:59+03'::timestamptz, '2027-03-01 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-biologiya:final:1', 'vsosh-biologiya', 'final', 'Заключительный этап', NULL, '2027-04-30 23:59:59+03'::timestamptz, '2027-04-30 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-ekologiya:school:1', 'vsosh-ekologiya', 'school', 'Школьный этап', NULL, '2026-11-01 23:59:59+03'::timestamptz, '2026-11-01 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-ekologiya:municipal:1', 'vsosh-ekologiya', 'municipal', 'Муниципальный этап', NULL, '2026-12-25 23:59:59+03'::timestamptz, '2026-12-25 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-ekologiya:regional:1', 'vsosh-ekologiya', 'regional', 'Региональный этап', NULL, '2027-03-01 23:59:59+03'::timestamptz, '2027-03-01 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-ekologiya:final:1', 'vsosh-ekologiya', 'final', 'Заключительный этап', NULL, '2027-04-30 23:59:59+03'::timestamptz, '2027-04-30 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-ekonomika:school:1', 'vsosh-ekonomika', 'school', 'Школьный этап', NULL, '2026-11-01 23:59:59+03'::timestamptz, '2026-11-01 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-ekonomika:municipal:1', 'vsosh-ekonomika', 'municipal', 'Муниципальный этап', NULL, '2026-12-25 23:59:59+03'::timestamptz, '2026-12-25 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-ekonomika:regional:1', 'vsosh-ekonomika', 'regional', 'Региональный этап', NULL, '2027-03-01 23:59:59+03'::timestamptz, '2027-03-01 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-ekonomika:final:1', 'vsosh-ekonomika', 'final', 'Заключительный этап', NULL, '2027-04-30 23:59:59+03'::timestamptz, '2027-04-30 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-fizika:school:1', 'vsosh-fizika', 'school', 'Школьный этап', '2026-09-29 00:00:00+03'::timestamptz, '2026-10-02 23:59:59+03'::timestamptz, '2026-10-02 23:59:59+03'::timestamptz, true, false, 'src-848dc7264ddc'),
  ('vsosh-fizika:municipal:1', 'vsosh-fizika', 'municipal', 'Муниципальный этап', NULL, '2026-12-25 23:59:59+03'::timestamptz, '2026-12-25 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-fizika:regional:1', 'vsosh-fizika', 'regional', 'Региональный этап', NULL, '2027-03-01 23:59:59+03'::timestamptz, '2027-03-01 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-fizika:final:1', 'vsosh-fizika', 'final', 'Заключительный этап', NULL, '2027-04-30 23:59:59+03'::timestamptz, '2027-04-30 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-himiya:school:1', 'vsosh-himiya', 'school', 'Школьный этап', '2026-10-12 00:00:00+03'::timestamptz, '2026-10-16 23:59:59+03'::timestamptz, '2026-10-16 23:59:59+03'::timestamptz, true, false, 'src-848dc7264ddc'),
  ('vsosh-himiya:municipal:1', 'vsosh-himiya', 'municipal', 'Муниципальный этап', NULL, '2026-12-25 23:59:59+03'::timestamptz, '2026-12-25 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-himiya:regional:1', 'vsosh-himiya', 'regional', 'Региональный этап', NULL, '2027-03-01 23:59:59+03'::timestamptz, '2027-03-01 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-himiya:final:1', 'vsosh-himiya', 'final', 'Заключительный этап', NULL, '2027-04-30 23:59:59+03'::timestamptz, '2027-04-30 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-informatika:school:1', 'vsosh-informatika', 'school', 'Школьный этап', '2026-10-19 00:00:00+03'::timestamptz, '2026-10-23 23:59:59+03'::timestamptz, '2026-10-23 23:59:59+03'::timestamptz, true, false, 'src-848dc7264ddc'),
  ('vsosh-informatika:municipal:1', 'vsosh-informatika', 'municipal', 'Муниципальный этап', NULL, '2026-12-25 23:59:59+03'::timestamptz, '2026-12-25 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-informatika:regional:1', 'vsosh-informatika', 'regional', 'Региональный этап', NULL, '2027-03-01 23:59:59+03'::timestamptz, '2027-03-01 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-informatika:final:1', 'vsosh-informatika', 'final', 'Заключительный этап', NULL, '2027-04-30 23:59:59+03'::timestamptz, '2027-04-30 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-matematika:school:1', 'vsosh-matematika', 'school', 'Школьный этап', '2026-10-13 00:00:00+03'::timestamptz, '2026-10-16 23:59:59+03'::timestamptz, '2026-10-16 23:59:59+03'::timestamptz, true, false, 'src-848dc7264ddc'),
  ('vsosh-matematika:municipal:1', 'vsosh-matematika', 'municipal', 'Муниципальный этап', NULL, '2026-12-25 23:59:59+03'::timestamptz, '2026-12-25 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-matematika:regional:1', 'vsosh-matematika', 'regional', 'Региональный этап', NULL, '2027-03-01 23:59:59+03'::timestamptz, '2027-03-01 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-matematika:final:1', 'vsosh-matematika', 'final', 'Заключительный этап', NULL, '2027-04-30 23:59:59+03'::timestamptz, '2027-04-30 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-obschestvoznanie:school:1', 'vsosh-obschestvoznanie', 'school', 'Школьный этап', NULL, '2026-11-01 23:59:59+03'::timestamptz, '2026-11-01 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-obschestvoznanie:municipal:1', 'vsosh-obschestvoznanie', 'municipal', 'Муниципальный этап', NULL, '2026-12-25 23:59:59+03'::timestamptz, '2026-12-25 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-obschestvoznanie:regional:1', 'vsosh-obschestvoznanie', 'regional', 'Региональный этап', NULL, '2027-03-01 23:59:59+03'::timestamptz, '2027-03-01 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb'),
  ('vsosh-obschestvoznanie:final:1', 'vsosh-obschestvoznanie', 'final', 'Заключительный этап', NULL, '2027-04-30 23:59:59+03'::timestamptz, '2027-04-30 23:59:59+03'::timestamptz, false, false, 'src-0be9bd40dfeb')
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

INSERT INTO stages (id, olympiad_profile_id, kind, title, starts_at, ends_at, deadline_at, is_online, is_demo, source_id) VALUES
  ('vsosh-astronomiya:school:1', 'vsosh-astronomiya', 'school', 'Школьный этап', '2026-09-23 00:00:00+03'::timestamptz, '2026-10-28 23:59:59+03'::timestamptz, '2026-10-28 23:59:59+03'::timestamptz, false, true, NULL),
  ('vsosh-astronomiya:municipal:1', 'vsosh-astronomiya', 'municipal', 'Муниципальный этап', '2026-11-27 00:00:00+03'::timestamptz, '2026-11-27 23:59:59+03'::timestamptz, '2026-11-27 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-astronomiya:regional:1', 'vsosh-astronomiya', 'regional', 'Региональный этап', '2027-01-22 00:00:00+03'::timestamptz, '2027-01-23 23:59:59+03'::timestamptz, '2027-01-22 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-astronomiya:final:1', 'vsosh-astronomiya', 'final', 'Заключительный этап', '2027-04-08 00:00:00+03'::timestamptz, '2027-04-14 23:59:59+03'::timestamptz, '2027-04-08 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-biologiya:school:1', 'vsosh-biologiya', 'school', 'Школьный этап', '2026-09-23 00:00:00+03'::timestamptz, '2026-10-28 23:59:59+03'::timestamptz, '2026-10-28 23:59:59+03'::timestamptz, false, true, NULL),
  ('vsosh-biologiya:municipal:1', 'vsosh-biologiya', 'municipal', 'Муниципальный этап', '2026-11-18 00:00:00+03'::timestamptz, '2026-11-18 23:59:59+03'::timestamptz, '2026-11-18 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-biologiya:regional:1', 'vsosh-biologiya', 'regional', 'Региональный этап', '2027-01-18 00:00:00+03'::timestamptz, '2027-01-19 23:59:59+03'::timestamptz, '2027-01-18 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-biologiya:final:1', 'vsosh-biologiya', 'final', 'Заключительный этап', '2027-03-30 00:00:00+03'::timestamptz, '2027-04-05 23:59:59+03'::timestamptz, '2027-03-30 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-ekologiya:school:1', 'vsosh-ekologiya', 'school', 'Школьный этап', '2026-09-25 00:00:00+03'::timestamptz, '2026-10-30 23:59:59+03'::timestamptz, '2026-10-30 23:59:59+03'::timestamptz, false, true, NULL),
  ('vsosh-ekologiya:municipal:1', 'vsosh-ekologiya', 'municipal', 'Муниципальный этап', '2026-11-25 00:00:00+03'::timestamptz, '2026-11-25 23:59:59+03'::timestamptz, '2026-11-25 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-ekologiya:regional:1', 'vsosh-ekologiya', 'regional', 'Региональный этап', '2027-01-30 00:00:00+03'::timestamptz, '2027-01-31 23:59:59+03'::timestamptz, '2027-01-30 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-ekologiya:final:1', 'vsosh-ekologiya', 'final', 'Заключительный этап', '2027-04-06 00:00:00+03'::timestamptz, '2027-04-12 23:59:59+03'::timestamptz, '2027-04-06 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-ekonomika:school:1', 'vsosh-ekonomika', 'school', 'Школьный этап', '2026-09-21 00:00:00+03'::timestamptz, '2026-10-26 23:59:59+03'::timestamptz, '2026-10-26 23:59:59+03'::timestamptz, false, true, NULL),
  ('vsosh-ekonomika:municipal:1', 'vsosh-ekonomika', 'municipal', 'Муниципальный этап', '2026-11-23 00:00:00+03'::timestamptz, '2026-11-23 23:59:59+03'::timestamptz, '2026-11-23 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-ekonomika:regional:1', 'vsosh-ekonomika', 'regional', 'Региональный этап', '2027-02-02 00:00:00+03'::timestamptz, '2027-02-03 23:59:59+03'::timestamptz, '2027-02-02 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-ekonomika:final:1', 'vsosh-ekonomika', 'final', 'Заключительный этап', '2027-04-04 00:00:00+03'::timestamptz, '2027-04-10 23:59:59+03'::timestamptz, '2027-04-04 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-fizika:school:1', 'vsosh-fizika', 'school', 'Школьный этап', '2026-09-22 00:00:00+03'::timestamptz, '2026-10-27 23:59:59+03'::timestamptz, '2026-10-27 23:59:59+03'::timestamptz, false, true, NULL),
  ('vsosh-fizika:municipal:1', 'vsosh-fizika', 'municipal', 'Муниципальный этап', '2026-11-21 00:00:00+03'::timestamptz, '2026-11-21 23:59:59+03'::timestamptz, '2026-11-21 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-fizika:regional:1', 'vsosh-fizika', 'regional', 'Региональный этап', '2027-01-31 00:00:00+03'::timestamptz, '2027-02-01 23:59:59+03'::timestamptz, '2027-01-31 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-fizika:final:1', 'vsosh-fizika', 'final', 'Заключительный этап', '2027-04-02 00:00:00+03'::timestamptz, '2027-04-08 23:59:59+03'::timestamptz, '2027-04-02 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-himiya:school:1', 'vsosh-himiya', 'school', 'Школьный этап', '2026-09-26 00:00:00+03'::timestamptz, '2026-10-31 23:59:59+03'::timestamptz, '2026-10-31 23:59:59+03'::timestamptz, false, true, NULL),
  ('vsosh-himiya:municipal:1', 'vsosh-himiya', 'municipal', 'Муниципальный этап', '2026-11-10 00:00:00+03'::timestamptz, '2026-11-10 23:59:59+03'::timestamptz, '2026-11-10 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-himiya:regional:1', 'vsosh-himiya', 'regional', 'Региональный этап', '2027-01-30 00:00:00+03'::timestamptz, '2027-01-31 23:59:59+03'::timestamptz, '2027-01-30 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-himiya:final:1', 'vsosh-himiya', 'final', 'Заключительный этап', '2027-03-22 00:00:00+03'::timestamptz, '2027-03-28 23:59:59+03'::timestamptz, '2027-03-22 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-informatika:school:1', 'vsosh-informatika', 'school', 'Школьный этап', '2026-09-23 00:00:00+03'::timestamptz, '2026-10-28 23:59:59+03'::timestamptz, '2026-10-28 23:59:59+03'::timestamptz, false, true, NULL),
  ('vsosh-informatika:municipal:1', 'vsosh-informatika', 'municipal', 'Муниципальный этап', '2026-11-28 00:00:00+03'::timestamptz, '2026-11-28 23:59:59+03'::timestamptz, '2026-11-28 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-informatika:regional:1', 'vsosh-informatika', 'regional', 'Региональный этап', '2027-01-23 00:00:00+03'::timestamptz, '2027-01-24 23:59:59+03'::timestamptz, '2027-01-23 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-informatika:final:1', 'vsosh-informatika', 'final', 'Заключительный этап', '2027-04-09 00:00:00+03'::timestamptz, '2027-04-15 23:59:59+03'::timestamptz, '2027-04-09 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-matematika:school:1', 'vsosh-matematika', 'school', 'Школьный этап', '2026-09-24 00:00:00+03'::timestamptz, '2026-10-29 23:59:59+03'::timestamptz, '2026-10-29 23:59:59+03'::timestamptz, false, true, NULL),
  ('vsosh-matematika:municipal:1', 'vsosh-matematika', 'municipal', 'Муниципальный этап', '2026-11-29 00:00:00+03'::timestamptz, '2026-11-29 23:59:59+03'::timestamptz, '2026-11-29 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-matematika:regional:1', 'vsosh-matematika', 'regional', 'Региональный этап', '2027-01-29 00:00:00+03'::timestamptz, '2027-01-30 23:59:59+03'::timestamptz, '2027-01-29 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-matematika:final:1', 'vsosh-matematika', 'final', 'Заключительный этап', '2027-04-10 00:00:00+03'::timestamptz, '2027-04-16 23:59:59+03'::timestamptz, '2027-04-10 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-obschestvoznanie:school:1', 'vsosh-obschestvoznanie', 'school', 'Школьный этап', '2026-09-24 00:00:00+03'::timestamptz, '2026-10-29 23:59:59+03'::timestamptz, '2026-10-29 23:59:59+03'::timestamptz, false, true, NULL),
  ('vsosh-obschestvoznanie:municipal:1', 'vsosh-obschestvoznanie', 'municipal', 'Муниципальный этап', '2026-11-20 00:00:00+03'::timestamptz, '2026-11-20 23:59:59+03'::timestamptz, '2026-11-20 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-obschestvoznanie:regional:1', 'vsosh-obschestvoznanie', 'regional', 'Региональный этап', '2027-01-20 00:00:00+03'::timestamptz, '2027-01-21 23:59:59+03'::timestamptz, '2027-01-20 00:00:00+03'::timestamptz, false, true, NULL),
  ('vsosh-obschestvoznanie:final:1', 'vsosh-obschestvoznanie', 'final', 'Заключительный этап', '2027-04-01 00:00:00+03'::timestamptz, '2027-04-07 23:59:59+03'::timestamptz, '2027-04-01 00:00:00+03'::timestamptz, false, true, NULL)
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

DELETE FROM sources s WHERE s.id IN (
  'src-0be9bd40dfeb',
  'src-848dc7264ddc')
  AND NOT EXISTS (SELECT 1 FROM benefits b WHERE b.source_id = s.id)
  AND NOT EXISTS (SELECT 1 FROM direction_benefits d WHERE d.source_id = s.id)
  AND NOT EXISTS (SELECT 1 FROM stages st WHERE st.source_id = s.id)
  AND NOT EXISTS (SELECT 1 FROM olympiad_profiles p WHERE p.source_id = s.id);
