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
}

type Student struct {
	DirectionSubjects []string
	RegionCode        string
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
}

var benefitValue = map[string]float64{"bvi": 1, "bvi_winners": 0.8, "score100": 0.6}
var levelValue = map[string]float64{"I": 1, "II": 0.6, "III": 0.3}

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
	r.Factors[Benefit] = w.Benefit * benefitValue[c.BestBenefit]
	switch {
	case c.Kind == "vsosh":
		// У ВсОШ нет уровня перечня, а диплом заключительного этапа даёт
		// БВИ по профилю — по силе это не ниже I уровня.
		r.Factors[Level] = w.Level
	case c.Level != nil:
		r.Factors[Level] = w.Level * levelValue[*c.Level]
	}
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

// Recommend — подбор для экрана C3. Профили, у которых все этапы в
// прошлом, исключаются. Из перечня — топ-5 по скору; ВсОШ — всегда, если
// предмет связан с целью; вне перечня — отдельным списком. Оба списка
// отсортированы по близости срока.
func Recommend(cands []Candidate, s Student, w Weights, now time.Time, filter string) (items, outside []Result) {
	var perechen []Result
	for _, c := range cands {
		if stages.AllPassed(c.Stages, now) {
			continue
		}
		r := Score(c, s, w, now)
		if !passes(r, filter, now) {
			continue
		}
		switch c.Kind {
		case "other":
			outside = append(outside, r)
		case "vsosh":
			if len(s.DirectionSubjects) == 0 || contains(s.DirectionSubjects, c.SubjectCode) {
				items = append(items, r)
			}
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
	perechen = bestPerOlympiad(perechen)
	if len(perechen) > TopN {
		perechen = perechen[:TopN]
	}
	items = append(items, perechen...)
	byDeadline(items)
	byDeadline(outside)
	return items, outside
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
