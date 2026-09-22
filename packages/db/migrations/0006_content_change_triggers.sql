-- Уведомления об изменениях контента (F33, ТЗ §6.5): событие изменения
-- рождается в самой базе, когда меняются сроки этапов или льготы. Так его
-- не забудет ни сид, ни ручная правка контент-администратором.
--
-- Пишем только настоящие изменения: сид идемпотентен и при повторном
-- прогоне делает ON CONFLICT DO UPDATE теми же значениями — такие UPDATE
-- событий не создают. Первичная загрузка контента была раньше этой
-- миграции, поэтому уведомления о ней никому не придут.
--
-- entity_id: для этапа — профиль олимпиады, для льготы — «профиль@вуз»;
-- по ним content-notifier находит затронутые траектории.

-- +goose Up

-- +goose StatementBegin
CREATE FUNCTION content_change_stage() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE FUNCTION content_change_benefit() RETURNS trigger LANGUAGE plpgsql AS $$
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

CREATE TRIGGER stages_content_change AFTER INSERT OR UPDATE OR DELETE ON stages
  FOR EACH ROW EXECUTE FUNCTION content_change_stage();

CREATE TRIGGER benefits_content_change AFTER INSERT OR UPDATE OR DELETE ON benefits
  FOR EACH ROW EXECUTE FUNCTION content_change_benefit();

-- +goose Down

DROP TRIGGER IF EXISTS benefits_content_change ON benefits;
DROP TRIGGER IF EXISTS stages_content_change ON stages;
DROP FUNCTION IF EXISTS content_change_benefit();
DROP FUNCTION IF EXISTS content_change_stage();
