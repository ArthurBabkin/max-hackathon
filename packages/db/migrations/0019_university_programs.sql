-- Направления подготовки вузов (образовательные программы датасета A) и
-- какие олимпиады дают льготу на каждое (датасет B). Раньше льгота была
-- связкой «профиль олимпиады — вуз», а вуз знал только топ названий
-- направлений в universities.directions. Теперь в каталоге можно выбрать
-- направление в вузе, увидеть олимпиады именно для него и сохранить к себе.
--
-- Схема здесь, данные — в 0020 (генерирует datasets/parser/build_seed.py).
-- benefits остаётся: по нему считаются подбор, карточка олимпиады и
-- уведомления, а program_benefits — уточнение на уровне программы.

-- +goose Up

CREATE TABLE university_programs (
    id             text PRIMARY KEY,  -- program_id датасета A: msu__prikladnaya-matematika-i-informatika
    university_id  text NOT NULL REFERENCES universities(id) ON DELETE CASCADE,
    name           text NOT NULL,
    faculty        text,
    code           text,              -- код направления по ОКСО: 01.03.02
    -- Направление-цель из справочника (F8), если код программы в него входит:
    -- по нему фильтр вузов в каталоге. У остальных программ — NULL.
    direction_id   text REFERENCES directions(id) ON DELETE SET NULL,
    -- Бюджетные места; NULL — число не подтверждено источником.
    budget_places  integer CHECK (budget_places >= 0),
    admission_year smallint NOT NULL CHECK (admission_year BETWEEN 2020 AND 2100)
);

CREATE INDEX university_programs_university_idx ON university_programs (university_id);
CREATE INDEX university_programs_direction_idx ON university_programs (direction_id)
    WHERE direction_id IS NOT NULL;

-- Льгота по профилю олимпиады на программу: лучшая из победителя и призёра,
-- как в benefits. Доп. баллов в датасете на уровне программ нет.
CREATE TABLE program_benefits (
    program_id          text NOT NULL REFERENCES university_programs(id) ON DELETE CASCADE,
    olympiad_profile_id text NOT NULL REFERENCES olympiad_profiles(id) ON DELETE CASCADE,
    benefit             text NOT NULL CHECK (benefit IN ('bvi', 'bvi_winners', 'score100')),
    PRIMARY KEY (program_id, olympiad_profile_id)
);

-- Карточка олимпиады и обратные обходы — по профилю.
CREATE INDEX program_benefits_profile_idx ON program_benefits (olympiad_profile_id);

-- Направления, которые ученик сохранил в каталоге. Вуз программы всегда
-- в trajectory_universities: это держит сервер, убранный вуз уносит и
-- свои программы.
CREATE TABLE trajectory_programs (
    trajectory_id uuid NOT NULL REFERENCES trajectories(id) ON DELETE CASCADE,
    program_id    text NOT NULL REFERENCES university_programs(id) ON DELETE CASCADE,
    PRIMARY KEY (trajectory_id, program_id)
);

-- +goose Down

DROP TABLE trajectory_programs;
DROP TABLE program_benefits;
DROP TABLE university_programs;
