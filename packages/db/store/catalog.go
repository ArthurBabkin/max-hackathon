package store

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// Source — источник факта. VerifiedAt пуст — показывать без метки «Факт».
type Source struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	URL        string     `json:"url"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
}

// Profile — профиль олимпиады со всем, что нужно карточке.
type Profile struct {
	ID              string
	OlympiadID      string
	OlympiadName    string
	Organizer       *string
	Kind            string
	Level           *string
	SubjectCode     string
	SubjectName     string
	ProfileName     *string
	Format          *string
	FinalCity       *string
	FinalRegionCode *string
	OfficialURL     *string
	Description     *string
	GradesFrom      int
	GradesTo        int
	Source          *Source
}

// ProfileQuery — отбор профилей. Пустые поля не фильтруют.
type ProfileQuery struct {
	IDs          []string
	OlympiadIDs  []string
	SubjectCodes []string
	Grade        int    // класс допускается к участию
	Search       string // название или организатор, без учёта регистра
	City         string // город финала
}

func (s *Store) Profiles(ctx context.Context, q ProfileQuery) ([]Profile, error) {
	sql := `
		SELECT p.id, o.id, o.name, o.organizer, o.kind, p.level, p.subject_code, sub.name, p.profile_name,
		       o.format, o.final_city, o.final_region_code, o.official_url, o.description, p.grades_from, p.grades_to,
		       src.id, src.kind, src.title, src.url, src.verified_at
		FROM olympiad_profiles p
		JOIN olympiads o ON o.id = p.olympiad_id
		JOIN subjects sub ON sub.code = p.subject_code
		LEFT JOIN sources src ON src.id = p.source_id
		WHERE true`
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if q.IDs != nil {
		sql += ` AND p.id = ANY(` + arg(q.IDs) + `)`
	}
	if q.OlympiadIDs != nil {
		sql += ` AND o.id = ANY(` + arg(q.OlympiadIDs) + `)`
	}
	if q.SubjectCodes != nil {
		sql += ` AND p.subject_code = ANY(` + arg(q.SubjectCodes) + `)`
	}
	if q.Grade > 0 {
		g := arg(q.Grade)
		sql += ` AND p.grades_from <= ` + g + ` AND p.grades_to >= ` + g
	}
	if q.Search != "" {
		// ILIKE по шаблону с экранированием: пользовательский % не должен
		// превращаться в «всё подряд».
		pat := arg("%" + escapeLike(q.Search) + "%")
		sql += ` AND (o.name ILIKE ` + pat + ` OR o.organizer ILIKE ` + pat + `)`
	}
	if q.City != "" {
		sql += ` AND o.final_city = ` + arg(q.City)
	}
	sql += ` ORDER BY o.name, p.id`
	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (Profile, error) {
		var p Profile
		var src sourceCols
		err := r.Scan(&p.ID, &p.OlympiadID, &p.OlympiadName, &p.Organizer, &p.Kind, &p.Level, &p.SubjectCode,
			&p.SubjectName, &p.ProfileName, &p.Format, &p.FinalCity, &p.FinalRegionCode, &p.OfficialURL,
			&p.Description, &p.GradesFrom, &p.GradesTo, &src.id, &src.kind, &src.title, &src.url, &src.verified)
		p.Source = src.source()
		return p, err
	})
}

type sourceCols struct {
	id, kind, title, url *string
	verified             *time.Time
}

func (c sourceCols) source() *Source {
	if c.id == nil {
		return nil
	}
	return &Source{ID: *c.id, Kind: *c.kind, Title: *c.title, URL: *c.url, VerifiedAt: c.verified}
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// BenefitRow — льгота профиля в вузе за последний год приёма.
type BenefitRow struct {
	ProfileID       string
	UniversityID    string
	UniversityName  string
	UniversityShort string
	City            *string
	Benefit         string
	ExtraPoints     *int
	EgeMin          *int
	DiplomaGrades   []int32
	Note            *string
	AdmissionYear   int
	Source          *Source
}

// Benefits — по одной строке на пару (профиль, вуз): последний год приёма и
// лучшая льгота. universityIDs == nil — все вузы.
func (s *Store) Benefits(ctx context.Context, profileIDs, universityIDs []string) ([]BenefitRow, error) {
	if len(profileIDs) == 0 {
		return []BenefitRow{}, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT ON (b.olympiad_profile_id, b.university_id)
		       b.olympiad_profile_id, u.id, u.name, u.short_name, u.city, b.benefit, b.extra_points,
		       b.ege_min, b.diploma_grades, b.note, b.admission_year,
		       src.id, src.kind, src.title, src.url, src.verified_at
		FROM benefits b
		JOIN universities u ON u.id = b.university_id
		LEFT JOIN sources src ON src.id = b.source_id
		WHERE b.olympiad_profile_id = ANY($1) AND ($2::text[] IS NULL OR b.university_id = ANY($2))
		ORDER BY b.olympiad_profile_id, b.university_id, b.admission_year DESC,
		         array_position(ARRAY['bvi','bvi_winners','score100','extra_points'], b.benefit)`,
		profileIDs, universityIDs)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (BenefitRow, error) {
		var b BenefitRow
		var src sourceCols
		err := r.Scan(&b.ProfileID, &b.UniversityID, &b.UniversityName, &b.UniversityShort, &b.City, &b.Benefit,
			&b.ExtraPoints, &b.EgeMin, &b.DiplomaGrades, &b.Note, &b.AdmissionYear,
			&src.id, &src.kind, &src.title, &src.url, &src.verified)
		b.Source = src.source()
		return b, err
	})
}

// TrackerState — что из профилей уже в трекере (и отмечено ли) и по чему
// ждёт ответа предложение: для кнопок карточки. TrackedOlympiads и
// PendingOlympiads — то же по олимпиадам: «Подбор» прячет их целиком.
type TrackerState struct {
	InTracker        map[string]bool // профиль → отмечена регистрация
	Pending          map[string]bool
	Registered       map[string]bool
	TrackedOlympiads map[string]bool
	PendingOlympiads map[string]bool
}

func (s *Store) TrackerState(ctx context.Context, trajectoryID string) (TrackerState, error) {
	st := TrackerState{InTracker: map[string]bool{}, Pending: map[string]bool{}, Registered: map[string]bool{},
		TrackedOlympiads: map[string]bool{}, PendingOlympiads: map[string]bool{}}
	rows, err := s.db.Query(ctx, `
		SELECT ti.olympiad_profile_id, p.olympiad_id, ti.registered_at IS NOT NULL, false
		FROM tracker_items ti JOIN olympiad_profiles p ON p.id = ti.olympiad_profile_id
		WHERE ti.trajectory_id = $1
		UNION ALL
		SELECT pr.olympiad_profile_id, p.olympiad_id, false, true
		FROM proposals pr JOIN olympiad_profiles p ON p.id = pr.olympiad_profile_id
		WHERE pr.trajectory_id = $1 AND pr.status = 'pending'`,
		trajectoryID)
	if err != nil {
		return st, wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var pid, oid string
		var registered, pending bool
		if err := rows.Scan(&pid, &oid, &registered, &pending); err != nil {
			return st, wrap(err)
		}
		if pending {
			st.Pending[pid] = true
			st.PendingOlympiads[oid] = true
			continue
		}
		st.InTracker[pid] = true
		st.Registered[pid] = registered
		st.TrackedOlympiads[oid] = true
	}
	return st, wrap(rows.Err())
}

// Universities — каталог вузов с отметкой «мой».
func (s *Store) Universities(ctx context.Context, trajectoryID, search, city string) ([]University, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+universityColumns+`,
		       EXISTS (SELECT 1 FROM trajectory_universities tu WHERE tu.trajectory_id = $1 AND tu.university_id = u.id)
		FROM universities u
		WHERE ($2 = '' OR u.name ILIKE $4 OR u.short_name ILIKE $4) AND ($3 = '' OR u.city = $3)
		ORDER BY u.name`, trajectoryID, search, city, "%"+escapeLike(search)+"%")
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, scanUniversity)
}

// UniversityDetail — карточка вуза.
type UniversityDetail struct {
	University
	Directions      []string
	EgeNote         *string
	RulesURL        *string
	RulesVerifiedAt *time.Time
	Description     *string
	SiteURL         *string
}

func (s *Store) University(ctx context.Context, trajectoryID, id string) (UniversityDetail, error) {
	var d UniversityDetail
	err := s.db.QueryRow(ctx, `
		SELECT `+universityColumns+`,
		       EXISTS (SELECT 1 FROM trajectory_universities tu WHERE tu.trajectory_id = $1 AND tu.university_id = u.id),
		       u.directions, u.ege_note, u.rules_url, u.rules_verified_at, u.description, u.site_url
		FROM universities u WHERE u.id = $2`, trajectoryID, id).Scan(
		&d.ID, &d.ShortName, &d.Name, &d.City, &d.RegionCode, &d.BenefitOlympiads, &d.IsMine,
		&d.Directions, &d.EgeNote, &d.RulesURL, &d.RulesVerifiedAt, &d.Description, &d.SiteURL)
	return d, wrap(err)
}

// UniversityOlympiad — строка «Олимпиады с льготой» в карточке вуза.
type UniversityOlympiad struct {
	ProfileID    string
	OlympiadID   string
	OlympiadName string
	SubjectCode  string
	SubjectName  string
	ProfileName  *string
	Level        *string
	Benefit      string
}

// UniversityOlympiads — профили с льготой БВИ или 100 баллов в вузе за
// последний год приёма, сильные льготы и уровни первыми.
func (s *Store) UniversityOlympiads(ctx context.Context, universityID string) ([]UniversityOlympiad, error) {
	rows, err := s.db.Query(ctx, `
		SELECT p.id, o.id, o.name, p.subject_code, sub.name, p.profile_name, p.level, b.benefit
		FROM (SELECT DISTINCT ON (olympiad_profile_id) olympiad_profile_id, benefit
		      FROM benefits
		      WHERE university_id = $1 AND benefit IN ('bvi', 'bvi_winners', 'score100')
		        AND admission_year = (SELECT max(admission_year) FROM benefits WHERE university_id = $1)
		      ORDER BY olympiad_profile_id,
		               array_position(ARRAY['bvi','bvi_winners','score100'], benefit)) b
		JOIN olympiad_profiles p ON p.id = b.olympiad_profile_id
		JOIN olympiads o ON o.id = p.olympiad_id
		JOIN subjects sub ON sub.code = p.subject_code
		ORDER BY array_position(ARRAY['bvi','bvi_winners','score100'], b.benefit),
		         o.kind = 'vsosh' DESC, p.level NULLS FIRST, o.name, p.id`, universityID)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (UniversityOlympiad, error) {
		var x UniversityOlympiad
		return x, r.Scan(&x.ProfileID, &x.OlympiadID, &x.OlympiadName, &x.SubjectCode, &x.SubjectName, &x.ProfileName,
			&x.Level, &x.Benefit)
	})
}
