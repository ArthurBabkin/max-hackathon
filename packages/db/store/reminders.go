package store

import (
	"context"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/schedule"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
)

// SyncReminders приводит плановые напоминания траектории в соответствие с
// трекером, этапами, отметками и часовым поясом (ТЗ §6.3). Вызывается
// после каждого изменения, а воркер повторяет его для всех траекторий как
// страховку. Отправленные напоминания не трогает; наступившие, но ещё не
// отправленные оставляет воркеру — иначе «догоняющий» порог отодвинул бы
// их на завтра.
func (s *Store) SyncReminders(ctx context.Context, trajectoryID string, hour int, now time.Time) error {
	var tz string
	if err := s.db.QueryRow(ctx, `SELECT tz FROM trajectories WHERE id = $1 AND deleted_at IS NULL`,
		trajectoryID).Scan(&tz); err != nil {
		return wrap(err)
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	rows, err := s.db.Query(ctx, `
		SELECT id::text, olympiad_profile_id, registered_at IS NOT NULL FROM tracker_items WHERE trajectory_id = $1`,
		trajectoryID)
	if err != nil {
		return wrap(err)
	}
	type item struct {
		id, profileID string
		registered    bool
	}
	items, err := collect(rows, func(r rowScanner) (item, error) {
		var it item
		return it, r.Scan(&it.id, &it.profileID, &it.registered)
	})
	if err != nil || len(items) == 0 {
		return err
	}
	profileIDs := make([]string, len(items))
	itemIDs := make([]string, len(items))
	for i, it := range items {
		profileIDs[i], itemIDs[i] = it.profileID, it.id
	}
	byProfile, err := s.StagesFor(ctx, profileIDs)
	if err != nil {
		return err
	}

	var keep, add struct {
		items, stages []string
		offsets       []int16
		fireAt        []time.Time
	}
	for _, it := range items {
		for _, st := range byProfile[it.profileID] {
			if st.DeadlineAt == nil || !st.DeadlineAt.After(now) || (it.registered && stages.RegistrationLike(st.Kind)) {
				continue
			}
			for _, t := range schedule.Thresholds(*st.DeadlineAt, loc, hour) {
				keep.items, keep.stages = append(keep.items, it.id), append(keep.stages, st.ID)
				keep.offsets = append(keep.offsets, int16(t.Offset))
			}
			for _, t := range schedule.Upcoming(*st.DeadlineAt, loc, hour, now) {
				add.items, add.stages = append(add.items, it.id), append(add.stages, st.ID)
				add.offsets, add.fireAt = append(add.offsets, int16(t.Offset)), append(add.fireAt, t.FireAt)
			}
		}
	}
	return s.Tx(ctx, func(tx *Store) error {
		if _, err := tx.db.Exec(ctx, `
			INSERT INTO reminders (tracker_item_id, stage_id, offset_days, fire_at)
			SELECT * FROM unnest($1::uuid[], $2::text[], $3::smallint[], $4::timestamptz[])
			ON CONFLICT (tracker_item_id, stage_id, offset_days) WHERE offset_days > 0
			DO UPDATE SET fire_at = EXCLUDED.fire_at, status = 'planned'
			WHERE reminders.status = 'cancelled'
			   OR (reminders.status = 'planned' AND reminders.fire_at > $5 AND reminders.fire_at <> EXCLUDED.fire_at)`,
			add.items, add.stages, add.offsets, add.fireAt, now); err != nil {
			return wrap(err)
		}
		_, err := tx.db.Exec(ctx, `
			UPDATE reminders SET status = 'cancelled'
			WHERE tracker_item_id = ANY($1::uuid[]) AND status = 'planned' AND offset_days > 0
			  AND (tracker_item_id, stage_id, offset_days) NOT IN (
			      SELECT * FROM unnest($2::uuid[], $3::text[], $4::smallint[]))`,
			itemIDs, keep.items, keep.stages, keep.offsets)
		return wrap(err)
	})
}

// SyncAllReminders — страховка воркера: пересчитать план у всех живых
// траекторий с трекером.
func (s *Store) SyncAllReminders(ctx context.Context, hour int, now time.Time) error {
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT t.id::text FROM trajectories t JOIN tracker_items ti ON ti.trajectory_id = t.id
		WHERE t.deleted_at IS NULL`)
	if err != nil {
		return wrap(err)
	}
	ids, err := collect(rows, func(r rowScanner) (string, error) {
		var id string
		return id, r.Scan(&id)
	})
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.SyncReminders(ctx, id, hour, now); err != nil {
			return err
		}
	}
	return nil
}

// RemindTomorrow — разовое напоминание тому, кто нажал «Напомнить завтра»
// (F30). Повторное нажатие, пока первое не отправлено, ничего не добавляет.
func (s *Store) RemindTomorrow(ctx context.Context, trackerItemID, stageID, memberID string, fireAt time.Time) (created bool, err error) {
	tag, err := s.db.Exec(ctx, `
		INSERT INTO reminders (tracker_item_id, stage_id, offset_days, fire_at, requested_by_member_id)
		SELECT $1, $2, 0, $4, $3
		WHERE NOT EXISTS (SELECT 1 FROM reminders WHERE tracker_item_id = $1 AND stage_id = $2 AND offset_days = 0
		                  AND requested_by_member_id = $3 AND status = 'planned')`,
		trackerItemID, stageID, memberID, fireAt)
	if err != nil {
		return false, wrap(err)
	}
	return tag.RowsAffected() == 1, nil
}

// ExpireReminders отменяет устаревшие запланированные напоминания: срок уже
// прошёл — напоминать поздно; наступил более близкий порог того же этапа —
// прошлый не нужен (воркер простаивал, и слать пачку «за неделю», «за три
// дня» и «за день» разом бессмысленно).
func (s *Store) ExpireReminders(ctx context.Context, now time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE reminders r SET status = 'cancelled'
		FROM stages st
		WHERE st.id = r.stage_id AND r.status = 'planned'
		  AND (st.deadline_at <= $1
		       OR (r.offset_days > 0 AND EXISTS (
		           SELECT 1 FROM reminders n
		           WHERE n.tracker_item_id = r.tracker_item_id AND n.stage_id = r.stage_id
		             AND n.offset_days > 0 AND n.offset_days < r.offset_days
		             AND n.fire_at <= $1 AND n.status IN ('planned', 'sent'))))`, now)
	if err != nil {
		return 0, wrap(err)
	}
	return tag.RowsAffected(), nil
}

// DueReminder — напоминание, которое пора отправить, со всем для текста.
type DueReminder struct {
	ID            string
	TrackerItemID string
	TrajectoryID  string
	StageID       string
	StageKind     string
	StageTitle    string
	Deadline      time.Time
	Offset        int
	ProfileID     string
	OlympiadName  string
	OfficialURL   *string
	StudentName   string
	TZ            string
	Registered    bool
	RequestedBy   *string
}

const dueSelect = `
	SELECT r.id::text, ti.id::text, t.id::text, st.id, st.kind, COALESCE(st.title, ''), st.deadline_at,
	       r.offset_days, p.id, o.name, o.official_url, t.student_name, t.tz, ti.registered_at IS NOT NULL,
	       r.requested_by_member_id::text
	FROM reminders r
	JOIN tracker_items ti ON ti.id = r.tracker_item_id
	JOIN trajectories t ON t.id = ti.trajectory_id AND t.deleted_at IS NULL
	JOIN stages st ON st.id = r.stage_id
	JOIN olympiad_profiles p ON p.id = ti.olympiad_profile_id
	JOIN olympiads o ON o.id = p.olympiad_id`

func scanDue(r rowScanner) (DueReminder, error) {
	var d DueReminder
	err := r.Scan(&d.ID, &d.TrackerItemID, &d.TrajectoryID, &d.StageID, &d.StageKind, &d.StageTitle,
		&d.Deadline, &d.Offset, &d.ProfileID, &d.OlympiadName, &d.OfficialURL, &d.StudentName, &d.TZ,
		&d.Registered, &d.RequestedBy)
	return d, wrap(err)
}

func (s *Store) DueReminders(ctx context.Context, now time.Time, limit int) ([]DueReminder, error) {
	rows, err := s.db.Query(ctx, dueSelect+`
		WHERE r.status = 'planned' AND r.fire_at <= $1 AND st.deadline_at > $1
		ORDER BY r.fire_at
		LIMIT $2`, now, limit)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, scanDue)
}

// Recipient — кому отправить: участник и его чат с ботом.
type Recipient struct {
	MemberID  string
	MaxUserID int64
	Name      string
	Role      string
}

// ReminderRecipients — активные участники с включённым порогом (F44, F51);
// разовое напоминание — только тому, кто его попросил.
func (s *Store) ReminderRecipients(ctx context.Context, d DueReminder) ([]Recipient, error) {
	rows, err := s.db.Query(ctx, `
		SELECT m.id::text, u.max_user_id, u.first_name, m.role
		FROM members m JOIN users u ON u.id = m.user_id
		WHERE m.trajectory_id = $1 AND m.left_at IS NULL AND m.removed_at IS NULL
		  AND (($2 = 0 AND m.id::text = $3) OR ($2 > 0 AND $2 = ANY(m.reminder_offsets)))
		ORDER BY m.joined_at`, d.TrajectoryID, d.Offset, d.RequestedBy)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (Recipient, error) {
		var x Recipient
		return x, r.Scan(&x.MemberID, &x.MaxUserID, &x.Name, &x.Role)
	})
}

// ClaimDelivery занимает пару (напоминание, получатель). false — уже
// отправлено или отправляется параллельным запуском: второй раз не слать.
func (s *Store) ClaimDelivery(ctx context.Context, reminderID, memberID string) (bool, error) {
	tag, err := s.db.Exec(ctx, `
		INSERT INTO reminder_deliveries (reminder_id, member_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		reminderID, memberID)
	if err != nil {
		return false, wrap(err)
	}
	return tag.RowsAffected() == 1, nil
}

// ReleaseDelivery освобождает пару после неудачной отправки: следующий
// запуск воркера попробует снова, пока не прошёл срок.
func (s *Store) ReleaseDelivery(ctx context.Context, reminderID, memberID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM reminder_deliveries WHERE reminder_id = $1 AND member_id = $2`,
		reminderID, memberID)
	return wrap(err)
}

func (s *Store) SetDeliveryMessage(ctx context.Context, reminderID, memberID, messageID string) error {
	_, err := s.db.Exec(ctx, `UPDATE reminder_deliveries SET max_message_id = $3 WHERE reminder_id = $1 AND member_id = $2`,
		reminderID, memberID, messageID)
	return wrap(err)
}

func (s *Store) MarkReminderSent(ctx context.Context, reminderID string) error {
	_, err := s.db.Exec(ctx, `UPDATE reminders SET status = 'sent' WHERE id = $1 AND status = 'planned'`, reminderID)
	return wrap(err)
}
