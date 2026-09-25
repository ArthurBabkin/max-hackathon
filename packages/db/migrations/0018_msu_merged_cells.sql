-- Льготы МГУ, которые терял парсер датасета: в olymp_benefits.pdf профиль,
-- перечень олимпиад, класс и предмет ЕГЭ — объединённые ячейки на несколько
-- уровней или статусов («Химия, *: I уровень — …, II уровень — 100 баллов»).
-- pdfplumber кладёт текст в первую строку, у продолжений ячейки пустые, и 88
-- строк выпадали. Теперь в базе: химия II–III уровня и астрономия на
-- 100 баллов, призёр по экологии — 100 баллов (было «БВИ только победителю»),
-- по экономике ЕГЭ — математика или обществознание; у остальных уточнена
-- страница источника.
--
-- Парсер исправлен (datasets/parser/build_b.py, msu_rows), датасет B и сид
-- 0003 пересобраны — здесь те же 9 новых и 22 уточнённые льготы и 3 источника
-- для накатанных баз. Остальные вузы не меняются.

-- +goose Up

INSERT INTO sources (id, kind, title, url, verified_at) VALUES
  ('src-199dcf7fea59', 'rules', 'МГУ: особые права победителей и призёров олимпиад, 2026', 'https://cpk.msu.ru/files/2026/olymp_benefits.pdf#page=51', '2026-09-21'::date),
  ('src-2d531ebafa6e', 'rules', 'МГУ: особые права победителей и призёров олимпиад, 2026', 'https://cpk.msu.ru/files/2026/olymp_benefits.pdf#page=34', '2026-09-21'::date),
  ('src-424cfea8860a', 'rules', 'МГУ: особые права победителей и призёров олимпиад, 2026', 'https://cpk.msu.ru/files/2026/olymp_benefits.pdf#page=11', '2026-09-21'::date)
ON CONFLICT (id) DO UPDATE SET
  kind = EXCLUDED.kind,
  title = EXCLUDED.title,
  url = EXCLUDED.url,
  verified_at = EXCLUDED.verified_at;

INSERT INTO benefits (id, olympiad_profile_id, university_id, admission_year, benefit, extra_points, ege_min, diploma_grades, note, source_id) VALUES
  ('p669-14-fizika__msu__2026__score100', 'p669-14-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-9c806d0df100'),
  ('p669-17-fizika__msu__2026__score100', 'p669-17-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-9c806d0df100'),
  ('p669-19-fizika__msu__2026__score100', 'p669-19-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-9c806d0df100'),
  ('p669-26-himiya__msu__2026__score100', 'p669-26-himiya', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Химия', 'src-2d531ebafa6e'),
  ('p669-33-himiya__msu__2026__score100', 'p669-33-himiya', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Химия', 'src-2d531ebafa6e'),
  ('p669-37-astronomiya__msu__2026__score100', 'p669-37-astronomiya', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-9c806d0df100'),
  ('p669-37-ekologiya__msu__2026__bvi_winners', 'p669-37-ekologiya', 'msu', 2026, 'bvi_winners', NULL, 75, ARRAY[11]::int[], 'Победителю — БВИ, призёру — 100 баллов. Подтвердить ЕГЭ: Биология', 'src-9b544ebbb79f'),
  ('p669-37-ekonomika__msu__2026__score100', 'p669-37-ekonomika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Математика или Обществознание', 'src-199dcf7fea59'),
  ('p669-37-obschestvoznanie__msu__2026__score100', 'p669-37-obschestvoznanie', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Обществознание', 'src-62e30c04be77'),
  ('p669-4-ekonomika__msu__2026__bvi', 'p669-4-ekonomika', 'msu', 2026, 'bvi', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Математика или Обществознание', 'src-199dcf7fea59'),
  ('p669-43-fizika__msu__2026__score100', 'p669-43-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-9c806d0df100'),
  ('p669-47-ekonomika__msu__2026__score100', 'p669-47-ekonomika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Математика или Обществознание', 'src-94062916dbdc'),
  ('p669-48-obschestvoznanie__msu__2026__score100', 'p669-48-obschestvoznanie', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Обществознание', 'src-62e30c04be77'),
  ('p669-49-himiya__msu__2026__score100', 'p669-49-himiya', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Химия', 'src-2d531ebafa6e'),
  ('p669-50-ekologiya__msu__2026__bvi_winners', 'p669-50-ekologiya', 'msu', 2026, 'bvi_winners', NULL, 75, ARRAY[11]::int[], 'Победителю — БВИ, призёру — 100 баллов. Подтвердить ЕГЭ: Биология', 'src-9b544ebbb79f'),
  ('p669-50-vysokie-tehnologii__msu__2026__bvi_winners', 'p669-50-vysokie-tehnologii', 'msu', 2026, 'bvi_winners', NULL, 75, ARRAY[11]::int[], 'Победителю — БВИ, призёру — 100 баллов. Подтвердить ЕГЭ: Математика или Химия', 'src-424cfea8860a'),
  ('p669-53-fizika__msu__2026__score100', 'p669-53-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-9c806d0df100'),
  ('p669-54-himiya__msu__2026__score100', 'p669-54-himiya', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Химия', 'src-2d531ebafa6e'),
  ('p669-55-fizika__msu__2026__score100', 'p669-55-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-9c806d0df100'),
  ('p669-55-himiya__msu__2026__score100', 'p669-55-himiya', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Химия', 'src-2d531ebafa6e'),
  ('p669-58-obschestvoznanie__msu__2026__score100', 'p669-58-obschestvoznanie', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Обществознание', 'src-62e30c04be77'),
  ('p669-59-ekonomika__msu__2026__score100', 'p669-59-ekonomika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Математика или Обществознание', 'src-199dcf7fea59'),
  ('p669-59-fizika__msu__2026__score100', 'p669-59-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-9c806d0df100'),
  ('p669-62-fizika__msu__2026__score100', 'p669-62-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-9c806d0df100'),
  ('p669-67-himiya__msu__2026__score100', 'p669-67-himiya', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Химия', 'src-2d531ebafa6e'),
  ('p669-68-himiya__msu__2026__score100', 'p669-68-himiya', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Химия', 'src-2d531ebafa6e'),
  ('p669-71-himiya__msu__2026__score100', 'p669-71-himiya', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Химия', 'src-2d531ebafa6e'),
  ('p669-79-fizika__msu__2026__score100', 'p669-79-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-9c806d0df100'),
  ('p669-8-fizika__msu__2026__score100', 'p669-8-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-9c806d0df100'),
  ('p669-82-fizika__msu__2026__score100', 'p669-82-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-9c806d0df100'),
  ('p669-9-ekonomika__msu__2026__score100', 'p669-9-ekonomika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Математика или Обществознание', 'src-199dcf7fea59')
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

-- +goose Down
DELETE FROM benefits WHERE id IN (
  'p669-26-himiya__msu__2026__score100',
  'p669-33-himiya__msu__2026__score100',
  'p669-37-astronomiya__msu__2026__score100',
  'p669-49-himiya__msu__2026__score100',
  'p669-54-himiya__msu__2026__score100',
  'p669-55-himiya__msu__2026__score100',
  'p669-67-himiya__msu__2026__score100',
  'p669-68-himiya__msu__2026__score100',
  'p669-71-himiya__msu__2026__score100');
INSERT INTO benefits (id, olympiad_profile_id, university_id, admission_year, benefit, extra_points, ege_min, diploma_grades, note, source_id) VALUES
  ('p669-14-fizika__msu__2026__score100', 'p669-14-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-e0c2b2948924'),
  ('p669-17-fizika__msu__2026__score100', 'p669-17-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-e0c2b2948924'),
  ('p669-19-fizika__msu__2026__score100', 'p669-19-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-e0c2b2948924'),
  ('p669-37-ekologiya__msu__2026__bvi_winners', 'p669-37-ekologiya', 'msu', 2026, 'bvi_winners', NULL, 75, ARRAY[11]::int[], 'БВИ только победителю. Подтвердить ЕГЭ: Биология', 'src-9b544ebbb79f'),
  ('p669-37-ekonomika__msu__2026__score100', 'p669-37-ekonomika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Математика', 'src-abbf33a7eb41'),
  ('p669-37-obschestvoznanie__msu__2026__score100', 'p669-37-obschestvoznanie', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Обществознание', 'src-abbf33a7eb41'),
  ('p669-4-ekonomika__msu__2026__bvi', 'p669-4-ekonomika', 'msu', 2026, 'bvi', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Математика или Обществознание', 'src-abbf33a7eb41'),
  ('p669-43-fizika__msu__2026__score100', 'p669-43-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-e0c2b2948924'),
  ('p669-47-ekonomika__msu__2026__score100', 'p669-47-ekonomika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Математика', 'src-94062916dbdc'),
  ('p669-48-obschestvoznanie__msu__2026__score100', 'p669-48-obschestvoznanie', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Обществознание', 'src-abbf33a7eb41'),
  ('p669-50-ekologiya__msu__2026__bvi_winners', 'p669-50-ekologiya', 'msu', 2026, 'bvi_winners', NULL, 75, ARRAY[11]::int[], 'БВИ только победителю. Подтвердить ЕГЭ: Биология', 'src-9b544ebbb79f'),
  ('p669-50-vysokie-tehnologii__msu__2026__bvi_winners', 'p669-50-vysokie-tehnologii', 'msu', 2026, 'bvi_winners', NULL, 75, ARRAY[11]::int[], 'Победителю — БВИ, призёру — 100 баллов. Подтвердить ЕГЭ: Математика или Химия', 'src-32d00598e90d'),
  ('p669-53-fizika__msu__2026__score100', 'p669-53-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-e0c2b2948924'),
  ('p669-55-fizika__msu__2026__score100', 'p669-55-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-e0c2b2948924'),
  ('p669-58-obschestvoznanie__msu__2026__score100', 'p669-58-obschestvoznanie', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Обществознание', 'src-abbf33a7eb41'),
  ('p669-59-ekonomika__msu__2026__score100', 'p669-59-ekonomika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Математика', 'src-abbf33a7eb41'),
  ('p669-59-fizika__msu__2026__score100', 'p669-59-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-e0c2b2948924'),
  ('p669-62-fizika__msu__2026__score100', 'p669-62-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-e0c2b2948924'),
  ('p669-79-fizika__msu__2026__score100', 'p669-79-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-e0c2b2948924'),
  ('p669-8-fizika__msu__2026__score100', 'p669-8-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-e0c2b2948924'),
  ('p669-82-fizika__msu__2026__score100', 'p669-82-fizika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-e0c2b2948924'),
  ('p669-9-ekonomika__msu__2026__score100', 'p669-9-ekonomika', 'msu', 2026, 'score100', NULL, 75, ARRAY[11]::int[], 'Подтвердить ЕГЭ: Математика', 'src-abbf33a7eb41')
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
DELETE FROM sources WHERE id IN (
  'src-199dcf7fea59',
  'src-2d531ebafa6e',
  'src-424cfea8860a');
