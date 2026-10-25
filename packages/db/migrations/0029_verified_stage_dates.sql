-- Сроки восьми олимпиад, сверенные 26.10.2026 с сайтами и положениями
-- организаторов и с codolymp.ru. Часы — по Москве, там, где организатор их
-- называет (в C — start_time и end_time этапа):
--
-- * Innopolis Open: приём заявок закрывается в 15:00. Второй тур ИИ —
--   23–24 января 2027, ИБ — 17 января (были перепутаны). У математики и
--   информатики отдельный приём заявок на 2-й тур: 26 октября 10:00 —
--   23 ноября 15:00 и 19 октября 10:00 — 18 ноября 12:00.
-- * Интернет-олимпиада по физике (СПбГУ): всё верно, только дата сверки.
-- * Олимпиада им. Верченко: сроки верны; математика и криптография —
--   9–11 классы.
-- * «Гранит науки»: 9–11 классы; запись на финал до 10:00. У профиля
--   «естественные науки» не было этапов: отбор вместе со всеми профилями
--   5 октября — 13 января, запись на финал до 5 февраля 10:00, финал 13 февраля.
-- * «Изумруд»: регистрация и отбор до 23:59 по Екатеринбургу — 21:59 МСК.
-- * МПО КФУ: отбор с 10:00 26 октября (Регламент, п. 4.1); информатика —
--   10–11 классы.
-- * «Бельчонок»: задания отбора закрываются в 17:00 по Красноярску —
--   13:00 МСК.
-- * «Физтех»: у всех семи профилей стояли одни даты (регистрация 7 сентября,
--   отбор 4 октября, финал 13–14 февраля). Теперь по профилям: у физики и
--   математики — три тура онлайн-этапа с регистрацией до 10:00 или 8:00
--   накануне, финал 13 и 14 февраля; у биологии, химии, инженерного дела,
--   информатики и «Старта в науку» — свои отборы и финалы. Финал инженерного
--   дела и информатики объявлен без дня — этап без дат, «дата пока не
--   объявлена». Классы: химия и информатика 7–11, инженерное дело 8–11.
--   Регистрации, которой у профиля нет, больше нет.
--
-- В сиде 0003 это уже есть — здесь то же самое для накатанных баз. Триггер
-- 0006 запишет события изменения сроков: кто следит за этими олимпиадами,
-- узнает о новых датах. На этапы Физтеха, которые удаляются, на проде не
-- ссылается ни одно напоминание и ни один результат трекера.
--
-- Этапы: удаляется 5, добавляется или меняется 54. Источников новых 5.
-- Олимпиад меняется 0, профилей 8.
--
-- Сгенерировано datasets/parser/diff_seed.py --tables stages.

-- +goose Up

INSERT INTO sources (id, kind, title, url, verified_at) VALUES
  ('src-036e87aec746', 'site', 'Межрегиональные предметные олимпиады Казанского (Приволжского) федерального университета — сроки этапов', 'https://malun.kpfu.ru/mpo', '2026-10-26'::date),
  ('src-0f8eff934f85', 'site', 'Университетская олимпиада школьников «Бельчонок» — сроки этапов', 'https://dovuz.sfu.ru/abiturientu-sfu/olimpiady/belchonok/2026-2027-uchebnyy-god/', '2026-10-26'::date),
  ('src-413c79723be5', 'site', 'Международная олимпиада «Innopolis Open» — сроки этапов', 'https://dovuz.innopolis.university/innopolis-open/math', '2026-10-26'::date),
  ('src-431c4749b3b7', 'site', 'Международная олимпиада «Innopolis Open» — сроки этапов', 'https://dovuz.innopolis.university/innopolis-open/informatics', '2026-10-26'::date),
  ('src-446faa35dbb2', 'site', 'Международная олимпиада «Innopolis Open» — сроки этапов', 'https://dovuz.innopolis.university/innopolis-open/ai', '2026-10-26'::date),
  ('src-4c686d3e2dc2', 'site', 'Межрегиональная олимпиада школьников имени И.Я. Верченко — сроки этапов', 'https://v-olymp.ru/olymp/cryptography', '2026-10-26'::date),
  ('src-58fe428b5abc', 'site', 'Олимпиада школьников «Физтех» — сроки этапов', 'https://chem-olymp.mipt.ru/', '2026-10-26'::date),
  ('src-5b372a8be104', 'site', 'Олимпиада школьников «Физтех» — сроки этапов', 'https://start.mipt.ru/view/olymp', '2026-10-26'::date),
  ('src-5e1726ea8b84', 'site', 'Олимпиада школьников «Физтех» — сроки этапов', 'https://olymp-online.mipt.ru', '2026-10-26'::date),
  ('src-64c9731d2e73', 'site', 'Олимпиада школьников «Физтех» — сроки этапов', 'https://uc1.1c.ru/olymp-mipt/uchastnikam/', '2026-10-26'::date),
  ('src-763311cac6e7', 'site', 'Международная олимпиада «Innopolis Open» — сроки этапов', 'https://dovuz.innopolis.university/innopolis-open/robo', '2026-10-26'::date),
  ('src-78bc9874eca5', 'site', 'Международная олимпиада «Innopolis Open» — сроки этапов', 'https://dovuz.innopolis.university/innopolis-open/cyberbez', '2026-10-26'::date),
  ('src-7ae3e4b404bc', 'site', 'Олимпиада школьников «Физтех» — сроки этапов', 'https://bio-olymp.mipt.ru/', '2026-10-26'::date),
  ('src-88f0326adf8f', 'site', 'Интернет-олимпиада школьников по физике — сроки этапов', 'https://distolymp2.spbu.ru/', '2026-10-26'::date),
  ('src-b394c16b7875', 'site', 'Олимпиада школьников «Физтех» — сроки этапов', 'https://olymp.mipt-rse.ru/', '2026-10-26'::date),
  ('src-c1add80231f0', 'site', 'Международная олимпиада школьников Уральского федерального университета «Изумруд» — сроки этапов', 'https://dovuz.urfu.ru/olymps/izumrud', '2026-10-26'::date),
  ('src-c4caa010c8c2', 'site', 'Межрегиональная олимпиада школьников имени И.Я. Верченко — сроки этапов', 'https://v-olymp.ru/olymp/information_security', '2026-10-26'::date),
  ('src-ec710b9ce093', 'site', 'Олимпиада школьников «Гранит науки» — сроки этапов', 'https://ogn.spmi.ru/sites/default/files/manager/2027/KalendPlan_2026-2027.pdf', '2026-10-26'::date)
ON CONFLICT (id) DO UPDATE SET
  kind = EXCLUDED.kind,
  title = EXCLUDED.title,
  url = EXCLUDED.url,
  verified_at = EXCLUDED.verified_at;

INSERT INTO olympiad_profiles (id, olympiad_id, subject_code, profile_slug, profile_name, level, school_year, grades_from, grades_to, source_id) VALUES
  ('p669-31-matematika', 'p669-31', 'math', 'matematika', 'математика', 'II', '2026/27', 9, 11, 'src-cc212c88451d'),
  ('p669-34-informatika', 'p669-34', 'inf', 'informatika', 'информатика', 'III', '2026/27', 10, 11, 'src-cc212c88451d'),
  ('p669-49-estestvennye-nauki', 'p669-49', 'phys', 'estestvennye-nauki', 'естественные науки', 'III', '2026/27', 9, 11, 'src-cc212c88451d'),
  ('p669-49-himiya', 'p669-49', 'chem', 'himiya', 'химия', 'III', '2026/27', 9, 11, 'src-cc212c88451d'),
  ('p669-49-informatika', 'p669-49', 'inf', 'informatika', 'информатика', 'II', '2026/27', 9, 11, 'src-cc212c88451d'),
  ('p669-54-himiya', 'p669-54', 'chem', 'himiya', 'химия', 'III', '2026/27', 7, 11, 'src-cc212c88451d'),
  ('p669-54-informatika-i-programmirovanie', 'p669-54', 'inf', 'informatika-i-programmirovanie', 'информатика и программирование', 'III', '2026/27', 7, 11, 'src-cc212c88451d'),
  ('p669-54-inzhenernoe-delo', 'p669-54', 'phys', 'inzhenernoe-delo', 'инженерное дело', 'II', '2026/27', 8, 11, 'src-cc212c88451d')
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

DELETE FROM stages WHERE id IN (
  'p669-54-biologiya:registration:1',
  'p669-54-himiya:registration:1',
  'p669-54-informatika-i-programmirovanie:registration:1',
  'p669-54-inzhenernoe-delo:registration:1',
  'p669-54-nauchno-tehnicheskiy:registration:1'
);

INSERT INTO stages (id, olympiad_profile_id, kind, title, starts_at, ends_at, deadline_at, is_online, is_demo, source_id) VALUES
  ('p669-22-informacionnaya-bezopasnost:registration:1', 'p669-22-informacionnaya-bezopasnost', 'registration', 'Регистрация на профиль', NULL, '2026-11-11 15:00:00+03'::timestamptz, '2026-11-11 15:00:00+03'::timestamptz, true, false, 'src-78bc9874eca5'),
  ('p669-22-informacionnaya-bezopasnost:qualifying:2', 'p669-22-informacionnaya-bezopasnost', 'qualifying', 'Второй отборочный этап (2-й тур)', '2027-01-17 00:00:00+03'::timestamptz, '2027-01-17 23:59:59+03'::timestamptz, '2027-01-17 23:59:59+03'::timestamptz, true, false, 'src-78bc9874eca5'),
  ('p669-22-informatika:registration:1', 'p669-22-informatika', 'registration', 'Приём заявок на 1-й отборочный тур', NULL, '2026-10-14 15:00:00+03'::timestamptz, '2026-10-14 15:00:00+03'::timestamptz, true, false, 'src-431c4749b3b7'),
  ('p669-22-informatika:registration:2', 'p669-22-informatika', 'registration', 'Приём заявок на 2-й отборочный тур', '2026-10-19 10:00:00+03'::timestamptz, '2026-11-18 12:00:00+03'::timestamptz, '2026-11-18 12:00:00+03'::timestamptz, true, false, 'src-431c4749b3b7'),
  ('p669-22-iskusstvennyy-intellekt:registration:1', 'p669-22-iskusstvennyy-intellekt', 'registration', 'Регистрация на профиль', NULL, '2026-11-01 15:00:00+03'::timestamptz, '2026-11-01 15:00:00+03'::timestamptz, true, false, 'src-446faa35dbb2'),
  ('p669-22-iskusstvennyy-intellekt:qualifying:2', 'p669-22-iskusstvennyy-intellekt', 'qualifying', 'Второй отборочный профильный этап', '2027-01-23 00:00:00+03'::timestamptz, '2027-01-24 23:59:59+03'::timestamptz, '2027-01-24 23:59:59+03'::timestamptz, true, false, 'src-446faa35dbb2'),
  ('p669-22-matematika:registration:1', 'p669-22-matematika', 'registration', 'Регистрация на профиль', NULL, '2026-10-20 15:00:00+03'::timestamptz, '2026-10-20 15:00:00+03'::timestamptz, true, false, 'src-413c79723be5'),
  ('p669-22-matematika:registration:2', 'p669-22-matematika', 'registration', 'Приём заявок на 2-й отборочный тур', '2026-10-26 10:00:00+03'::timestamptz, '2026-11-23 15:00:00+03'::timestamptz, '2026-11-23 15:00:00+03'::timestamptz, true, false, 'src-413c79723be5'),
  ('p669-22-robototehnika:registration:1', 'p669-22-robototehnika', 'registration', 'Регистрация на профиль', NULL, '2026-11-10 15:00:00+03'::timestamptz, '2026-11-10 15:00:00+03'::timestamptz, true, false, 'src-763311cac6e7'),
  ('p669-26-fizika:qualifying:1', 'p669-26-fizika', 'qualifying', 'Регистрация и отборочный этап', '2026-09-01 00:00:00+03'::timestamptz, '2027-01-19 21:59:00+03'::timestamptz, '2027-01-19 21:59:00+03'::timestamptz, true, false, 'src-c1add80231f0'),
  ('p669-26-himiya:qualifying:1', 'p669-26-himiya', 'qualifying', 'Регистрация и отборочный этап', '2026-09-01 00:00:00+03'::timestamptz, '2027-01-19 21:59:00+03'::timestamptz, '2027-01-19 21:59:00+03'::timestamptz, true, false, 'src-c1add80231f0'),
  ('p669-26-informatika:qualifying:1', 'p669-26-informatika', 'qualifying', 'Регистрация и отборочный этап', '2026-09-01 00:00:00+03'::timestamptz, '2027-01-19 21:59:00+03'::timestamptz, '2027-01-19 21:59:00+03'::timestamptz, true, false, 'src-c1add80231f0'),
  ('p669-26-matematika:qualifying:1', 'p669-26-matematika', 'qualifying', 'Регистрация и отборочный этап', '2026-09-01 00:00:00+03'::timestamptz, '2027-01-19 21:59:00+03'::timestamptz, '2027-01-19 21:59:00+03'::timestamptz, true, false, 'src-c1add80231f0'),
  ('p669-26-obschestvoznanie:qualifying:1', 'p669-26-obschestvoznanie', 'qualifying', 'Регистрация и отборочный этап', '2026-09-01 00:00:00+03'::timestamptz, '2027-01-19 21:59:00+03'::timestamptz, '2027-01-19 21:59:00+03'::timestamptz, true, false, 'src-c1add80231f0'),
  ('p669-34-fizika:qualifying:1', 'p669-34-fizika', 'qualifying', 'Отборочный этап', '2026-10-26 10:00:00+03'::timestamptz, '2026-11-23 23:59:59+03'::timestamptz, '2026-11-23 23:59:59+03'::timestamptz, true, false, 'src-036e87aec746'),
  ('p669-34-himiya:qualifying:1', 'p669-34-himiya', 'qualifying', 'Отборочный этап', '2026-10-26 10:00:00+03'::timestamptz, '2026-11-23 23:59:59+03'::timestamptz, '2026-11-23 23:59:59+03'::timestamptz, true, false, 'src-036e87aec746'),
  ('p669-34-informatika:qualifying:1', 'p669-34-informatika', 'qualifying', 'Отборочный этап', '2026-10-26 10:00:00+03'::timestamptz, '2026-11-23 23:59:59+03'::timestamptz, '2026-11-23 23:59:59+03'::timestamptz, true, false, 'src-036e87aec746'),
  ('p669-49-estestvennye-nauki:registration:1', 'p669-49-estestvennye-nauki', 'registration', 'Регистрация на отборочный тур по всем профилям', '2026-10-05 00:00:00+03'::timestamptz, '2027-01-13 23:59:59+03'::timestamptz, '2027-01-13 23:59:59+03'::timestamptz, true, false, 'src-ec710b9ce093'),
  ('p669-49-estestvennye-nauki:qualifying:1', 'p669-49-estestvennye-nauki', 'qualifying', 'Отборочный тур', '2026-10-05 00:00:00+03'::timestamptz, '2027-01-13 23:59:59+03'::timestamptz, '2027-01-13 23:59:59+03'::timestamptz, true, false, 'src-ec710b9ce093'),
  ('p669-49-estestvennye-nauki:registration:2', 'p669-49-estestvennye-nauki', 'registration', 'Регистрация на заключительный тур олимпиады (профиль Естественные науки)', '2027-01-15 00:00:00+03'::timestamptz, '2027-02-05 10:00:00+03'::timestamptz, '2027-02-05 10:00:00+03'::timestamptz, true, false, 'src-ec710b9ce093'),
  ('p669-49-estestvennye-nauki:final:1', 'p669-49-estestvennye-nauki', 'final', 'Заключительный тур (профиль Естественные науки)', '2027-02-13 00:00:00+03'::timestamptz, '2027-02-13 23:59:59+03'::timestamptz, '2027-02-13 00:00:00+03'::timestamptz, false, false, 'src-ec710b9ce093'),
  ('p669-49-himiya:registration:2', 'p669-49-himiya', 'registration', 'Регистрация на заключительный тур олимпиады (профиль Химия)', '2027-01-15 00:00:00+03'::timestamptz, '2027-02-12 10:00:00+03'::timestamptz, '2027-02-12 10:00:00+03'::timestamptz, true, false, 'src-ec710b9ce093'),
  ('p669-49-informatika:registration:2', 'p669-49-informatika', 'registration', 'Регистрация на заключительный тур олимпиады (профиль Информатика)', '2027-01-15 00:00:00+03'::timestamptz, '2027-01-29 10:00:00+03'::timestamptz, '2027-01-29 10:00:00+03'::timestamptz, true, false, 'src-ec710b9ce093'),
  ('p669-54-biologiya:qualifying:1', 'p669-54-biologiya', 'qualifying', 'Отборочный этап, попытка 1', '2026-12-20 00:00:00+03'::timestamptz, '2026-12-20 23:59:59+03'::timestamptz, '2026-12-20 23:59:59+03'::timestamptz, true, false, 'src-7ae3e4b404bc'),
  ('p669-54-biologiya:qualifying:2', 'p669-54-biologiya', 'qualifying', 'Отборочный этап, попытка 2', '2027-01-09 00:00:00+03'::timestamptz, '2027-01-09 23:59:59+03'::timestamptz, '2027-01-09 23:59:59+03'::timestamptz, true, false, 'src-7ae3e4b404bc'),
  ('p669-54-biologiya:final:1', 'p669-54-biologiya', 'final', 'Заключительный этап', '2027-02-22 00:00:00+03'::timestamptz, '2027-02-22 23:59:59+03'::timestamptz, '2027-02-22 00:00:00+03'::timestamptz, false, false, 'src-7ae3e4b404bc'),
  ('p669-54-fizika:registration:1', 'p669-54-fizika', 'registration', 'Регистрация и подтверждение профиля на 1-й тур', '2026-09-07 10:00:00+03'::timestamptz, '2026-10-03 10:00:00+03'::timestamptz, '2026-10-03 10:00:00+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-fizika:qualifying:1', 'p669-54-fizika', 'qualifying', 'Онлайн-этап, 1-й тур', '2026-10-04 10:00:00+03'::timestamptz, '2026-10-04 15:00:00+03'::timestamptz, '2026-10-04 15:00:00+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-fizika:registration:2', 'p669-54-fizika', 'registration', 'Регистрация на 2-й тур', NULL, '2026-10-24 08:00:00+03'::timestamptz, '2026-10-24 08:00:00+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-fizika:qualifying:2', 'p669-54-fizika', 'qualifying', 'Онлайн-этап, 2-й тур', '2026-10-25 08:00:00+03'::timestamptz, '2026-10-25 13:00:00+03'::timestamptz, '2026-10-25 13:00:00+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-fizika:registration:3', 'p669-54-fizika', 'registration', 'Регистрация на 3-й тур (только 11 класс)', NULL, '2026-11-07 10:00:00+03'::timestamptz, '2026-11-07 10:00:00+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-fizika:qualifying:3', 'p669-54-fizika', 'qualifying', 'Онлайн-этап, 3-й тур (только 11 класс)', '2026-11-08 10:00:00+03'::timestamptz, '2026-11-08 15:00:00+03'::timestamptz, '2026-11-08 15:00:00+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-fizika:final:1', 'p669-54-fizika', 'final', 'Заключительный этап', '2027-02-13 00:00:00+03'::timestamptz, '2027-02-13 23:59:59+03'::timestamptz, '2027-02-13 00:00:00+03'::timestamptz, false, false, 'src-5e1726ea8b84'),
  ('p669-54-himiya:qualifying:1', 'p669-54-himiya', 'qualifying', 'Отборочный этап', '2026-12-15 00:00:00+03'::timestamptz, '2027-01-10 23:59:59+03'::timestamptz, '2027-01-10 23:59:59+03'::timestamptz, true, false, 'src-58fe428b5abc'),
  ('p669-54-himiya:final:1', 'p669-54-himiya', 'final', 'Заключительный этап', '2027-02-21 00:00:00+03'::timestamptz, '2027-02-21 23:59:59+03'::timestamptz, '2027-02-21 00:00:00+03'::timestamptz, false, false, 'src-58fe428b5abc'),
  ('p669-54-informatika-i-programmirovanie:qualifying:1', 'p669-54-informatika-i-programmirovanie', 'qualifying', 'Первый отборочный этап', '2026-09-28 00:00:00+03'::timestamptz, '2026-11-05 23:59:59+03'::timestamptz, '2026-11-05 23:59:59+03'::timestamptz, true, false, 'src-64c9731d2e73'),
  ('p669-54-informatika-i-programmirovanie:qualifying:2', 'p669-54-informatika-i-programmirovanie', 'qualifying', 'Второй отборочный этап', '2026-11-20 00:00:00+03'::timestamptz, '2027-01-25 23:59:59+03'::timestamptz, '2027-01-25 23:59:59+03'::timestamptz, true, false, 'src-64c9731d2e73'),
  ('p669-54-informatika-i-programmirovanie:final:1', 'p669-54-informatika-i-programmirovanie', 'final', 'Заключительный этап, март 2027 — дата пока не объявлена', NULL, NULL, NULL, false, false, 'src-64c9731d2e73'),
  ('p669-54-inzhenernoe-delo:qualifying:1', 'p669-54-inzhenernoe-delo', 'qualifying', 'Первый отборочный этап', '2026-10-15 00:00:00+03'::timestamptz, '2026-11-25 23:59:59+03'::timestamptz, '2026-11-25 23:59:59+03'::timestamptz, true, false, 'src-b394c16b7875'),
  ('p669-54-inzhenernoe-delo:qualifying:2', 'p669-54-inzhenernoe-delo', 'qualifying', 'Второй отборочный этап', '2026-12-05 00:00:00+03'::timestamptz, '2027-01-15 23:59:59+03'::timestamptz, '2027-01-15 23:59:59+03'::timestamptz, true, false, 'src-b394c16b7875'),
  ('p669-54-inzhenernoe-delo:final:1', 'p669-54-inzhenernoe-delo', 'final', 'Заключительный этап — дата пока не объявлена', NULL, NULL, NULL, false, false, 'src-b394c16b7875'),
  ('p669-54-matematika:registration:1', 'p669-54-matematika', 'registration', 'Регистрация и подтверждение профиля на 1-й тур', '2026-09-07 10:00:00+03'::timestamptz, '2026-10-10 10:00:00+03'::timestamptz, '2026-10-10 10:00:00+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-matematika:qualifying:1', 'p669-54-matematika', 'qualifying', 'Онлайн-этап, 1-й тур', '2026-10-11 10:00:00+03'::timestamptz, '2026-10-11 15:00:00+03'::timestamptz, '2026-10-11 15:00:00+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-matematika:registration:2', 'p669-54-matematika', 'registration', 'Регистрация на 2-й тур', NULL, '2026-10-31 08:00:00+03'::timestamptz, '2026-10-31 08:00:00+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-matematika:qualifying:2', 'p669-54-matematika', 'qualifying', 'Онлайн-этап, 2-й тур', '2026-11-01 08:00:00+03'::timestamptz, '2026-11-01 13:00:00+03'::timestamptz, '2026-11-01 13:00:00+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-matematika:registration:3', 'p669-54-matematika', 'registration', 'Регистрация на 3-й тур (только 11 класс)', NULL, '2026-11-06 10:00:00+03'::timestamptz, '2026-11-06 10:00:00+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-matematika:qualifying:3', 'p669-54-matematika', 'qualifying', 'Онлайн-этап, 3-й тур (только 11 класс)', '2026-11-07 10:00:00+03'::timestamptz, '2026-11-07 15:00:00+03'::timestamptz, '2026-11-07 15:00:00+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-matematika:final:1', 'p669-54-matematika', 'final', 'Заключительный этап', '2027-02-14 00:00:00+03'::timestamptz, '2027-02-14 23:59:59+03'::timestamptz, '2027-02-14 00:00:00+03'::timestamptz, false, false, 'src-5e1726ea8b84'),
  ('p669-54-nauchno-tehnicheskiy:qualifying:1', 'p669-54-nauchno-tehnicheskiy', 'qualifying', 'Отборочный этап: онлайн-тестирование и научная работа', '2026-10-10 10:00:00+03'::timestamptz, '2026-12-30 23:55:00+03'::timestamptz, '2026-12-30 23:55:00+03'::timestamptz, true, false, 'src-5b372a8be104'),
  ('p669-54-nauchno-tehnicheskiy:final:1', 'p669-54-nauchno-tehnicheskiy', 'final', 'Заключительный этап в Долгопрудном', '2027-03-27 00:00:00+03'::timestamptz, '2027-04-04 23:59:59+03'::timestamptz, '2027-03-27 00:00:00+03'::timestamptz, false, false, 'src-5b372a8be104'),
  ('p669-83-fizika:qualifying:1', 'p669-83-fizika', 'qualifying', '1 этап (отборочный)', '2026-10-01 00:00:00+03'::timestamptz, '2027-01-12 13:00:00+03'::timestamptz, '2027-01-12 13:00:00+03'::timestamptz, true, false, 'src-0f8eff934f85'),
  ('p669-83-himiya:qualifying:1', 'p669-83-himiya', 'qualifying', '1 этап (отборочный)', '2026-10-01 00:00:00+03'::timestamptz, '2027-01-12 13:00:00+03'::timestamptz, '2027-01-12 13:00:00+03'::timestamptz, true, false, 'src-0f8eff934f85'),
  ('p669-83-informatika:qualifying:1', 'p669-83-informatika', 'qualifying', '1 этап (отборочный)', '2026-10-01 00:00:00+03'::timestamptz, '2027-01-12 13:00:00+03'::timestamptz, '2027-01-12 13:00:00+03'::timestamptz, true, false, 'src-0f8eff934f85'),
  ('p669-83-matematika:qualifying:1', 'p669-83-matematika', 'qualifying', '1 этап (отборочный)', '2026-10-01 00:00:00+03'::timestamptz, '2027-01-12 13:00:00+03'::timestamptz, '2027-01-12 13:00:00+03'::timestamptz, true, false, 'src-0f8eff934f85')
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
  'p669-22-informatika:registration:2',
  'p669-22-matematika:registration:2',
  'p669-49-estestvennye-nauki:final:1',
  'p669-49-estestvennye-nauki:qualifying:1',
  'p669-49-estestvennye-nauki:registration:1',
  'p669-49-estestvennye-nauki:registration:2',
  'p669-54-biologiya:qualifying:2',
  'p669-54-fizika:qualifying:2',
  'p669-54-fizika:qualifying:3',
  'p669-54-fizika:registration:2',
  'p669-54-fizika:registration:3',
  'p669-54-informatika-i-programmirovanie:qualifying:2',
  'p669-54-inzhenernoe-delo:qualifying:2',
  'p669-54-matematika:qualifying:2',
  'p669-54-matematika:qualifying:3',
  'p669-54-matematika:registration:2',
  'p669-54-matematika:registration:3'
);

INSERT INTO stages (id, olympiad_profile_id, kind, title, starts_at, ends_at, deadline_at, is_online, is_demo, source_id) VALUES
  ('p669-22-informacionnaya-bezopasnost:registration:1', 'p669-22-informacionnaya-bezopasnost', 'registration', 'Регистрация на профиль', NULL, '2026-11-11 23:59:59+03'::timestamptz, '2026-11-11 23:59:59+03'::timestamptz, true, false, 'src-78bc9874eca5'),
  ('p669-22-informacionnaya-bezopasnost:qualifying:2', 'p669-22-informacionnaya-bezopasnost', 'qualifying', 'Второй отборочный этап (2-й тур)', '2027-01-24 00:00:00+03'::timestamptz, '2027-01-24 23:59:59+03'::timestamptz, '2027-01-24 23:59:59+03'::timestamptz, true, false, 'src-78bc9874eca5'),
  ('p669-22-informatika:registration:1', 'p669-22-informatika', 'registration', 'Приём заявок на 1-й отборочный тур', NULL, '2026-10-14 23:59:59+03'::timestamptz, '2026-10-14 23:59:59+03'::timestamptz, true, false, 'src-431c4749b3b7'),
  ('p669-22-iskusstvennyy-intellekt:registration:1', 'p669-22-iskusstvennyy-intellekt', 'registration', 'Регистрация на профиль', NULL, '2026-11-01 23:59:59+03'::timestamptz, '2026-11-01 23:59:59+03'::timestamptz, true, false, 'src-446faa35dbb2'),
  ('p669-22-iskusstvennyy-intellekt:qualifying:2', 'p669-22-iskusstvennyy-intellekt', 'qualifying', 'Второй отборочный профильный этап', '2027-01-16 00:00:00+03'::timestamptz, '2027-01-17 23:59:59+03'::timestamptz, '2027-01-17 23:59:59+03'::timestamptz, true, false, 'src-446faa35dbb2'),
  ('p669-22-matematika:registration:1', 'p669-22-matematika', 'registration', 'Регистрация на профиль', NULL, '2026-10-20 23:59:59+03'::timestamptz, '2026-10-20 23:59:59+03'::timestamptz, true, false, 'src-413c79723be5'),
  ('p669-22-robototehnika:registration:1', 'p669-22-robototehnika', 'registration', 'Регистрация на профиль', NULL, '2026-11-10 23:59:59+03'::timestamptz, '2026-11-10 23:59:59+03'::timestamptz, true, false, 'src-763311cac6e7'),
  ('p669-26-fizika:qualifying:1', 'p669-26-fizika', 'qualifying', 'Регистрация и отборочный этап', '2026-09-01 00:00:00+03'::timestamptz, '2027-01-19 23:59:59+03'::timestamptz, '2027-01-19 23:59:59+03'::timestamptz, true, false, 'src-c1add80231f0'),
  ('p669-26-himiya:qualifying:1', 'p669-26-himiya', 'qualifying', 'Регистрация и отборочный этап', '2026-09-01 00:00:00+03'::timestamptz, '2027-01-19 23:59:59+03'::timestamptz, '2027-01-19 23:59:59+03'::timestamptz, true, false, 'src-c1add80231f0'),
  ('p669-26-informatika:qualifying:1', 'p669-26-informatika', 'qualifying', 'Регистрация и отборочный этап', '2026-09-01 00:00:00+03'::timestamptz, '2027-01-19 23:59:59+03'::timestamptz, '2027-01-19 23:59:59+03'::timestamptz, true, false, 'src-c1add80231f0'),
  ('p669-26-matematika:qualifying:1', 'p669-26-matematika', 'qualifying', 'Регистрация и отборочный этап', '2026-09-01 00:00:00+03'::timestamptz, '2027-01-19 23:59:59+03'::timestamptz, '2027-01-19 23:59:59+03'::timestamptz, true, false, 'src-c1add80231f0'),
  ('p669-26-obschestvoznanie:qualifying:1', 'p669-26-obschestvoznanie', 'qualifying', 'Регистрация и отборочный этап', '2026-09-01 00:00:00+03'::timestamptz, '2027-01-19 23:59:59+03'::timestamptz, '2027-01-19 23:59:59+03'::timestamptz, true, false, 'src-c1add80231f0'),
  ('p669-34-fizika:qualifying:1', 'p669-34-fizika', 'qualifying', 'Отборочный этап', '2026-10-26 00:00:00+03'::timestamptz, '2026-11-23 23:59:59+03'::timestamptz, '2026-11-23 23:59:59+03'::timestamptz, true, false, 'src-036e87aec746'),
  ('p669-34-himiya:qualifying:1', 'p669-34-himiya', 'qualifying', 'Отборочный этап', '2026-10-26 00:00:00+03'::timestamptz, '2026-11-23 23:59:59+03'::timestamptz, '2026-11-23 23:59:59+03'::timestamptz, true, false, 'src-036e87aec746'),
  ('p669-34-informatika:qualifying:1', 'p669-34-informatika', 'qualifying', 'Отборочный этап', '2026-10-26 00:00:00+03'::timestamptz, '2026-11-23 23:59:59+03'::timestamptz, '2026-11-23 23:59:59+03'::timestamptz, true, false, 'src-036e87aec746'),
  ('p669-49-himiya:registration:2', 'p669-49-himiya', 'registration', 'Регистрация на заключительный тур олимпиады (профиль Химия)', '2027-01-15 00:00:00+03'::timestamptz, '2027-02-12 23:59:59+03'::timestamptz, '2027-02-12 23:59:59+03'::timestamptz, true, false, 'src-ec710b9ce093'),
  ('p669-49-informatika:registration:2', 'p669-49-informatika', 'registration', 'Регистрация на заключительный тур олимпиады (профиль Информатика)', '2027-01-15 00:00:00+03'::timestamptz, '2027-01-29 23:59:59+03'::timestamptz, '2027-01-29 23:59:59+03'::timestamptz, true, false, 'src-ec710b9ce093'),
  ('p669-54-biologiya:registration:1', 'p669-54-biologiya', 'registration', 'Регистрация', '2026-09-07 00:00:00+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-biologiya:qualifying:1', 'p669-54-biologiya', 'qualifying', 'Отборочный этап', '2026-10-04 00:00:00+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-biologiya:final:1', 'p669-54-biologiya', 'final', 'Заключительный этап', '2027-02-13 00:00:00+03'::timestamptz, '2027-02-14 23:59:59+03'::timestamptz, '2027-02-13 00:00:00+03'::timestamptz, false, false, 'src-5e1726ea8b84'),
  ('p669-54-fizika:registration:1', 'p669-54-fizika', 'registration', 'Регистрация', '2026-09-07 00:00:00+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-fizika:qualifying:1', 'p669-54-fizika', 'qualifying', 'Отборочный этап', '2026-10-04 00:00:00+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-fizika:final:1', 'p669-54-fizika', 'final', 'Заключительный этап', '2027-02-13 00:00:00+03'::timestamptz, '2027-02-14 23:59:59+03'::timestamptz, '2027-02-13 00:00:00+03'::timestamptz, false, false, 'src-5e1726ea8b84'),
  ('p669-54-himiya:registration:1', 'p669-54-himiya', 'registration', 'Регистрация', '2026-09-07 00:00:00+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-himiya:qualifying:1', 'p669-54-himiya', 'qualifying', 'Отборочный этап', '2026-10-04 00:00:00+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-himiya:final:1', 'p669-54-himiya', 'final', 'Заключительный этап', '2027-02-13 00:00:00+03'::timestamptz, '2027-02-14 23:59:59+03'::timestamptz, '2027-02-13 00:00:00+03'::timestamptz, false, false, 'src-5e1726ea8b84'),
  ('p669-54-informatika-i-programmirovanie:registration:1', 'p669-54-informatika-i-programmirovanie', 'registration', 'Регистрация', '2026-09-07 00:00:00+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-informatika-i-programmirovanie:qualifying:1', 'p669-54-informatika-i-programmirovanie', 'qualifying', 'Отборочный этап', '2026-10-04 00:00:00+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-informatika-i-programmirovanie:final:1', 'p669-54-informatika-i-programmirovanie', 'final', 'Заключительный этап', '2027-02-13 00:00:00+03'::timestamptz, '2027-02-14 23:59:59+03'::timestamptz, '2027-02-13 00:00:00+03'::timestamptz, false, false, 'src-5e1726ea8b84'),
  ('p669-54-inzhenernoe-delo:registration:1', 'p669-54-inzhenernoe-delo', 'registration', 'Регистрация', '2026-09-07 00:00:00+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-inzhenernoe-delo:qualifying:1', 'p669-54-inzhenernoe-delo', 'qualifying', 'Отборочный этап', '2026-10-04 00:00:00+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-inzhenernoe-delo:final:1', 'p669-54-inzhenernoe-delo', 'final', 'Заключительный этап', '2027-02-13 00:00:00+03'::timestamptz, '2027-02-14 23:59:59+03'::timestamptz, '2027-02-13 00:00:00+03'::timestamptz, false, false, 'src-5e1726ea8b84'),
  ('p669-54-matematika:registration:1', 'p669-54-matematika', 'registration', 'Регистрация', '2026-09-07 00:00:00+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-matematika:qualifying:1', 'p669-54-matematika', 'qualifying', 'Отборочный этап', '2026-10-04 00:00:00+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-matematika:final:1', 'p669-54-matematika', 'final', 'Заключительный этап', '2027-02-13 00:00:00+03'::timestamptz, '2027-02-14 23:59:59+03'::timestamptz, '2027-02-13 00:00:00+03'::timestamptz, false, false, 'src-5e1726ea8b84'),
  ('p669-54-nauchno-tehnicheskiy:registration:1', 'p669-54-nauchno-tehnicheskiy', 'registration', 'Регистрация', '2026-09-07 00:00:00+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, '2026-09-07 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-nauchno-tehnicheskiy:qualifying:1', 'p669-54-nauchno-tehnicheskiy', 'qualifying', 'Отборочный этап', '2026-10-04 00:00:00+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, '2026-10-04 23:59:59+03'::timestamptz, true, false, 'src-5e1726ea8b84'),
  ('p669-54-nauchno-tehnicheskiy:final:1', 'p669-54-nauchno-tehnicheskiy', 'final', 'Заключительный этап', '2027-02-13 00:00:00+03'::timestamptz, '2027-02-14 23:59:59+03'::timestamptz, '2027-02-13 00:00:00+03'::timestamptz, false, false, 'src-5e1726ea8b84'),
  ('p669-83-fizika:qualifying:1', 'p669-83-fizika', 'qualifying', '1 этап (отборочный)', '2026-10-01 00:00:00+03'::timestamptz, '2027-01-12 23:59:59+03'::timestamptz, '2027-01-12 23:59:59+03'::timestamptz, true, false, 'src-0f8eff934f85'),
  ('p669-83-himiya:qualifying:1', 'p669-83-himiya', 'qualifying', '1 этап (отборочный)', '2026-10-01 00:00:00+03'::timestamptz, '2027-01-12 23:59:59+03'::timestamptz, '2027-01-12 23:59:59+03'::timestamptz, true, false, 'src-0f8eff934f85'),
  ('p669-83-informatika:qualifying:1', 'p669-83-informatika', 'qualifying', '1 этап (отборочный)', '2026-10-01 00:00:00+03'::timestamptz, '2027-01-12 23:59:59+03'::timestamptz, '2027-01-12 23:59:59+03'::timestamptz, true, false, 'src-0f8eff934f85'),
  ('p669-83-matematika:qualifying:1', 'p669-83-matematika', 'qualifying', '1 этап (отборочный)', '2026-10-01 00:00:00+03'::timestamptz, '2027-01-12 23:59:59+03'::timestamptz, '2027-01-12 23:59:59+03'::timestamptz, true, false, 'src-0f8eff934f85')
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

INSERT INTO olympiad_profiles (id, olympiad_id, subject_code, profile_slug, profile_name, level, school_year, grades_from, grades_to, source_id) VALUES
  ('p669-31-matematika', 'p669-31', 'math', 'matematika', 'математика', 'II', '2026/27', 8, 11, 'src-cc212c88451d'),
  ('p669-34-informatika', 'p669-34', 'inf', 'informatika', 'информатика', 'III', '2026/27', 8, 11, 'src-cc212c88451d'),
  ('p669-49-estestvennye-nauki', 'p669-49', 'phys', 'estestvennye-nauki', 'естественные науки', 'III', '2026/27', 8, 11, 'src-cc212c88451d'),
  ('p669-49-himiya', 'p669-49', 'chem', 'himiya', 'химия', 'III', '2026/27', 8, 11, 'src-cc212c88451d'),
  ('p669-49-informatika', 'p669-49', 'inf', 'informatika', 'информатика', 'II', '2026/27', 8, 11, 'src-cc212c88451d'),
  ('p669-54-himiya', 'p669-54', 'chem', 'himiya', 'химия', 'III', '2026/27', 9, 11, 'src-cc212c88451d'),
  ('p669-54-informatika-i-programmirovanie', 'p669-54', 'inf', 'informatika-i-programmirovanie', 'информатика и программирование', 'III', '2026/27', 9, 11, 'src-cc212c88451d'),
  ('p669-54-inzhenernoe-delo', 'p669-54', 'phys', 'inzhenernoe-delo', 'инженерное дело', 'II', '2026/27', 9, 11, 'src-cc212c88451d')
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

DELETE FROM sources s WHERE s.id IN (
  'src-58fe428b5abc',
  'src-5b372a8be104',
  'src-64c9731d2e73',
  'src-7ae3e4b404bc',
  'src-b394c16b7875')
  AND NOT EXISTS (SELECT 1 FROM benefits b WHERE b.source_id = s.id)
  AND NOT EXISTS (SELECT 1 FROM direction_benefits d WHERE d.source_id = s.id)
  AND NOT EXISTS (SELECT 1 FROM stages st WHERE st.source_id = s.id)
  AND NOT EXISTS (SELECT 1 FROM olympiad_profiles p WHERE p.source_id = s.id);
