-- Несколько чатов с помощником (F58, F59). Чат принадлежит участнику, как
-- раньше вся история: у каждого свои, другим не видны (F37). Пустых чатов не
-- бывает — чат создаётся вместе с первым вопросом.

-- +goose Up

CREATE TABLE ai_chats (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id       uuid NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    -- По умолчанию — день первого вопроса, «Чат 21 сентября»; участник может переименовать.
    title           text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 60),
    created_at      timestamptz NOT NULL DEFAULT now(),
    -- Время последней реплики: по нему список идёт от свежих чатов к старым.
    last_message_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ai_chats_member_idx ON ai_chats (member_id, last_message_at DESC);

ALTER TABLE ai_messages ADD COLUMN chat_id uuid REFERENCES ai_chats(id) ON DELETE CASCADE;

-- Прежняя история участника — его первый чат. Название по тому же правилу,
-- что у нового чата: день первой реплики в часовом поясе траектории.
INSERT INTO ai_chats (member_id, title, created_at, last_message_at)
SELECT h.member_id,
       'Чат ' || extract(day FROM h.first_at AT TIME ZONE t.tz)::int || ' ' ||
       (ARRAY['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа',
              'сентября', 'октября', 'ноября', 'декабря'])[extract(month FROM h.first_at AT TIME ZONE t.tz)::int],
       h.first_at, h.last_at
FROM (SELECT member_id, min(created_at) AS first_at, max(created_at) AS last_at
      FROM ai_messages GROUP BY member_id) h
JOIN members m ON m.id = h.member_id
JOIN trajectories t ON t.id = m.trajectory_id;

UPDATE ai_messages a SET chat_id = c.id FROM ai_chats c WHERE c.member_id = a.member_id;

ALTER TABLE ai_messages ALTER COLUMN chat_id SET NOT NULL;
CREATE INDEX ai_messages_chat_idx ON ai_messages (chat_id, created_at DESC);

-- +goose Down

ALTER TABLE ai_messages DROP COLUMN IF EXISTS chat_id;
DROP TABLE IF EXISTS ai_chats;
