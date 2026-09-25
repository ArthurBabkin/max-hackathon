package bot

import (
	"slices"
	"sort"
	"strings"

	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

// Коды ответов В1 «что нравится» и В2 «какая работа ближе»
// (docs/onboarding-v2/SPEC.md, 5.2–5.3).
var (
	interestCodes = []string{"code", "tasks", "world", "bio", "society"}
	workCodes     = []string{"build", "research", "health", "manage", "unknown"}
)

// maxInterests — сколько вариантов можно отметить в В1.
const maxInterests = 2

// dirWeight — веса направления по ответам В1 и В2 (SPEC 5.4).
type dirWeight struct {
	interest map[string]int
	work     map[string]int
}

var directionWeights = map[string]dirWeight{
	"napr-09-03-04": {map[string]int{"code": 2, "tasks": 1}, map[string]int{"build": 2}},
	"napr-01-03-02": {map[string]int{"code": 1, "tasks": 2}, map[string]int{"research": 1, "build": 1}},
	"napr-09-03-01": {map[string]int{"code": 2, "world": 1}, map[string]int{"build": 2}},
	"napr-10-03-01": {map[string]int{"code": 2, "tasks": 1}, map[string]int{"build": 1}},
	"napr-03-03-02": {map[string]int{"world": 2, "tasks": 1}, map[string]int{"research": 2}},
	"napr-03-03-01": {map[string]int{"tasks": 2, "world": 2}, map[string]int{"research": 2}},
	"napr-03-05-01": {map[string]int{"world": 2}, map[string]int{"research": 2}},
	"napr-16-03-01": {map[string]int{"world": 2}, map[string]int{"build": 2}},
	"napr-31-05-01": {map[string]int{"bio": 2}, map[string]int{"health": 2}},
	"napr-19-03-01": {map[string]int{"bio": 2, "world": 1}, map[string]int{"build": 1, "research": 1}},
	"napr-33-05-01": {map[string]int{"bio": 2}, map[string]int{"health": 1, "research": 1}},
	"napr-06-03-01": {map[string]int{"bio": 2, "world": 1}, map[string]int{"research": 2}},
	"napr-38-03-01": {map[string]int{"society": 2, "tasks": 1}, map[string]int{"manage": 1, "research": 1}},
	"napr-38-03-02": {map[string]int{"society": 2}, map[string]int{"manage": 2}},
	"napr-38-03-05": {map[string]int{"society": 1, "code": 1}, map[string]int{"manage": 1, "build": 1}},
	"napr-38-03-04": {map[string]int{"society": 2}, map[string]int{"manage": 2}},
}

// suggestDirections — 2–3 направления по ответам В1, В2 и любимым
// предметам (SPEC 5.4). Счёт — веса интересов и работы плюс 1 за каждый
// любимый предмет среди ключевых. Берутся до трёх со счётом от 3; если
// таких меньше двух — два лучших с ненулевым счётом. При равенстве —
// порядок справочника. Направление без веса в таблице (новое в
// справочнике) получает только баллы за предметы. Детерминированно, без LLM.
func suggestDirections(interests []string, work string, subjects []string, dirs []store.Direction) []string {
	type scored struct {
		id    string
		score int
	}
	// Порядок справочника — по коду направления (ОКСО), а не по названию.
	dirs = slices.Clone(dirs)
	slices.SortStableFunc(dirs, func(a, b store.Direction) int { return strings.Compare(a.ID, b.ID) })
	var all []scored
	for _, d := range dirs {
		w := directionWeights[d.ID]
		s := w.work[work]
		for _, i := range interests {
			s += w.interest[i]
		}
		for _, c := range d.SubjectCodes {
			if slices.Contains(subjects, c) {
				s++
			}
		}
		all = append(all, scored{d.ID, s})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].score > all[j].score })
	var out []string
	for _, x := range all {
		if x.score >= 3 && len(out) < 3 {
			out = append(out, x.id)
		}
	}
	if len(out) >= 2 {
		return out
	}
	out = nil
	for _, x := range all {
		if x.score > 0 && len(out) < 2 {
			out = append(out, x.id)
		}
	}
	return out
}
