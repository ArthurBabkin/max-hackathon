-- Онбординг v2 (docs/onboarding-v2/SPEC.md, раздел 1): опыт в олимпиадах,
-- город ученика, «пусть ребёнок ответит» и несколько мест «Где учиться»
-- вместо одного target_region_code. Новые шаги диалога: интересы, работа,
-- предложение направлений и опыт.

-- +goose Up

-- Опыт в олимпиадах: none — пока не было, school — школьный или
-- муниципальный этап, region — региональный и выше. NULL — траектория
-- создана до вопроса, подбор считает её как none.
ALTER TABLE trajectories ADD COLUMN experience text
    CONSTRAINT trajectories_experience_check CHECK (experience IN ('none', 'school', 'region'));
-- Город ученика, если известен (поиск или геолокация). Регион — region_code.
ALTER TABLE trajectories ADD COLUMN home_city text;
-- Родитель выбрал «Пусть ребёнок ответит»: интересы спросим у ученика при
-- входе по приглашению.
ALTER TABLE trajectories ADD COLUMN goal_by_kid boolean NOT NULL DEFAULT false;

-- Где ученик хочет учиться: несколько мест. city NULL — весь регион.
-- Строк нет — «не важно». Москва и Петербург раскрываются в область через
-- refdata.Metro.
CREATE TABLE trajectory_places (
    trajectory_id uuid NOT NULL REFERENCES trajectories(id) ON DELETE CASCADE,
    position      smallint NOT NULL,
    region_code   text NOT NULL,
    city          text,
    PRIMARY KEY (trajectory_id, position)
);

INSERT INTO trajectory_places (trajectory_id, position, region_code)
SELECT id, 0, target_region_code FROM trajectories WHERE target_region_code IS NOT NULL;

-- Код больше не читает и не пишет target_region_code. Колонка остаётся на
-- время раскатки: миграции идут раньше функций. Удалить следующей миграцией.
COMMENT ON COLUMN trajectories.target_region_code IS 'Устарела: места в trajectory_places';

ALTER TABLE bot_dialogs DROP CONSTRAINT bot_dialogs_step_check;
ALTER TABLE bot_dialogs ADD CONSTRAINT bot_dialogs_step_check CHECK (step IN (
    'role', 'name_confirm', 'name_input', 'grade', 'region',
    'subjects', 'direction', 'interest', 'work', 'suggest', 'experience',
    'target', 'universities', 'university_search', 'join_confirm', 'join_direction',
    'summary', 'done'));

-- +goose Down

-- Диалоги на новых шагах продолжаются со старых: интересы — с выбора
-- направлений, опыт — с вопроса «Где учиться».
UPDATE bot_dialogs SET step = 'direction' WHERE step IN ('interest', 'work', 'suggest');
UPDATE bot_dialogs SET step = 'target' WHERE step = 'experience';
ALTER TABLE bot_dialogs DROP CONSTRAINT bot_dialogs_step_check;
ALTER TABLE bot_dialogs ADD CONSTRAINT bot_dialogs_step_check CHECK (step IN (
    'role', 'name_confirm', 'name_input', 'grade', 'region',
    'subjects', 'direction', 'target', 'universities',
    'university_search', 'join_confirm', 'join_direction',
    'summary', 'done'));

UPDATE trajectories t SET target_region_code = (
    SELECT p.region_code FROM trajectory_places p WHERE p.trajectory_id = t.id ORDER BY p.position LIMIT 1);
COMMENT ON COLUMN trajectories.target_region_code IS NULL;
DROP TABLE trajectory_places;

ALTER TABLE trajectories DROP COLUMN goal_by_kid;
ALTER TABLE trajectories DROP COLUMN home_city;
ALTER TABLE trajectories DROP COLUMN experience;
