-- Согласие с политикой конфиденциальности (152-ФЗ): бот спрашивает его до
-- анкеты и до входа по приглашению (шаг диалога consent), «Принимаю»
-- записывает время. NULL — не давал или отозвал: /delete создателем, выход
-- из траектории и /delete с брошенной анкетой снимают отметку и стирают
-- черновик анкеты, вернувшегося бот спросит снова. Кто зарегистрировался
-- раньше, согласия не давал — отметки у него нет.

-- +goose Up

ALTER TABLE users ADD COLUMN privacy_accepted_at timestamptz;

ALTER TABLE bot_dialogs DROP CONSTRAINT bot_dialogs_step_check;
ALTER TABLE bot_dialogs ADD CONSTRAINT bot_dialogs_step_check CHECK (step IN (
    'consent', 'role', 'name_confirm', 'name_input', 'grade', 'region',
    'subjects', 'direction', 'interest', 'work', 'suggest', 'experience',
    'target', 'universities', 'university_search', 'join_confirm', 'join_direction',
    'summary', 'done'));

-- +goose Down

-- Ждавшие согласия начнут с роли: /start спросит заново.
UPDATE bot_dialogs SET step = 'role' WHERE step = 'consent';
ALTER TABLE bot_dialogs DROP CONSTRAINT bot_dialogs_step_check;
ALTER TABLE bot_dialogs ADD CONSTRAINT bot_dialogs_step_check CHECK (step IN (
    'role', 'name_confirm', 'name_input', 'grade', 'region',
    'subjects', 'direction', 'interest', 'work', 'suggest', 'experience',
    'target', 'universities', 'university_search', 'join_confirm', 'join_direction',
    'summary', 'done'));

ALTER TABLE users DROP COLUMN IF EXISTS privacy_accepted_at;
