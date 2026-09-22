-- Правки схемы, без которых не ложатся реальные данные и не работает бот.
-- Только аддитивные изменения: 0001 заморожена.
--
-- 1. olympiad_profiles. В перечне у одной олимпиады бывает несколько профилей
--    с одним школьным предметом: у НТО 15 профилей с предметом «Информатика»
--    («искусственный интеллект», «информационная безопасность», …). Исходный
--    UNIQUE (olympiad_id, subject_code, school_year) такие данные не пускает,
--    поэтому уникальность переезжает на слаг профиля, а предмет остаётся
--    атрибутом для подбора.
-- 2. stages. Даты этапов частью опубликованы, частью взяты из прошлого сезона
--    или сгенерированы для демонстрации. Признак is_demo и источник нужны,
--    чтобы API честно отдавал stages_are_demo, а не выдавал демо за факт.
--    title — название этапа из датасета («Отборочный этап (онлайн)»).
-- 3. bot_dialogs — состояние онбординга в чате. Функции без состояния, а
--    траектория создаётся только в конце диалога (у неё NOT NULL на имени,
--    классе и регионе), поэтому ответы копятся в черновике.
-- 4. bot_updates_seen — MAX повторяет доставку обновления, если не получил 200
--    за 30 секунд. Ключ дедупликации защищает неидемпотентные действия:
--    расход приглашения, «напомнить завтра», отправку сообщений.

-- +goose Up

ALTER TABLE olympiad_profiles
    ADD COLUMN profile_slug text,
    ADD COLUMN profile_name text;
UPDATE olympiad_profiles SET profile_slug = subject_code WHERE profile_slug IS NULL;
ALTER TABLE olympiad_profiles
    ALTER COLUMN profile_slug SET NOT NULL,
    DROP CONSTRAINT olympiad_profiles_olympiad_id_subject_code_school_year_key,
    ADD CONSTRAINT olympiad_profiles_slug_uniq UNIQUE (olympiad_id, profile_slug, school_year);

ALTER TABLE stages
    ADD COLUMN title     text,
    ADD COLUMN is_demo   boolean NOT NULL DEFAULT false,
    ADD COLUMN source_id text REFERENCES sources(id) ON DELETE SET NULL;

CREATE TABLE bot_dialogs (
    user_id        uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    step           text NOT NULL CHECK (step IN (
                       'role', 'name_confirm', 'name_input', 'grade', 'region',
                       'subjects', 'goal', 'interests', 'direction', 'universities',
                       'university_search', 'join_confirm', 'join_direction',
                       'summary', 'done')),
    role           text CHECK (role IN ('kid', 'parent')),
    -- Ответы онбординга до создания траектории: имя, класс, регион, предметы, вузы.
    draft          jsonb NOT NULL DEFAULT '{}'::jsonb,
    -- Приглашение, по которому пришёл пользователь: вход в чужую траекторию (F39).
    invite_id      uuid REFERENCES invites(id) ON DELETE SET NULL,
    -- Payload из bot_started — метка источника перехода, не больше лимита MAX.
    source_payload text CHECK (char_length(source_payload) <= 512),
    chat_id        bigint,
    -- Сообщение с текущим вопросом: его клавиатуру бот правит на месте.
    prompt_mid     text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE bot_updates_seen (
    dedup_key text PRIMARY KEY,
    seen_at   timestamptz NOT NULL DEFAULT now()
);
-- Старые ключи чистит воркер напоминаний: повтор MAX приходит в пределах минут.
CREATE INDEX bot_updates_seen_at_idx ON bot_updates_seen (seen_at);

-- +goose Down

DROP TABLE IF EXISTS bot_updates_seen;
DROP TABLE IF EXISTS bot_dialogs;

ALTER TABLE stages
    DROP COLUMN IF EXISTS source_id,
    DROP COLUMN IF EXISTS is_demo,
    DROP COLUMN IF EXISTS title;

-- Откат возможен только пока данные укладываются в старый UNIQUE: если сид
-- уже положил два профиля с одним предметом, ADD CONSTRAINT упадёт — это
-- правильно, молча терять профили нельзя.
ALTER TABLE olympiad_profiles
    DROP CONSTRAINT olympiad_profiles_slug_uniq,
    ADD CONSTRAINT olympiad_profiles_olympiad_id_subject_code_school_year_key
        UNIQUE (olympiad_id, subject_code, school_year),
    DROP COLUMN profile_name,
    DROP COLUMN profile_slug;
