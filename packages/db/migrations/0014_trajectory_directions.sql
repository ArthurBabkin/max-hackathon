-- Новый онбординг (F8, F9): несколько направлений вместо одного, «пока не
-- решил» вместо опросника про интересы и город, где ученик хочет учиться, —
-- по нему бот предлагает вузы.

-- +goose Up

-- Направления траектории. position — порядок выбора: первое направление
-- показывается в сводке первым.
CREATE TABLE trajectory_directions (
    trajectory_id uuid NOT NULL REFERENCES trajectories(id) ON DELETE CASCADE,
    direction_id  text NOT NULL REFERENCES directions(id) ON DELETE CASCADE,
    position      smallint NOT NULL,
    PRIMARY KEY (trajectory_id, direction_id)
);

INSERT INTO trajectory_directions (trajectory_id, direction_id, position)
SELECT id, direction_id, 0 FROM trajectories WHERE direction_id IS NOT NULL;

-- trajectories.direction_id код больше не читает и не пишет. Колонка остаётся
-- на время раскатки: миграции идут раньше функций, и старая версия ещё
-- несколько минут её читает. Удалить следующей миграцией.
COMMENT ON COLUMN trajectories.direction_id IS 'Устарела: направления в trajectory_directions';

-- exploring — ученик пока не решил, направлений нет. suggested остаётся для
-- старых траекторий: так цель выбиралась по опроснику, которого больше нет.
ALTER TABLE trajectories DROP CONSTRAINT trajectories_goal_status_check;
ALTER TABLE trajectories ADD CONSTRAINT trajectories_goal_status_check
    CHECK (goal_status IN ('known', 'suggested', 'exploring'));
UPDATE trajectories SET goal_status = 'exploring' WHERE direction_id IS NULL;

-- Где ученик хочет учиться: код субъекта РФ, NULL — не важно. Москва
-- включает область, Петербург — Ленинградскую область (refdata.Metro).
ALTER TABLE trajectories ADD COLUMN target_region_code text;

-- Субъект РФ вуза: по нему бот предлагает вузы в выбранном городе.
ALTER TABLE universities ADD COLUMN region_code text;
UPDATE universities SET region_code = CASE city
    WHEN 'Москва' THEN '77'
    WHEN 'Санкт-Петербург' THEN '78'
    WHEN 'Долгопрудный' THEN '50'
    WHEN 'Казань' THEN '16'
    WHEN 'Иннополис' THEN '16'
    WHEN 'Новосибирск' THEN '54'
END;

-- Шаги goal и interests ушли вместе с опросником, target — новый вопрос о
-- городе.
ALTER TABLE bot_dialogs DROP CONSTRAINT bot_dialogs_step_check;
ALTER TABLE bot_dialogs ADD CONSTRAINT bot_dialogs_step_check CHECK (step IN (
    'role', 'name_confirm', 'name_input', 'grade', 'region',
    'subjects', 'goal', 'interests', 'direction', 'target', 'universities',
    'university_search', 'join_confirm', 'join_direction',
    'summary', 'done'));

-- Незаконченные диалоги продолжаются с выбора направлений.
UPDATE bot_dialogs SET step = 'direction', draft = draft - 'interests' - 'goal_status' - 'direction_id'
WHERE step IN ('goal', 'interests');
-- Кто уже выбирает вузы, ещё не отвечал, где учиться: без города бот не
-- знает, какие вузы предложить, поэтому сначала этот вопрос.
UPDATE bot_dialogs SET step = 'target',
    draft = (draft - 'goal_status' - 'direction_id' - 'interests') || jsonb_build_object('direction_ids',
        CASE WHEN draft ? 'direction_id' THEN jsonb_build_array(draft->'direction_id') ELSE '[]'::jsonb END)
WHERE step IN ('universities', 'university_search');

ALTER TABLE bot_dialogs DROP CONSTRAINT bot_dialogs_step_check;
ALTER TABLE bot_dialogs ADD CONSTRAINT bot_dialogs_step_check CHECK (step IN (
    'role', 'name_confirm', 'name_input', 'grade', 'region',
    'subjects', 'direction', 'target', 'universities',
    'university_search', 'join_confirm', 'join_direction',
    'summary', 'done'));

-- +goose Down

UPDATE bot_dialogs SET step = 'goal' WHERE step = 'target';
ALTER TABLE bot_dialogs DROP CONSTRAINT bot_dialogs_step_check;
ALTER TABLE bot_dialogs ADD CONSTRAINT bot_dialogs_step_check CHECK (step IN (
    'role', 'name_confirm', 'name_input', 'grade', 'region',
    'subjects', 'goal', 'interests', 'direction', 'universities',
    'university_search', 'join_confirm', 'join_direction',
    'summary', 'done'));

ALTER TABLE universities DROP COLUMN region_code;
ALTER TABLE trajectories DROP COLUMN target_region_code;

UPDATE trajectories t SET direction_id = td.direction_id
FROM trajectory_directions td WHERE td.trajectory_id = t.id AND td.position = 0;
UPDATE trajectories SET goal_status = 'known' WHERE goal_status = 'exploring';
ALTER TABLE trajectories DROP CONSTRAINT trajectories_goal_status_check;
ALTER TABLE trajectories ADD CONSTRAINT trajectories_goal_status_check
    CHECK (goal_status IN ('known', 'suggested'));
COMMENT ON COLUMN trajectories.direction_id IS NULL;

DROP TABLE trajectory_directions;
