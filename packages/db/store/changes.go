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
	// ChangeIDs — строки content_changes, из которых сложился этот пункт:
	// несколько правок одного профиля подряд дают одну строку в сообщении.
	// По ним идёт захват доставки, поэтому пункт обязан их помнить.
	ChangeIDs []string
}

// PendingContentChanges — неразосланные изменения контента и кого они
// касаются (ТЗ §6.5): сроки — траектории с этой олимпиадой в трекере;
// льготы — с олимпиадой в трекере или с вузом в профиле, если предмет
// олимпиады среди предметов ученика.
//
// Ничего не помечает. Раньше здесь стоял UPDATE notified_at до отправки, и
// это был единственный след доставки — один флаг на изменение, которое
// касается многих людей. Такой флаг не способен описать «Ольге ушло, Артёму
// нет», поэтому сорвавшийся запуск уносил с собой и тех, кому воркер даже не
// писал. Теперь состояние доставки живёт в content_change_deliveries по паре
// (изменение, получатель), а notified_at ставится после — MarkChangesNotified.
func (s *Store) PendingContentChanges(ctx context.Context, limit int) (ids []string, items []ChangeItem, err error) {
	err = s.Tx(ctx, func(tx *Store) error {
		// Старые вперёд: при переполнении пачки отставшие не голодают.
		rows, err := tx.db.Query(ctx, `
			SELECT id::text FROM content_changes WHERE notified_at IS NULL
			ORDER BY created_at LIMIT $1`, limit)
		if err != nil {
			return wrap(err)
		}
		if ids, err = collect(rows, func(r rowScanner) (string, error) {
			var id string
			return id, r.Scan(&id)
		}); err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		// pending группирует, а не отбрасывает дубли: массив id нужен целиком,
		// иначе захватывать доставку было бы нечем. Он функционально зависит
		// от (entity, entity_id), поэтому дедупликация UNION ниже не меняется.
		rows, err = tx.db.Query(ctx, `
			WITH pending AS (
			  SELECT entity, entity_id, array_agg(id::text) AS change_ids
			  FROM content_changes WHERE id = ANY($1::uuid[])
			  GROUP BY entity, entity_id
			), affected AS (
			  SELECT ti.trajectory_id, p.entity_id AS profile_id, NULL::text AS university_id, 'stages' AS kind,
			         p.change_ids
			  FROM pending p JOIN tracker_items ti ON ti.olympiad_profile_id = p.entity_id
			  WHERE p.entity = 'olympiad_profile'
			  UNION
			  SELECT t.id, split_part(p.entity_id, '@', 1), split_part(p.entity_id, '@', 2), 'benefits',
			         p.change_ids
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
			       u.id, u.short_name, u.rules_url, a.change_ids
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
				&c.OfficialURL, &c.Kind, &c.UniversityID, &c.UniversityShort, &c.RulesURL, &c.ChangeIDs)
		})
		return err
	})
	return ids, items, err
}

// ClaimChangeDeliveries занимает пары (изменение, получатель).
//
// claimed — достались нам, их и отправляем. unfinished — занято другим
// запуском, который ещё не закончил: слать нельзя, но и обслуженными считать
// нельзя, иначе изменение пометят разосланным, пока сообщение ещё не ушло, и
// при срыве того запуска оно не достанется никому. Остальные уже доставлены —
// вызывающему про них знать нечего, они просто обслужены.
//
// Вставка и чтение одним оператором: его снимок не видит собственную вставку
// из CTE, поэтому только что занятая пара честно отвечает «не завершена». Тот
// же приём, что в ClaimDelivery для напоминаний.
func (s *Store) ClaimChangeDeliveries(ctx context.Context, changeIDs []string, memberID string) (claimed, unfinished []string, err error) {
	if len(changeIDs) == 0 {
		return nil, nil, nil
	}
	rows, err := s.db.Query(ctx, `
		WITH want AS (
			SELECT DISTINCT unnest($1::uuid[]) AS change_id
		), ins AS (
			INSERT INTO content_change_deliveries (change_id, member_id)
			SELECT w.change_id, $2 FROM want w
			ON CONFLICT DO NOTHING
			RETURNING change_id
		)
		SELECT w.change_id::text,
		       w.change_id IN (SELECT change_id FROM ins),
		       COALESCE((SELECT d.done_at IS NOT NULL FROM content_change_deliveries d
		                 WHERE d.change_id = w.change_id AND d.member_id = $2), false)
		FROM want w`, changeIDs, memberID)
	if err != nil {
		return nil, nil, wrap(err)
	}
	type row struct {
		id           string
		mine, isDone bool
	}
	got, err := collect(rows, func(r rowScanner) (row, error) {
		var x row
		return x, r.Scan(&x.id, &x.mine, &x.isDone)
	})
	if err != nil {
		return nil, nil, err
	}
	for _, x := range got {
		switch {
		case x.mine:
			claimed = append(claimed, x.id)
		case !x.isDone:
			unfinished = append(unfinished, x.id)
		}
	}
	return claimed, unfinished, nil
}

// FinishChangeDeliveries закрывает захват: сообщение ушло. Пустой messageID —
// отправлять было некому (получатель остановил бота); повторять нечего, но и
// «в работе» пара висеть не должна.
func (s *Store) FinishChangeDeliveries(ctx context.Context, changeIDs []string, memberID, messageID string) error {
	if len(changeIDs) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, `
		UPDATE content_change_deliveries SET done_at = now(), max_message_id = NULLIF($3, '')
		WHERE member_id = $2 AND change_id = ANY($1::uuid[])`, changeIDs, memberID, messageID)
	return wrap(err)
}

// ReleaseChangeDeliveries освобождает пары после неудачной отправки:
// следующий запуск попробует снова.
func (s *Store) ReleaseChangeDeliveries(ctx context.Context, changeIDs []string, memberID string) error {
	if len(changeIDs) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, `
		DELETE FROM content_change_deliveries
		WHERE member_id = $2 AND change_id = ANY($1::uuid[])`, changeIDs, memberID)
	return wrap(err)
}

// ReapStaleChangeDeliveries освобождает захваты, по которым результат так и
// не записали: база отказала между успешной отправкой и отметкой о ней. Без
// этого пара навсегда остаётся «в работе», изменение по ней не закрывается
// никогда, а GiveUpOldChanges через неделю запишет его в потери — хотя
// сообщение ушло. Ровно тот же сборщик, что у напоминаний.
//
// Порог обязан заведомо превышать время жизни запуска, иначе сборщик отнял
// бы пару у работающего воркера и получатель увидел бы сводку дважды.
func (s *Store) ReapStaleChangeDeliveries(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM content_change_deliveries WHERE done_at IS NULL AND claimed_at < $1`, before)
	if err != nil {
		return 0, wrap(err)
	}
	return tag.RowsAffected(), nil
}

// MarkChangesNotified закрывает изменения, обслуженные полностью: всем, кого
// они касались, сообщение ушло (или слать было некому). Вызывается после
// отправки — в этом и весь смысл переделки.
func (s *Store) MarkChangesNotified(ctx context.Context, ids []string, now time.Time) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE content_changes SET notified_at = $2
		WHERE id = ANY($1::uuid[]) AND notified_at IS NULL`, ids, now)
	if err != nil {
		return 0, wrap(err)
	}
	return tag.RowsAffected(), nil
}

// GiveUpOldChanges перестаёт пытаться доставить слишком старые изменения:
// иначе недоставляемое событие перепроверялось бы каждым запуском вечно, а
// таблица доставок росла бы без верхней границы.
func (s *Store) GiveUpOldChanges(ctx context.Context, now, before time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE content_changes SET notified_at = $1
		WHERE notified_at IS NULL AND created_at < $2`, now, before)
	if err != nil {
		return 0, wrap(err)
	}
	return tag.RowsAffected(), nil
}

// PurgeContentChanges стирает давно закрытые изменения вместе с их
// доставками (ON DELETE CASCADE). Раньше content_changes не чистилась вовсе.
func (s *Store) PurgeContentChanges(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM content_changes WHERE notified_at IS NOT NULL AND created_at < $1`, before)
	if err != nil {
		return 0, wrap(err)
	}
	return tag.RowsAffected(), nil
}
