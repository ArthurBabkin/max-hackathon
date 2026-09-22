-- Первая миграция схемы «Траектории».
-- Источник истины — раздел 9 «Модель данных» в docs/TZ.md.
--
-- Соглашения по идентификаторам:
--   * Справочники контента (subjects, directions, universities, sources, olympiads,
--     olympiad_profiles, stages, benefits) имеют текстовый PK — слаг из датасетов
--     (`msu`, `p669-2-matematika`, `matematika`). Так сиды из datasets/*.json
--     идемпотентны: повторный прогон делает ON CONFLICT DO UPDATE по тому же id,
--     а не плодит дубли со случайными uuid.
--   * Пользовательские данные — uuid с gen_random_uuid() (функция ядра начиная
--     с PostgreSQL 13, расширение pgcrypto ставить не нужно).
--   * Перечисления сделаны через CHECK, а не ENUM-типы: добавить значение
--     обычным ALTER ... DROP/ADD CONSTRAINT проще, чем ALTER TYPE, который
--     до PG 12 не откатывался в транзакции и до сих пор плохо ложится на down-миграции.

-- +goose Up

-- ---------------------------------------------------------------------------
-- 1. Справочники контента
-- ---------------------------------------------------------------------------

CREATE TABLE subjects (
    code text PRIMARY KEY,               -- слаг из datasets/data/subject_slugs.json: matematika, fizika, ...
    name text NOT NULL UNIQUE
);

CREATE TABLE directions (
    id            text PRIMARY KEY,      -- направление-цель ученика (F8)
    name          text NOT NULL,
    -- Ключевые предметы направления. В ТЗ поле задано как text[], и массив здесь уместен:
    -- он читается одним запросом при скоринге (раздел 6.1, фактор «совпадение профиля
    -- с направлением») и не требует join. Внешний ключ на subjects(code) для элементов
    -- массива PostgreSQL не поддерживает — целостность обеспечивают сиды.
    subject_codes text[] NOT NULL DEFAULT '{}'
);

CREATE TABLE universities (
    id                text PRIMARY KEY,  -- vuz_id из датасетов: msu, hse, itmo, ...
    short_name        text NOT NULL,
    name              text NOT NULL,
    city              text,
    -- Профильные группы вуза («ИТ», «Физика», «Биомед», «Экономика»).
    -- ТЗ допускает «directions (или таблица программ)»; в v1 берём массив,
    -- полноценная таблица программ появится отдельной миграцией, когда
    -- понадобятся льготы на уровне конкретной программы.
    directions        text[] NOT NULL DEFAULT '{}',
    ege_note          text,              -- как вуз подтверждает льготу ЕГЭ (порог может быть выше 75)
    rules_url         text,
    -- Дата проверки источника, а не момент времени, поэтому date, а не timestamptz.
    rules_verified_at date
);

CREATE TABLE sources (
    id          text PRIMARY KEY,
    kind        text NOT NULL CHECK (kind IN ('order', 'rules', 'site')),
    title       text NOT NULL,
    url         text NOT NULL,
    -- Раздел 6.2: без источника и даты проверки запись не показывается как «Факт».
    -- Поле оставлено nullable намеренно — фильтрация на стороне приложения,
    -- чтобы недопроверенный источник можно было положить в базу и дозаполнить.
    verified_at date
);

CREATE TABLE olympiads (
    id               text PRIMARY KEY,   -- olympiad_id из датасета B/C
    name             text NOT NULL,
    organizer        text,
    kind             text NOT NULL CHECK (kind IN ('perechen', 'vsosh', 'other')),
    official_url     text,
    -- Формат проведения в ТЗ не перечислен закрытым списком, поэтому без CHECK.
    format           text,
    final_city       text,
    final_region_code text                -- фактор скоринга «финал в регионе ученика» (раздел 6.1)
);

CREATE TABLE olympiad_profiles (
    id                text PRIMARY KEY,
    olympiad_id       text NOT NULL REFERENCES olympiads(id) ON DELETE CASCADE,
    subject_code      text NOT NULL REFERENCES subjects(code) ON DELETE RESTRICT,
    -- Уровень есть только у олимпиад перечня; у ВсОШ уровня нет, поэтому NULL допустим.
    level             text CHECK (level IN ('I', 'II', 'III')),
    school_year       text NOT NULL CHECK (school_year ~ '^\d{4}/\d{2}$'),  -- '2026/27'
    grades_from       smallint NOT NULL CHECK (grades_from BETWEEN 1 AND 11),
    grades_to         smallint NOT NULL CHECK (grades_to BETWEEN 1 AND 11),
    source_id         text REFERENCES sources(id) ON DELETE SET NULL,
    CHECK (grades_from <= grades_to),
    -- Один профиль на предмет в рамках олимпиады и учебного года.
    UNIQUE (olympiad_id, subject_code, school_year)
);

-- Подбор (раздел 6.1) стартует с «профили текущего учебного года по предметам ученика».
CREATE INDEX olympiad_profiles_subject_year_idx ON olympiad_profiles (subject_code, school_year);
CREATE INDEX olympiad_profiles_olympiad_idx ON olympiad_profiles (olympiad_id);

CREATE TABLE stages (
    id                  text PRIMARY KEY,
    olympiad_profile_id text NOT NULL REFERENCES olympiad_profiles(id) ON DELETE CASCADE,
    -- registration/qualifying/final — перечень; school/municipal/regional/final — ВсОШ (F14).
    kind                text NOT NULL CHECK (kind IN ('registration', 'qualifying', 'final',
                                                      'school', 'municipal', 'regional')),
    starts_at           timestamptz,
    ends_at             timestamptz,
    deadline_at         timestamptz,     -- от него планируются напоминания (раздел 6.3)
    is_online           boolean NOT NULL DEFAULT false,
    CHECK (starts_at IS NULL OR ends_at IS NULL OR ends_at >= starts_at)
);

-- Карточка олимпиады тянет все этапы профиля одним запросом (F14, F21).
CREATE INDEX stages_profile_idx ON stages (olympiad_profile_id);
-- Частичный индекс: ближайшие сроки, календарь месяца (F32) и планировщик
-- напоминаний ходят только по этапам с дедлайном, строки без него бесполезны.
CREATE INDEX stages_deadline_idx ON stages (deadline_at) WHERE deadline_at IS NOT NULL;

CREATE TABLE benefits (
    id                  text PRIMARY KEY,
    olympiad_profile_id text NOT NULL REFERENCES olympiad_profiles(id) ON DELETE CASCADE,
    university_id       text NOT NULL REFERENCES universities(id) ON DELETE CASCADE,
    admission_year      smallint NOT NULL CHECK (admission_year BETWEEN 2020 AND 2100),
    benefit             text NOT NULL CHECK (benefit IN ('bvi', 'score100', 'bvi_winners', 'extra_points')),
    -- Вне перечня — только баллы за индивидуальные достижения, не больше 10 в сумме (раздел 1.5).
    extra_points        smallint CHECK (extra_points BETWEEN 0 AND 10),
    ege_min             smallint CHECK (ege_min BETWEEN 0 AND 100),
    diploma_grades      int[],           -- за какие классы засчитывается диплом
    note                text,
    source_id           text REFERENCES sources(id) ON DELETE SET NULL,
    -- Если льгота — «доп. баллы», их количество обязано быть указано, иначе показывать нечего.
    CHECK (benefit <> 'extra_points' OR extra_points IS NOT NULL),
    -- Льгота — связка профиль + вуз + год приёма (раздел 6.2), но у одного вуза
    -- в одном году могут быть разные льготы победителю и призёру,
    -- поэтому в ключ уникальности входит и сам benefit.
    UNIQUE (olympiad_profile_id, university_id, admission_year, benefit)
);

-- Карточка вуза: «какие олимпиады этот вуз учитывает» (F25, F26).
CREATE INDEX benefits_university_idx ON benefits (university_id);
-- Карточка олимпиады: льготы в вузах ученика (F17–F19) и фактор скоринга «льгота в вузах ученика».
CREATE INDEX benefits_profile_lookup_idx ON benefits (olympiad_profile_id, university_id, admission_year);

-- ---------------------------------------------------------------------------
-- 2. Пользователи, траектории, семья
-- ---------------------------------------------------------------------------

CREATE TABLE users (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Идентификатор пользователя в MAX. bigint, потому что платформа отдаёт числовой id.
    max_user_id bigint NOT NULL UNIQUE,
    first_name  text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE trajectories (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    student_name   text NOT NULL CHECK (char_length(student_name) BETWEEN 1 AND 40),  -- F4
    grade          smallint NOT NULL CHECK (grade IN (8, 9, 10, 11)),                 -- F5
    region_code    text NOT NULL,        -- код субъекта РФ; координаты геолокации не храним (раздел 9)
    tz             text NOT NULL DEFAULT 'Europe/Moscow',  -- IANA-зона региона, по ней считается 10:00 (раздел 6.3)
    direction_id   text REFERENCES directions(id) ON DELETE SET NULL,
    -- known — ученик выбрал направление сам, suggested — бот предложил по 3 вопросам (F8).
    goal_status    text NOT NULL CHECK (goal_status IN ('known', 'suggested')),
    -- payload deep-link'а, с которого начался онбординг (start/startapp, до 512 символов, раздел 11.2).
    source_payload text CHECK (source_payload IS NULL OR char_length(source_payload) <= 512),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    -- Мягкое удаление (F50): доступ закрывается сразу, физическое удаление
    -- персональных данных — отдельным джобом не позже 24 часов.
    deleted_at     timestamptz
);

-- Все выборки идут только по живым траекториям.
CREATE INDEX trajectories_alive_idx ON trajectories (id) WHERE deleted_at IS NULL;

CREATE TABLE members (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Траектория удаляется вместе с участием: members не имеет смысла без траектории.
    trajectory_id    uuid NOT NULL REFERENCES trajectories(id) ON DELETE CASCADE,
    user_id          uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role             text NOT NULL CHECK (role IN ('kid', 'parent')),
    is_creator       boolean NOT NULL DEFAULT false,
    joined_at        timestamptz NOT NULL DEFAULT now(),
    left_at          timestamptz,        -- вышел сам (F48)
    removed_at       timestamptz,        -- удалён создателем (F41)
    -- Пороги напоминаний в днях, настраиваются каждым участником в /settings (F51).
    reminder_offsets int[] NOT NULL DEFAULT '{30,7,3,1}'
        CHECK (reminder_offsets <@ ARRAY[30, 7, 3, 1]),  -- пустой массив = напоминания выключены
    CHECK (left_at IS NULL OR removed_at IS NULL)        -- взаимоисключающие способы уйти
);

-- ТЗ: «активный участник один раз на траекторию». Индекс частичный, потому что
-- обычный UNIQUE (trajectory_id, user_id) запретил бы вернуться в траекторию
-- по новой ссылке после выхода или удаления — а старые строки мы храним
-- ради истории (кто добавил пункт в трекер, кому ушло напоминание).
CREATE UNIQUE INDEX members_active_uniq
    ON members (trajectory_id, user_id)
    WHERE left_at IS NULL AND removed_at IS NULL;

-- Раздел 3.2: «У траектории ровно один создатель». Создатель не может выйти,
-- может только удалить траекторию, поэтому условие на left_at/removed_at здесь не нужно.
CREATE UNIQUE INDEX members_single_creator_uniq
    ON members (trajectory_id)
    WHERE is_creator;

-- F42: «Если ученик уже есть, доступна только роль Родитель» — активный ученик
-- в траектории не больше одного. Родителей может быть сколько угодно.
CREATE UNIQUE INDEX members_single_active_kid_uniq
    ON members (trajectory_id)
    WHERE role = 'kid' AND left_at IS NULL AND removed_at IS NULL;

-- Вход по initData: max_user_id -> users.id -> активное участие.
CREATE INDEX members_active_user_idx
    ON members (user_id)
    WHERE left_at IS NULL AND removed_at IS NULL;

CREATE TABLE trajectory_subjects (
    trajectory_id uuid NOT NULL REFERENCES trajectories(id) ON DELETE CASCADE,
    -- RESTRICT: предмет из справочника нельзя удалить, пока он выбран в чьей-то траектории,
    -- иначе молча испортится подбор.
    subject_code  text NOT NULL REFERENCES subjects(code) ON DELETE RESTRICT,
    PRIMARY KEY (trajectory_id, subject_code)
);

CREATE TABLE trajectory_universities (
    trajectory_id uuid NOT NULL REFERENCES trajectories(id) ON DELETE CASCADE,
    university_id text NOT NULL REFERENCES universities(id) ON DELETE CASCADE,
    PRIMARY KEY (trajectory_id, university_id)
);

-- Обратный обход для рассылки об изменениях: «кого касается правка правил вуза» (раздел 6.5).
CREATE INDEX trajectory_universities_university_idx ON trajectory_universities (university_id);

CREATE TABLE invites (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trajectory_id        uuid NOT NULL REFERENCES trajectories(id) ON DELETE CASCADE,
    -- F39: 12+ символов из [A-Za-z0-9_-], ссылка одноразовая.
    token                text NOT NULL UNIQUE
        CHECK (char_length(token) >= 12 AND token ~ '^[A-Za-z0-9_-]+$'),
    role                 text NOT NULL CHECK (role IN ('kid', 'parent')),
    -- SET NULL, а не CASCADE: удалённый участник не должен уносить с собой
    -- ссылку, по которой в траекторию уже кто-то зашёл.
    created_by_member_id uuid REFERENCES members(id) ON DELETE SET NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    used_at              timestamptz,
    used_by_user_id      uuid REFERENCES users(id) ON DELETE SET NULL,
    revoked_at           timestamptz     -- создатель отозвал неиспользованную ссылку (раздел 16)
);

-- GET /family показывает только активные ссылки (F43).
CREATE INDEX invites_active_idx
    ON invites (trajectory_id)
    WHERE used_at IS NULL AND revoked_at IS NULL;

-- ---------------------------------------------------------------------------
-- 3. Трекер, предложения, напоминания
-- ---------------------------------------------------------------------------

CREATE TABLE tracker_items (
    id                       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trajectory_id            uuid NOT NULL REFERENCES trajectories(id) ON DELETE CASCADE,
    olympiad_profile_id      text NOT NULL REFERENCES olympiad_profiles(id) ON DELETE CASCADE,
    -- SET NULL: участника могли удалить из траектории, но пункт трекера и подпись
    -- «добавил(а) <имя>» (раздел 3.2) должны остаться у остальных.
    added_by_member_id       uuid REFERENCES members(id) ON DELETE SET NULL,
    created_at               timestamptz NOT NULL DEFAULT now(),
    registered_at            timestamptz,   -- отметка «зарегистрирован» (F46)
    registered_by_member_id  uuid REFERENCES members(id) ON DELETE SET NULL,
    -- F28: повторное добавление не создаёт дубль.
    UNIQUE (trajectory_id, olympiad_profile_id)
);

-- F31/F34: «нужно зарегистрироваться» — счётчик на вкладке и «следующий шаг» на главной.
CREATE INDEX tracker_items_pending_idx
    ON tracker_items (trajectory_id)
    WHERE registered_at IS NULL;
-- Раздел 6.5: кому рассылать уведомление об изменении олимпиады.
CREATE INDEX tracker_items_profile_idx ON tracker_items (olympiad_profile_id);

CREATE TABLE proposals (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trajectory_id         uuid NOT NULL REFERENCES trajectories(id) ON DELETE CASCADE,
    olympiad_profile_id   text NOT NULL REFERENCES olympiad_profiles(id) ON DELETE CASCADE,
    proposed_by_member_id uuid REFERENCES members(id) ON DELETE SET NULL,
    status                text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'declined')),
    created_at            timestamptz NOT NULL DEFAULT now(),
    resolved_at           timestamptz,
    -- Предложение либо ждёт ответа, либо разрешено — и тогда у него есть время ответа (F45).
    CHECK ((status = 'pending') = (resolved_at IS NULL))
);

-- Родитель не может завалить ученика одним и тем же предложением дважды,
-- но после отказа предложить ту же олимпиаду снова можно — поэтому индекс частичный.
CREATE UNIQUE INDEX proposals_pending_uniq
    ON proposals (trajectory_id, olympiad_profile_id)
    WHERE status = 'pending';

CREATE INDEX proposals_trajectory_status_idx ON proposals (trajectory_id, status);

CREATE TABLE reminders (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tracker_item_id uuid NOT NULL REFERENCES tracker_items(id) ON DELETE CASCADE,
    -- Этап пропал из контента — напоминать не о чем, поэтому CASCADE.
    stage_id        text NOT NULL REFERENCES stages(id) ON DELETE CASCADE,
    -- 30/7/3/1 — плановые пороги, 0 — разовое «напомнить завтра» (F30).
    offset_days     smallint NOT NULL CHECK (offset_days IN (30, 7, 3, 1, 0)),
    fire_at         timestamptz NOT NULL,   -- 10:00 в часовом поясе региона ученика
    status          text NOT NULL DEFAULT 'planned'
        CHECK (status IN ('planned', 'sent', 'cancelled'))
);

-- Плановое напоминание на пару (дедлайн, порог) существует в единственном экземпляре
-- (раздел 6.3). Условие offset_days > 0 обязательно: «напомнить завтра» ученик может
-- нажать в разные дни по одному и тому же этапу, и это законные разные напоминания.
CREATE UNIQUE INDEX reminders_planned_uniq
    ON reminders (tracker_item_id, stage_id, offset_days)
    WHERE offset_days > 0;

-- Основной запрос воркера по Timer-триггеру: «что пора отправить».
-- Частичный индекс по status='planned' держит в дереве только неотправленные строки.
CREATE INDEX reminders_due_idx
    ON reminders (fire_at)
    WHERE status = 'planned';

-- Отмена напоминаний о регистрации после отметки «зарегистрирован» (раздел 6.3).
CREATE INDEX reminders_tracker_item_idx ON reminders (tracker_item_id);

CREATE TABLE reminder_deliveries (
    reminder_id    uuid NOT NULL REFERENCES reminders(id) ON DELETE CASCADE,
    member_id      uuid NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    max_message_id text,                   -- id сообщения в MAX, текстовый
    sent_at        timestamptz NOT NULL DEFAULT now(),
    -- Дедупликация напоминаний (раздел 8.2): serverless-воркер может быть запущен
    -- повторно или параллельно, поэтому единственная защита от двойной отправки —
    -- уникальность на уровне БД. Составной первичный ключ и есть требуемый
    -- UNIQUE (reminder_id, member_id): вставка идёт как
    -- INSERT ... ON CONFLICT DO NOTHING, и отправка выполняется только если
    -- строка реально вставилась.
    PRIMARY KEY (reminder_id, member_id)
);

CREATE INDEX reminder_deliveries_member_idx ON reminder_deliveries (member_id);

-- ---------------------------------------------------------------------------
-- 4. Изменения контента, помощник, аудит
-- ---------------------------------------------------------------------------

CREATE TABLE content_changes (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entity      text NOT NULL CHECK (entity IN ('olympiad', 'olympiad_profile', 'stage',
                                                'benefit', 'university', 'source')),
    -- text, а не uuid: все справочники контента имеют текстовые ключи.
    entity_id   text NOT NULL,
    summary     text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    notified_at timestamptz
);

-- Ежедневный content-notifier забирает только неразосланные изменения (раздел 6.5).
CREATE INDEX content_changes_pending_idx
    ON content_changes (created_at)
    WHERE notified_at IS NULL;

CREATE INDEX content_changes_entity_idx ON content_changes (entity, entity_id);

CREATE TABLE ai_messages (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Привязка к member, а не к user: у каждого участника свой личный чат,
    -- невидимый остальным (F37). Уход из траектории уносит переписку — CASCADE.
    member_id  uuid NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    role       text NOT NULL CHECK (role IN ('user', 'assistant')),
    text       text NOT NULL,
    -- Ограничение длины вопроса из раздела 6.4 (защита от prompt injection);
    -- на ответ модели оно не распространяется.
    CHECK (role <> 'user' OR char_length(text) <= 500),
    -- Ссылки на карточки из контекста RAG, например [{"type":"olympiad","id":"p669-2-matematika"}].
    card_refs  jsonb NOT NULL DEFAULT '[]'::jsonb,
    refused    boolean NOT NULL DEFAULT false,  -- сработал шаблон «данных нет» (F36)
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ai_messages_member_idx ON ai_messages (member_id, created_at DESC);

CREATE TABLE audit_log (
    -- Append-only журнал: монотонный bigint дешевле uuid и по вставке, и по индексу.
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- SET NULL, а не CASCADE: запись аудита должна пережить удаление участника
    -- и удаление траектории, иначе журнал теряет смысл именно в тех случаях,
    -- ради которых он ведётся.
    member_id  uuid REFERENCES members(id) ON DELETE SET NULL,
    action     text NOT NULL,
    entity     text NOT NULL,
    entity_id  text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_member_idx ON audit_log (member_id, created_at DESC);
CREATE INDEX audit_log_entity_idx ON audit_log (entity, entity_id, created_at DESC);

-- +goose Down

-- Порядок обратный созданию: сначала таблицы, которые ссылаются на другие.
-- Индексы и ограничения удаляются вместе со своими таблицами.

DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS ai_messages;
DROP TABLE IF EXISTS content_changes;
DROP TABLE IF EXISTS reminder_deliveries;
DROP TABLE IF EXISTS reminders;
DROP TABLE IF EXISTS proposals;
DROP TABLE IF EXISTS tracker_items;
DROP TABLE IF EXISTS invites;
DROP TABLE IF EXISTS trajectory_universities;
DROP TABLE IF EXISTS trajectory_subjects;
DROP TABLE IF EXISTS members;
DROP TABLE IF EXISTS trajectories;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS benefits;
DROP TABLE IF EXISTS stages;
DROP TABLE IF EXISTS olympiad_profiles;
DROP TABLE IF EXISTS olympiads;
DROP TABLE IF EXISTS sources;
DROP TABLE IF EXISTS universities;
DROP TABLE IF EXISTS directions;
DROP TABLE IF EXISTS subjects;
