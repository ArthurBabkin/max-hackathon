package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ArthurBabkin/max-hackathon/packages/core/refdata"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

type subjectDTO struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type universityItem struct {
	ID                    string  `json:"id"`
	ShortName             string  `json:"short_name"`
	Nick                  string  `json:"nick"`
	Name                  string  `json:"name"`
	City                  *string `json:"city"`
	Color                 *string `json:"color"`
	BenefitOlympiadsCount int     `json:"benefit_olympiads_count"`
	IsMine                bool    `json:"is_mine"`
}

func universityItemOf(u store.University) universityItem {
	return universityItem{ID: u.ID, ShortName: u.ShortName, Nick: nick(u.ID, u.ShortName), Name: u.Name, City: u.City,
		BenefitOlympiadsCount: u.BenefitOlympiads, IsMine: u.IsMine}
}

type profileResponse struct {
	StudentName string          `json:"student_name"`
	Grade       int             `json:"grade"`
	RegionCode  string          `json:"region_code"`
	RegionName  string          `json:"region_name"`
	Subjects    []subjectDTO    `json:"subjects"`
	Directions  []directionItem `json:"directions"`
	GoalStatus  string          `json:"goal_status"`
	// TargetRegionCode — первое место «Где учиться»; устарело, оставлено
	// для мини-приложения до раскатки places.
	TargetRegionCode *string             `json:"target_region_code"`
	TargetRegionName *string             `json:"target_region_name"`
	Experience       *string             `json:"experience"`
	HomeCity         *string             `json:"home_city"`
	Places           []placeDTO          `json:"places"`
	Universities     []profileUniversity `json:"universities"`
	OtherMemberNames []string            `json:"other_member_names"`
}

// profileUniversity — мой вуз и на какие его направления я смотрю (F65):
// выбранные у вуза (chosen), иначе покрывающие цель (goal), иначе вуз
// целиком (university, направлений нет).
type profileUniversity struct {
	universityItem
	ChosenDirections []directionItem `json:"chosen_directions"`
	TargetBasis      string          `json:"target_basis"`
	TargetDirections []directionItem `json:"target_directions"`
}

// placeDTO — место «Где учиться»: регион целиком (city = null) или город.
type placeDTO struct {
	RegionCode string  `json:"region_code"`
	RegionName string  `json:"region_name"`
	City       *string `json:"city"`
}

func (s *Server) profileOf(ctx context.Context, m store.Member) (profileResponse, error) {
	t, err := s.store.Trajectory(ctx, m.TrajectoryID)
	if err != nil {
		return profileResponse{}, err
	}
	subs, err := s.store.TrajectorySubjects(ctx, m.TrajectoryID)
	if err != nil {
		return profileResponse{}, err
	}
	unis, err := s.store.TrajectoryUniversities(ctx, m.TrajectoryID)
	if err != nil {
		return profileResponse{}, err
	}
	others, err := s.store.OtherMemberNames(ctx, m.TrajectoryID, m.MemberID)
	if err != nil {
		return profileResponse{}, err
	}
	uniIDs := make([]string, len(unis))
	for i, u := range unis {
		uniIDs[i] = u.ID
	}
	tg, err := s.store.TargetsOf(ctx, m.TrajectoryID, uniIDs)
	if err != nil {
		return profileResponse{}, err
	}
	sum := summaryOf(t)
	out := profileResponse{
		StudentName: t.StudentName, Grade: t.Grade, RegionCode: t.RegionCode, RegionName: sum.RegionName,
		Directions: sum.Directions, GoalStatus: t.GoalStatus, HomeCity: t.HomeCity,
		Subjects: make([]subjectDTO, len(subs)), Universities: make([]profileUniversity, len(unis)),
		Places: make([]placeDTO, len(t.Places)), OtherMemberNames: others,
	}
	if t.Experience != "" {
		out.Experience = &t.Experience
	}
	for i, p := range t.Places {
		out.Places[i] = placeDTO{RegionCode: p.RegionCode, RegionName: p.RegionCode}
		if reg, ok := refdata.ByCode(p.RegionCode); ok {
			out.Places[i].RegionName = reg.Name
		}
		if p.City != "" {
			out.Places[i].City = &t.Places[i].City
		}
	}
	if len(t.Places) > 0 {
		out.TargetRegionCode, out.TargetRegionName = &out.Places[0].RegionCode, &out.Places[0].RegionName
	}
	for i, x := range subs {
		out.Subjects[i] = subjectDTO{Code: x.Code, Name: x.Name}
	}
	for i, u := range unis {
		x := tg[u.ID]
		pu := profileUniversity{universityItem: universityItemOf(u), ChosenDirections: []directionItem{},
			TargetBasis: x.Basis, TargetDirections: make([]directionItem, len(x.DirectionIDs))}
		for j, id := range x.DirectionIDs {
			pu.TargetDirections[j] = directionItem{ID: id, Name: x.DirectionNames[j]}
		}
		if x.Basis == "chosen" {
			pu.ChosenDirections = pu.TargetDirections
		}
		out.Universities[i] = pu
	}
	return out, nil
}

// getProfile — GET /profile (F49).
func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) error {
	p, err := s.profileOf(r.Context(), me(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, p)
	return nil
}

type profilePatch struct {
	StudentName  *string  `json:"student_name"`
	Grade        *int     `json:"grade"`
	RegionCode   *string  `json:"region_code"`
	SubjectCodes []string `json:"subject_codes"`
	// DirectionIDs: пустой список — «пока не решил», отсутствие — не менять.
	DirectionIDs []string `json:"direction_ids"`
	// TargetRegionCode: "" — «не важно». Устарело: вместо него places.
	TargetRegionCode *string `json:"target_region_code"`
	// Places: пустой список — «не важно», отсутствие — не менять.
	Places        []placePatch `json:"places"`
	Experience    *string      `json:"experience"`
	UniversityIDs []string     `json:"university_ids"`
}

type placePatch struct {
	RegionCode string  `json:"region_code"`
	City       *string `json:"city"`
}

// maxPlaces — сколько мест «Где учиться» можно сохранить.
const maxPlaces = 10

// patchProfile — PATCH /profile (F49): только переданные поля.
func (s *Server) patchProfile(w http.ResponseWriter, r *http.Request) error {
	m := me(r)
	if !permissionsOf(m).EditProfile {
		return forbidden("Править профиль нельзя.")
	}
	var req profilePatch
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	patch, err := validatePatch(req)
	if err != nil {
		return err
	}
	return s.applyPatch(w, r, patch)
}

type universitiesRequest struct {
	UniversityIDs []string `json:"university_ids"`
}

// putUniversities — PUT /profile/universities (F9, F25, F49).
func (s *Server) putUniversities(w http.ResponseWriter, r *http.Request) error {
	if !permissionsOf(me(r)).EditProfile {
		return forbidden("Править профиль нельзя.")
	}
	var req universitiesRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if req.UniversityIDs == nil {
		// Вузы выбирать не обязательно (F9), но поле должно быть.
		return badRequest("Не передан список вузов.")
	}
	return s.applyPatch(w, r, store.TrajectoryPatch{UniversityIDs: req.UniversityIDs})
}

type universityDirectionsRequest struct {
	DirectionIDs []string `json:"direction_ids"`
}

// putUniversityDirections — PUT /profile/universities/{id}/directions (F65):
// направления в вузе. Вуз становится моим, новые направления — в цель;
// пустой список снимает выбор, цель не трогает.
func (s *Server) putUniversityDirections(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	if !permissionsOf(m).EditProfile {
		return forbidden("Править профиль нельзя.")
	}
	var req universityDirectionsRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if req.DirectionIDs == nil {
		return badRequest("Не передан список направлений.")
	}
	err := s.store.SetUniversityDirections(ctx, m.TrajectoryID, r.PathValue("id"), m.MemberID, req.DirectionIDs)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return notFound("Вуз не найден.")
	case errors.Is(err, store.ErrNotAllowed):
		return badRequest("Такого направления в вузе нет.")
	case err != nil:
		return err
	}
	p, err := s.profileOf(ctx, m)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, p)
	return nil
}

func (s *Server) applyPatch(w http.ResponseWriter, r *http.Request, patch store.TrajectoryPatch) error {
	ctx, m := r.Context(), me(r)
	err := s.store.UpdateTrajectory(ctx, m.TrajectoryID, m.MemberID, patch)
	if errors.Is(err, store.ErrNotFound) {
		// Внешний ключ: предмета, направления или вуза с таким кодом нет.
		return badRequest("Неизвестный предмет, направление или вуз.")
	}
	if err != nil {
		return err
	}
	if patch.TZ != nil {
		// Сменился регион — 10:00 теперь по другому часовому поясу.
		s.replan(r, m.TrajectoryID)
	}
	p, err := s.profileOf(ctx, m)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, p)
	return nil
}

func validatePatch(req profilePatch) (store.TrajectoryPatch, error) {
	var p store.TrajectoryPatch
	if req.StudentName == nil && req.Grade == nil && req.RegionCode == nil && req.SubjectCodes == nil &&
		req.DirectionIDs == nil && req.TargetRegionCode == nil && req.UniversityIDs == nil &&
		req.Places == nil && req.Experience == nil {
		return p, badRequest("Нечего менять.")
	}
	if req.StudentName != nil {
		name := strings.TrimSpace(*req.StudentName)
		if n := utf8.RuneCountInString(name); n < 1 || n > 40 {
			return p, badRequest("Имя — от 1 до 40 символов.")
		}
		p.StudentName = &name
	}
	if req.Grade != nil {
		if *req.Grade < 8 || *req.Grade > 11 {
			return p, badRequest("Класс — с 8 по 11.")
		}
		p.Grade = req.Grade
	}
	if req.RegionCode != nil {
		// "" — регион не указан («Не важно» в боте): московское время.
		reg, ok := refdata.Region{TZ: refdata.DefaultTZ}, true
		if *req.RegionCode != "" {
			reg, ok = refdata.ByCode(*req.RegionCode)
		}
		if !ok {
			return p, badRequest("Неизвестный регион.")
		}
		p.RegionCode, p.TZ = &reg.Code, &reg.TZ
	}
	if req.SubjectCodes != nil {
		if len(req.SubjectCodes) == 0 {
			return p, badRequest("Выберите хотя бы один предмет.")
		}
		p.SubjectCodes = req.SubjectCodes
	}
	p.DirectionIDs = req.DirectionIDs
	if req.Experience != nil {
		if !slices.Contains([]string{"none", "school", "region"}, *req.Experience) {
			return p, badRequest("Опыт — none, school или region.")
		}
		p.Experience = req.Experience
	}
	switch {
	case req.Places != nil:
		if len(req.Places) > maxPlaces {
			return p, badRequest("Слишком много мест, где учиться.")
		}
		p.Places = []store.Place{}
		for _, x := range req.Places {
			if _, ok := refdata.ByCode(x.RegionCode); !ok {
				return p, badRequest("Неизвестный регион, где учиться.")
			}
			place := store.Place{RegionCode: x.RegionCode}
			if x.City != nil {
				place.City = strings.TrimSpace(*x.City)
				if n := utf8.RuneCountInString(place.City); n > 80 {
					return p, badRequest("Слишком длинное название города.")
				}
			}
			if !slices.Contains(p.Places, place) {
				p.Places = append(p.Places, place)
			}
		}
	case req.TargetRegionCode != nil:
		p.Places = []store.Place{}
		if *req.TargetRegionCode != "" {
			if _, ok := refdata.ByCode(*req.TargetRegionCode); !ok {
				return p, badRequest("Неизвестный регион, где учиться.")
			}
			p.Places = []store.Place{{RegionCode: *req.TargetRegionCode}}
		}
	}
	p.UniversityIDs = req.UniversityIDs
	return p, nil
}
