package store

import (
	"context"
	"maps"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
)

// StageMarks — отметки этапов по пунктам трекера: регистрации, кроме
// первой (она в tracker_items.registered_at), и итоги.
func (s *Store) StageMarks(ctx context.Context, itemIDs []string) (map[string]map[string]stages.Mark, error) {
	out := map[string]map[string]stages.Mark{}
	if len(itemIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT tracker_item_id::text, stage_id, registered, COALESCE(result, '')
		FROM tracker_stage_results WHERE tracker_item_id = ANY($1::uuid[])`, itemIDs)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var item, stage string
		var m stages.Mark
		if err := rows.Scan(&item, &stage, &m.Registered, &m.Result); err != nil {
			return nil, wrap(err)
		}
		if out[item] == nil {
			out[item] = map[string]stages.Mark{}
		}
		out[item][stage] = m
	}
	return out, wrap(rows.Err())
}

// SetStageMark ставит или снимает отметку этапа (регистрацию или итог)
// одной транзакцией: правила — stages.Apply, их ошибки возвращаются как
// есть. Итог подразумевает регистрацию и ставит registered_at, если его не
// было. Напоминания отмеченных этапов (а после закрывающего итога — всей
// олимпиады) отменяются сразу; вернуть снятые — дело SyncReminders.
// changed сообщает, изменилось ли что-то, — уведомлять ли остальных.
func (s *Store) SetStageMark(ctx context.Context, trajectoryID, itemID, stageID, memberID string, m stages.Mark, now time.Time) (changed bool, err error) {
	err = s.Tx(ctx, func(tx *Store) error {
		var profileID string
		var registered bool
		if err := tx.db.QueryRow(ctx, `
			SELECT olympiad_profile_id, registered_at IS NOT NULL FROM tracker_items
			WHERE trajectory_id = $1 AND id = $2 FOR UPDATE`, trajectoryID, itemID).Scan(&profileID, &registered); err != nil {
			return wrap(err)
		}
		byProfile, err := tx.StagesFor(ctx, []string{profileID})
		if err != nil {
			return err
		}
		marks, err := tx.StageMarks(ctx, []string{itemID})
		if err != nil {
			return err
		}
		st := byProfile[profileID]
		was := stages.Progress{Registered: registered, Marks: marks[itemID]}
		next, err := stages.Apply(st, was, stageID, m, now)
		if err != nil {
			return err
		}
		if next.Registered == was.Registered && maps.Equal(next.Marks, was.Marks) {
			return nil
		}
		changed = true
		if next.Registered != was.Registered {
			if _, err := tx.db.Exec(ctx, `
				UPDATE tracker_items SET registered_at = CASE WHEN $2 THEN now() END,
				       registered_by_member_id = CASE WHEN $2 THEN $3::uuid END
				WHERE id = $1`, itemID, next.Registered, memberID); err != nil {
				return wrap(err)
			}
		}
		if x, ok := next.Marks[stageID]; ok {
			var result *string
			if x.Result != "" {
				result = &x.Result
			}
			_, err = tx.db.Exec(ctx, `
				INSERT INTO tracker_stage_results (tracker_item_id, stage_id, registered, result, set_by_member_id)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (tracker_item_id, stage_id) DO UPDATE
				SET registered = EXCLUDED.registered, result = EXCLUDED.result,
				    set_by_member_id = EXCLUDED.set_by_member_id, set_at = now()`,
				itemID, stageID, x.Registered, result, memberID)
		} else {
			_, err = tx.db.Exec(ctx, `DELETE FROM tracker_stage_results WHERE tracker_item_id = $1 AND stage_id = $2`,
				itemID, stageID)
		}
		if err != nil {
			return wrap(err)
		}
		var settled, noAsk []string
		for i, x := range st {
			if stages.Settled(st, next, i) {
				settled = append(settled, x.ID)
			}
			if !stages.NeedsResult(st, next, i) {
				noAsk = append(noAsk, x.ID)
			}
		}
		if _, err := tx.db.Exec(ctx, `
			UPDATE reminders SET status = 'cancelled'
			WHERE tracker_item_id = $1 AND status = 'planned'
			  AND ((offset_days >= 0 AND stage_id = ANY($2)) OR (offset_days < 0 AND stage_id = ANY($3)))`,
			itemID, settled, noAsk); err != nil {
			return wrap(err)
		}
		return tx.Audit(ctx, memberID, "stage_mark", "tracker_item", itemID)
	})
	return changed, err
}
