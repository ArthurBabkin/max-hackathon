package store

import (
	"context"
	"errors"
	"slices"
	"sort"

	"github.com/ArthurBabkin/max-hackathon/packages/core/targets"
)

// ErrNotAllowed — значение не из допустимых: например, направления нет в вузе.
var ErrNotAllowed = errors.New("store: недопустимое значение")

// UniversityDirection — направление подготовки в карточке вуза.
type UniversityDirection struct {
	ID           string
	Code         string
	Name         string
	Status       string // offered — льготы проверены, to_check — уточняются
	Programs     int
	BudgetPlaces *int
	ProgramNames []string
	// BenefitOlympiads — олимпиады с льготой на направлении (последний год).
	BenefitOlympiads int
	IsMine           bool // ученик выбрал направление в этом вузе
	IsGoal           bool // направление покрывает цель ученика
}

// goalCodes — коды направлений цели траектории.
func (s *Store) goalCodes(ctx context.Context, trajectoryID string) ([]string, error) {
	rows, err := s.db.Query(ctx, `
		SELECT d.code FROM trajectory_directions td JOIN directions d ON d.id = td.direction_id
		WHERE td.trajectory_id = $1 ORDER BY td.position`, trajectoryID)
	if err != nil {
		return nil, wrap(err)
	}
	return collect(rows, func(r rowScanner) (string, error) {
		var c string
		return c, r.Scan(&c)
	})
}

// UniversityDirections — направления вуза: выбранные учеником первыми, за
// ними покрывающие цель, дальше — где больше олимпиад с льготой.
func (s *Store) UniversityDirections(ctx context.Context, trajectoryID, universityID string) ([]UniversityDirection, error) {
	goal, err := s.goalCodes(ctx, trajectoryID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT d.id, d.code, d.name, ud.status, ud.programs, ud.budget_places, ud.program_names,
		       (SELECT count(DISTINCT p.olympiad_id)
		        FROM direction_benefits db JOIN olympiad_profiles p ON p.id = db.olympiad_profile_id
		        WHERE db.university_id = ud.university_id AND db.direction_id = ud.direction_id
		          AND db.admission_year = (SELECT max(admission_year) FROM direction_benefits x
		                                   WHERE x.university_id = ud.university_id)),
		       EXISTS (SELECT 1 FROM trajectory_university_directions tud
		               WHERE tud.trajectory_id = $1 AND tud.university_id = ud.university_id
		                 AND tud.direction_id = ud.direction_id)
		FROM university_directions ud JOIN directions d ON d.id = ud.direction_id
		WHERE ud.university_id = $2
		ORDER BY d.name, d.code`, trajectoryID, universityID)
	if err != nil {
		return nil, wrap(err)
	}
	out, err := collect(rows, func(r rowScanner) (UniversityDirection, error) {
		var x UniversityDirection
		var programs int16
		err := r.Scan(&x.ID, &x.Code, &x.Name, &x.Status, &programs, &x.BudgetPlaces, &x.ProgramNames,
			&x.BenefitOlympiads, &x.IsMine)
		x.Programs = int(programs)
		return x, err
	})
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].IsGoal = slices.ContainsFunc(goal, func(g string) bool { return targets.Covers(out[i].Code, g) })
	}
	rank := func(d UniversityDirection) int {
		switch {
		case d.IsMine:
			return 0
		case d.IsGoal:
			return 1
		}
		return 2
	}
	sort.SliceStable(out, func(i, j int) bool {
		if a, b := rank(out[i]), rank(out[j]); a != b {
			return a < b
		}
		return out[i].BenefitOlympiads > out[j].BenefitOlympiads
	})
	return out, nil
}

// SetUniversityDirections заменяет выбор направлений в вузе одной
// транзакцией. Вуз становится «моим», новые направления дописываются в
// цель (она становится осознанной — known). Пустой список снимает выбор;
// цель при этом не меняется. Направления не из вуза — ErrNotAllowed.
func (s *Store) SetUniversityDirections(ctx context.Context, trajectoryID, universityID, memberID string, directionIDs []string) error {
	ids := slices.Compact(slices.Sorted(slices.Values(directionIDs)))
	return s.Tx(ctx, func(tx *Store) error {
		// Выбор и цель правят двое в семье: запросы траектории — по очереди,
		// иначе второй споткнётся о вставку первого, а позиции в цели совпадут.
		if _, err := tx.db.Exec(ctx, `SELECT 1 FROM trajectories WHERE id = $1 FOR UPDATE`, trajectoryID); err != nil {
			return wrap(err)
		}
		var exists bool
		var offered int
		if err := tx.db.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM universities WHERE id = $1),
			       (SELECT count(*) FROM university_directions WHERE university_id = $1 AND direction_id = ANY($2))`,
			universityID, ids).Scan(&exists, &offered); err != nil {
			return wrap(err)
		}
		if !exists {
			return ErrNotFound
		}
		if offered != len(ids) {
			return ErrNotAllowed
		}
		if _, err := tx.db.Exec(ctx, `
			INSERT INTO trajectory_universities (trajectory_id, university_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, trajectoryID, universityID); err != nil {
			return wrap(err)
		}
		if _, err := tx.db.Exec(ctx, `
			DELETE FROM trajectory_university_directions WHERE trajectory_id = $1 AND university_id = $2`,
			trajectoryID, universityID); err != nil {
			return wrap(err)
		}
		if len(ids) > 0 {
			// Порядок в цели — как прислали: сначала то, что выбрали первым.
			if _, err := tx.db.Exec(ctx, `
				INSERT INTO trajectory_university_directions (trajectory_id, university_id, direction_id)
				SELECT $1, $2, unnest($3::text[])`, trajectoryID, universityID, ids); err != nil {
				return wrap(err)
			}
			tag, err := tx.db.Exec(ctx, `
				INSERT INTO trajectory_directions (trajectory_id, direction_id, position)
				SELECT $1, x.id, (SELECT COALESCE(max(position) + 1, 0) FROM trajectory_directions WHERE trajectory_id = $1) + x.n - 1
				FROM unnest($2::text[]) WITH ORDINALITY AS x(id, n)
				WHERE NOT EXISTS (SELECT 1 FROM trajectory_directions td WHERE td.trajectory_id = $1 AND td.direction_id = x.id)
				ON CONFLICT DO NOTHING`, trajectoryID, slices.DeleteFunc(slices.Clone(directionIDs), func(id string) bool {
				return !slices.Contains(ids, id)
			}))
			if err != nil {
				return wrap(err)
			}
			if tag.RowsAffected() > 0 {
				if _, err := tx.db.Exec(ctx, `
					UPDATE trajectories SET goal_status = 'known', updated_at = now()
					WHERE id = $1 AND goal_status <> 'known'`, trajectoryID); err != nil {
					return wrap(err)
				}
			}
		}
		return tx.Audit(ctx, memberID, "university_directions", "trajectory", trajectoryID)
	})
}

// targetsOf — цель траектории в каждом из вузов (правило core/targets) и
// названия направлений.
func (s *Store) targetsOf(ctx context.Context, trajectoryID string, universityIDs []string) (map[string]targets.Target, map[string]string, error) {
	goal, err := s.goalCodes(ctx, trajectoryID)
	if err != nil {
		return nil, nil, err
	}
	return s.resolveTargets(ctx, trajectoryID, universityIDs, goal)
}

// targetsOn — цель «направления directionIDs» в каждом из вузов, без выбора
// ученика в вузе.
func (s *Store) targetsOn(ctx context.Context, directionIDs, universityIDs []string) (map[string]targets.Target, map[string]string, error) {
	var codes []string
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(array_agg(code), '{}') FROM directions WHERE id = ANY($1)`,
		directionIDs).Scan(&codes); err != nil {
		return nil, nil, wrap(err)
	}
	return s.resolveTargets(ctx, nil, universityIDs, codes)
}

// resolveTargets — цель по кодам goal в каждом из вузов; trajectory — чей
// выбор направлений в вузах учитывать (nil — ничей).
func (s *Store) resolveTargets(ctx context.Context, trajectory any, universityIDs, goal []string) (map[string]targets.Target, map[string]string, error) {
	rows, err := s.db.Query(ctx, `
		SELECT ud.university_id, d.id, d.code, d.name, ud.status,
		       EXISTS (SELECT 1 FROM trajectory_university_directions tud
		               WHERE tud.trajectory_id = $1 AND tud.university_id = ud.university_id
		                 AND tud.direction_id = ud.direction_id)
		FROM university_directions ud JOIN directions d ON d.id = ud.direction_id
		WHERE ud.university_id = ANY($2)
		ORDER BY ud.university_id, d.name, d.code`, trajectory, universityIDs)
	if err != nil {
		return nil, nil, wrap(err)
	}
	defer rows.Close()
	offered := map[string][]targets.Offered{}
	chosen := map[string][]string{}
	names := map[string]string{}
	for rows.Next() {
		var uni, id, code, name, status string
		var mine bool
		if err := rows.Scan(&uni, &id, &code, &name, &status, &mine); err != nil {
			return nil, nil, wrap(err)
		}
		offered[uni] = append(offered[uni], targets.Offered{DirectionID: id, Code: code, Status: status})
		if mine {
			chosen[uni] = append(chosen[uni], id)
		}
		names[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, nil, wrap(err)
	}
	out := make(map[string]targets.Target, len(universityIDs))
	for _, u := range universityIDs {
		out[u] = targets.Resolve(u, offered[u], chosen[u], goal)
	}
	return out, names, nil
}

// UniversityTarget — цель ученика в вузе (core/targets) с названиями
// направлений: на что смотреть льготы.
type UniversityTarget struct {
	Basis          string
	DirectionIDs   []string
	DirectionNames []string
	Unverified     bool
}

// TargetsOf — цель ученика в каждом из вузов.
func (s *Store) TargetsOf(ctx context.Context, trajectoryID string, universityIDs []string) (map[string]UniversityTarget, error) {
	tg, names, err := s.targetsOf(ctx, trajectoryID, universityIDs)
	if err != nil {
		return nil, err
	}
	return universityTargets(tg, names), nil
}

// TargetsOn — цель «направления directionIDs» в каждом из вузов: вопрос про
// направление смотрит льготы на него, а не на цель ученика. Вуз без
// направления — Basis university.
func (s *Store) TargetsOn(ctx context.Context, directionIDs, universityIDs []string) (map[string]UniversityTarget, error) {
	tg, names, err := s.targetsOn(ctx, directionIDs, universityIDs)
	if err != nil {
		return nil, err
	}
	return universityTargets(tg, names), nil
}

func universityTargets(tg map[string]targets.Target, names map[string]string) map[string]UniversityTarget {
	out := make(map[string]UniversityTarget, len(tg))
	for u, t := range tg {
		x := UniversityTarget{Basis: string(t.Basis), DirectionIDs: t.DirectionIDs, Unverified: t.Unverified}
		for _, id := range t.DirectionIDs {
			x.DirectionNames = append(x.DirectionNames, names[id])
		}
		out[u] = x
	}
	return out
}

// Coverage — на скольких направлениях вуза (из Total) олимпиада даёт льготу.
type Coverage struct{ Count, Total int }

// DirectionCoverage — по паре «профиль/вуз» покрытие направлений вуза
// льготами профиля за последний год приёма вуза.
func (s *Store) DirectionCoverage(ctx context.Context, profileIDs, universityIDs []string) (map[string]Coverage, error) {
	rows, err := s.db.Query(ctx, `
		WITH last AS (
			SELECT university_id, max(admission_year) AS year FROM direction_benefits
			WHERE university_id = ANY($2) GROUP BY university_id
		), total AS (
			SELECT university_id, count(*) AS n FROM university_directions
			WHERE university_id = ANY($2) GROUP BY university_id
		)
		SELECT db.olympiad_profile_id, db.university_id, count(DISTINCT db.direction_id), total.n
		FROM direction_benefits db
		JOIN last ON last.university_id = db.university_id AND last.year = db.admission_year
		JOIN total ON total.university_id = db.university_id
		WHERE db.olympiad_profile_id = ANY($1)
		GROUP BY 1, 2, 4`, profileIDs, universityIDs)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := map[string]Coverage{}
	for rows.Next() {
		var p, u string
		var c Coverage
		if err := rows.Scan(&p, &u, &c.Count, &c.Total); err != nil {
			return nil, wrap(err)
		}
		out[p+"/"+u] = c
	}
	return out, wrap(rows.Err())
}

// directionBest — лучшая льгота профиля на направлениях цели в одном вузе
// и более слабые — на остальных.
type directionBest struct {
	row    BenefitRow
	names  []string
	others []DirectionBenefit
}

// TargetBenefits — льготы профилей в вузах с учётом целей ученика: на
// выбранные в вузе направления, иначе на направления цели, иначе — вуз
// целиком (как Benefits). Нет льготы на мои направления — строки нет. Если
// льготы на направления цели ещё уточняются, строка — льгота вуза с
// пометкой Unverified. Баллы вне перечня от направления не зависят.
func (s *Store) TargetBenefits(ctx context.Context, trajectoryID string, profileIDs, universityIDs []string) ([]BenefitRow, error) {
	base, err := s.Benefits(ctx, profileIDs, universityIDs)
	if err != nil || len(base) == 0 {
		return base, err
	}
	tg, names, err := s.targetsOf(ctx, trajectoryID, universityIDs)
	if err != nil {
		return nil, err
	}
	return s.benefitsOn(ctx, base, tg, names, profileIDs)
}

// BenefitsOn — льготы профилей в вузах на направления directionIDs (как
// TargetBenefits с целью TargetsOn). В вузе без этих направлений льготы на
// них нет — строк нет.
func (s *Store) BenefitsOn(ctx context.Context, directionIDs, profileIDs, universityIDs []string) ([]BenefitRow, error) {
	base, err := s.Benefits(ctx, profileIDs, universityIDs)
	if err != nil || len(base) == 0 {
		return base, err
	}
	tg, names, err := s.targetsOn(ctx, directionIDs, universityIDs)
	if err != nil {
		return nil, err
	}
	base = slices.DeleteFunc(base, func(b BenefitRow) bool { return tg[b.UniversityID].Basis == targets.University })
	return s.benefitsOn(ctx, base, tg, names, profileIDs)
}

// benefitsOn — строки base (льготы вузов целиком) на цели tg.
func (s *Store) benefitsOn(ctx context.Context, base []BenefitRow, tg map[string]targets.Target, names map[string]string, profileIDs []string) ([]BenefitRow, error) {
	var pairUnis, pairDirs []string
	for u, t := range tg {
		if t.Basis == targets.University || t.Unverified {
			continue
		}
		for _, d := range t.DirectionIDs {
			pairUnis, pairDirs = append(pairUnis, u), append(pairDirs, d)
		}
	}
	best, err := s.directionBenefits(ctx, profileIDs, pairUnis, pairDirs, tg, names)
	if err != nil {
		return nil, err
	}

	out := make([]BenefitRow, 0, len(base))
	for _, b := range base {
		t := tg[b.UniversityID]
		b.Basis = string(targets.University)
		switch {
		case t.Basis == targets.University || b.Benefit == "extra_points":
		case t.Unverified:
			b.Basis, b.Unverified = string(t.Basis), true
			for _, id := range t.DirectionIDs {
				b.DirectionNames = append(b.DirectionNames, names[id])
			}
		default:
			d, ok := best[b.ProfileID+"/"+b.UniversityID]
			if !ok {
				continue
			}
			b.Benefit, b.EgeMin, b.DiplomaGrades, b.Note = d.row.Benefit, d.row.EgeMin, d.row.DiplomaGrades, d.row.Note
			b.AdmissionYear, b.Varies, b.ExtraPoints = d.row.AdmissionYear, d.row.Varies, nil
			if d.row.Source != nil {
				b.Source = d.row.Source
			}
			b.Basis, b.DirectionNames, b.OtherDirections = string(t.Basis), d.names, d.others
		}
		out = append(out, b)
	}
	return out, nil
}

// directionBenefits — по паре (профиль, вуз) лучшая льгота среди
// направлений цели за последний год и направления, на которые она даётся,
// в порядке цели.
func (s *Store) directionBenefits(ctx context.Context, profileIDs, unis, dirs []string,
	tg map[string]targets.Target, names map[string]string) (map[string]directionBest, error) {
	out := map[string]directionBest{}
	if len(unis) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT ON (db.olympiad_profile_id, db.university_id, db.direction_id)
		       db.olympiad_profile_id, db.university_id, db.direction_id, db.benefit, db.ege_min,
		       db.diploma_grades, db.note, db.varies, db.admission_year,
		       src.id, src.kind, src.title, src.url, src.verified_at
		FROM direction_benefits db
		JOIN unnest($2::text[], $3::text[]) AS t(u, d) ON t.u = db.university_id AND t.d = db.direction_id
		LEFT JOIN sources src ON src.id = db.source_id
		WHERE db.olympiad_profile_id = ANY($1)
		ORDER BY db.olympiad_profile_id, db.university_id, db.direction_id, db.admission_year DESC`,
		profileIDs, unis, dirs)
	if err != nil {
		return nil, wrap(err)
	}
	type dirRow struct {
		row BenefitRow
		dir string
	}
	found, err := collect(rows, func(r rowScanner) (dirRow, error) {
		var x dirRow
		var src sourceCols
		var year int16
		err := r.Scan(&x.row.ProfileID, &x.row.UniversityID, &x.dir, &x.row.Benefit, &x.row.EgeMin,
			&x.row.DiplomaGrades, &x.row.Note, &x.row.Varies, &year,
			&src.id, &src.kind, &src.title, &src.url, &src.verified)
		x.row.AdmissionYear, x.row.Source = int(year), src.source()
		return x, err
	})
	if err != nil {
		return nil, err
	}
	rank := map[string]int{"bvi": 0, "bvi_winners": 1, "score100": 2}
	byPair := map[string][]dirRow{}
	for _, x := range found {
		key := x.row.ProfileID + "/" + x.row.UniversityID
		byPair[key] = append(byPair[key], x)
	}
	for key, xs := range byPair {
		order := tg[xs[0].row.UniversityID].DirectionIDs
		sort.SliceStable(xs, func(i, j int) bool {
			if a, b := rank[xs[i].row.Benefit], rank[xs[j].row.Benefit]; a != b {
				return a < b
			}
			return slices.Index(order, xs[i].dir) < slices.Index(order, xs[j].dir)
		})
		d := directionBest{row: xs[0].row}
		var same []BenefitRow
		for _, x := range xs {
			if x.row.Benefit == d.row.Benefit {
				d.names = append(d.names, names[x.dir])
				same = append(same, x.row)
				continue
			}
			// Одна льгота, но призёру разное — разные строки.
			if n := len(d.others); n == 0 || d.others[n-1].Benefit != x.row.Benefit ||
				grantNote(d.others[n-1].Note) != grantNote(x.row.Note) {
				d.others = append(d.others, DirectionBenefit{Benefit: x.row.Benefit, Note: x.row.Note})
			}
			last := &d.others[len(d.others)-1]
			last.Names = append(last.Names, names[x.dir])
		}
		if len(same) > 1 {
			d.row = mergeDirections(same, d.names)
		}
		out[key] = d
	}
	return out, nil
}

// DirectionMatch — чем вуз подходит под направление из фильтра каталога (F67).
type DirectionMatch struct {
	DirectionIDs []string // направления вуза, покрывающие искомое (targets.Covers)
	Olympiads    int      // олимпиады с льготой на них, последний год приёма
	Unverified   bool     // льготы на всех таких направлениях ещё проверяются
}

// UniversitiesByDirection — каталог вузов (поиск и город — как в
// Universities), где есть направление, покрывающее directionID: сначала
// где больше олимпиад с льготой на нём, вузы с непроверенными льготами —
// в конце. Нет направления — ErrNotFound.
func (s *Store) UniversitiesByDirection(ctx context.Context, trajectoryID, search, city, directionID string) ([]University, map[string]DirectionMatch, error) {
	var code string
	if err := s.db.QueryRow(ctx, `SELECT code FROM directions WHERE id = $1`, directionID).Scan(&code); err != nil {
		return nil, nil, wrap(err)
	}
	rows, err := s.db.Query(ctx, `
		SELECT ud.university_id, ud.direction_id, d.code, ud.status
		FROM university_directions ud JOIN directions d ON d.id = ud.direction_id
		ORDER BY ud.university_id, d.code`)
	if err != nil {
		return nil, nil, wrap(err)
	}
	type offer struct{ uni, dir, code, status string }
	offers, err := collect(rows, func(r rowScanner) (offer, error) {
		var o offer
		return o, r.Scan(&o.uni, &o.dir, &o.code, &o.status)
	})
	if err != nil {
		return nil, nil, err
	}
	matches := map[string]DirectionMatch{}
	var pairUnis, pairDirs []string
	for _, o := range offers {
		if !targets.Covers(o.code, code) {
			continue
		}
		m, seen := matches[o.uni]
		if !seen {
			m.Unverified = true
		}
		m.DirectionIDs = append(m.DirectionIDs, o.dir)
		if o.status != "to_check" {
			m.Unverified = false
			pairUnis, pairDirs = append(pairUnis, o.uni), append(pairDirs, o.dir)
		}
		matches[o.uni] = m
	}

	rows, err = s.db.Query(ctx, `
		SELECT db.university_id, count(DISTINCT p.olympiad_id)
		FROM direction_benefits db JOIN olympiad_profiles p ON p.id = db.olympiad_profile_id
		WHERE (db.university_id, db.direction_id) IN (SELECT * FROM unnest($1::text[], $2::text[]))
		  AND db.admission_year = (SELECT max(admission_year) FROM direction_benefits x
		                           WHERE x.university_id = db.university_id)
		GROUP BY 1`, pairUnis, pairDirs)
	if err != nil {
		return nil, nil, wrap(err)
	}
	type count struct {
		uni string
		n   int
	}
	counts, err := collect(rows, func(r rowScanner) (count, error) {
		var c count
		return c, r.Scan(&c.uni, &c.n)
	})
	if err != nil {
		return nil, nil, err
	}
	for _, c := range counts {
		m := matches[c.uni]
		m.Olympiads = c.n
		matches[c.uni] = m
	}

	all, err := s.Universities(ctx, trajectoryID, search, city)
	if err != nil {
		return nil, nil, err
	}
	out := slices.DeleteFunc(all, func(u University) bool { _, ok := matches[u.ID]; return !ok })
	sort.SliceStable(out, func(i, j int) bool {
		a, b := matches[out[i].ID], matches[out[j].ID]
		if a.Unverified != b.Unverified {
			return b.Unverified
		}
		return a.Olympiads > b.Olympiads
	})
	return out, matches, nil
}
