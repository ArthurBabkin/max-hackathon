-- Статусы этапов в трекере: регистрация на каждый этап, где она есть, и
-- итог этапа — прошёл дальше, не прошёл, диплом финала. Первую регистрацию
-- по-прежнему хранит tracker_items.registered_at: её ставят галочка в
-- приложении и кнопка в напоминании. Правила отметок — core/stages.
--
-- Бот спрашивает итог этапа на следующий день после его окончания и ещё раз
-- через неделю: это напоминания с отрицательным смещением -1 и -8.

-- +goose Up

CREATE TABLE tracker_stage_results (
    tracker_item_id  uuid NOT NULL REFERENCES tracker_items(id) ON DELETE CASCADE,
    stage_id         text NOT NULL REFERENCES stages(id) ON DELETE CASCADE,
    -- Регистрация на этап — для всех регистраций, кроме первой.
    registered       boolean NOT NULL DEFAULT false,
    -- failed, winner, prizer и participant (финал без диплома) заканчивают участие.
    result           text CHECK (result IN ('passed', 'failed', 'winner', 'prizer', 'participant')),
    set_by_member_id uuid REFERENCES members(id) ON DELETE SET NULL,
    set_at           timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tracker_item_id, stage_id),
    -- Пустая отметка не хранится: её снятие — удаление строки.
    CHECK (registered OR result IS NOT NULL)
);

ALTER TABLE reminders DROP CONSTRAINT reminders_offset_days_check;
ALTER TABLE reminders ADD CONSTRAINT reminders_offset_days_check
    CHECK (offset_days IN (30, 7, 3, 1, 0, -1, -8));

-- Отдельный индекс, а не расширение reminders_planned_uniq: прежний
-- ON CONFLICT … WHERE offset_days > 0 работает, пока выкладывается код.
CREATE UNIQUE INDEX reminders_result_ask_uniq ON reminders (tracker_item_id, stage_id, offset_days)
    WHERE offset_days < 0;

-- +goose Down

DROP INDEX IF EXISTS reminders_result_ask_uniq;
DELETE FROM reminders WHERE offset_days < 0;
ALTER TABLE reminders DROP CONSTRAINT reminders_offset_days_check;
ALTER TABLE reminders ADD CONSTRAINT reminders_offset_days_check
    CHECK (offset_days IN (30, 7, 3, 1, 0));
DROP TABLE IF EXISTS tracker_stage_results;
