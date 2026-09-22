package store

import (
	"context"
	"time"
)

// Member — активное участие пользователя в живой траектории. Из него
// собираются JWT, права и проверка «участник ещё в траектории».
type Member struct {
	UserID          string
	MaxUserID       int64
	FirstName       string
	MemberID        string
	TrajectoryID    string
	Role            string
	IsCreator       bool
	ReminderOffsets []int32
	HasKid          bool
}

// Условие «участие живо»: не вышел, не удалён, траектория не удалена.
const activeMember = `m.left_at IS NULL AND m.removed_at IS NULL AND t.deleted_at IS NULL`

const memberColumns = `u.id::text, u.max_user_id, u.first_name, m.id::text, t.id::text, m.role,
	m.is_creator, m.reminder_offsets,
	EXISTS (SELECT 1 FROM members k WHERE k.trajectory_id = t.id AND k.role = 'kid'
	        AND k.left_at IS NULL AND k.removed_at IS NULL)`

func scanMember(row interface{ Scan(...any) error }) (Member, error) {
	var m Member
	err := row.Scan(&m.UserID, &m.MaxUserID, &m.FirstName, &m.MemberID, &m.TrajectoryID,
		&m.Role, &m.IsCreator, &m.ReminderOffsets, &m.HasKid)
	return m, wrap(err)
}

// UpsertUser заводит пользователя MAX при первом входе и обновляет имя при
// следующих: имя в профиле MAX может поменяться.
func (s *Store) UpsertUser(ctx context.Context, maxUserID int64, firstName string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (max_user_id, first_name) VALUES ($1, $2)
		ON CONFLICT (max_user_id) DO UPDATE
		   SET first_name = EXCLUDED.first_name,
		       updated_at = CASE WHEN users.first_name = EXCLUDED.first_name
		                         THEN users.updated_at ELSE now() END
		RETURNING id::text`, maxUserID, firstName).Scan(&id)
	return id, wrap(err)
}

// CurrentMember — траектория, в которую пользователь входит сейчас. Если
// участий несколько (родитель двоих детей), берётся последнее по времени входа.
func (s *Store) CurrentMember(ctx context.Context, maxUserID int64) (Member, error) {
	return scanMember(s.db.QueryRow(ctx, `
		SELECT `+memberColumns+`
		FROM users u
		JOIN members m ON m.user_id = u.id
		JOIN trajectories t ON t.id = m.trajectory_id
		WHERE u.max_user_id = $1 AND `+activeMember+`
		ORDER BY m.joined_at DESC
		LIMIT 1`, maxUserID))
}

// ActiveMember перечитывает участие из токена. ErrNotFound означает, что
// участника удалили, он вышел или траекторию удалили, — токен ещё жив, а
// доступа уже нет.
func (s *Store) ActiveMember(ctx context.Context, memberID string) (Member, error) {
	return scanMember(s.db.QueryRow(ctx, `
		SELECT `+memberColumns+`
		FROM members m
		JOIN users u ON u.id = m.user_id
		JOIN trajectories t ON t.id = m.trajectory_id
		WHERE m.id = $1 AND `+activeMember, memberID))
}

// Trajectory — сводка траектории, схема TrajectorySummary без названия
// региона (его знает справочник в Go).
type Trajectory struct {
	ID            string
	StudentName   string
	Grade         int
	RegionCode    string
	TZ            string
	DirectionID   *string
	DirectionName *string
	// DirectionSubjects — ключевые предметы направления для подбора.
	DirectionSubjects []string
	GoalStatus        string
	HasKid            bool
	MembersCount      int
	CreatedAt         time.Time
}

func (s *Store) Trajectory(ctx context.Context, id string) (Trajectory, error) {
	var t Trajectory
	err := s.db.QueryRow(ctx, `
		SELECT t.id::text, t.student_name, t.grade, t.region_code, t.tz, t.direction_id, d.name,
		       COALESCE(d.subject_codes, '{}'), t.goal_status,
		       EXISTS (SELECT 1 FROM members k WHERE k.trajectory_id = t.id AND k.role = 'kid'
		               AND k.left_at IS NULL AND k.removed_at IS NULL),
		       (SELECT count(*) FROM members k WHERE k.trajectory_id = t.id
		               AND k.left_at IS NULL AND k.removed_at IS NULL),
		       t.created_at
		FROM trajectories t
		LEFT JOIN directions d ON d.id = t.direction_id
		WHERE t.id = $1 AND t.deleted_at IS NULL`, id).Scan(
		&t.ID, &t.StudentName, &t.Grade, &t.RegionCode, &t.TZ, &t.DirectionID, &t.DirectionName,
		&t.DirectionSubjects, &t.GoalStatus, &t.HasKid, &t.MembersCount, &t.CreatedAt)
	return t, wrap(err)
}
