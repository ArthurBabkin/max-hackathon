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
	// Potential — льготы профилей в вузах с направлениями ученика в
	// выбранных местах; заполняется, только если вузы не выбраны (SPEC 2.3).
	Potential map[string][]store.BenefitRow
	Tracker   store.TrackerState
	Now       time.Time
}

// potentialLimit — сколько вузов берётся для «потенциальной» льготы:
// заведомо больше, чем есть в базе.
const potentialLimit = 1000

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
	if len(s.Universities) == 0 && len(ids) > 0 {
		if s.Potential, err = potential(ctx, st, t, ids); err != nil {
			return s, err
		}
	}
	return s, nil
}

// potential — лучшая льгота каждого профиля в вузах, где есть хотя бы одно
// направление траектории (у exploring — в любых), в выбранных местах (нет
// мест — везде). Так без выбранных вузов фактор Benefit не обнуляется.
func potential(ctx context.Context, st *store.Store, t store.Trajectory, profileIDs []string) (map[string][]store.BenefitRow, error) {
	dirIDs := make([]string, len(t.Directions))
	for i, d := range t.Directions {
		dirIDs[i] = d.ID
	}
	unis, err := st.SuggestUniversities(ctx, dirIDs, t.Places, potentialLimit, 0)
	if err != nil || len(unis) == 0 {
		return nil, err
	}
	uniIDs := make([]string, len(unis))
	for i, u := range unis {
		uniIDs[i] = u.ID
	}
	rows, err := st.Benefits(ctx, profileIDs, uniIDs)
	if err != nil {
		return nil, err
	}
	out := map[string][]store.BenefitRow{}
	for _, b := range rows {
		out[b.ProfileID] = append(out[b.ProfileID], b)
	}
	return out, nil
}

func (s Set) Student() match.Student {
	t := s.Trajectory
	return match.Student{DirectionSubjects: t.DirectionSubjects, RegionCode: t.RegionCode,
		Experience: t.Experience, Grade: t.Grade}
}

func (s Set) Candidate(p store.Profile) match.Candidate {
	c := match.Candidate{
		ProfileID: p.ID, OlympiadID: p.OlympiadID, Kind: p.Kind, SubjectCode: p.SubjectCode, Level: p.Level,
		BestBenefit: BestBenefit(s.Benefits[p.ID]), Stages: s.Stages[p.ID], Registered: s.Tracker.Registered[p.ID],
		FinalRegionCode: p.FinalRegionCode,
	}
	if best := BestBenefit(s.Potential[p.ID]); c.BestBenefit == "" && best != "" {
		c.BestBenefit, c.PotentialBenefit = best, true
	}
	return c
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

// Result — подбор: перечень и ВсОШ, «вне перечня» отдельно. More,
// Tracked, Proposed и State — для экрана C3/C7 (см. match.Picked).
type Result struct {
	Items    []match.Result
	More     []match.Result
	Outside  []match.Result
	Tracked  int
	Proposed int
	State    string
	Profiles map[string]store.Profile
	Set      Set
}

// Options — фильтр подбора и HideTracked: спрятать олимпиады, которые уже
// в трекере или ждут ответа на предложение (только в приложении; бот
// показывает подбор целиком).
type Options struct {
	Filter      string
	HideTracked bool
}

// Recommend — подбор F13–F16 без сокрытия добавленного: итог онбординга и
// «Напоминать о сроках» в боте.
func Recommend(ctx context.Context, st *store.Store, t store.Trajectory, now time.Time, filter string) (Result, error) {
	return Pick(ctx, st, t, now, Options{Filter: filter})
}

// Pick — подбор: кандидаты — профили по предметам ученика, в которых
// участвует его класс; что подходит и в каком порядке — в core/match.
func Pick(ctx context.Context, st *store.Store, t store.Trajectory, now time.Time, o Options) (Result, error) {
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
	mo := match.Options{Filter: o.Filter}
	if o.HideTracked {
		mo.Tracked, mo.Proposed = set.Tracker.TrackedOlympiads, set.Tracker.PendingOlympiads
	}
	p := match.Pick(cands, set.Student(), match.DefaultWeights, now, mo)
	res.Items, res.More, res.Outside, res.Tracked, res.Proposed, res.State =
		p.Items, p.More, p.Outside, p.Tracked, p.Proposed, p.State()
	return res, nil
}
