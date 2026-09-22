package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/match"
	"github.com/ArthurBabkin/max-hackathon/packages/core/pick"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

type sourceDTO struct {
	ID         string  `json:"id"`
	Kind       string  `json:"kind"`
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	VerifiedAt *string `json:"verified_at"`
}

func dateOf(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}

func sourceOf(s *store.Source) *sourceDTO {
	if s == nil {
		return nil
	}
	return &sourceDTO{ID: s.ID, Kind: s.Kind, Title: s.Title, URL: s.URL, VerifiedAt: dateOf(s.VerifiedAt)}
}

type profileLevel struct {
	OlympiadProfileID string  `json:"olympiad_profile_id"`
	SubjectCode       string  `json:"subject_code"`
	SubjectName       string  `json:"subject_name"`
	Level             *string `json:"level"`
	IsMine            bool    `json:"is_mine"`
}

type benefitRow struct {
	UniversityID        string     `json:"university_id"`
	UniversityName      string     `json:"university_name"`
	UniversityShortName string     `json:"university_short_name"`
	City                *string    `json:"city"`
	Color               *string    `json:"color"`
	Benefit             *string    `json:"benefit"`
	BenefitLabel        *string    `json:"benefit_label"`
	ExtraPoints         *int       `json:"extra_points"`
	EgeMin              *int       `json:"ege_min"`
	DiplomaGrades       []int32    `json:"diploma_grades"`
	Note                *string    `json:"note"`
	Source              *sourceDTO `json:"source"`
}

func benefitRowOf(b store.BenefitRow) benefitRow {
	benefit, label := b.Benefit, benefitLabels[b.Benefit]
	return benefitRow{
		UniversityID: b.UniversityID, UniversityName: b.UniversityName, UniversityShortName: b.UniversityShort,
		City: b.City, Benefit: &benefit, BenefitLabel: &label, ExtraPoints: b.ExtraPoints, EgeMin: b.EgeMin,
		DiplomaGrades: b.DiplomaGrades, Note: b.Note, Source: sourceOf(b.Source),
	}
}

type stageDTO struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	Subtitle   *string    `json:"subtitle"`
	StartsAt   *time.Time `json:"starts_at"`
	EndsAt     *time.Time `json:"ends_at"`
	DeadlineAt *time.Time `json:"deadline_at"`
	IsOnline   bool       `json:"is_online"`
	State      string     `json:"state"`
}

type olympiadDetail struct {
	olympiadCard
	OfficialURL         *string        `json:"official_url"`
	Profiles            []profileLevel `json:"profiles"`
	ProfilesSource      *sourceDTO     `json:"profiles_source"`
	Benefits            []benefitRow   `json:"benefits"`
	BenefitsSource      *sourceDTO     `json:"benefits_source"`
	Conditions          []string       `json:"conditions"`
	Stages              []stageDTO     `json:"stages"`
	StagesAreDemo       bool           `json:"stages_are_demo"`
	Why                 string         `json:"why"`
	BenefitUniversities []benefitRow   `json:"benefit_universities"`
}

var levelOrder = map[string]int{"I": 0, "II": 1, "III": 2}

func levelRank(l *string) int {
	if l == nil {
		return len(levelOrder)
	}
	return levelOrder[*l]
}

// olympiad — GET /olympiads/{id}, id — профиль олимпиады (F17–F23).
func (s *Server) olympiad(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	found, err := s.store.Profiles(ctx, store.ProfileQuery{IDs: []string{r.PathValue("id")}})
	if err != nil {
		return err
	}
	if len(found) == 0 {
		return notFound("Олимпиада не найдена.")
	}
	p := found[0]
	t, err := s.store.Trajectory(ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	codes, err := pick.SubjectCodes(ctx, s.store, m.TrajectoryID)
	if err != nil {
		return err
	}
	cs, err := s.cardSet(ctx, m, t, codes, []store.Profile{p})
	if err != nil {
		return err
	}
	siblings, err := s.store.Profiles(ctx, store.ProfileQuery{OlympiadIDs: []string{p.OlympiadID}})
	if err != nil {
		return err
	}
	all, err := s.store.Benefits(ctx, []string{p.ID}, nil)
	if err != nil {
		return err
	}
	sort.SliceStable(all, func(i, j int) bool {
		if a, b := pick.BenefitRank[all[i].Benefit], pick.BenefitRank[all[j].Benefit]; a != b {
			return a < b
		}
		return all[i].UniversityName < all[j].UniversityName
	})

	res := match.Score(cs.Candidate(p), cs.Student(), match.DefaultWeights, cs.Now)
	mine := cs.Benefits[p.ID]
	st := cs.Stages[p.ID]
	out := olympiadDetail{
		olympiadCard: cs.card(p, res), OfficialURL: p.OfficialURL, ProfilesSource: sourceOf(p.Source),
		BenefitsSource: benefitsSource(mine), Conditions: cs.conditions(p, mine, all),
		Stages: stagesOf(st, cs.Tracker.Registered[p.ID], cs.Now), StagesAreDemo: len(st) == 0,
		Why: cs.why(p, res, mine, all), BenefitUniversities: make([]benefitRow, len(all)),
	}
	for _, x := range st {
		out.StagesAreDemo = out.StagesAreDemo || x.IsDemo
	}
	for i, b := range all {
		out.BenefitUniversities[i] = benefitRowOf(b)
	}

	sort.SliceStable(siblings, func(i, j int) bool {
		if a, b := levelRank(siblings[i].Level), levelRank(siblings[j].Level); a != b {
			return a < b
		}
		return profileLabel(siblings[i].SubjectName, siblings[i].ProfileName) <
			profileLabel(siblings[j].SubjectName, siblings[j].ProfileName)
	})
	for _, x := range siblings {
		out.Profiles = append(out.Profiles, profileLevel{
			OlympiadProfileID: x.ID, SubjectCode: x.SubjectCode, SubjectName: profileLabel(x.SubjectName, x.ProfileName),
			Level: x.Level, IsMine: x.ID == p.ID,
		})
	}

	// По строке на каждый вуз ученика, «не учитывает» — тоже строка (F18).
	byUni := map[string]store.BenefitRow{}
	for _, b := range mine {
		byUni[b.UniversityID] = b
	}
	for _, u := range cs.Universities {
		if b, ok := byUni[u.ID]; ok {
			out.Benefits = append(out.Benefits, benefitRowOf(b))
			continue
		}
		out.Benefits = append(out.Benefits, benefitRow{
			UniversityID: u.ID, UniversityName: u.Name, UniversityShortName: u.ShortName, City: u.City,
		})
	}
	if out.Benefits == nil {
		out.Benefits = []benefitRow{}
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func stagesOf(st []stages.Stage, registered bool, now time.Time) []stageDTO {
	states := stages.States(st, registered, now)
	out := make([]stageDTO, len(st))
	for i, x := range st {
		out[i] = stageDTO{
			ID: x.ID, Kind: x.Kind, Title: x.Title, StartsAt: utc(x.StartsAt), EndsAt: utc(x.EndsAt),
			DeadlineAt: utc(x.DeadlineAt), IsOnline: x.IsOnline, State: states[i],
		}
		if sub := stages.Subtitle(x, moscow); sub != "" {
			out[i].Subtitle = &sub
		}
	}
	return out
}

// benefitsSource — источник блока «Льгота в вузах ученика». Метка «Факт»
// ставится, только если у каждой строки есть проверенный источник; у
// разных вузов источники разные, и тогда блок ссылается на правила всех.
func benefitsSource(rows []store.BenefitRow) *sourceDTO {
	if len(rows) == 0 {
		return nil
	}
	ids := map[string]bool{}
	var oldest *time.Time
	var nicks []string
	for _, b := range rows {
		if b.Source == nil || b.Source.VerifiedAt == nil {
			return nil
		}
		ids[b.Source.ID] = true
		if oldest == nil || b.Source.VerifiedAt.Before(*oldest) {
			oldest = b.Source.VerifiedAt
		}
		nicks = append(nicks, nick(b.UniversityID, b.UniversityShort))
	}
	if len(ids) == 1 {
		return sourceOf(rows[0].Source)
	}
	year := rows[0].AdmissionYear
	return &sourceDTO{
		ID: fmt.Sprintf("rules-%d", year), Kind: "rules",
		Title: fmt.Sprintf("Правила приёма %d: %s", year, strings.Join(nicks, ", ")),
		URL:   rows[0].Source.URL, VerifiedAt: dateOf(oldest),
	}
}

// conditions — условия подтверждения льготы (F19), только из данных о
// льготах: по вузам ученика, а если там льгот нет — по всем вузам базы.
func (cs cardSet) conditions(p store.Profile, mine, all []store.BenefitRow) []string {
	v := cs.voice
	if p.Kind == "other" {
		return []string{v.T("cond.outside", nil), v.T("cond.extraPoints", nil)}
	}
	out := []string{v.T("cond.diploma", nil)}
	if p.Kind == "vsosh" {
		out[0] = v.T("cond.diplomaVsosh", nil)
	}
	rows := mine
	if len(rows) == 0 {
		rows = all
	}
	if len(rows) == 0 {
		return append(out, v.T("cond.noBenefits", nil))
	}

	var minEge, maxEge *int
	varies := false
	subjects := map[string]int{}
	var subjectOrder []string
	type note struct{ text, nicks string }
	var notes []note
	noteAt := map[string]int{}
	for _, b := range rows {
		if b.EgeMin != nil {
			if minEge == nil || *b.EgeMin < *minEge {
				minEge = b.EgeMin
			}
			if maxEge == nil || *b.EgeMin > *maxEge {
				maxEge = b.EgeMin
			}
		}
		if b.Note == nil {
			continue
		}
		for _, sentence := range strings.Split(*b.Note, ". ") {
			sentence = strings.TrimSuffix(strings.TrimSpace(sentence), ".")
			switch {
			case strings.HasPrefix(sentence, "Подтвердить ЕГЭ: "):
				subj := strings.TrimPrefix(sentence, "Подтвердить ЕГЭ: ")
				if subjects[subj] == 0 {
					subjectOrder = append(subjectOrder, subj)
				}
				subjects[subj]++
			case strings.Contains(sentence, "зависит от программы"):
				varies = true
			case strings.HasPrefix(sentence, "Победителю") || strings.Contains(sentence, "только победителю"):
				n := nick(b.UniversityID, b.UniversityShort)
				if i, ok := noteAt[sentence]; ok {
					notes[i].nicks += ", " + n
				} else {
					noteAt[sentence] = len(notes)
					notes = append(notes, note{sentence, n})
				}
			}
		}
	}
	subject := ""
	for _, subj := range subjectOrder {
		if subjects[subj] > subjects[subject] {
			subject = subj
		}
	}
	switch {
	case minEge != nil && subject != "":
		out = append(out, v.T("cond.ege", voice.Vars{"subject": subject, "count": *minEge}))
	case minEge != nil:
		out = append(out, v.T("cond.egeAny", voice.Vars{"count": *minEge}))
	case subject != "":
		out = append(out, v.T("cond.egeSubject", voice.Vars{"subject": subject}))
	}
	if minEge != nil && (varies || *maxEge != *minEge) {
		out = append(out, v.T("cond.egeVaries", nil))
	}
	if g := commonGrades(rows); g != "" {
		out = append(out, v.T("cond.grades", voice.Vars{"grade": g}))
	}
	for _, n := range notes {
		out = append(out, v.T("cond.note", voice.Vars{"names": n.nicks, "note": lowerFirst(n.text)}))
	}
	if b := pick.BestBenefit(rows); b == "bvi" || b == "bvi_winners" {
		out = append(out, v.T("cond.bviOnce", nil))
	}
	return out
}

// commonGrades — «9–11», если у всех строк одинаковые классы диплома; иначе "".
func commonGrades(rows []store.BenefitRow) string {
	first := rows[0].DiplomaGrades
	if len(first) == 0 {
		return ""
	}
	for _, b := range rows[1:] {
		if fmt.Sprint(b.DiplomaGrades) != fmt.Sprint(first) {
			return ""
		}
	}
	contiguous := true
	for i := 1; i < len(first); i++ {
		contiguous = contiguous && first[i] == first[i-1]+1
	}
	switch {
	case len(first) == 1:
		return fmt.Sprint(first[0])
	case contiguous:
		return fmt.Sprintf("%d–%d", first[0], first[len(first)-1])
	default:
		parts := make([]string, len(first))
		for i, g := range first {
			parts[i] = fmt.Sprint(g)
		}
		return strings.Join(parts, ", ")
	}
}

// why — блок «Почему подходит» (F20): те же факторы, что в скоринге, но
// развёрнуто.
func (cs cardSet) why(p store.Profile, r match.Result, mine, all []store.BenefitRow) string {
	v := cs.voice
	subject := strings.ToLower(profileLabel(p.SubjectName, p.ProfileName))
	var parts []string
	if p.Kind == "other" {
		parts = append(parts, v.T("why.outside", nil))
		if len(all) > 0 {
			parts = append(parts, v.T("why.extraPoints", voice.Vars{"names": nicksWith(all, pick.BestBenefit(all))}))
		}
		return strings.Join(parts, " ")
	}
	if p.Kind == "vsosh" {
		parts = append(parts, v.T("why.vsosh", nil))
	}
	switch {
	case r.Factors[match.Direction] > 0 && cs.Trajectory.DirectionName != nil:
		parts = append(parts, v.T("why.direction", voice.Vars{"subject": subject, "direction": *cs.Trajectory.DirectionName}))
	case cs.Subjects[p.SubjectCode]:
		parts = append(parts, v.T("why.subject", voice.Vars{"subject": subject}))
	}
	switch best := pick.BestBenefit(mine); {
	case best != "":
		parts = append(parts, v.T("why.benefit", voice.Vars{
			"title": upperFirst(benefitLabels[best]), "names": nicksWith(mine, best)}))
	case len(all) > 0:
		var nicks []string
		for _, b := range all[:min(3, len(all))] {
			nicks = append(nicks, nick(b.UniversityID, b.UniversityShort))
		}
		parts = append(parts, v.T("why.benefitElsewhere", voice.Vars{"names": strings.Join(nicks, ", ")}))
	}
	if r.Online {
		parts = append(parts, v.T("why.online", nil))
	}
	if r.Factors[match.Region] > 0 {
		parts = append(parts, v.T("why.region", nil))
	}
	return strings.Join(parts, " ")
}
