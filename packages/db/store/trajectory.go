package store

import (
	"context"
	"fmt"
)

// NewTrajectory — всё, что собирает онбординг, чтобы создать траекторию.
type NewTrajectory struct {
	CreatorUserID string
	Role          string // роль создателя: kid | parent
	StudentName   string
	Grade         int
	RegionCode    string
	TZ            string
	// DirectionIDs — направления в порядке выбора; пусто — «пока не решил».
	DirectionIDs []string
	// GoalStatus — "" значит по направлениям (GoalStatusOf); suggested —
	// направления предложил бот по интересам.
	GoalStatus string
	// Places — где ученик хочет учиться; пусто — не важно.
	Places        []Place
	Experience    string // none | school | region | "" — не спрашивали
	HomeCity      string
	GoalByKid     bool
	SourcePayload *string
	SubjectCodes  []string
	UniversityIDs []string
}

// GoalStatusOf — known, если направления выбраны, иначе exploring.
func GoalStatusOf(directionIDs []string) string {
	if len(directionIDs) == 0 {
		return "exploring"
	}
	return "known"
}

// CreateTrajectory создаёт траекторию целиком в одной транзакции: сама
// траектория, создатель, предметы, вузы, запись аудита. Частично заполненной
// траектории в базе не бывает — у неё NOT NULL на имени, классе и регионе.
func (s *Store) CreateTrajectory(ctx context.Context, n NewTrajectory) (Member, error) {
	var out Member
	err := s.Tx(ctx, func(tx *Store) error {
		var tid string
		goal := n.GoalStatus
		if goal == "" || len(n.DirectionIDs) == 0 {
			goal = GoalStatusOf(n.DirectionIDs)
		}
		if err := tx.db.QueryRow(ctx, `
			INSERT INTO trajectories (student_name, grade, region_code, tz, goal_status, source_payload,
			                          experience, home_city, goal_by_kid)
			VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), $9) RETURNING id::text`,
			n.StudentName, n.Grade, n.RegionCode, n.TZ, goal, n.SourcePayload, n.Experience, n.HomeCity, n.GoalByKid,
		).Scan(&tid); err != nil {
			return wrap(err)
		}
		mid, err := tx.addMember(ctx, tid, n.CreatorUserID, n.Role, true)
		if err != nil {
			return err
		}
		if err := tx.ReplaceSubjects(ctx, tid, n.SubjectCodes); err != nil {
			return err
		}
		if err := tx.ReplaceDirections(ctx, tid, n.DirectionIDs); err != nil {
			return err
		}
		if err := tx.ReplaceUniversities(ctx, tid, n.UniversityIDs); err != nil {
			return err
		}
		if err := tx.ReplacePlaces(ctx, tid, n.Places); err != nil {
			return err
		}
		if err := tx.Audit(ctx, mid, "create", "trajectory", tid); err != nil {
			return err
		}
		out, err = tx.ActiveMember(ctx, mid)
		return err
	})
	return out, err
}

// AddMember подключает пользователя к траектории. Нарушение
// members_single_active_kid_uniq означает, что ученик в траектории уже есть
// (F42), members_active_uniq — что пользователь уже участник.
func (s *Store) AddMember(ctx context.Context, trajectoryID, userID, role string) (Member, error) {
	mid, err := s.addMember(ctx, trajectoryID, userID, role, false)
	if err != nil {
		return Member{}, err
	}
	return s.ActiveMember(ctx, mid)
}

func (s *Store) addMember(ctx context.Context, trajectoryID, userID, role string, creator bool) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `
		INSERT INTO members (trajectory_id, user_id, role, is_creator)
		VALUES ($1, $2, $3, $4) RETURNING id::text`, trajectoryID, userID, role, creator).Scan(&id)
	return id, wrap(err)
}

// ReplaceSubjects заменяет предметы траектории. Пустой список запрещён
// правилом F7 — проверка на стороне сценария, здесь только запись.
func (s *Store) ReplaceSubjects(ctx context.Context, trajectoryID string, codes []string) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM trajectory_subjects WHERE trajectory_id = $1`, trajectoryID); err != nil {
		return wrap(err)
	}
	if len(codes) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO trajectory_subjects (trajectory_id, subject_code)
		SELECT $1, unnest($2::text[]) ON CONFLICT DO NOTHING`, trajectoryID, codes)
	return wrap(err)
}

// ReplaceDirections заменяет направления траектории; порядок списка —
// порядок выбора. Снятое с цели уходит и из выбора в вузах: выбор у вуза —
// часть цели, иначе «мои направления» в каталоге и в настройках расходятся.
func (s *Store) ReplaceDirections(ctx context.Context, trajectoryID string, ids []string) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM trajectory_directions WHERE trajectory_id = $1`, trajectoryID); err != nil {
		return wrap(err)
	}
	if _, err := s.db.Exec(ctx, `
		DELETE FROM trajectory_university_directions WHERE trajectory_id = $1 AND direction_id <> ALL($2::text[])`,
		trajectoryID, nonNil(ids)); err != nil {
		return wrap(err)
	}
	if len(ids) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO trajectory_directions (trajectory_id, direction_id, position)
		SELECT $1, x.id, x.n - 1 FROM unnest($2::text[]) WITH ORDINALITY AS x(id, n)
		ON CONFLICT DO NOTHING`, trajectoryID, ids)
	return wrap(err)
}

// ReplaceUniversities заменяет вузы траектории по разнице: убранные уходят
// вместе с выбранными в них направлениями, оставшиеся не трогаются — иначе
// каждое сохранение профиля стирало бы выбор направлений.
func (s *Store) ReplaceUniversities(ctx context.Context, trajectoryID string, ids []string) error {
	if _, err := s.db.Exec(ctx, `
		DELETE FROM trajectory_universities WHERE trajectory_id = $1 AND university_id <> ALL($2::text[])`,
		trajectoryID, nonNil(ids)); err != nil {
		return wrap(err)
	}
	if len(ids) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO trajectory_universities (trajectory_id, university_id)
		SELECT $1, unnest($2::text[]) ON CONFLICT DO NOTHING`, trajectoryID, ids)
	return wrap(err)
}

// ReplacePlaces заменяет места «Где учиться»; порядок списка — порядок
// выбора. Пустой список — «не важно».
func (s *Store) ReplacePlaces(ctx context.Context, trajectoryID string, places []Place) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM trajectory_places WHERE trajectory_id = $1`, trajectoryID); err != nil {
		return wrap(err)
	}
	if len(places) == 0 {
		return nil
	}
	regions := make([]string, len(places))
	cities := make([]*string, len(places))
	for i, p := range places {
		regions[i] = p.RegionCode
		if p.City != "" {
			cities[i] = &places[i].City
		}
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO trajectory_places (trajectory_id, position, region_code, city)
		SELECT $1, x.n - 1, x.region, x.city FROM unnest($2::text[], $3::text[]) WITH ORDINALITY AS x(region, city, n)`,
		trajectoryID, regions, cities)
	return wrap(err)
}

// Audit дописывает строку в журнал. Журнал только добавляется.
func (s *Store) Audit(ctx context.Context, memberID, action, entity, entityID string) error {
	var mid any
	if memberID != "" {
		mid = memberID
	}
	_, err := s.db.Exec(ctx, `INSERT INTO audit_log (member_id, action, entity, entity_id) VALUES ($1, $2, $3, $4)`,
		mid, action, entity, entityID)
	if err != nil {
		return fmt.Errorf("аудит %s %s: %w", action, entity, wrap(err))
	}
	return nil
}
