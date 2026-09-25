// Package match — подбор олимпиад (ТЗ §6.1): скоринг по шести факторам,
// топ-5 перечня, ВсОШ отдельной веткой, олимпиады вне перечня отдельным
// блоком. Пакет чистый: кандидатов собирает вызывающий.
package match

import (
	"sort"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
)

type Factor string

const (
	Direction Factor = "direction" // профиль входит в ключевые предметы направления
	Benefit   Factor = "benefit"   // лучшая льгота среди вузов ученика
	Level     Factor = "level"     // уровень по перечню
	Deadline  Factor = "deadline"  // близость ближайшего срока
	Online    Factor = "online"    // есть онлайн-отбор
	Region    Factor = "region"    // финал в регионе ученика
)

// Weights — веса факторов. ТЗ разрешает менять их конфигом.
type Weights struct {
	Direction, Benefit, Level, Deadline, Online, Region float64
}

var DefaultWeights = Weights{Direction: 0.30, Benefit: 0.30, Level: 0.15, Deadline: 0.15, Online: 0.05, Region: 0.05}

// TopN — сколько олимпиад перечня показываем (ТЗ §6.1 п. 5).
const TopN = 5

// SoonDays — порог чипа «Срок скоро» (контракт, фильтр soon).
const SoonDays = 16

type Candidate struct {
	ProfileID   string
	OlympiadID  string // пусто — профиль считается отдельной олимпиадой
	Kind        string // perechen | vsosh | other
	SubjectCode string
	Level       *string
	// BestBenefit — лучшая льгота в вузах ученика: bvi | bvi_winners | score100 | extra_points | "".
	BestBenefit     string
	Stages          []stages.Stage
	Registered      bool
	FinalRegionCode *string
	// PotentialBenefit — вузы у ученика не выбраны, BestBenefit посчитан по
	// вузам с его направлениями в выбранных местах (SPEC 2.3).
	PotentialBenefit bool
}

type Student struct {
	DirectionSubjects []string
	RegionCode        string
	// Experience — опыт в олимпиадах: none | school | region; "" — как none.
	Experience string
	// Grade — класс; 0 — не известен, льгота считается полностью.
	Grade int
}

type Result struct {
	Candidate
	Score   float64
	Factors map[Factor]float64 // вклад каждого фактора с учётом веса
	// Top — до двух самых сильных факторов с ненулевым вкладом: из них
	// складывается причина рекомендации (ТЗ §6.1 п. 6).
	Top []Factor
	// Deadline и Stage — текущий этап: ближайший ещё не прошедший.
	Deadline *time.Time
	Stage    *stages.Stage
	Online   bool
	// LevelFit — уровень олимпиады подходит под опыт ученика: опыт задан и
	// вклад уровня не меньше LevelFitShare от максимума. Причина звучит
	// «Подходит под твой опыт» вместо «Олимпиада I уровня».
	LevelFit bool
}

var benefitValue = map[string]float64{"bvi": 1, "bvi_winners": 0.8, "score100": 0.6}

// levelValue[опыт][уровень]: новичку выше II–III уровень, опытному — I
// (SPEC 2.1). Пустой опыт — none.
var levelValue = map[string]map[string]float64{
	"none":   {"I": 0.4, "II": 0.8, "III": 1.0},
	"school": {"I": 0.7, "II": 1.0, "III": 0.6},
	"region": {"I": 1.0, "II": 0.6, "III": 0.3},
}

// LevelFitShare — с какой доли максимума вклад уровня считается «под опыт».
const LevelFitShare = 0.8

func experienceOf(s Student) string {
	if _, ok := levelValue[s.Experience]; ok {
		return s.Experience
	}
	return "none"
}

// LevelValue — вес уровня олимпиады для опыта ученика. ВсОШ для опытного
// ученика весит как I уровень (диплом финала — БВИ), для остальных — как
// II: школьный этап доступен всем.
func LevelValue(experience, kind string, level *string) float64 {
	row, ok := levelValue[experience]
	if !ok {
		row = levelValue["none"]
	}
	switch {
	case kind == "vsosh" && experience == "region":
		return row["I"]
	case kind == "vsosh":
		return row["II"]
	case level != nil:
		return row[*level]
	}
	return 0
}

// BenefitGradeFactor — множитель льготы по классу: у многих вузов диплом
// засчитывается только за 10–11 класс, поэтому для 8–9 класса льгота весит
// вдвое меньше (SPEC 2.2).
func BenefitGradeFactor(grade int) float64 {
	if grade == 8 || grade == 9 {
		return 0.5
	}
	return 1
}

// factorOrder — порядок при равном вкладе: так причина стабильна.
var factorOrder = []Factor{Direction, Benefit, Level, Deadline, Online, Region}

func Score(c Candidate, s Student, w Weights, now time.Time) Result {
	st := append([]stages.Stage(nil), c.Stages...)
	stages.Sort(st)
	r := Result{Candidate: c, Factors: map[Factor]float64{}}
	if cur := stages.Current(st, c.Registered, now); cur >= 0 {
		r.Stage = &st[cur]
		r.Deadline = st[cur].DeadlineAt
	}
	for _, x := range st {
		if x.Kind == "qualifying" && x.IsOnline {
			r.Online = true
		}
	}

	if contains(s.DirectionSubjects, c.SubjectCode) {
		r.Factors[Direction] = w.Direction
	}
	r.Factors[Benefit] = w.Benefit * BenefitGradeFactor(s.Grade) * benefitValue[c.BestBenefit]
	exp := experienceOf(s)
	r.Factors[Level] = w.Level * LevelValue(exp, c.Kind, c.Level)
	r.LevelFit = s.Experience != "" && w.Level > 0 && r.Factors[Level] >= LevelFitShare*w.Level
	r.Factors[Deadline] = w.Deadline * deadlineValue(r.Deadline, now)
	if r.Online {
		r.Factors[Online] = w.Online
	}
	if c.FinalRegionCode != nil && *c.FinalRegionCode == s.RegionCode {
		r.Factors[Region] = w.Region
	}

	for _, f := range factorOrder {
		r.Score += r.Factors[f]
	}
	top := append([]Factor(nil), factorOrder...)
	sort.SliceStable(top, func(i, j int) bool { return r.Factors[top[i]] > r.Factors[top[j]] })
	for _, f := range top {
		if len(r.Top) < 2 && r.Factors[f] > 0 {
			r.Top = append(r.Top, f)
		}
	}
	return r
}

// deadlineValue: срок через 3–30 дней — 1; дальше убывает как 30/d; меньше
// трёх дней — 0,5: успеть ещё можно, но впритык. Нет срока — 0.
func deadlineValue(deadline *time.Time, now time.Time) float64 {
	if deadline == nil {
		return 0
	}
	days := deadline.Sub(now).Hours() / 24
	switch {
	case days < 3:
		return 0.5
	case days <= 30:
		return 1
	default:
		return 30 / days
	}
}

// Options — параметры подбора. Tracked и Proposed — ключи олимпиад (Key),
// которые уже в трекере или ждут ответа на предложение: в приложении они
// прячутся до ранжирования, их места занимают следующие по скору.
type Options struct {
	Filter   string
	Tracked  map[string]bool
	Proposed map[string]bool
}

// Состояния подбора для экрана C3/C7.
const (
	StateOK           = "ok"
	StateAllTracked   = "all_tracked"
	StateAllProposed  = "all_proposed"
	StateNoneSuitable = "none_suitable"
)

// Picked — подбор. Items — ВсОШ по предметам цели и топ перечня, по сроку;
// More — остальные подходящие перечня по скору («Показать ещё»); Outside —
// вне перечня. Tracked и Proposed — сколько подходящих под фильтр олимпиад
// спрятано, потому что они уже в трекере или предложены.
type Picked struct {
	Items, More, Outside []Result
	Tracked, Proposed    int
}

// State — что показать, когда список пуст: всё подходящее уже в трекере,
// часть ждёт ответа на предложение или подходящих нет вовсе.
func (p Picked) State() string {
	switch {
	case len(p.Items) > 0:
		return StateOK
	case p.Proposed > 0:
		return StateAllProposed
	case p.Tracked > 0:
		return StateAllTracked
	}
	return StateNoneSuitable
}

// Key — ключ олимпиады кандидата: профили одной олимпиады прячутся вместе.
func Key(c Candidate) string {
	if c.OlympiadID == "" {
		return "profile:" + c.ProfileID
	}
	return c.OlympiadID
}

// Pick — подбор для экрана C3 и бота. Подходящая олимпиада: во что ещё
// можно вступить (stages.Joinable); перечневая — только с льготой в вузах
// ученика (или потенциальной, если вузы не выбраны); ВсОШ — по предметам
// цели. Из перечня — топ-5 по скору; ВсОШ — всегда; вне перечня —
// отдельным списком. Items и Outside отсортированы по близости срока.
func Pick(cands []Candidate, s Student, w Weights, now time.Time, o Options) Picked {
	var p Picked
	var perechen []Result
	tracked, proposed := map[string]bool{}, map[string]bool{}
	for _, c := range cands {
		if !stages.Joinable(c.Stages, now) {
			continue
		}
		switch c.Kind {
		case "other":
		case "vsosh":
			if len(s.DirectionSubjects) > 0 && !contains(s.DirectionSubjects, c.SubjectCode) {
				continue
			}
		default:
			if c.BestBenefit == "" {
				continue
			}
		}
		r := Score(c, s, w, now)
		if !passes(r, o.Filter, now) {
			continue
		}
		if key := Key(c); o.Tracked[key] || o.Proposed[key] {
			if c.Kind != "other" {
				if o.Tracked[key] {
					tracked[key] = true
				} else {
					proposed[key] = true
				}
			}
			continue
		}
		switch c.Kind {
		case "other":
			p.Outside = append(p.Outside, r)
		case "vsosh":
			p.Items = append(p.Items, r)
		default:
			perechen = append(perechen, r)
		}
	}
	sort.SliceStable(perechen, func(i, j int) bool {
		if perechen[i].Score != perechen[j].Score {
			return perechen[i].Score > perechen[j].Score
		}
		return perechen[i].ProfileID < perechen[j].ProfileID
	})
	top, more := diverse(bestPerOlympiad(perechen), TopN, MaxPerSubject)
	p.Items = append(p.Items, top...)
	p.More = more
	p.Tracked, p.Proposed = len(tracked), len(proposed)
	byDeadline(p.Items)
	byDeadline(p.Outside)
	return p
}

// Recommend — Pick без трекера: подбор для итога онбординга в боте.
func Recommend(cands []Candidate, s Student, w Weights, now time.Time, filter string) (items, outside []Result) {
	p := Pick(cands, s, w, now, Options{Filter: filter})
	return p.Items, p.Outside
}

// bestPerOlympiad оставляет у каждой олимпиады первый (лучший) профиль:
// у НТО пятнадцать профилей по информатике, и без этого они заняли бы
// весь топ. Остальные профили видны в карточке олимпиады.
func bestPerOlympiad(sorted []Result) []Result {
	seen := map[string]bool{}
	out := sorted[:0]
	for _, r := range sorted {
		key := r.OlympiadID
		if key == "" {
			key = "profile:" + r.ProfileID
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, r)
		}
	}
	return out
}

// MaxPerSubject — сколько олимпиад одного предмета может быть в топе.
const MaxPerSubject = 2

// diverse — первые n по скору, но не больше perSubject олимпиад одного
// предмета: лишние сдвигаются вниз, их места занимают следующие по скору
// (SPEC 2.4). Если других предметов не хватает, топ добирается лишними —
// у ученика с одним предметом иначе осталось бы две олимпиады. rest —
// всё, что не вошло в топ, в исходном порядке.
func diverse(sorted []Result, n, perSubject int) (top, rest []Result) {
	in := make([]bool, len(sorted))
	count := map[string]int{}
	taken := 0
	for i, r := range sorted {
		if taken == n {
			break
		}
		if count[r.SubjectCode] >= perSubject {
			continue
		}
		count[r.SubjectCode]++
		in[i] = true
		taken++
	}
	for i := range sorted {
		if taken == n {
			break
		}
		if !in[i] {
			in[i] = true
			taken++
		}
	}
	for i, r := range sorted {
		if in[i] {
			top = append(top, r)
		} else {
			rest = append(rest, r)
		}
	}
	return top, rest
}

func passes(r Result, filter string, now time.Time) bool {
	switch filter {
	case "level1":
		return r.Level != nil && *r.Level == "I"
	case "soon":
		return r.Deadline != nil && r.Deadline.Sub(now) <= SoonDays*24*time.Hour
	case "online":
		return r.Online
	default:
		return true
	}
}

// byDeadline — ближайший срок первым, без срока в конце, при равенстве — скор.
func byDeadline(rs []Result) {
	sort.SliceStable(rs, func(i, j int) bool {
		a, b := rs[i].Deadline, rs[j].Deadline
		if (a == nil) != (b == nil) {
			return a != nil
		}
		if a != nil && !a.Equal(*b) {
			return a.Before(*b)
		}
		return rs[i].Score > rs[j].Score
	})
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
