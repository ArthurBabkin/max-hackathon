package match

import (
	"math"
	"testing"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
)

var now = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

func in(days float64) *time.Time {
	t := now.Add(time.Duration(days * 24 * float64(time.Hour)))
	return &t
}

func lvl(s string) *string { return &s }

// base — кандидат, у которого все факторы нулевые: сравнивая с ним,
// проверяем каждый фактор отдельно.
func base() Candidate {
	return Candidate{ProfileID: "p", Kind: "perechen", SubjectCode: "chem",
		Stages: []stages.Stage{{Kind: "final", DeadlineAt: in(200)}}}
}

// student — опытный ученик 11 класса: уровень и льгота весят полностью.
var student = Student{DirectionSubjects: []string{"inf", "math"}, RegionCode: "16", Experience: "region", Grade: 11}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestScore_EachFactorInIsolation(t *testing.T) {
	zero := Score(base(), student, DefaultWeights, now).Score
	deadlineZero := DefaultWeights.Deadline * 30.0 / 200.0 // срок через 200 дней убывает до 30/200
	if !near(zero, deadlineZero) {
		t.Fatalf("базовый кандидат: %v, ожидали только хвост срока %v", zero, deadlineZero)
	}
	cases := []struct {
		name   string
		mutate func(*Candidate)
		want   float64
	}{
		{"профиль совпадает с направлением", func(c *Candidate) { c.SubjectCode = "inf" }, 0.30},
		{"БВИ в вузе ученика", func(c *Candidate) { c.BestBenefit = "bvi" }, 0.30},
		{"БВИ победителям", func(c *Candidate) { c.BestBenefit = "bvi_winners" }, 0.30 * 0.8},
		{"100 баллов", func(c *Candidate) { c.BestBenefit = "score100" }, 0.30 * 0.6},
		{"I уровень", func(c *Candidate) { c.Level = lvl("I") }, 0.15},
		{"II уровень", func(c *Candidate) { c.Level = lvl("II") }, 0.15 * 0.6},
		{"III уровень", func(c *Candidate) { c.Level = lvl("III") }, 0.15 * 0.3},
		{"онлайн-отбор", func(c *Candidate) {
			c.Stages = append(c.Stages, stages.Stage{Kind: "qualifying", IsOnline: true, DeadlineAt: in(300)})
		}, 0.05},
		{"финал в регионе ученика", func(c *Candidate) { c.FinalRegionCode = lvl("16") }, 0.05},
	}
	for _, tc := range cases {
		c := base()
		tc.mutate(&c)
		got := Score(c, student, DefaultWeights, now).Score - zero
		if !near(got, tc.want) {
			t.Errorf("%s: прибавка %.4f, ожидали %.4f", tc.name, got, tc.want)
		}
	}
}

func TestScore_DeadlineFactor(t *testing.T) {
	cases := []struct {
		days float64
		want float64
	}{{10, 1}, {3, 1}, {30, 1}, {60, 0.5}, {1, 0.5}}
	for _, tc := range cases {
		c := base()
		c.Stages = []stages.Stage{{Kind: "registration", DeadlineAt: in(tc.days)}}
		r := Score(c, student, DefaultWeights, now)
		if !near(r.Factors[Deadline], DefaultWeights.Deadline*tc.want) {
			t.Errorf("срок через %v дн.: %v, ожидали %v", tc.days, r.Factors[Deadline], DefaultWeights.Deadline*tc.want)
		}
	}
	noStages := base()
	noStages.Stages = nil
	if r := Score(noStages, student, DefaultWeights, now); r.Factors[Deadline] != 0 || r.Deadline != nil {
		t.Errorf("без этапов фактор срока 0: %+v", r)
	}
}

func TestScore_TopFactorsAreTheTwoStrongest(t *testing.T) {
	c := base()
	c.SubjectCode, c.BestBenefit, c.Level = "inf", "score100", lvl("I")
	r := Score(c, student, DefaultWeights, now)
	if len(r.Top) != 2 || r.Top[0] != Direction || r.Top[1] != Benefit {
		t.Fatalf("два сильнейших: направление 0.30 и льгота 0.18, получили %v", r.Top)
	}
}

func cand(id, kind, subject string, level *string, deadlineDays float64) Candidate {
	return Candidate{ProfileID: id, Kind: kind, SubjectCode: subject, Level: level,
		Stages: []stages.Stage{{Kind: "qualifying", DeadlineAt: in(deadlineDays), IsOnline: true}}}
}

func TestRecommend_TopFivePlusVsoshSortedByDeadline(t *testing.T) {
	var cs []Candidate
	for i, d := range []float64{40, 35, 20, 25, 45, 50, 55} {
		cs = append(cs, cand(string(rune('a'+i)), "perechen", "inf", lvl("I"), d))
	}
	cs[6].BestBenefit = "bvi" // самый дальний срок, но лучший скор — попадёт в топ
	cs = append(cs,
		cand("vsosh-inf", "vsosh", "inf", nil, 60),
		cand("vsosh-chem", "vsosh", "chem", nil, 5), // предмет не связан с целью
		cand("other", "other", "inf", nil, 10),
	)
	passed := cand("old", "perechen", "inf", lvl("I"), -3)
	cs = append(cs, passed)

	items, outside := Recommend(cs, student, DefaultWeights, now, "all")
	ids := []string{}
	for _, r := range items {
		ids = append(ids, r.ProfileID)
	}
	if len(items) != 6 {
		t.Fatalf("топ-5 перечня и ВсОШ по предмету цели: %v", ids)
	}
	for i := 1; i < len(items); i++ {
		if items[i].Deadline.Before(*items[i-1].Deadline) {
			t.Fatalf("список отсортирован по близости срока: %v", ids)
		}
	}
	has := func(id string) bool {
		for _, x := range ids {
			if x == id {
				return true
			}
		}
		return false
	}
	if !has("g") || !has("vsosh-inf") || has("vsosh-chem") || has("old") || has("other") {
		t.Fatalf("состав: %v", ids)
	}
	if len(outside) != 1 || outside[0].ProfileID != "other" {
		t.Fatalf("вне перечня — отдельно: %v", outside)
	}
}

func TestRecommend_Filters(t *testing.T) {
	cs := []Candidate{
		cand("i-soon", "perechen", "inf", lvl("I"), 10),
		cand("ii-late", "perechen", "inf", lvl("II"), 40),
		{ProfileID: "offline", Kind: "perechen", SubjectCode: "inf", Level: lvl("III"),
			Stages: []stages.Stage{{Kind: "final", DeadlineAt: in(12)}}},
		cand("vsosh-inf", "vsosh", "inf", nil, 15),
	}
	ids := func(filter string) map[string]bool {
		items, _ := Recommend(cs, student, DefaultWeights, now, filter)
		out := map[string]bool{}
		for _, r := range items {
			out[r.ProfileID] = true
		}
		return out
	}
	if got := ids("all"); len(got) != 4 {
		t.Errorf("all: %v", got)
	}
	if got := ids("level1"); len(got) != 1 || !got["i-soon"] {
		t.Errorf("level1 — только I уровень: %v", got)
	}
	if got := ids("soon"); len(got) != 3 || got["ii-late"] {
		t.Errorf("soon — срок не дальше 16 дней: %v", got)
	}
	if got := ids("online"); got["offline"] || !got["i-soon"] {
		t.Errorf("online — только с онлайн-отбором: %v", got)
	}
}

func TestRecommend_OneProfilePerOlympiad(t *testing.T) {
	// У НТО пятнадцать профилей с предметом «Информатика»: без этого правила
	// они заняли бы весь топ-5. Остаётся профиль с лучшим скором.
	var cs []Candidate
	for i := 0; i < 6; i++ {
		c := cand(string(rune('a'+i)), "perechen", "inf", lvl("II"), 10)
		c.OlympiadID = "nto"
		cs = append(cs, c)
	}
	best := cand("nto-best", "perechen", "inf", lvl("I"), 10)
	best.OlympiadID = "nto"
	other := cand("x", "perechen", "math", lvl("III"), 10)
	other.OlympiadID = "other-olymp"
	items, _ := Recommend(append(cs, best, other), student, DefaultWeights, now, "all")
	var ids []string
	for _, r := range items {
		ids = append(ids, r.ProfileID)
	}
	if len(ids) != 2 || ids[0] != "nto-best" || ids[1] != "x" {
		t.Fatalf("items = %v, ждали [nto-best x]", ids)
	}
}

func TestRecommend_ProfileWithoutStagesStays(t *testing.T) {
	// 14 профилей из missing_in_C без дат: «данные уточняются», но из подбора
	// они не выпадают — срок просто не добавляет им очков.
	bare := Candidate{ProfileID: "bare", OlympiadID: "bare", Kind: "perechen", SubjectCode: "inf", Level: lvl("I")}
	dated := cand("dated", "perechen", "inf", lvl("I"), 10)
	items, _ := Recommend([]Candidate{bare, dated}, student, DefaultWeights, now, "all")
	if len(items) != 2 || items[0].ProfileID != "dated" || items[1].ProfileID != "bare" || items[1].Deadline != nil {
		t.Fatalf("профиль без этапов — в конце списка, без срока: %+v", items)
	}
}

func TestScore_LevelByExperience(t *testing.T) {
	score := func(exp, level string) float64 {
		c := base()
		c.Level = lvl(level)
		s := student
		s.Experience = exp
		return Score(c, s, DefaultWeights, now).Score
	}
	// Новичку II–III уровень выше I, опытному — наоборот.
	if !(score("none", "III") > score("none", "I") && score("none", "II") > score("none", "I")) {
		t.Errorf("новичок: I=%v II=%v III=%v", score("none", "I"), score("none", "II"), score("none", "III"))
	}
	if !(score("region", "I") > score("region", "II") && score("region", "II") > score("region", "III")) {
		t.Errorf("опытный: I=%v II=%v III=%v", score("region", "I"), score("region", "II"), score("region", "III"))
	}
	if !(score("school", "II") > score("school", "I") && score("school", "I") > score("school", "III")) {
		t.Errorf("школьный этап: I=%v II=%v III=%v", score("school", "I"), score("school", "II"), score("school", "III"))
	}
	if score("", "II") != score("none", "II") {
		t.Error("пустой опыт — как none")
	}
	// ВсОШ: опытному — как I уровень, новичку — как II.
	v := base()
	v.Kind = "vsosh"
	for exp, want := range map[string]float64{"region": 1.0, "none": 0.8, "school": 1.0} {
		s := student
		s.Experience = exp
		if got := Score(v, s, DefaultWeights, now).Factors[Level]; !near(got, DefaultWeights.Level*want) {
			t.Errorf("ВсОШ при опыте %s: %v", exp, got)
		}
	}
}

func TestScore_LevelFit(t *testing.T) {
	c := base()
	c.Level = lvl("III")
	s := student
	s.Experience = "none"
	if r := Score(c, s, DefaultWeights, now); !r.LevelFit {
		t.Error("III уровень новичку — под опыт")
	}
	c.Level = lvl("I")
	if r := Score(c, s, DefaultWeights, now); r.LevelFit {
		t.Error("I уровень новичку — не под опыт")
	}
	s.Experience = ""
	c.Level = lvl("III")
	if r := Score(c, s, DefaultWeights, now); r.LevelFit {
		t.Error("опыт не задан — причина по уровню, а не по опыту")
	}
}

func TestScore_BenefitByGrade(t *testing.T) {
	c := base()
	c.BestBenefit = "bvi"
	for grade, want := range map[int]float64{8: 0.15, 9: 0.15, 10: 0.30, 11: 0.30, 0: 0.30} {
		s := student
		s.Grade = grade
		if got := Score(c, s, DefaultWeights, now).Factors[Benefit]; !near(got, want) {
			t.Errorf("%d класс: льгота %v, ждали %v", grade, got, want)
		}
	}
}

func TestRecommend_AtMostTwoPerSubject(t *testing.T) {
	var cs []Candidate
	// Четыре сильных по информатике и три слабее — по другим предметам.
	for i := 0; i < 4; i++ {
		c := cand(string(rune('a'+i)), "perechen", "inf", lvl("I"), 10)
		c.BestBenefit = "bvi"
		cs = append(cs, c)
	}
	cs = append(cs, cand("m1", "perechen", "math", lvl("II"), 10), cand("p1", "perechen", "phys", lvl("III"), 10),
		cand("c1", "perechen", "chem", lvl("III"), 10))
	items, _ := Recommend(cs, student, DefaultWeights, now, "all")
	count := map[string]int{}
	for _, r := range items {
		count[r.SubjectCode]++
	}
	if len(items) != TopN || count["inf"] != 2 {
		t.Fatalf("топ-5 без трёх олимпиад одного предмета: %v", count)
	}
	// Других предметов нет — топ добирается тем, что есть.
	items, _ = Recommend(cs[:4], student, DefaultWeights, now, "all")
	if len(items) != 4 {
		t.Fatalf("один предмет — топ не пустеет: %d", len(items))
	}
}
