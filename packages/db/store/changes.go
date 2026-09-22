package store

import (
	"context"
	"time"
)

// ChangeItem — что изменилось для одной траектории: профиль олимпиады и,
// для льгот, вуз.
type ChangeItem struct {
	TrajectoryID string
	ProfileID    string
	ProfileName  *string
	OlympiadID   string
	OlympiadName string
	OfficialURL  *string
	Kind         string // stages | benefits
	// Для benefits — вуз и его правила приёма как первоисточник.
	UniversityID    *string
	UniversityShort *string
	RulesURL        *string
}

// TakeContentChanges забирает неразосланные изменения контента и
// возвращает, кого они касаются (ТЗ §6.5): сроки — траектории с этой
// олимпиадой в трекере; льготы — с олимпиадой в трекере или с вузом в
// профиле, если предмет олимпиады среди предметов ученика. Изменения
// помечаются разосланными в той же транзакции, до отправки: лучше не
// дослать, чем прислать одно и то же дважды при повторе таймера.
func (s *Store) TakeContentChanges(ctx context.Context, now time.Time) (changes int, items []ChangeItem, err error) {
	err = s.Tx(ctx, func(tx *Store) error {
		var ids []string
		rows, err := tx.db.Query(ctx, `
			UPDATE content_changes SET notified_at = $1 WHERE notified_at IS NULL RETURNING id::text`, now)
		if err != nil {
			return wrap(err)
		}
		if ids, err = collect(rows, func(r rowScanner) (string, error) {
			var id string
			return id, r.Scan(&id)
		}); err != nil {
			return err
		}
		changes = len(ids)
		if changes == 0 {
			return nil
		}
		rows, err = tx.db.Query(ctx, `
			WITH pending AS (
			  SELECT DISTINCT entity, entity_id FROM content_changes WHERE id = ANY($1::uuid[])
			), affected AS (
			  SELECT ti.trajectory_id, p.entity_id AS profile_id, NULL::text AS university_id, 'stages' AS kind
			  FROM pending p JOIN tracker_items ti ON ti.olympiad_profile_id = p.entity_id
			  WHERE p.entity = 'olympiad_profile'
			  UNION
			  SELECT t.id, split_part(p.entity_id, '@', 1), split_part(p.entity_id, '@', 2), 'benefits'
			  FROM pending p
			  JOIN olympiad_profiles op ON op.id = split_part(p.entity_id, '@', 1)
			  JOIN trajectories t ON t.deleted_at IS NULL
			  WHERE p.entity = 'benefit' AND (
			    EXISTS (SELECT 1 FROM tracker_items ti
			            WHERE ti.trajectory_id = t.id AND ti.olympiad_profile_id = op.id)
			    OR (EXISTS (SELECT 1 FROM trajectory_universities tu
			                WHERE tu.trajectory_id = t.id AND tu.university_id = split_part(p.entity_id, '@', 2))
			        AND EXISTS (SELECT 1 FROM trajectory_subjects ts
			                    WHERE ts.trajectory_id = t.id AND ts.subject_code = op.subject_code)))
			)
			SELECT a.trajectory_id::text, a.profile_id, op.profile_name, o.id, o.name, o.official_url, a.kind,
			       u.id, u.short_name, u.rules_url
			FROM affected a
			JOIN trajectories t ON t.id = a.trajectory_id AND t.deleted_at IS NULL
			JOIN olympiad_profiles op ON op.id = a.profile_id
			JOIN olympiads o ON o.id = op.olympiad_id
			LEFT JOIN universities u ON u.id = a.university_id
			ORDER BY a.trajectory_id, o.name, a.profile_id, a.kind, u.short_name`, ids)
		if err != nil {
			return wrap(err)
		}
		items, err = collect(rows, func(r rowScanner) (ChangeItem, error) {
			var c ChangeItem
			return c, r.Scan(&c.TrajectoryID, &c.ProfileID, &c.ProfileName, &c.OlympiadID, &c.OlympiadName,
				&c.OfficialURL, &c.Kind, &c.UniversityID, &c.UniversityShort, &c.RulesURL)
		})
		return err
	})
	return changes, items, err
}
