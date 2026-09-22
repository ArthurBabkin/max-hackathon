-- Ответ помощника несёт ссылки на первоисточники (F35, F36): контракт
-- отдаёт их в AiMessage.sources. Храним снимок источников на момент ответа —
-- история не должна меняться задним числом, если источник обновят.

-- +goose Up

ALTER TABLE ai_messages ADD COLUMN sources jsonb NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down

ALTER TABLE ai_messages DROP COLUMN IF EXISTS sources;
