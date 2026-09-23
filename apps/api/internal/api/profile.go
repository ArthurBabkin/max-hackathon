package api

import (
	"context"
	"errors"
	"net/http"
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
	Name                  string  `json:"name"`
	City                  *string `json:"city"`
	Color                 *string `json:"color"`
	BenefitOlympiadsCount int     `json:"benefit_olympiads_count"`
	IsMine                bool    `json:"is_mine"`
}

func universityItemOf(u store.University) universityItem {
	return universityItem{ID: u.ID, ShortName: u.ShortName, Name: u.Name, City: u.City,
		BenefitOlympiadsCount: u.BenefitOlympiads, IsMine: u.IsMine}
}

type profileResponse struct {
	StudentName      string           `json:"student_name"`
	Grade            int              `json:"grade"`
	RegionCode       string           `json:"region_code"`
	RegionName       string           `json:"region_name"`
	Subjects         []subjectDTO     `json:"subjects"`
	Directions       []directionItem  `json:"directions"`
	GoalStatus       string           `json:"goal_status"`
	TargetRegionCode *string          `json:"target_region_code"`
	TargetRegionName *string          `json:"target_region_name"`
	Universities     []universityItem `json:"universities"`
	OtherMemberNames []string         `json:"other_member_names"`
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
	sum := summaryOf(t)
	out := profileResponse{
		StudentName: t.StudentName, Grade: t.Grade, RegionCode: t.RegionCode, RegionName: sum.RegionName,
		Directions: sum.Directions, GoalStatus: t.GoalStatus, TargetRegionCode: t.TargetRegionCode,
		Subjects: make([]subjectDTO, len(subs)), Universities: make([]universityItem, len(unis)),
		OtherMemberNames: others,
	}
	if t.TargetRegionCode != nil {
		if reg, ok := refdata.ByCode(*t.TargetRegionCode); ok {
			out.TargetRegionName = &reg.Name
		}
	}
	for i, x := range subs {
		out.Subjects[i] = subjectDTO{Code: x.Code, Name: x.Name}
	}
	for i, u := range unis {
		out.Universities[i] = universityItemOf(u)
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
	// TargetRegionCode: "" — «не важно».
	TargetRegionCode *string  `json:"target_region_code"`
	UniversityIDs    []string `json:"university_ids"`
}

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
		req.DirectionIDs == nil && req.TargetRegionCode == nil && req.UniversityIDs == nil {
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
		reg, ok := refdata.ByCode(*req.RegionCode)
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
	if req.TargetRegionCode != nil && *req.TargetRegionCode != "" {
		if _, ok := refdata.ByCode(*req.TargetRegionCode); !ok {
			return p, badRequest("Неизвестный регион, где учиться.")
		}
	}
	p.TargetRegionCode = req.TargetRegionCode
	p.UniversityIDs = req.UniversityIDs
	return p, nil
}
