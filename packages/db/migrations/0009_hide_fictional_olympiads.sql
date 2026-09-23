-- Три вымышленные олимпиады вне перечня (0003) нужны только чтобы показать
-- блок F16 локально — в проде пользователь принял бы их за настоящие.
-- Локально их возвращает migrations-demo/0002.
--
-- Олимпиаду, которую уже взяли в трекер или предложили, не трогаем: каскад
-- снёс бы данные пользователя. Каскадное удаление этапов и льгот рождает
-- события изменения (0006) — стираем их в той же транзакции, иначе
-- content-notifier разослал бы «изменения» об олимпиаде, которой нет.

-- +goose Up

DELETE FROM olympiads o
WHERE o.id IN ('other-tyk', 'other-impuls', 'other-biznes-start')
  AND NOT EXISTS (
    SELECT 1 FROM olympiad_profiles p
    WHERE p.olympiad_id = o.id
      AND (EXISTS (SELECT 1 FROM tracker_items t WHERE t.olympiad_profile_id = p.id)
        OR EXISTS (SELECT 1 FROM proposals r WHERE r.olympiad_profile_id = p.id)));

DELETE FROM content_changes c
WHERE c.notified_at IS NULL
  AND c.entity_id LIKE 'other-%'
  AND NOT EXISTS (SELECT 1 FROM olympiad_profiles p WHERE p.id = split_part(c.entity_id, '@', 1));

-- +goose Down
-- Удалённое не возвращается: 0003 goose второй раз не накатывает, а контент
-- был вымышленным.
