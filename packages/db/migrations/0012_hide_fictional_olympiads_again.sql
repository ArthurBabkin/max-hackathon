-- Повтор 0009 для прода: вымышленные олимпиады вернула демо-миграция
-- migrations-demo/0002 — она катится и в прод, пока включён демо-режим в
-- браузере. Теперь они живут в migrations-local (только локальный стенд и
-- CI), а 0002 стала пустой.
--
-- Та же защита, что в 0009: олимпиаду из трекера или предложения не
-- трогаем, события изменения от каскада стираем.

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
-- Удалённое не возвращается: контент был вымышленным.
