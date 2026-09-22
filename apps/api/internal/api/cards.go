package api

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ArthurBabkin/max-hackathon/packages/core/match"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

type olympiadCard struct {
	badge
	OlympiadProfileID string     `json:"olympiad_profile_id"`
	OlympiadID        string     `json:"olympiad_id"`
	Name              string     `json:"name"`
	Organizer         *string    `json:"organizer"`
	Kind              string     `json:"kind"`
	Level             *string    `json:"level"`
	SubjectCode       string     `json:"subject_code"`
	SubjectName       string     `json:"subject_name"`
	Format            *string    `json:"format"`
	IsOnline          bool       `json:"is_online"`
	FinalCity         *string    `json:"final_city"`
	DeadlineAt        *time.Time `json:"deadline_at"`
	NextStageTitle    *string    `json:"next_stage_title"`
	BenefitsSummary   string     `json:"benefits_summary"`
	Reason            string     `json:"reason"`
	InTracker         bool       `json:"in_tracker"`
	ProposalStatus    *string    `json:"proposal_status"`
}

// benefitLabels — подписи льгот, те же, что BENEFIT_LABELS во фронте.
var benefitLabels = map[string]string{
	"bvi": "БВИ", "score100": "100 баллов", "bvi_winners": "БВИ победителям", "extra_points": "доп. баллы",
}

var benefitRank = map[string]int{"bvi": 0, "bvi_winners": 1, "score100": 2, "extra_points": 3}

// universityNicks — как вуз называют в строке льгот, если аббревиатура
// из справочника ничего не скажет школьнику.
var universityNicks = map[string]string{"innopolis": "Иннополис", "sechenov": "Сеченовский"}

func nick(universityID, shortName string) string {
	if n, ok := universityNicks[universityID]; ok {
		return n
	}
	return shortName
}

// cardSet — всё, что нужно карточкам набора профилей в контексте одной
// траектории: этапы, льготы в вузах ученика, трекер и голос читающего.
type cardSet struct {
	t        store.Trajectory
	voice    voice.Voice
	subjects map[string]bool // предметы ученика
	unis     []store.University
	stages   map[string][]stages.Stage
	benefits map[string][]store.BenefitRow // только вузы ученика, в порядке списка вузов
	tracker  store.TrackerState
	now      time.Time
}

func (s *Server) cardSet(ctx context.Context, m store.Member, t store.Trajectory, subjects []string,
	profiles []store.Profile) (cardSet, error) {
	cs := cardSet{t: t, voice: voice.New(voice.Role(m.Role), t.StudentName, m.FirstName), now: s.now(),
		subjects: map[string]bool{}, benefits: map[string][]store.BenefitRow{}}
	for _, c := range subjects {
		cs.subjects[c] = true
	}
	ids := make([]string, len(profiles))
	for i, p := range profiles {
		ids[i] = p.ID
	}
	var err error
	if cs.stages, err = s.store.StagesFor(ctx, ids); err != nil {
		return cs, err
	}
	if cs.tracker, err = s.store.TrackerState(ctx, t.ID); err != nil {
		return cs, err
	}
	if cs.unis, err = s.store.TrajectoryUniversities(ctx, t.ID); err != nil {
		return cs, err
	}
	uniIDs := make([]string, len(cs.unis))
	for i, u := range cs.unis {
		uniIDs[i] = u.ID
	}
	rows, err := s.store.Benefits(ctx, ids, uniIDs)
	if err != nil {
		return cs, err
	}
	byPair := map[string]store.BenefitRow{}
	for _, b := range rows {
		byPair[b.ProfileID+"/"+b.UniversityID] = b
	}
	for _, id := range ids {
		for _, u := range uniIDs {
			if b, ok := byPair[id+"/"+u]; ok {
				cs.benefits[id] = append(cs.benefits[id], b)
			}
		}
	}
	return cs, nil
}

func (cs cardSet) student() match.Student {
	return match.Student{DirectionSubjects: cs.t.DirectionSubjects, RegionCode: cs.t.RegionCode}
}

func (cs cardSet) candidate(p store.Profile) match.Candidate {
	return match.Candidate{
		ProfileID: p.ID, OlympiadID: p.OlympiadID, Kind: p.Kind, SubjectCode: p.SubjectCode, Level: p.Level,
		BestBenefit: bestBenefit(cs.benefits[p.ID]), Stages: cs.stages[p.ID], Registered: cs.tracker.Registered[p.ID],
		FinalRegionCode: p.FinalRegionCode,
	}
}

// bestBenefit — самая сильная льгота среди строк, "" если строк нет.
func bestBenefit(rows []store.BenefitRow) string {
	best := ""
	for _, b := range rows {
		if best == "" || benefitRank[b.Benefit] < benefitRank[best] {
			best = b.Benefit
		}
	}
	return best
}

// nicksWith — вузы с этой льготой, через запятую, в порядке строк.
func nicksWith(rows []store.BenefitRow, benefit string) string {
	var nicks []string
	for _, b := range rows {
		if b.Benefit == benefit {
			nicks = append(nicks, nick(b.UniversityID, b.UniversityShort))
		}
	}
	return strings.Join(nicks, ", ")
}

func (cs cardSet) card(p store.Profile, r match.Result) olympiadCard {
	c := olympiadCard{
		OlympiadProfileID: p.ID, OlympiadID: p.OlympiadID, Name: p.OlympiadName, Organizer: p.Organizer,
		Kind: p.Kind, Level: p.Level, SubjectCode: p.SubjectCode, SubjectName: profileLabel(p.SubjectName, p.ProfileName),
		Format: p.Format, IsOnline: r.Online, FinalCity: p.FinalCity, DeadlineAt: utc(r.Deadline),
		BenefitsSummary: cs.benefitsSummary(p.ID), Reason: cs.reason(r), InTracker: cs.tracker.InTracker[p.ID],
	}
	if r.Stage != nil {
		title := r.Stage.Title
		c.NextStageTitle = &title
	}
	if cs.tracker.Pending[p.ID] {
		pending := "pending"
		c.ProposalStatus = &pending
	}
	return c
}

// benefitsSummary — строка под карточкой: вузы ученика с самой сильной
// льготой по этому профилю — «ВШЭ, Иннополис: БВИ».
func (cs cardSet) benefitsSummary(profileID string) string {
	rows := cs.benefits[profileID]
	if len(rows) == 0 {
		return cs.voice.T("match.noBenefits", nil)
	}
	best := bestBenefit(rows)
	return nicksWith(rows, best) + ": " + benefitLabels[best]
}

// reason — причина рекомендации из двух самых сильных факторов (ТЗ §6.1
// п. 6). У ВсОШ и олимпиад вне перечня причина одна и не зависит от скора.
func (cs cardSet) reason(r match.Result) string {
	switch r.Kind {
	case "vsosh":
		return cs.voice.T("reason.vsosh", nil)
	case "other":
		return cs.voice.T("reason.outside", nil)
	}
	var parts []string
	for _, f := range r.Top {
		if s := cs.factorText(f, r); s != "" {
			parts = append(parts, s)
		}
	}
	switch len(parts) {
	case 0:
		return cs.voice.T("reason.subject", nil)
	case 1:
		return parts[0]
	default:
		return parts[0] + ", " + lowerFirst(parts[1])
	}
}

func (cs cardSet) factorText(f match.Factor, r match.Result) string {
	switch f {
	case match.Direction:
		return cs.voice.T("reason.direction", nil)
	case match.Benefit:
		return cs.voice.T("reason.benefit", nil)
	case match.Level:
		if r.Level != nil {
			return cs.voice.T("reason.level"+*r.Level, nil)
		}
	case match.Deadline:
		if r.Deadline != nil {
			return cs.voice.T("reason.deadline", voice.Vars{"date": stages.Day(r.Deadline.In(moscow))})
		}
	case match.Online:
		return cs.voice.T("reason.online", nil)
	case match.Region:
		return cs.voice.T("reason.region", nil)
	}
	return ""
}

// lowerFirst опускает первую букву, если это не аббревиатура: «БВИ» остаётся.
func lowerFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if next, _ := utf8.DecodeRuneInString(s[size:]); unicode.IsUpper(next) {
		return s
	}
	return string(unicode.ToLower(r)) + s[size:]
}

func upperFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
