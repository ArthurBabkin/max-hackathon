-- Направления демо-траектории (основная миграция 0014): Артём целится в
-- программную инженерию и информационную безопасность, учиться хочет в
-- Татарстане. Демо 0001 пишет направление в устаревший direction_id, а
-- переносит его основная миграция только для уже существующих строк.

-- +goose Up

INSERT INTO trajectory_directions (trajectory_id, direction_id, position) VALUES
  ('00000000-0000-4000-8000-000000000101', 'napr-09-03-04', 0),
  ('00000000-0000-4000-8000-000000000101', 'napr-10-03-01', 1)
ON CONFLICT DO NOTHING;

UPDATE trajectories SET target_region_code = '16', goal_status = 'known'
WHERE id = '00000000-0000-4000-8000-000000000101';

-- +goose Down

DELETE FROM trajectory_directions WHERE trajectory_id = '00000000-0000-4000-8000-000000000101';
UPDATE trajectories SET target_region_code = NULL WHERE id = '00000000-0000-4000-8000-000000000101';
