package api

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ArthurBabkin/max-hackathon/packages/core/match"
	"github.com/ArthurBabkin/max-hackathon/packages/core/names"
	"github.com/ArthurBabkin/max-hackathon/packages/core/pick"
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
	// RegistrationClosed — срок первого этапа прошёл: вступить в этом сезоне
	// нельзя, хотя следующий этап ещё впереди.
	RegistrationClosed bool `json:"registration_closed"`
}

// benefitLabels — подписи льгот, те же, что BENEFIT_LABELS во фронте.
var benefitLabels = map[string]string{
	"bvi": "БВИ", "score100": "100 баллов", "bvi_winners": "БВИ победителям", "extra_points": "доп. баллы",
}

func nick(universityID, shortName string) string { return pick.Nick(universityID, shortName) }

// cardSet — набор профилей в контексте траектории (pick.Set) плюс голос
// читающего: из этого собираются карточки.
type cardSet struct {
	pick.Set
	voice voice.Voice
}

func (s *Server) cardSet(ctx context.Context, m store.Member, t store.Trajectory, subjects []string,
	profiles []store.Profile) (cardSet, error) {
	set, err := pick.Load(ctx, s.store, t, subjects, profiles, s.now())
	return cardSet{Set: set, voice: voice.New(voice.Role(m.Role), t.StudentName, m.FirstName)}, err
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
		OlympiadProfileID: p.ID, OlympiadID: p.OlympiadID, Name: names.Olympiad(p.OlympiadName), Organizer: p.Organizer,
		Kind: p.Kind, Level: p.Level, SubjectCode: p.SubjectCode, SubjectName: profileLabel(p.SubjectName, p.ProfileName),
		Format: p.Format, IsOnline: r.Online, FinalCity: p.FinalCity, DeadlineAt: utc(r.Deadline),
		BenefitsSummary: cs.benefitsSummary(p.ID), Reason: cs.reason(r), InTracker: cs.Tracker.InTracker[p.ID],
		RegistrationClosed: !stages.Joinable(cs.Stages[p.ID], cs.Now),
	}
	if r.Stage != nil {
		title := r.Stage.Title
		c.NextStageTitle = &title
	}
	if cs.Tracker.Pending[p.ID] {
		pending := "pending"
		c.ProposalStatus = &pending
	}
	return c
}

// benefitsSummary — строка под карточкой: вузы ученика с самой сильной
// льготой по этому профилю — «ВШЭ, Иннополис: БВИ».
func (cs cardSet) benefitsSummary(profileID string) string {
	rows := cs.Benefits[profileID]
	if len(cs.Universities) == 0 {
		// Вузы не выбраны — льготы в вузах с направлением (SPEC 2.3).
		rows = cs.Potential[profileID]
	}
	if len(rows) == 0 {
		return cs.voice.T("match.noBenefits", nil)
	}
	best := pick.BestBenefit(rows)
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
		if r.PotentialBenefit {
			return cs.voice.T("reason.benefitPotential", nil)
		}
		return cs.voice.T("reason.benefit", nil)
	case match.Level:
		if r.LevelFit {
			return cs.voice.T("reason.levelFit", nil)
		}
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
