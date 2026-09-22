-- Демо-траектория для проверки мини-приложения в обычном браузере.
--
-- Накатывается ТОЛЬКО локально, отдельным compose-сервисом seed-demo со своей
-- таблицей версий goose (goose_demo_version). В Terraform ссылок на этот
-- каталог нет, поэтому в прод демо-данные не попадают.
--
-- Персонажи прототипа: Артём — ученик и создатель траектории, Ольга — мама.
-- Их id MAX лежат в дев-диапазоне 900000000–900000999: заглушка MAX Bridge
-- в браузере представляется Артёмом (900000001), а с ?dev_user=parent —
-- Ольгой (900000002). Живых аккаунтов MAX с такими id нет.
--
-- Идентификаторы фиксированы, вставки — ON CONFLICT DO NOTHING: повторный
-- прогон ничего не ломает.

-- +goose Up

INSERT INTO users (id, max_user_id, first_name) VALUES
  ('00000000-0000-4000-8000-000000000001', 900000001, 'Артём'),
  ('00000000-0000-4000-8000-000000000002', 900000002, 'Ольга')
ON CONFLICT DO NOTHING;

INSERT INTO trajectories (id, student_name, grade, region_code, tz, direction_id, goal_status) VALUES
  ('00000000-0000-4000-8000-000000000101', 'Артём', 9, '16', 'Europe/Moscow', 'napr-09-03-04', 'known')
ON CONFLICT DO NOTHING;

INSERT INTO members (id, trajectory_id, user_id, role, is_creator) VALUES
  ('00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000101',
   '00000000-0000-4000-8000-000000000001', 'kid', true),
  ('00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000000101',
   '00000000-0000-4000-8000-000000000002', 'parent', false)
ON CONFLICT DO NOTHING;

INSERT INTO trajectory_subjects (trajectory_id, subject_code) VALUES
  ('00000000-0000-4000-8000-000000000101', 'inf'),
  ('00000000-0000-4000-8000-000000000101', 'math')
ON CONFLICT DO NOTHING;

INSERT INTO trajectory_universities (trajectory_id, university_id) VALUES
  ('00000000-0000-4000-8000-000000000101', 'innopolis'),
  ('00000000-0000-4000-8000-000000000101', 'hse'),
  ('00000000-0000-4000-8000-000000000101', 'kfu')
ON CONFLICT DO NOTHING;

-- Два пункта трекера: «Высшую пробу» Артём добавил сам, а регистрацию отметила
-- Ольга — так видна подпись «Отметил(а) Ольга». ВсОШ ещё без регистрации — это
-- «следующий шаг» на главной.
INSERT INTO tracker_items (id, trajectory_id, olympiad_profile_id, added_by_member_id,
                           registered_at, registered_by_member_id) VALUES
  ('00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000101',
   'p669-8-informatika', '00000000-0000-4000-8000-000000000201',
   '2026-09-20 18:30:00+03', '00000000-0000-4000-8000-000000000202'),
  ('00000000-0000-4000-8000-000000000302', '00000000-0000-4000-8000-000000000101',
   'vsosh-informatika', '00000000-0000-4000-8000-000000000201', NULL, NULL)
ON CONFLICT DO NOTHING;

-- Ольга предложила НТО по искусственному интеллекту — Артём ещё не ответил.
INSERT INTO proposals (id, trajectory_id, olympiad_profile_id, proposed_by_member_id) VALUES
  ('00000000-0000-4000-8000-000000000401', '00000000-0000-4000-8000-000000000101',
   'p669-5-iskusstvennyy-intellekt', '00000000-0000-4000-8000-000000000202')
ON CONFLICT DO NOTHING;

-- Неиспользованная ссылка-приглашение для второго родителя.
INSERT INTO invites (id, trajectory_id, token, role, created_by_member_id) VALUES
  ('00000000-0000-4000-8000-000000000501', '00000000-0000-4000-8000-000000000101',
   'demoInviteParent01', 'parent', '00000000-0000-4000-8000-000000000201')
ON CONFLICT DO NOTHING;

-- +goose Down

DELETE FROM trajectories WHERE id = '00000000-0000-4000-8000-000000000101';
DELETE FROM users WHERE id IN ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000002');
