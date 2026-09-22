// Package pick — подбор олимпиад для траектории поверх store и core/match.
// Один расчёт на REST (/recommendations, карточки) и бота (итог онбординга,
// «Напоминать о сроках»): цифры в чате и в мини-приложении не расходятся.
package pick

import (
	"context"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/match"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

// BenefitRank — порядок льгот от сильной к слабой.
var BenefitRank = map[string]int{"bvi": 0, "bvi_winners": 1, "score100": 2, "extra_points": 3}

// BestBenefit — самая сильная льгота среди строк, "" если строк нет.
func BestBenefit(rows []store.BenefitRow) string {
	best := ""
	for _, b := range rows {
		if best == "" || BenefitRank[b.Benefit] < BenefitRank[best] {
			best = b.Benefit
		}
	}
	return best
}

// Set — всё, что нужно для скоринга набора профилей в контексте одной
// траектории: этапы, льготы в вузах ученика, трекер.
type Set struct {
	Trajectory   store.Trajectory
	Subjects     map[string]bool // предметы ученика
	Universities []store.University
	Stages       map[string][]stages.Stage
	Benefits     map[string][]store.BenefitRow // только вузы ученика, в порядке списка вузов
	Tracker      store.TrackerState
	Now          time.Time
}

func Load(ctx context.Context, st *store.Store, t store.Trajectory, subjects []string,
	profiles []store.Profile, now time.Time) (Set, error) {
	s := Set{Trajectory: t, Now: now, Subjects: map[string]bool{}, Benefits: map[string][]store.BenefitRow{}}
	for _, c := range subjects {
		s.Subjects[c] = true
	}
	ids := make([]string, len(profiles))
	for i, p := range profiles {
		ids[i] = p.ID
	}
	var err error
	if s.Stages, err = st.StagesFor(ctx, ids); err != nil {
		return s, err
	}
	if s.Tracker, err = st.TrackerState(ctx, t.ID); err != nil {
		return s, err
	}
	if s.Universities, err = st.TrajectoryUniversities(ctx, t.ID); err != nil {
		return s, err
	}
	uniIDs := make([]string, len(s.Universities))
	for i, u := range s.Universities {
		uniIDs[i] = u.ID
	}
	rows, err := st.Benefits(ctx, ids, uniIDs)
	if err != nil {
		return s, err
	}
	byPair := map[string]store.BenefitRow{}
	for _, b := range rows {
		byPair[b.ProfileID+"/"+b.UniversityID] = b
	}
	for _, id := range ids {
		for _, u := range uniIDs {
			if b, ok := byPair[id+"/"+u]; ok {
				s.Benefits[id] = append(s.Benefits[id], b)
			}
		}
	}
	return s, nil
}

func (s Set) Student() match.Student {
	return match.Student{DirectionSubjects: s.Trajectory.DirectionSubjects, RegionCode: s.Trajectory.RegionCode}
}

func (s Set) Candidate(p store.Profile) match.Candidate {
	return match.Candidate{
		ProfileID: p.ID, OlympiadID: p.OlympiadID, Kind: p.Kind, SubjectCode: p.SubjectCode, Level: p.Level,
		BestBenefit: BestBenefit(s.Benefits[p.ID]), Stages: s.Stages[p.ID], Registered: s.Tracker.Registered[p.ID],
		FinalRegionCode: p.FinalRegionCode,
	}
}

// SubjectCodes — коды предметов ученика; пустой срез, а не nil: nil в
// ProfileQuery означает «без фильтра».
func SubjectCodes(ctx context.Context, st *store.Store, trajectoryID string) ([]string, error) {
	subjects, err := st.TrajectorySubjects(ctx, trajectoryID)
	if err != nil {
		return nil, err
	}
	codes := make([]string, len(subjects))
	for i, sub := range subjects {
		codes[i] = sub.Code
	}
	return codes, nil
}

// Result — подбор: перечень и ВсОШ, «вне перечня» отдельно.
type Result struct {
	Items    []match.Result
	Outside  []match.Result
	Profiles map[string]store.Profile
	Set      Set
}

// Recommend — подбор F13–F16: кандидаты — профили по предметам ученика, в
// которых участвует его класс; скоринг и отбор — в core/match.
func Recommend(ctx context.Context, st *store.Store, t store.Trajectory, now time.Time, filter string) (Result, error) {
	codes, err := SubjectCodes(ctx, st, t.ID)
	if err != nil {
		return Result{}, err
	}
	profiles, err := st.Profiles(ctx, store.ProfileQuery{SubjectCodes: codes, Grade: t.Grade})
	if err != nil {
		return Result{}, err
	}
	set, err := Load(ctx, st, t, codes, profiles, now)
	if err != nil {
		return Result{}, err
	}
	res := Result{Profiles: make(map[string]store.Profile, len(profiles)), Set: set}
	cands := make([]match.Candidate, len(profiles))
	for i, p := range profiles {
		res.Profiles[p.ID] = p
		cands[i] = set.Candidate(p)
	}
	res.Items, res.Outside = match.Recommend(cands, set.Student(), match.DefaultWeights, now, filter)
	return res, nil
}
