-- До этой миграции ежедневный воркер помечал события разосланными ДО отправки:
-- единственным состоянием был один флаг notified_at на изменение. Но изменение
-- касается многих людей, и флаг на всю пачку не способен описать «Ольге ушло,
-- Артёму нет». Отсюда выбор без хороших вариантов: пометить заранее — и потерять
-- всё, что не успели отправить; пометить потом — и завтра разослать дубли тем,
-- кто уже прочитал. Состояние на получателя убирает саму развилку.
--
-- Таблица повторяет reminder_deliveries: строка — это захват пары, а done_at —
-- уже результат. Различать их обязательно, иначе незавершённый захват соседнего
-- запуска выглядит как доставка (см. 0007).

-- +goose Up

CREATE TABLE content_change_deliveries (
    change_id      uuid NOT NULL REFERENCES content_changes(id) ON DELETE CASCADE,
    member_id      uuid NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    max_message_id text,
    -- Проставляется после отправки, а также когда отправлять некому:
    -- получатель остановил бота, повторять бессмысленно.
    done_at        timestamptz,
    claimed_at     timestamptz NOT NULL DEFAULT now(),
    -- Защита от двойной отправки — уникальность на уровне БД, а не состояние
    -- в памяти: воркер serverless, запусков может быть несколько сразу.
    PRIMARY KEY (change_id, member_id)
);

-- Незавершённые доставки ищет каждый запуск.
CREATE INDEX content_change_deliveries_pending_idx
    ON content_change_deliveries (change_id)
    WHERE done_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS content_change_deliveries;
