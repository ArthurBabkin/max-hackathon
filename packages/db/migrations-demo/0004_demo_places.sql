-- Места и опыт демо-траектории (основная миграция 0016): Артём из Казани,
-- был на школьном этапе, учиться хочет в Татарстане. Демо 0003 пишет место
-- в устаревший target_region_code, а переносит его основная миграция только
-- для уже существующих строк — на чистой базе демо идут после неё.

-- +goose Up

INSERT INTO trajectory_places (trajectory_id, position, region_code)
SELECT id, 0, '16' FROM trajectories
WHERE id = '00000000-0000-4000-8000-000000000101'
  AND NOT EXISTS (SELECT 1 FROM trajectory_places WHERE trajectory_id = '00000000-0000-4000-8000-000000000101');

UPDATE trajectories SET experience = 'school', home_city = 'Казань'
WHERE id = '00000000-0000-4000-8000-000000000101' AND experience IS NULL;

-- +goose Down

UPDATE trajectories SET experience = NULL, home_city = NULL
WHERE id = '00000000-0000-4000-8000-000000000101';
