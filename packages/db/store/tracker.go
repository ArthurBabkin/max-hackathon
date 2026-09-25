package store

import (
	"context"
	"errors"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/jackc/pgx/v5"
)

// AddTrackerItem добавляет профиль в трекер. Повторное добавление
// возвращает существующий пункт и created = false (F28). Если по профилю
// ждало ответа предложение, ученик им и ответил — оно принимается.
func (s *Store) AddTrackerItem(ctx context.Context, trajectoryID, profileID, memberID string) (id string, created bool, err error) {
	err = s.Tx(ctx, func(tx *Store) error {
		err := tx.db.QueryRow(ctx, `
			INSERT INTO tracker_items (trajectory_id, olympiad_profile_id, added_by_member_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (trajectory_id, olympiad_profile_id) DO NOTHING
			RETURNING id::text`, trajectoryID, profileID, memberID).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return wrap(tx.db.QueryRow(ctx, `
				SELECT id::text FROM tracker_items WHERE trajectory_id = $1 AND olympiad_profile_id = $2`,
				trajectoryID, profileID).Scan(&id))
		}
		if err != nil {
			return wrap(err)
		}
		created = true
		if _, err := tx.db.Exec(ctx, `
			UPDATE proposals SET status = 'accepted', resolved_at = now()
			WHERE trajectory_id = $1 AND olympiad_profile_id = $2 AND status = 'pending'`,
			trajectoryID, profileID); err != nil {
			return wrap(err)
		}
		return tx.Audit(ctx, memberID, "create", "tracker_item", id)
	})
	return id, created, err
}

func (s *Store) TrackerItem(ctx context.Context, trajectoryID, itemID string) (TrackerRow, error) {
	row, err := scanTracker(s.db.QueryRow(ctx, trackerSelect+` WHERE ti.trajectory_id = $1 AND ti.id = $2`,
		trajectoryID, itemID))
	return row, wrap(err)
}

// DeleteTrackerItem убирает пункт; напоминания уходят каскадом.
func (s *Store) DeleteTrackerItem(ctx context.Context, trajectoryID, itemID, memberID string) error {
	return s.Tx(ctx, func(tx *Store) error {
		tag, err := tx.db.Exec(ctx, `DELETE FROM tracker_items WHERE trajectory_id = $1 AND id = $2`,
			trajectoryID, itemID)
		if err != nil {
			return wrap(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return tx.Audit(ctx, memberID, "delete", "tracker_item", itemID)
	})
}

// SetRegistered ставит или снимает отметку «зарегистрирован» (F46) — первую
// регистрацию олимпиады. Повторная отметка ничего не меняет: остаётся тот,
// кто отметил первым. Отметка отменяет запланированные напоминания о первой
// регистрации (ТЗ §6.3); регистрация на заключительный — отдельный этап со
// своей отметкой (SetStageMark). Снять отметку, на которой держатся итоги
// этапов, нельзя — ErrConflict. changed сообщает, изменилось ли что-то, —
// уведомлять ли остальных.
func (s *Store) SetRegistered(ctx context.Context, trajectoryID, itemID, memberID string, on bool) (changed bool, err error) {
	err = s.Tx(ctx, func(tx *Store) error {
		var was *time.Time
		var profileID string
		if err := tx.db.QueryRow(ctx, `
			SELECT registered_at, olympiad_profile_id FROM tracker_items WHERE trajectory_id = $1 AND id = $2 FOR UPDATE`,
			trajectoryID, itemID).Scan(&was, &profileID); err != nil {
			return wrap(err)
		}
		if on == (was != nil) {
			return nil
		}
		changed = true
		if !on {
			var marked bool
			if err := tx.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tracker_stage_results WHERE tracker_item_id = $1)`,
				itemID).Scan(&marked); err != nil {
				return wrap(err)
			}
			if marked {
				return ErrConflict
			}
			if _, err := tx.db.Exec(ctx, `
				UPDATE tracker_items SET registered_at = NULL, registered_by_member_id = NULL WHERE id = $1`,
				itemID); err != nil {
				return wrap(err)
			}
			return tx.Audit(ctx, memberID, "unregister", "tracker_item", itemID)
		}
		if _, err := tx.db.Exec(ctx, `
			UPDATE tracker_items SET registered_at = now(), registered_by_member_id = $2 WHERE id = $1`,
			itemID, memberID); err != nil {
			return wrap(err)
		}
		byProfile, err := tx.StagesFor(ctx, []string{profileID})
		if err != nil {
			return err
		}
		if i := stages.FirstRegistration(byProfile[profileID]); i >= 0 {
			if _, err := tx.db.Exec(ctx, `
				UPDATE reminders SET status = 'cancelled'
				WHERE tracker_item_id = $1 AND stage_id = $2 AND status = 'planned' AND offset_days >= 0`,
				itemID, byProfile[profileID][i].ID); err != nil {
				return wrap(err)
			}
		}
		return tx.Audit(ctx, memberID, "register", "tracker_item", itemID)
	})
	return changed, err
}

// ProposalRow — предложение родителя с данными олимпиады; этапы — отдельно.
type ProposalRow struct {
	ID           string
	ProfileID    string
	OlympiadID   string
	OlympiadName string
	Status       string
	ProposedBy   *MemberBrief
	CreatedAt    time.Time
	ResolvedAt   *time.Time
}

// PendingProposals — ждущие ответа предложения траектории, старые первыми.
// proposedBy != "" оставляет только предложения этого участника.
func (s *Store) PendingProposals(ctx context.Context, trajectoryID, proposedBy string) ([]ProposalRow, error) {
	rows, err := s.db.Query(ctx, proposalSelect+`
		WHERE pr.trajectory_id = $1 AND pr.status = 'pending'
		  AND ($2 = '' OR pr.proposed_by_member_id::text = $2)
		ORDER BY pr.created_at`, trajectoryID, proposedBy)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, scanProposal)
}
