-- Льготы Иннополиса, которые терял парсер датасета: одну и ту же таблицу
-- приложения 3 приказа от 19.01.2026 pdfplumber режет на 5, 7 или 11 колонок,
-- и строки со стр. 8, 9, 11 и 16 выпадали. Среди них Innopolis Open, часть
-- профилей НТО, Всесиб, Инженерная олимпиада, «Покори Воробьёвы горы»,
-- «Курчатов» — помощник отвечал, что Innopolis Open в Иннополисе ничего не даёт.
--
-- Парсер исправлен (datasets/parser/build_b.py, innopolis_rows), датасет B и
-- сид 0003 пересобраны — здесь те же 34 льготы и 4 источника для накатанных
-- баз. Остальные вузы не меняются. Триггер 0006 запишет события новых льгот.

-- +goose Up

INSERT INTO sources (id, kind, title, url, verified_at) VALUES
  ('src-04b6b407f896', 'rules', 'УИ: особые права победителей и призёров олимпиад, 2026', 'https://innopolis.university/upload/iblock/af0/5zbznxu6eam9wuz7zi2s6xeegtv512ln/%D0%9E%D0%BB%D0%B8%D0%BC%D0%BF%D0%B8%D0%B0%D0%B4%D1%8B_2026.pdf#page=8', '2026-09-21'::date),
  ('src-2af44cdfb333', 'rules', 'УИ: особые права победителей и призёров олимпиад, 2026', 'https://innopolis.university/upload/iblock/af0/5zbznxu6eam9wuz7zi2s6xeegtv512ln/%D0%9E%D0%BB%D0%B8%D0%BC%D0%BF%D0%B8%D0%B0%D0%B4%D1%8B_2026.pdf#page=11', '2026-09-21'::date),
  ('src-52fc88810aaf', 'rules', 'УИ: особые права победителей и призёров олимпиад, 2026', 'https://innopolis.university/upload/iblock/af0/5zbznxu6eam9wuz7zi2s6xeegtv512ln/%D0%9E%D0%BB%D0%B8%D0%BC%D0%BF%D0%B8%D0%B0%D0%B4%D1%8B_2026.pdf#page=9', '2026-09-21'::date),
  ('src-b89e1faeb309', 'rules', 'УИ: особые права победителей и призёров олимпиад, 2026', 'https://innopolis.university/upload/iblock/af0/5zbznxu6eam9wuz7zi2s6xeegtv512ln/%D0%9E%D0%BB%D0%B8%D0%BC%D0%BF%D0%B8%D0%B0%D0%B4%D1%8B_2026.pdf#page=16', '2026-09-21'::date)
ON CONFLICT (id) DO UPDATE SET
  kind = EXCLUDED.kind,
  title = EXCLUDED.title,
  url = EXCLUDED.url,
  verified_at = EXCLUDED.verified_at;

INSERT INTO benefits (id, olympiad_profile_id, university_id, admission_year, benefit, extra_points, ege_min, diploma_grades, note, source_id) VALUES
  ('p669-13-inzhenernye-nauki__innopolis__2026__bvi', 'p669-13-inzhenernye-nauki', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-2af44cdfb333'),
  ('p669-14-informatika__innopolis__2026__bvi', 'p669-14-informatika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-2af44cdfb333'),
  ('p669-14-matematika__innopolis__2026__bvi', 'p669-14-matematika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Математика', 'src-2af44cdfb333'),
  ('p669-15-informatika__innopolis__2026__bvi', 'p669-15-informatika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-2af44cdfb333'),
  ('p669-17-fizika__innopolis__2026__bvi', 'p669-17-fizika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-2af44cdfb333'),
  ('p669-18-fizika__innopolis__2026__bvi', 'p669-18-fizika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-2af44cdfb333'),
  ('p669-19-fizika__innopolis__2026__bvi', 'p669-19-fizika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-2af44cdfb333'),
  ('p669-22-informacionnaya-bezopasnost__innopolis__2026__bvi', 'p669-22-informacionnaya-bezopasnost', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-2af44cdfb333'),
  ('p669-22-informatika__innopolis__2026__bvi', 'p669-22-informatika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-2af44cdfb333'),
  ('p669-22-iskusstvennyy-intellekt__innopolis__2026__bvi', 'p669-22-iskusstvennyy-intellekt', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-2af44cdfb333'),
  ('p669-22-matematika__innopolis__2026__bvi', 'p669-22-matematika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Математика', 'src-2af44cdfb333'),
  ('p669-22-robototehnika__innopolis__2026__bvi', 'p669-22-robototehnika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-2af44cdfb333'),
  ('p669-23-finansovaya-bezopasnost__innopolis__2026__bvi', 'p669-23-finansovaya-bezopasnost', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-2af44cdfb333'),
  ('p669-26-fizika__innopolis__2026__bvi', 'p669-26-fizika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-2af44cdfb333'),
  ('p669-26-informatika__innopolis__2026__bvi', 'p669-26-informatika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-2af44cdfb333'),
  ('p669-26-matematika__innopolis__2026__bvi', 'p669-26-matematika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Математика', 'src-2af44cdfb333'),
  ('p669-43-fizika__innopolis__2026__bvi', 'p669-43-fizika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-b89e1faeb309'),
  ('p669-43-matematika__innopolis__2026__bvi', 'p669-43-matematika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Математика', 'src-b89e1faeb309'),
  ('p669-49-informatika__innopolis__2026__bvi', 'p669-49-informatika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-b89e1faeb309'),
  ('p669-5-infohimiya__innopolis__2026__bvi', 'p669-5-infohimiya', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-04b6b407f896'),
  ('p669-5-informacionnaya-bezopasnost__innopolis__2026__bvi', 'p669-5-informacionnaya-bezopasnost', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-04b6b407f896'),
  ('p669-5-iskusstvennyy-intellekt__innopolis__2026__bvi', 'p669-5-iskusstvennyy-intellekt', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-04b6b407f896'),
  ('p669-5-kosmicheskie-sistemy-analiz-kosmicheskih-snimkov-i-geoprostranstvennyh-dannyh-sputnikovye-sistemy__innopolis__2026__bvi', 'p669-5-kosmicheskie-sistemy-analiz-kosmicheskih-snimkov-i-geoprostranstvennyh-dannyh-sputnikovye-sistemy', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-52fc88810aaf'),
  ('p669-5-programmnaya-inzheneriya-v-finansovyh-tehnologiyah__innopolis__2026__bvi', 'p669-5-programmnaya-inzheneriya-v-finansovyh-tehnologiyah', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-52fc88810aaf'),
  ('p669-50-fizika__innopolis__2026__bvi', 'p669-50-fizika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-b89e1faeb309'),
  ('p669-50-informatika__innopolis__2026__bvi', 'p669-50-informatika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-b89e1faeb309'),
  ('p669-50-matematika__innopolis__2026__bvi', 'p669-50-matematika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Математика', 'src-b89e1faeb309'),
  ('p669-50-mehanika-i-matematicheskoe-modelirovanie__innopolis__2026__bvi', 'p669-50-mehanika-i-matematicheskoe-modelirovanie', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-b89e1faeb309'),
  ('p669-51-fizika__innopolis__2026__bvi', 'p669-51-fizika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-b89e1faeb309'),
  ('p669-52-fizika__innopolis__2026__bvi', 'p669-52-fizika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-b89e1faeb309'),
  ('p669-52-matematika__innopolis__2026__bvi', 'p669-52-matematika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Математика', 'src-b89e1faeb309'),
  ('p669-53-fizika__innopolis__2026__bvi', 'p669-53-fizika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-b89e1faeb309'),
  ('p669-8-fizika__innopolis__2026__bvi', 'p669-8-fizika', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Физика', 'src-2af44cdfb333'),
  ('p669-8-promyshlennoe-programmirovanie__innopolis__2026__bvi', 'p669-8-promyshlennoe-programmirovanie', 'innopolis', 2026, 'bvi', NULL, 75, ARRAY[9,10,11]::int[], 'Подтвердить ЕГЭ: Информатика', 'src-2af44cdfb333')
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
  'p669-13-inzhenernye-nauki__innopolis__2026__bvi',
  'p669-14-informatika__innopolis__2026__bvi',
  'p669-14-matematika__innopolis__2026__bvi',
  'p669-15-informatika__innopolis__2026__bvi',
  'p669-17-fizika__innopolis__2026__bvi',
  'p669-18-fizika__innopolis__2026__bvi',
  'p669-19-fizika__innopolis__2026__bvi',
  'p669-22-informacionnaya-bezopasnost__innopolis__2026__bvi',
  'p669-22-informatika__innopolis__2026__bvi',
  'p669-22-iskusstvennyy-intellekt__innopolis__2026__bvi',
  'p669-22-matematika__innopolis__2026__bvi',
  'p669-22-robototehnika__innopolis__2026__bvi',
  'p669-23-finansovaya-bezopasnost__innopolis__2026__bvi',
  'p669-26-fizika__innopolis__2026__bvi',
  'p669-26-informatika__innopolis__2026__bvi',
  'p669-26-matematika__innopolis__2026__bvi',
  'p669-43-fizika__innopolis__2026__bvi',
  'p669-43-matematika__innopolis__2026__bvi',
  'p669-49-informatika__innopolis__2026__bvi',
  'p669-5-infohimiya__innopolis__2026__bvi',
  'p669-5-informacionnaya-bezopasnost__innopolis__2026__bvi',
  'p669-5-iskusstvennyy-intellekt__innopolis__2026__bvi',
  'p669-5-kosmicheskie-sistemy-analiz-kosmicheskih-snimkov-i-geoprostranstvennyh-dannyh-sputnikovye-sistemy__innopolis__2026__bvi',
  'p669-5-programmnaya-inzheneriya-v-finansovyh-tehnologiyah__innopolis__2026__bvi',
  'p669-50-fizika__innopolis__2026__bvi',
  'p669-50-informatika__innopolis__2026__bvi',
  'p669-50-matematika__innopolis__2026__bvi',
  'p669-50-mehanika-i-matematicheskoe-modelirovanie__innopolis__2026__bvi',
  'p669-51-fizika__innopolis__2026__bvi',
  'p669-52-fizika__innopolis__2026__bvi',
  'p669-52-matematika__innopolis__2026__bvi',
  'p669-53-fizika__innopolis__2026__bvi',
  'p669-8-fizika__innopolis__2026__bvi',
  'p669-8-promyshlennoe-programmirovanie__innopolis__2026__bvi');
DELETE FROM sources WHERE id IN (
  'src-04b6b407f896',
  'src-2af44cdfb333',
  'src-52fc88810aaf',
  'src-b89e1faeb309');
