-- Вузы и направления: льгота олимпиады уточняется до направления подготовки
-- в вузе, а ученик выбирает, на какие направления вуза он смотрит.
--
-- Раньше льгота была связкой «профиль олимпиады + вуз + год» (benefits), и
-- вуз, где олимпиада даёт БВИ на одну программу, выглядел так, будто БВИ
-- дают везде. Контент новых таблиц — сгенерированная миграция 0021.
--
-- Триггеров на новых таблицах нет намеренно: триггер 0006 на benefits
-- рассылает семьям «изменились льготы», а загрузка и пересборка льгот по
-- направлениям изменением правил вуза не является.

-- +goose Up

ALTER TABLE directions
    -- Код направления из id: napr-09-03-04 → 09.03.04. По нему укрупнённая
    -- группа 09.00.00 (Иннополис) покрывает направления своей группы.
    ADD COLUMN code text GENERATED ALWAYS AS (replace(substr(id, 6), '-', '.')) STORED,
    -- Профильные группы программ направления: ИТ, Физика, Биомед, Экономика.
    ADD COLUMN groups text[] NOT NULL DEFAULT '{}',
    -- Показывать в клавиатуре бота. Цели онбординга — короткий список из
    -- шестнадцати направлений, остальные есть только у вузов.
    ADD COLUMN onboarding boolean NOT NULL DEFAULT false;
UPDATE directions SET onboarding = true;  -- существующие 16

-- Направление подготовки в вузе: пара из датасетов A и B.
CREATE TABLE university_directions (
    university_id text NOT NULL REFERENCES universities(id) ON DELETE CASCADE,
    direction_id  text NOT NULL REFERENCES directions(id) ON DELETE CASCADE,
    -- offered — льготы проверены хотя бы у одной программы направления,
    -- to_check — программы есть, но их правила приёма ещё не разобраны.
    status        text NOT NULL CHECK (status IN ('offered', 'to_check')),
    programs      smallint NOT NULL DEFAULT 0,   -- образовательных программ на направлении
    budget_places integer,                       -- бюджетных мест 2026; NULL — вуз не опубликовал
    program_names text[] NOT NULL DEFAULT '{}',
    PRIMARY KEY (university_id, direction_id)
);

-- «В каких вузах есть направление» — подбор вузов по цели.
CREATE INDEX university_directions_direction_idx ON university_directions (direction_id);

-- Льгота олимпиады на направлении вуза. Гранулярность та же, что у benefits,
-- плюс направление: одна строка на профиль, вуз, направление и год, различия
-- победителя и призёра — в benefit и note.
CREATE TABLE direction_benefits (
    olympiad_profile_id text NOT NULL REFERENCES olympiad_profiles(id) ON DELETE CASCADE,
    university_id  text NOT NULL,
    direction_id   text NOT NULL,
    admission_year smallint NOT NULL CHECK (admission_year BETWEEN 2020 AND 2100),
    -- Баллов вне перечня здесь нет: у олимпиад вне перечня направлений нет.
    benefit        text NOT NULL CHECK (benefit IN ('bvi', 'bvi_winners', 'score100')),
    ege_min        smallint CHECK (ege_min BETWEEN 0 AND 100),
    diploma_grades int[],            -- за какие классы засчитывается диплом
    note           text,
    varies         boolean NOT NULL DEFAULT false,  -- льгота разная у программ направления
    source_id      text REFERENCES sources(id) ON DELETE SET NULL,
    PRIMARY KEY (olympiad_profile_id, university_id, direction_id, admission_year),
    FOREIGN KEY (university_id, direction_id) REFERENCES university_directions ON DELETE CASCADE
);

-- Карточка направления вуза: все олимпиады с льготой на нём.
CREATE INDEX direction_benefits_pair_idx ON direction_benefits (university_id, direction_id);

-- Направления, которые ученик выбрал в вузе траектории. Выбор живёт, пока
-- выбран вуз: убрали вуз — каскадом ушёл и выбор.
CREATE TABLE trajectory_university_directions (
    trajectory_id uuid NOT NULL,
    university_id text NOT NULL,
    direction_id  text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (trajectory_id, university_id, direction_id),
    FOREIGN KEY (trajectory_id, university_id) REFERENCES trajectory_universities ON DELETE CASCADE,
    FOREIGN KEY (university_id, direction_id) REFERENCES university_directions ON DELETE CASCADE
);

-- +goose Down

DROP TABLE trajectory_university_directions;
DROP TABLE direction_benefits;
DROP TABLE university_directions;
ALTER TABLE directions
    DROP COLUMN onboarding,
    DROP COLUMN groups,
    DROP COLUMN code;
