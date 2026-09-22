package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrAlreadyInTracker — предлагать то, что уже в трекере, незачем.
var ErrAlreadyInTracker = errors.New("store: олимпиада уже в трекере")

// ErrProposalClosed — на предложение уже ответили.
var ErrProposalClosed = errors.New("store: предложение уже закрыто")

const proposalSelect = `
	SELECT pr.id::text, p.id, o.id, o.name, pr.status, pm.id::text, pu.first_name, pm.role,
	       pr.created_at, pr.resolved_at
	FROM proposals pr
	JOIN olympiad_profiles p ON p.id = pr.olympiad_profile_id
	JOIN olympiads o ON o.id = p.olympiad_id
	LEFT JOIN members pm ON pm.id = pr.proposed_by_member_id
	LEFT JOIN users pu ON pu.id = pm.user_id`

func scanProposal(r rowScanner) (ProposalRow, error) {
	var p ProposalRow
	var id, name, role *string
	err := r.Scan(&p.ID, &p.ProfileID, &p.OlympiadID, &p.OlympiadName, &p.Status, &id, &name, &role,
		&p.CreatedAt, &p.ResolvedAt)
	p.ProposedBy = brief(id, name, role)
	return p, err
}

func (s *Store) Proposal(ctx context.Context, trajectoryID, id string) (ProposalRow, error) {
	p, err := scanProposal(s.db.QueryRow(ctx, proposalSelect+` WHERE pr.trajectory_id = $1 AND pr.id = $2`,
		trajectoryID, id))
	return p, wrap(err)
}

// CreateProposal — предложение родителя (F45). Второе по тому же профилю,
// пока первое ждёт ответа, возвращает первое и created = false.
func (s *Store) CreateProposal(ctx context.Context, trajectoryID, profileID, memberID string) (id string, created bool, err error) {
	err = s.Tx(ctx, func(tx *Store) error {
		var inTracker bool
		if err := tx.db.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM tracker_items WHERE trajectory_id = $1 AND olympiad_profile_id = $2)`,
			trajectoryID, profileID).Scan(&inTracker); err != nil {
			return wrap(err)
		}
		if inTracker {
			return ErrAlreadyInTracker
		}
		err := tx.db.QueryRow(ctx, `
			INSERT INTO proposals (trajectory_id, olympiad_profile_id, proposed_by_member_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (trajectory_id, olympiad_profile_id) WHERE status = 'pending' DO NOTHING
			RETURNING id::text`, trajectoryID, profileID, memberID).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return wrap(tx.db.QueryRow(ctx, `
				SELECT id::text FROM proposals
				WHERE trajectory_id = $1 AND olympiad_profile_id = $2 AND status = 'pending'`,
				trajectoryID, profileID).Scan(&id))
		}
		if err != nil {
			return wrap(err)
		}
		created = true
		return tx.Audit(ctx, memberID, "create", "proposal", id)
	})
	return id, created, err
}

// ResolveProposal — ответ ученика. Принятие одной транзакцией закрывает
// предложение и кладёт олимпиаду в трекер; автор пункта — тот, кто
// предложил. Возвращает id пункта трекера (при отказе — пусто).
func (s *Store) ResolveProposal(ctx context.Context, trajectoryID, id, memberID string, accept bool) (trackerItemID string, err error) {
	status := "declined"
	if accept {
		status = "accepted"
	}
	err = s.Tx(ctx, func(tx *Store) error {
		var profileID string
		var proposedBy *string
		err := tx.db.QueryRow(ctx, `
			UPDATE proposals SET status = $3, resolved_at = now()
			WHERE trajectory_id = $1 AND id = $2 AND status = 'pending'
			RETURNING olympiad_profile_id, proposed_by_member_id::text`,
			trajectoryID, id, status).Scan(&profileID, &proposedBy)
		if errors.Is(err, pgx.ErrNoRows) {
			if _, err := tx.Proposal(ctx, trajectoryID, id); err != nil {
				return err
			}
			return ErrProposalClosed
		}
		if err != nil {
			return wrap(err)
		}
		if err := tx.Audit(ctx, memberID, status, "proposal", id); err != nil {
			return err
		}
		if !accept {
			return nil
		}
		err = tx.db.QueryRow(ctx, `
			INSERT INTO tracker_items (trajectory_id, olympiad_profile_id, added_by_member_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (trajectory_id, olympiad_profile_id) DO UPDATE SET trajectory_id = EXCLUDED.trajectory_id
			RETURNING id::text`, trajectoryID, profileID, proposedBy).Scan(&trackerItemID)
		if err != nil {
			return wrap(err)
		}
		return tx.Audit(ctx, memberID, "create", "tracker_item", trackerItemID)
	})
	return trackerItemID, err
}
