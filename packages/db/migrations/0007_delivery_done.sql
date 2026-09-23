-- Строка в reminder_deliveries появляется ДО отправки: она и есть захват
-- пары (напоминание, получатель), защищающий от дубля при параллельном или
-- повторном запуске воркера. Поэтому её существование означает «занято», а
-- не «доставлено», и этих двух состояний воркеру не хватало: занятую чужим
-- запуском пару он считал доставленной и помечал напоминание разосланным,
-- после чего оно больше не попадало в выборку planned — и не приходило
-- никому. done_at разделяет захват и результат.

-- +goose Up

ALTER TABLE reminder_deliveries
    -- Проставляется после отправки, а также когда отправлять некому:
    -- получатель остановил бота, повторять бессмысленно.
    ADD COLUMN done_at timestamptz;

-- Старые строки: сообщение ушло — значит, доставка завершена. Время
-- отправки точнее now(), а сам захват создавался в тот же момент.
UPDATE reminder_deliveries SET done_at = sent_at WHERE max_message_id IS NOT NULL;

-- Незавершённые доставки ищет каждый запуск воркера.
CREATE INDEX reminder_deliveries_pending_idx
    ON reminder_deliveries (reminder_id)
    WHERE done_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS reminder_deliveries_pending_idx;

ALTER TABLE reminder_deliveries
    DROP COLUMN IF EXISTS done_at;
