-- Сроки семи олимпиад, сверенные 26.10.2026 с сайтами и положениями
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
--
-- В сиде 0003 это уже есть — здесь то же самое для накатанных баз. Триггер
-- 0006 запишет события изменения сроков: кто следит за этими олимпиадами,
-- узнает о новых датах.
--
-- Этапы: удаляется 0, добавляется или меняется 27. Источников новых 0.
-- Олимпиад меняется 0, профилей 5.
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
  ('src-763311cac6e7', 'site', 'Международная олимпиада «Innopolis Open» — сроки этапов', 'https://dovuz.innopolis.university/innopolis-open/robo', '2026-10-26'::date),
  ('src-78bc9874eca5', 'site', 'Международная олимпиада «Innopolis Open» — сроки этапов', 'https://dovuz.innopolis.university/innopolis-open/cyberbez', '2026-10-26'::date),
  ('src-88f0326adf8f', 'site', 'Интернет-олимпиада школьников по физике — сроки этапов', 'https://distolymp2.spbu.ru/', '2026-10-26'::date),
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
  ('p669-49-informatika', 'p669-49', 'inf', 'informatika', 'информатика', 'II', '2026/27', 9, 11, 'src-cc212c88451d')
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
  'p669-49-estestvennye-nauki:registration:2'
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
  ('p669-49-informatika', 'p669-49', 'inf', 'informatika', 'информатика', 'II', '2026/27', 8, 11, 'src-cc212c88451d')
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
