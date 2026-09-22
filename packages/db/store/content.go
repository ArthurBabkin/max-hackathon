package store

import (
	"context"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
)

// StagesFor — этапы профилей, упорядоченные для таймлайна.
func (s *Store) StagesFor(ctx context.Context, profileIDs []string) (map[string][]stages.Stage, error) {
	out := map[string][]stages.Stage{}
	if len(profileIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT olympiad_profile_id, id, kind, COALESCE(title, ''), starts_at, ends_at, deadline_at,
		       is_online, is_demo
		FROM stages WHERE olympiad_profile_id = ANY($1)`, profileIDs)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var pid string
		var st stages.Stage
		if err := rows.Scan(&pid, &st.ID, &st.Kind, &st.Title, &st.StartsAt, &st.EndsAt, &st.DeadlineAt,
			&st.IsOnline, &st.IsDemo); err != nil {
			return nil, wrap(err)
		}
		out[pid] = append(out[pid], st)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap(err)
	}
	for pid := range out {
		stages.Sort(out[pid])
	}
	return out, nil
}

// Subject — предмет из справочника.
type Subject struct {
	Code string
	Name string
}

func (s *Store) TrajectorySubjects(ctx context.Context, trajectoryID string) ([]Subject, error) {
	rows, err := s.db.Query(ctx, `
		SELECT sub.code, sub.name FROM trajectory_subjects ts
		JOIN subjects sub ON sub.code = ts.subject_code
		WHERE ts.trajectory_id = $1 ORDER BY sub.name`, trajectoryID)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (Subject, error) {
		var x Subject
		return x, r.Scan(&x.Code, &x.Name)
	})
}

// University — строка каталога вузов (UniversityListItem).
type University struct {
	ID               string
	ShortName        string
	Name             string
	City             *string
	BenefitOlympiads int
	IsMine           bool
}

// benefitOlympiads — сколько разных олимпиад дают в вузе БВИ или 100 баллов
// в последнем году приёма, по которому есть данные. Доп. баллы не считаются.
const benefitOlympiads = `(
	SELECT count(DISTINCT p.olympiad_id) FROM benefits b
	JOIN olympiad_profiles p ON p.id = b.olympiad_profile_id
	WHERE b.university_id = u.id AND b.benefit IN ('bvi', 'bvi_winners', 'score100')
	  AND b.admission_year = (SELECT max(admission_year) FROM benefits))`

func (s *Store) TrajectoryUniversities(ctx context.Context, trajectoryID string) ([]University, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.short_name, u.name, u.city, `+benefitOlympiads+`, true
		FROM trajectory_universities tu JOIN universities u ON u.id = tu.university_id
		WHERE tu.trajectory_id = $1 ORDER BY u.name`, trajectoryID)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, scanUniversity)
}

func scanUniversity(r rowScanner) (University, error) {
	var u University
	return u, r.Scan(&u.ID, &u.ShortName, &u.Name, &u.City, &u.BenefitOlympiads, &u.IsMine)
}

// OtherMemberNames — имена остальных активных участников по времени входа.
func (s *Store) OtherMemberNames(ctx context.Context, trajectoryID, exceptMemberID string) ([]string, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.first_name FROM members m JOIN users u ON u.id = m.user_id
		WHERE m.trajectory_id = $1 AND m.id <> $2 AND m.left_at IS NULL AND m.removed_at IS NULL
		ORDER BY m.joined_at`, trajectoryID, exceptMemberID)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (string, error) {
		var n string
		return n, r.Scan(&n)
	})
}

// TrajectoryPatch — правка профиля ученика; nil означает «не менять».
type TrajectoryPatch struct {
	StudentName   *string
	Grade         *int
	RegionCode    *string
	TZ            *string
	DirectionID   *string
	SubjectCodes  []string
	UniversityIDs []string
}

// UpdateTrajectory применяет правку одной транзакцией. Выбор направления в
// профиле — осознанная цель, поэтому goal_status становится known.
func (s *Store) UpdateTrajectory(ctx context.Context, trajectoryID, memberID string, p TrajectoryPatch) error {
	return s.Tx(ctx, func(tx *Store) error {
		tag, err := tx.db.Exec(ctx, `
			UPDATE trajectories SET
			  student_name = COALESCE($2, student_name),
			  grade        = COALESCE($3, grade),
			  region_code  = COALESCE($4, region_code),
			  tz           = COALESCE($5, tz),
			  direction_id = COALESCE($6, direction_id),
			  goal_status  = CASE WHEN $6::text IS NULL THEN goal_status ELSE 'known' END,
			  updated_at   = now()
			WHERE id = $1 AND deleted_at IS NULL`,
			trajectoryID, p.StudentName, p.Grade, p.RegionCode, p.TZ, p.DirectionID)
		if err != nil {
			return wrap(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if p.SubjectCodes != nil {
			if err := tx.ReplaceSubjects(ctx, trajectoryID, p.SubjectCodes); err != nil {
				return err
			}
		}
		if p.UniversityIDs != nil {
			if err := tx.ReplaceUniversities(ctx, trajectoryID, p.UniversityIDs); err != nil {
				return err
			}
		}
		return tx.Audit(ctx, memberID, "update", "trajectory", trajectoryID)
	})
}

// MemberBrief — кто что сделал: «Отметила Ольга», «добавил(а) Артём».
type MemberBrief struct {
	ID   string
	Name string
	Role string
}

// TrackerRow — пункт трекера с данными профиля; этапы — отдельно (StagesFor).
type TrackerRow struct {
	ID           string
	ProfileID    string
	OlympiadID   string
	OlympiadName string
	SubjectName  string
	ProfileName  *string
	Kind         string
	Level        *string
	CreatedAt    time.Time
	RegisteredAt *time.Time
	RegisteredBy *MemberBrief
	AddedBy      *MemberBrief
}

const trackerSelect = `
	SELECT ti.id::text, p.id, o.id, o.name, sub.name, p.profile_name, o.kind, p.level,
	       ti.created_at, ti.registered_at,
	       rb.id::text, ru.first_name, rb.role,
	       ab.id::text, au.first_name, ab.role
	FROM tracker_items ti
	JOIN olympiad_profiles p ON p.id = ti.olympiad_profile_id
	JOIN olympiads o ON o.id = p.olympiad_id
	JOIN subjects sub ON sub.code = p.subject_code
	LEFT JOIN members rb ON rb.id = ti.registered_by_member_id
	LEFT JOIN users ru ON ru.id = rb.user_id
	LEFT JOIN members ab ON ab.id = ti.added_by_member_id
	LEFT JOIN users au ON au.id = ab.user_id`

func scanTracker(r rowScanner) (TrackerRow, error) {
	var t TrackerRow
	var rbID, rbName, rbRole, abID, abName, abRole *string
	err := r.Scan(&t.ID, &t.ProfileID, &t.OlympiadID, &t.OlympiadName, &t.SubjectName, &t.ProfileName,
		&t.Kind, &t.Level, &t.CreatedAt, &t.RegisteredAt, &rbID, &rbName, &rbRole, &abID, &abName, &abRole)
	t.RegisteredBy = brief(rbID, rbName, rbRole)
	t.AddedBy = brief(abID, abName, abRole)
	return t, err
}

func brief(id, name, role *string) *MemberBrief {
	if id == nil || name == nil || role == nil {
		return nil
	}
	return &MemberBrief{ID: *id, Name: *name, Role: *role}
}

func (s *Store) TrackerItems(ctx context.Context, trajectoryID string) ([]TrackerRow, error) {
	rows, err := s.db.Query(ctx, trackerSelect+` WHERE ti.trajectory_id = $1 ORDER BY ti.created_at`, trajectoryID)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, scanTracker)
}

func (s *Store) PendingProposalsCount(ctx context.Context, trajectoryID string) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM proposals WHERE trajectory_id = $1 AND status = 'pending'`,
		trajectoryID).Scan(&n)
	return n, wrap(err)
}
