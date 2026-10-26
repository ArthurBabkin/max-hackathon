-- Что именно изменилось (#141, отзыв экспертов: «показать обновление
-- информации при изменении правил»). Раньше событие говорило только «сроки»
-- или «льготы», и бот писал «Обновились данные по олимпиадам», не объясняя
-- что. Теперь триггеры 0006 кладут в detail, каким этап или льгота были и
-- какими стали; бот и карточка олимпиады пересказывают это словами.
--
-- detail этапа: {"title", "kind", "old": {starts_at, ends_at, deadline_at},
-- "new": {…}} — old нет у нового этапа, new — у удалённого. detail льготы:
-- {"old": {benefit, ege_min, diploma_grades}, "new": {…}}. Объявление вуза,
-- которое правит не столбцы, а смысл льготы (правила приёма следующего года),
-- миграция данных пишет сама: {"text": "…"}.
--
-- created_at — clock_timestamp(), а не now(): две правки одного этапа в одной
-- миграции иначе получают одно время, и «что стало последним» не узнать.

-- +goose Up

ALTER TABLE content_changes ADD COLUMN detail jsonb;
COMMENT ON COLUMN content_changes.detail IS 'Было и стало: этап — title, kind, old/new со сроками; льгота — old/new; объявление — text';

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION content_change_stage() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'UPDATE' AND (OLD.starts_at, OLD.ends_at, OLD.deadline_at, OLD.olympiad_profile_id)
       IS NOT DISTINCT FROM (NEW.starts_at, NEW.ends_at, NEW.deadline_at, NEW.olympiad_profile_id) THEN
    RETURN NULL;
  END IF;
  INSERT INTO content_changes (entity, entity_id, summary, detail, created_at)
  VALUES ('olympiad_profile',
          CASE WHEN TG_OP = 'DELETE' THEN OLD.olympiad_profile_id ELSE NEW.olympiad_profile_id END,
          'stages',
          jsonb_strip_nulls(jsonb_build_object(
            'title', CASE WHEN TG_OP = 'DELETE' THEN OLD.title ELSE NEW.title END,
            'kind', CASE WHEN TG_OP = 'DELETE' THEN OLD.kind ELSE NEW.kind END,
            'old', CASE WHEN TG_OP <> 'INSERT' THEN jsonb_build_object(
                     'starts_at', OLD.starts_at, 'ends_at', OLD.ends_at, 'deadline_at', OLD.deadline_at) END,
            'new', CASE WHEN TG_OP <> 'DELETE' THEN jsonb_build_object(
                     'starts_at', NEW.starts_at, 'ends_at', NEW.ends_at, 'deadline_at', NEW.deadline_at) END)),
          clock_timestamp());
  RETURN NULL;
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION content_change_benefit() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  r benefits;
BEGIN
  IF TG_OP = 'UPDATE' AND (OLD.benefit, OLD.ege_min, OLD.extra_points, OLD.diploma_grades, OLD.admission_year)
       IS NOT DISTINCT FROM (NEW.benefit, NEW.ege_min, NEW.extra_points, NEW.diploma_grades, NEW.admission_year) THEN
    RETURN NULL;
  END IF;
  IF TG_OP = 'DELETE' THEN r := OLD; ELSE r := NEW; END IF;
  INSERT INTO content_changes (entity, entity_id, summary, detail, created_at)
  VALUES ('benefit', r.olympiad_profile_id || '@' || r.university_id, 'benefits',
          jsonb_strip_nulls(jsonb_build_object(
            'old', CASE WHEN TG_OP <> 'INSERT' THEN jsonb_build_object(
                     'benefit', OLD.benefit, 'ege_min', OLD.ege_min, 'diploma_grades', OLD.diploma_grades) END,
            'new', CASE WHEN TG_OP <> 'DELETE' THEN jsonb_build_object(
                     'benefit', NEW.benefit, 'ege_min', NEW.ege_min, 'diploma_grades', NEW.diploma_grades) END)),
          clock_timestamp());
  RETURN NULL;
END
$$;
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION content_change_stage() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'UPDATE' AND (OLD.starts_at, OLD.ends_at, OLD.deadline_at, OLD.olympiad_profile_id)
       IS NOT DISTINCT FROM (NEW.starts_at, NEW.ends_at, NEW.deadline_at, NEW.olympiad_profile_id) THEN
    RETURN NULL;
  END IF;
  INSERT INTO content_changes (entity, entity_id, summary)
  VALUES ('olympiad_profile',
          CASE WHEN TG_OP = 'DELETE' THEN OLD.olympiad_profile_id ELSE NEW.olympiad_profile_id END,
          'stages');
  RETURN NULL;
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION content_change_benefit() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  r benefits;
BEGIN
  IF TG_OP = 'UPDATE' AND (OLD.benefit, OLD.ege_min, OLD.extra_points, OLD.diploma_grades, OLD.admission_year)
       IS NOT DISTINCT FROM (NEW.benefit, NEW.ege_min, NEW.extra_points, NEW.diploma_grades, NEW.admission_year) THEN
    RETURN NULL;
  END IF;
  IF TG_OP = 'DELETE' THEN r := OLD; ELSE r := NEW; END IF;
  INSERT INTO content_changes (entity, entity_id, summary)
  VALUES ('benefit', r.olympiad_profile_id || '@' || r.university_id, 'benefits');
  RETURN NULL;
END
$$;
-- +goose StatementEnd

ALTER TABLE content_changes DROP COLUMN IF EXISTS detail;
