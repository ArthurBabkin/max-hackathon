package api

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
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

// benefitGrant — что получит победитель или призёр: вид льготы и подпись.
type benefitGrant struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

type benefitRow struct {
	UniversityID        string        `json:"university_id"`
	UniversityName      string        `json:"university_name"`
	UniversityShortName string        `json:"university_short_name"`
	City                *string       `json:"city"`
	Color               *string       `json:"color"`
	Benefit             *string       `json:"benefit"`
	BenefitLabel        *string       `json:"benefit_label"`
	ExtraPoints         *int          `json:"extra_points"`
	EgeMin              *int          `json:"ege_min"`
	DiplomaGrades       []int32       `json:"diploma_grades"`
	Note                *string       `json:"note"`
	Source              *sourceDTO    `json:"source"`
	UniversityNick      string        `json:"university_nick"`
	Winner              *benefitGrant `json:"winner"`
	Prizer              *benefitGrant `json:"prizer"`
	EgeMax              *int          `json:"ege_max"`
	// Conditions — своё у вуза в карточке олимпиады (F19); в других списках пусто.
	Conditions []string `json:"conditions,omitempty"`
}

func benefitRowOf(b store.BenefitRow) benefitRow {
	benefit, label := b.Benefit, benefitLabels[b.Benefit]
	winner, prizer := grants(b)
	row := benefitRow{
		UniversityID: b.UniversityID, UniversityName: b.UniversityName, UniversityShortName: b.UniversityShort,
		City: b.City, Benefit: &benefit, BenefitLabel: &label, ExtraPoints: b.ExtraPoints, EgeMin: b.EgeMin,
		DiplomaGrades: b.DiplomaGrades, Note: b.Note, Source: sourceOf(b.Source),
		UniversityNick: nick(b.UniversityID, b.UniversityShort), Winner: winner, Prizer: prizer,
	}
	if _, to, ok := egeRange(b.Note); ok {
		if n, err := strconv.Atoi(to); err == nil {
			row.EgeMax = &n
		}
	}
	return row
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
	Description         *string        `json:"description"`
	Profiles            []profileLevel `json:"profiles"`
	ProfilesSource      *sourceDTO     `json:"profiles_source"`
	Benefits            []benefitRow   `json:"benefits"`
	BenefitsSource      *sourceDTO     `json:"benefits_source"`
	Conditions          []string       `json:"conditions"`
	Stages              []stageDTO     `json:"stages"`
	StagesAreDemo       bool           `json:"stages_are_demo"`
	Why                 string         `json:"why"`
	BenefitUniversities []benefitRow   `json:"benefit_universities"`
	BenefitColumns      []string       `json:"benefit_columns"`
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
		olympiadCard: cs.card(p, res), OfficialURL: p.OfficialURL, Description: p.Description, ProfilesSource: sourceOf(p.Source),
		BenefitsSource: benefitsSource(mine), Conditions: cs.conditions(p, mine, all),
		Stages: stagesOf(st, cs.Tracker.Registered[p.ID], cs.Now), StagesAreDemo: len(st) == 0,
		Why: cs.why(p, res, mine, all), BenefitUniversities: []benefitRow{}, BenefitColumns: benefitColumns(p, mine),
	}
	for _, x := range st {
		out.StagesAreDemo = out.StagesAreDemo || x.IsDemo
	}
	// «Где ещё даёт льготу» — без вузов ученика: они уже в блоке льгот (F23).
	own := map[string]bool{}
	for _, u := range cs.Universities {
		own[u.ID] = true
	}
	for _, b := range all {
		if !own[b.UniversityID] {
			out.BenefitUniversities = append(out.BenefitUniversities, benefitRowOf(b))
		}
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
			row := benefitRowOf(b)
			if p.Kind != "other" {
				row.Conditions = cs.uniConditions(b, egeSubject(mine))
			}
			out.Benefits = append(out.Benefits, row)
			continue
		}
		out.Benefits = append(out.Benefits, benefitRow{
			UniversityID: u.ID, UniversityName: u.Name, UniversityShortName: u.ShortName, City: u.City,
			UniversityNick: nick(u.ID, u.ShortName),
		})
	}
	if out.Benefits == nil {
		out.Benefits = []benefitRow{}
	}
	sortBenefitRows(out.Benefits)
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

// conditions — общие условия подтверждения льготы (F19), только из данных о
// льготах. Если льготу дают вузы ученика, здесь то, что верно для всех них,
// а своё у вуза — в его строке (uniConditions). Если не дают — условия
// собираются по всем вузам базы, вместе с особенностями вузов.
func (cs cardSet) conditions(p store.Profile, mine, all []store.BenefitRow) []string {
	v := cs.voice
	if p.Kind == "other" {
		return []string{v.T("cond.outside", nil), v.T("cond.extraPoints", nil)}
	}
	out := []string{v.T("cond.diploma", nil)}
	if p.Kind == "vsosh" {
		out[0] = v.T("cond.diplomaVsosh", nil)
	}
	rows, own := mine, true
	if len(rows) == 0 {
		rows, own = all, false
	}
	if len(rows) == 0 {
		return append(out, v.T("cond.noBenefits", nil))
	}

	minEge, maxEge := egeBounds(rows)
	subject := egeSubject(rows)
	switch {
	case own && egeInTable(rows):
		// Порог — в столбце таблицы, здесь только предмет.
		if subject != "" {
			out = append(out, v.T("cond.egeSubject", voice.Vars{"subject": subject}))
		}
	case minEge != nil && subject != "":
		out = append(out, v.T("cond.ege", voice.Vars{"subject": subject, "count": *minEge}))
	case minEge != nil:
		out = append(out, v.T("cond.egeAny", voice.Vars{"count": *minEge}))
	case subject != "":
		out = append(out, v.T("cond.egeSubject", voice.Vars{"subject": subject}))
	}
	if !own {
		varies := false
		for _, b := range rows {
			_, _, ok := egeRange(b.Note)
			varies = varies || ok
		}
		if minEge != nil && (varies || *maxEge != *minEge) {
			out = append(out, v.T("cond.egeVaries", nil))
		}
		if g := commonGrades(rows); g != "" {
			out = append(out, v.T("cond.grades", voice.Vars{"grade": g}))
		}
		for _, n := range winnerNotes(rows) {
			out = append(out, v.T("cond.note", voice.Vars{"names": n.nicks, "note": lowerFirst(n.text)}))
		}
	}
	if b := pick.BestBenefit(rows); b == "bvi" || b == "bvi_winners" {
		out = append(out, v.T("cond.bviOnce", nil))
	}
	return out
}

// uniConditions — чем условия вуза отличаются от общих (F19), кроме того, что
// видно в столбцах таблицы: другой предмет ЕГЭ и — только если мешает — за
// какой класс вуз засчитывает диплом. subject — общий предмет ЕГЭ.
func (cs cardSet) uniConditions(b store.BenefitRow, subject string) []string {
	v := cs.voice
	var out []string
	if own := subjects(noteSubject(b.Note)); len(own) > 0 && !sameSubject(own, subjects(subject)) {
		out = append(out, v.T("cond.egeSubjectUni", voice.Vars{"subject": strings.Join(own, " или "), "common": subject}))
	}
	grade := int32(cs.Trajectory.Grade)
	if len(b.DiplomaGrades) > 0 && !slices.Contains(b.DiplomaGrades, grade) {
		out = append(out, v.T("cond.gradeMiss", voice.Vars{"grades": gradesLabel(b.DiplomaGrades), "grade": grade}))
	}
	return out
}

// noteSentences — предложения примечания к льготе без точек на концах.
func noteSentences(note *string) []string {
	if note == nil {
		return nil
	}
	var out []string
	for _, sentence := range strings.Split(*note, ". ") {
		out = append(out, strings.TrimSuffix(strings.TrimSpace(sentence), "."))
	}
	return out
}

var egeRangeRe = regexp.MustCompile(`Порог ЕГЭ зависит от программы: (\d+)–(\d+)`)

// egeRange — разброс порога ЕГЭ по программам вуза, если он есть.
func egeRange(note *string) (from, to string, ok bool) {
	if note == nil {
		return "", "", false
	}
	m := egeRangeRe.FindStringSubmatch(*note)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

func egeBounds(rows []store.BenefitRow) (minEge, maxEge *int) {
	for _, b := range rows {
		if b.EgeMin == nil {
			continue
		}
		if minEge == nil || *b.EgeMin < *minEge {
			minEge = b.EgeMin
		}
		if maxEge == nil || *b.EgeMin > *maxEge {
			maxEge = b.EgeMin
		}
	}
	return minEge, maxEge
}

// egeSubject — предмет ЕГЭ для подтверждения, который чаще всего называют
// вузы; при равенстве — тот, что встретился раньше.
func egeSubject(rows []store.BenefitRow) string {
	count := map[string]int{}
	var order []string
	for _, b := range rows {
		if subj := noteSubject(b.Note); subj != "" {
			if count[subj] == 0 {
				order = append(order, subj)
			}
			count[subj]++
		}
	}
	subject := ""
	for _, subj := range order {
		if count[subj] > count[subject] {
			subject = subj
		}
	}
	return subject
}

type winnerNote struct{ text, nicks string }

// winnerNotes — правила для победителя и призёра, с вузами, где они действуют.
func winnerNotes(rows []store.BenefitRow) []winnerNote {
	var notes []winnerNote
	at := map[string]int{}
	for _, b := range rows {
		for _, sentence := range noteSentences(b.Note) {
			if !strings.HasPrefix(sentence, "Победителю") && !strings.Contains(sentence, "только победителю") {
				continue
			}
			n := nick(b.UniversityID, b.UniversityShort)
			if i, ok := at[sentence]; ok {
				notes[i].nicks += ", " + n
			} else {
				at[sentence] = len(notes)
				notes = append(notes, winnerNote{sentence, n})
			}
		}
	}
	return notes
}

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
	return gradesLabel(first)
}

// gradesLabel — классы подряд через тире, иначе через запятую: «11», «9–11», «9, 11».
func gradesLabel(grades []int32) string {
	contiguous := true
	for i := 1; i < len(grades); i++ {
		contiguous = contiguous && grades[i] == grades[i-1]+1
	}
	switch {
	case len(grades) == 1:
		return fmt.Sprint(grades[0])
	case contiguous:
		return fmt.Sprintf("%d–%d", grades[0], grades[len(grades)-1])
	default:
		parts := make([]string, len(grades))
		for i, g := range grades {
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
	// Из нескольких направлений называем первое, куда входит предмет олимпиады.
	dirIdx := slices.IndexFunc(cs.Trajectory.Directions, func(d store.Direction) bool {
		return slices.Contains(d.SubjectCodes, p.SubjectCode)
	})
	switch {
	case r.Factors[match.Direction] > 0 && dirIdx >= 0:
		parts = append(parts, v.T("why.direction", voice.Vars{"subject": subject, "direction": cs.Trajectory.Directions[dirIdx].Name}))
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
