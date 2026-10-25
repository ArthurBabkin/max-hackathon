package bot

import (
	"slices"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

var rankDirs = []store.Direction{
	{ID: "biz", Name: "Бизнес-информатика", SubjectCodes: []string{"econ", "soc", "math"}},
	{ID: "bio", Name: "Биология", SubjectCodes: []string{"bio", "chem"}},
	{ID: "ivt", Name: "Информатика и вычислительная техника", SubjectCodes: []string{"inf", "math"}},
	{ID: "pmi", Name: "Прикладная математика и информатика", SubjectCodes: []string{"inf", "math"}},
	{ID: "phys", Name: "Физика", SubjectCodes: []string{"phys", "math"}},
}

func ids[T any](xs []T, id func(T) string) []string {
	out := []string{}
	for _, x := range xs {
		out = append(out, id(x))
	}
	return out
}

func dirIDs(ds []store.Direction) []string {
	return ids(ds, func(d store.Direction) string { return d.ID })
}

// Сначала все ключевые предметы, потом часть; при равенстве — где больше
// вузов, потом порядок справочника.
func TestRankDirections(t *testing.T) {
	got := dirIDs(rankDirections(rankDirs, []string{"inf", "math", "soc"}, map[string]int{"pmi": 7, "ivt": 5}))
	if want := []string{"pmi", "ivt", "biz", "phys", "bio"}; !slices.Equal(got, want) {
		t.Fatalf("порядок: %v, ждали %v", got, want)
	}
}

func TestDirectionChoice(t *testing.T) {
	uniDirs := map[string][]string{"hse": {"biz", "ivt"}, "med": {"bio"}}
	cases := []struct {
		name string
		d    store.Dialog
		want []string
		text string
	}{
		{"вузов нет — подходящие под предметы", store.Dialog{Step: stepDirection, Draft: store.Draft{SubjectCodes: []string{"inf", "math"}}},
			[]string{"ivt", "pmi", "phys", "biz"}, "bot.direction.ask"},
		{"вузы выбраны — их направления", store.Dialog{Step: stepDirection, Draft: store.Draft{SubjectCodes: []string{"inf", "math"}, UniversityIDs: []string{"hse"}}},
			[]string{"ivt", "biz"}, "bot.direction.askVuz"},
		{"в вузах под предметы ничего — все их направления", store.Dialog{Step: stepDirection, Draft: store.Draft{SubjectCodes: []string{"inf"}, UniversityIDs: []string{"med"}}},
			[]string{"bio"}, "bot.direction.askVuzAll"},
		{"под предметы ничего — все", store.Dialog{Step: stepDirection, Draft: store.Draft{SubjectCodes: []string{"astro"}}},
			[]string{"biz", "bio", "ivt", "pmi", "phys"}, "bot.direction.askAll"},
		{"«Показать все» — остальные в конце", store.Dialog{Step: stepDirection, Draft: store.Draft{SubjectCodes: []string{"inf", "math"}, UniversityIDs: []string{"hse"}, AllDirections: true}},
			[]string{"ivt", "biz", "pmi", "phys", "bio"}, "bot.direction.askAll"},
		{"предложенные — первыми", store.Dialog{Step: stepSuggest, Draft: store.Draft{SubjectCodes: []string{"inf", "math"}, Suggested: []string{"phys", "pmi"}, AllDirections: true}},
			[]string{"phys", "pmi", "ivt", "biz", "bio"}, "bot.suggest.ask"},
	}
	for _, c := range cases {
		pool, text := directionChoice(c.d, rankDirs, uniDirs)
		if got := dirIDs(pool); !slices.Equal(got, c.want) || text != c.text {
			t.Errorf("%s: %v %s, ждали %v %s", c.name, got, text, c.want, c.text)
		}
	}
}

// Сначала вузы, где больше направлений под предметы целиком; при равенстве
// — где больше олимпиад дают льготу.
func TestRankUniversities(t *testing.T) {
	unis := []store.University{{ID: "eco"}, {ID: "med"}, {ID: "it", BenefitOlympiads: 1}, {ID: "it2", BenefitOlympiads: 5}, {ID: "one"}}
	uniDirs := map[string][]string{
		"eco": {"biz", "pmi", "phys"}, "med": {"bio"}, "it": {"ivt", "pmi"}, "it2": {"ivt", "pmi"}, "one": {"ivt"},
	}
	got := ids(rankUniversities(unis, uniDirs, rankDirs, []string{"inf", "math"}), func(u store.University) string { return u.ID })
	if want := []string{"it2", "it", "eco", "one", "med"}; !slices.Equal(got, want) {
		t.Fatalf("порядок вузов: %v, ждали %v", got, want)
	}
}
