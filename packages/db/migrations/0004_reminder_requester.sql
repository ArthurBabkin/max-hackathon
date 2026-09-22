-- «Напомнить завтра» (F30) — разовое напоминание тому, кто нажал кнопку,
-- а не всей семье. Плановые пороги получателя не хранят: их получают все
-- активные участники с включённым порогом, это решается при отправке.

-- +goose Up

ALTER TABLE reminders
    ADD COLUMN requested_by_member_id uuid REFERENCES members(id) ON DELETE CASCADE,
    -- Адресат есть ровно у разовых напоминаний.
    ADD CONSTRAINT reminders_requester_only_once CHECK ((offset_days = 0) = (requested_by_member_id IS NOT NULL));

-- +goose Down

ALTER TABLE reminders
    DROP CONSTRAINT IF EXISTS reminders_requester_only_once,
    DROP COLUMN IF EXISTS requested_by_member_id;
