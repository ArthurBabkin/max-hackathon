package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"
)

// ErrKidExists — ученик в траектории уже есть, вторую роль kid выдать нельзя (F42).
var ErrKidExists = errors.New("store: ученик в траектории уже есть")

// FamilyMember — активный участник траектории.
type FamilyMember struct {
	ID        string
	UserID    string
	MaxUserID int64
	Name      string
	Role      string
	IsCreator bool
	JoinedAt  time.Time
}

func (s *Store) FamilyMembers(ctx context.Context, trajectoryID string) ([]FamilyMember, error) {
	rows, err := s.db.Query(ctx, `
		SELECT m.id::text, u.id::text, u.max_user_id, u.first_name, m.role, m.is_creator, m.joined_at
		FROM members m JOIN users u ON u.id = m.user_id
		WHERE m.trajectory_id = $1 AND m.left_at IS NULL AND m.removed_at IS NULL
		ORDER BY m.is_creator DESC, m.joined_at`, trajectoryID)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (FamilyMember, error) {
		var m FamilyMember
		return m, r.Scan(&m.ID, &m.UserID, &m.MaxUserID, &m.Name, &m.Role, &m.IsCreator, &m.JoinedAt)
	})
}

// Invite — неиспользованная и неотозванная ссылка-приглашение.
type Invite struct {
	ID    string
	Token string
	Role  string
	// CreatedBy — участник, создавший ссылку; пусто, если его удалили.
	CreatedBy string
	CreatedAt time.Time
}

func (s *Store) ActiveInvites(ctx context.Context, trajectoryID string) ([]Invite, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, token, role, COALESCE(created_by_member_id::text, ''), created_at FROM invites
		WHERE trajectory_id = $1 AND used_at IS NULL AND revoked_at IS NULL
		ORDER BY created_at`, trajectoryID)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (Invite, error) {
		var i Invite
		return i, r.Scan(&i.ID, &i.Token, &i.Role, &i.CreatedBy, &i.CreatedAt)
	})
}

// RevokeInvite — отозвать неиспользованную ссылку (ТЗ §16). Отзывает автор
// ссылки или создатель траектории (byCreator). Чужая ссылка, ссылка из другой
// траектории, уже использованная или отозванная — ErrNotFound.
func (s *Store) RevokeInvite(ctx context.Context, trajectoryID, inviteID, byMemberID string, byCreator bool) error {
	return s.Tx(ctx, func(tx *Store) error {
		tag, err := tx.db.Exec(ctx, `
			UPDATE invites SET revoked_at = now()
			WHERE id = $1 AND trajectory_id = $2 AND used_at IS NULL AND revoked_at IS NULL
			  AND ($3 OR created_by_member_id = $4)`, inviteID, trajectoryID, byCreator, byMemberID)
		if err != nil {
			return wrap(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return tx.Audit(ctx, byMemberID, "revoke", "invite", inviteID)
	})
}

// CreateInvite — новая одноразовая ссылка (F39, F40). Токен генерирует
// вызывающий. Роль kid при ученике в траектории — ErrKidExists.
func (s *Store) CreateInvite(ctx context.Context, trajectoryID, memberID, role, token string) (Invite, error) {
	var inv Invite
	err := s.Tx(ctx, func(tx *Store) error {
		if role == "kid" {
			var hasKid bool
			if err := tx.db.QueryRow(ctx, `
				SELECT EXISTS (SELECT 1 FROM members WHERE trajectory_id = $1 AND role = 'kid'
				               AND left_at IS NULL AND removed_at IS NULL)`, trajectoryID).Scan(&hasKid); err != nil {
				return wrap(err)
			}
			if hasKid {
				return ErrKidExists
			}
		}
		if err := tx.db.QueryRow(ctx, `
			INSERT INTO invites (trajectory_id, token, role, created_by_member_id) VALUES ($1, $2, $3, $4)
			RETURNING id::text, token, role, created_by_member_id::text, created_at`, trajectoryID, token, role, memberID).Scan(
			&inv.ID, &inv.Token, &inv.Role, &inv.CreatedBy, &inv.CreatedAt); err != nil {
			return wrap(err)
		}
		return tx.Audit(ctx, memberID, "create", "invite", inv.ID)
	})
	return inv, err
}

// RemoveMember — создатель удаляет приглашённого (F41). Создателя удалить
// нельзя: такой строки UPDATE не найдёт, и ответ — ErrNotFound. Возвращает
// удалённого, чтобы бот мог ему написать.
func (s *Store) RemoveMember(ctx context.Context, trajectoryID, targetID, byMemberID string) (FamilyMember, error) {
	return s.endMembership(ctx, trajectoryID, targetID, byMemberID, "removed_at", "remove")
}

// LeaveTrajectory — приглашённый выходит сам (F48); создатель не может.
func (s *Store) LeaveTrajectory(ctx context.Context, trajectoryID, memberID string) (FamilyMember, error) {
	return s.endMembership(ctx, trajectoryID, memberID, memberID, "left_at", "leave")
}

// endMembership завершает участие и отзывает неиспользованные ссылки
// ушедшего: раздавать доступ он больше не вправе. Олимпиады и напоминания
// остаются у остальных.
func (s *Store) endMembership(ctx context.Context, trajectoryID, targetID, byMemberID, column, action string) (FamilyMember, error) {
	var m FamilyMember
	err := s.Tx(ctx, func(tx *Store) error {
		err := tx.db.QueryRow(ctx, `
			UPDATE members m SET `+column+` = now()
			FROM users u
			WHERE u.id = m.user_id AND m.trajectory_id = $1 AND m.id = $2 AND NOT m.is_creator
			  AND m.left_at IS NULL AND m.removed_at IS NULL
			RETURNING m.id::text, u.id::text, u.max_user_id, u.first_name, m.role, m.is_creator, m.joined_at`,
			trajectoryID, targetID).Scan(&m.ID, &m.UserID, &m.MaxUserID, &m.Name, &m.Role, &m.IsCreator, &m.JoinedAt)
		if err != nil {
			return wrap(err)
		}
		if _, err := tx.db.Exec(ctx, `
			UPDATE invites SET revoked_at = now()
			WHERE created_by_member_id = $1 AND used_at IS NULL AND revoked_at IS NULL`, targetID); err != nil {
			return wrap(err)
		}
		// Вышел сам — отозвал согласие; удалённый создателем его не отзывал.
		if targetID == byMemberID {
			if _, err := tx.WithdrawPrivacy(ctx, m.UserID); err != nil {
				return err
			}
		}
		return tx.Audit(ctx, byMemberID, action, "member", targetID)
	})
	return m, err
}

// NewInviteToken — 16 случайных байт в base64url: 22 символа [A-Za-z0-9_-]
// (F39 требует 12+).
func NewInviteToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
