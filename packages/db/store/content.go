package store

import (
	"context"
	"slices"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/refdata"
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
	RegionCode       string // субъект РФ; "" — не указан
	BenefitOlympiads int
	IsMine           bool
}

// universityColumns — колонки University до признака «мой».
const universityColumns = `u.id, u.short_name, u.name, u.city, COALESCE(u.region_code, ''), ` + benefitOlympiads

// benefitOlympiads — сколько разных олимпиад дают в вузе БВИ или 100 баллов
// в последнем году приёма, по которому есть данные. Доп. баллы не считаются.
const benefitOlympiads = `(
	SELECT count(DISTINCT p.olympiad_id) FROM benefits b
	JOIN olympiad_profiles p ON p.id = b.olympiad_profile_id
	WHERE b.university_id = u.id AND b.benefit IN ('bvi', 'bvi_winners', 'score100')
	  AND b.admission_year = (SELECT max(admission_year) FROM benefits))`

func (s *Store) TrajectoryUniversities(ctx context.Context, trajectoryID string) ([]University, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+universityColumns+`, true
		FROM trajectory_universities tu JOIN universities u ON u.id = tu.university_id
		WHERE tu.trajectory_id = $1 ORDER BY u.name`, trajectoryID)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, scanUniversity)
}

func scanUniversity(r rowScanner) (University, error) {
	var u University
	return u, r.Scan(&u.ID, &u.ShortName, &u.Name, &u.City, &u.RegionCode, &u.BenefitOlympiads, &u.IsMine)
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
	StudentName *string
	Grade       *int
	RegionCode  *string
	TZ          *string
	// DirectionIDs: nil — не менять, пустой срез — «пока не решил».
	DirectionIDs []string
	// GoalStatus — вместе с DirectionIDs: suggested, если направления
	// предложил бот. nil — по направлениям (GoalStatusOf).
	GoalStatus *string
	// Places: nil — не менять, пустой срез — «не важно».
	Places []Place
	// Experience: none | school | region.
	Experience *string
	// HomeCity: "" — город не известен.
	HomeCity      *string
	GoalByKid     *bool
	SubjectCodes  []string
	UniversityIDs []string
}

// UpdateTrajectory применяет правку одной транзакцией. Выбранные в профиле
// направления — осознанная цель (known), пустой список — «пока не решил».
func (s *Store) UpdateTrajectory(ctx context.Context, trajectoryID, memberID string, p TrajectoryPatch) error {
	var goal *string
	if p.DirectionIDs != nil {
		g := GoalStatusOf(p.DirectionIDs)
		if p.GoalStatus != nil && len(p.DirectionIDs) > 0 {
			g = *p.GoalStatus
		}
		goal = &g
	}
	return s.Tx(ctx, func(tx *Store) error {
		tag, err := tx.db.Exec(ctx, `
			UPDATE trajectories SET
			  student_name = COALESCE($2, student_name),
			  grade        = COALESCE($3, grade),
			  region_code  = COALESCE($4, region_code),
			  tz           = COALESCE($5, tz),
			  goal_status  = COALESCE($6, goal_status),
			  experience   = COALESCE($7, experience),
			  -- Город принадлежит региону: сменился регион — город больше не известен.
			  home_city    = CASE WHEN $8::text IS NOT NULL THEN NULLIF($8, '')
			                      WHEN $4::text IS NOT NULL AND $4 <> region_code THEN NULL
			                      ELSE home_city END,
			  goal_by_kid  = COALESCE($9, goal_by_kid),
			  updated_at   = now()
			WHERE id = $1 AND deleted_at IS NULL`,
			trajectoryID, p.StudentName, p.Grade, p.RegionCode, p.TZ, goal, p.Experience, p.HomeCity, p.GoalByKid)
		if err != nil {
			return wrap(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if p.Places != nil {
			if err := tx.ReplacePlaces(ctx, trajectoryID, p.Places); err != nil {
				return err
			}
		}
		if p.DirectionIDs != nil {
			if err := tx.ReplaceDirections(ctx, trajectoryID, p.DirectionIDs); err != nil {
				return err
			}
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

// AllSubjects — школьные предметы для онбординга (F7).
func (s *Store) AllSubjects(ctx context.Context) ([]Subject, error) {
	rows, err := s.db.Query(ctx, `SELECT code, name FROM subjects ORDER BY name`)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (Subject, error) {
		var x Subject
		return x, r.Scan(&x.Code, &x.Name)
	})
}

type Direction struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	SubjectCodes []string `json:"subject_codes"`
}

// Directions — направления подготовки для выбора цели (F8).
func (s *Store) Directions(ctx context.Context) ([]Direction, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, subject_codes FROM directions ORDER BY name`)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (Direction, error) {
		var x Direction
		return x, r.Scan(&x.ID, &x.Name, &x.SubjectCodes)
	})
}

// SuggestUniversities — вузы для шага «Вузы» онбординга (F9): в одном из
// мест places (пусто — где угодно), с одним из направлений directionIDs
// (пусто — с любыми). Место-город — вузы этого города, место-регион — вузы
// региона; Москва и Петербург — вместе с областью (refdata.Metro). Сначала
// те, где совпало больше направлений, потом где больше олимпиад дают льготу.
func (s *Store) SuggestUniversities(ctx context.Context, directionIDs []string, places []Place, limit, offset int) ([]University, error) {
	regions, cities := placeFilter(places)
	rows, err := s.db.Query(ctx, `
		WITH names AS (SELECT array_agg(name) AS n FROM directions WHERE id = ANY($1::text[]))
		SELECT `+universityColumns+`, false
		FROM universities u, names
		WHERE (cardinality($2::text[]) + cardinality($3::text[]) = 0
		       OR u.region_code = ANY($2::text[]) OR u.city = ANY($3::text[]))
		  AND (cardinality($1::text[]) = 0 OR u.directions && names.n)
		ORDER BY cardinality(ARRAY(SELECT unnest(u.directions) INTERSECT SELECT unnest(names.n))) DESC,
		         6 DESC, u.name
		LIMIT $4 OFFSET $5`, nonNil(directionIDs), regions, cities, limit, offset)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, scanUniversity)
}

// placeFilter — места как условие на вузы: регионы (Москва и Петербург —
// вместе с областью) и города.
func placeFilter(places []Place) (regions, cities []string) {
	regions, cities = []string{}, []string{}
	for _, p := range places {
		if p.City != "" {
			cities = append(cities, p.City)
			continue
		}
		for _, code := range refdata.Metro(p.RegionCode) {
			if !slices.Contains(regions, code) {
				regions = append(regions, code)
			}
		}
	}
	return regions, cities
}

// DirectionsIn — id направлений, по которым есть программы в вузах мест
// places, в порядке справочника.
func (s *Store) DirectionsIn(ctx context.Context, places []Place) ([]string, error) {
	regions, cities := placeFilter(places)
	rows, err := s.db.Query(ctx, `
		SELECT d.id FROM directions d
		WHERE EXISTS (SELECT 1 FROM universities u
		              WHERE d.name = ANY(u.directions)
		                AND (cardinality($1::text[]) + cardinality($2::text[]) = 0
		                     OR u.region_code = ANY($1::text[]) OR u.city = ANY($2::text[])))
		ORDER BY d.name`, regions, cities)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (string, error) {
		var id string
		return id, r.Scan(&id)
	})
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// FindUniversities — вузы по подстроке названия, для онбординга (F9):
// там траектории ещё нет, поэтому без признака «мой вуз».
func (s *Store) FindUniversities(ctx context.Context, search string) ([]University, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+universityColumns+`, false
		FROM universities u
		WHERE $1 = '' OR u.name ILIKE $2 OR u.short_name ILIKE $2
		ORDER BY u.name`, search, "%"+escapeLike(search)+"%")
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, scanUniversity)
}
