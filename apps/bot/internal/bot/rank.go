package bot

import (
	"cmp"
	"slices"

	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

// Порядок вариантов на шагах вузов и направлений (#129): по предметам
// ученика и справочнику направлений вузов. Детерминированно, без LLM.

// dirLimit — сколько направлений бот показывает за раз, дальше «Показать ещё».
const dirLimit = 5

// coverage — доля ключевых предметов направления среди предметов ученика:
// у ПМИ (информатика, математика) при информатике и математике — 1, у
// экономики (экономика, обществознание, математика) — 2/3.
func coverage(d store.Direction, subjects []string) float64 {
	if len(d.SubjectCodes) == 0 {
		return 0
	}
	n := 0
	for _, c := range d.SubjectCodes {
		if slices.Contains(subjects, c) {
			n++
		}
	}
	return float64(n) / float64(len(d.SubjectCodes))
}

// rankDirections — сначала направления, чьи ключевые предметы ученик
// сдаёт целиком, потом частично; при равенстве — где больше вузов
// (count), потом порядок справочника (по названию).
func rankDirections(dirs []store.Direction, subjects []string, count map[string]int) []store.Direction {
	out := slices.Clone(dirs)
	slices.SortStableFunc(out, func(a, b store.Direction) int {
		if c := cmp.Compare(coverage(b, subjects), coverage(a, subjects)); c != 0 {
			return c
		}
		return cmp.Compare(count[b.ID], count[a.ID])
	})
	return out
}

// directionChoice — направления шага в порядке показа и ключ вопроса.
// Выбраны вузы — направления этих вузов, подходящие под предметы; не
// нашлось — все направления этих вузов. Вузов нет — подходящие под
// предметы, не нашлось — все. На шаге suggest — сначала предложенные по
// интересам. «Показать все направления» дописывает остальные в конец.
func directionChoice(d store.Dialog, dirs []store.Direction, uniDirs map[string][]string) ([]store.Direction, string) {
	dr := d.Draft
	// В скольких вузах есть направление — среди выбранных, если они есть.
	count, offered := map[string]int{}, map[string]bool{}
	for uni, ids := range uniDirs {
		if len(dr.UniversityIDs) > 0 && !slices.Contains(dr.UniversityIDs, uni) {
			continue
		}
		for _, id := range ids {
			count[id]++
			offered[id] = true
		}
	}
	var pool []store.Direction
	text := "bot.direction.ask"
	switch {
	case d.Step == stepSuggest:
		for _, id := range dr.Suggested {
			if i := slices.IndexFunc(dirs, func(x store.Direction) bool { return x.ID == id }); i >= 0 {
				pool = append(pool, dirs[i])
			}
		}
		text = "bot.suggest.ask"
	case len(dr.UniversityIDs) > 0:
		var base []store.Direction
		for _, x := range dirs {
			if offered[x.ID] {
				base = append(base, x)
			}
		}
		pool, text = matching(base, dr.SubjectCodes, count), "bot.direction.askVuz"
		if len(pool) == 0 {
			pool, text = rankDirections(base, dr.SubjectCodes, count), "bot.direction.askVuzAll"
		}
	default:
		pool = matching(dirs, dr.SubjectCodes, count)
	}
	if len(pool) == 0 {
		pool, text = rankDirections(dirs, dr.SubjectCodes, count), "bot.direction.askAll"
	}
	if dr.AllDirections {
		for _, x := range rankDirections(dirs, dr.SubjectCodes, count) {
			if !slices.ContainsFunc(pool, func(y store.Direction) bool { return y.ID == x.ID }) {
				pool = append(pool, x)
			}
		}
		if d.Step != stepSuggest {
			text = "bot.direction.askAll"
		}
	}
	return pool, text
}

// matching — направления, где ученик сдаёт хотя бы один ключевой предмет,
// в порядке rankDirections.
func matching(dirs []store.Direction, subjects []string, count map[string]int) []store.Direction {
	var out []store.Direction
	for _, x := range rankDirections(dirs, subjects, count) {
		if coverage(x, subjects) > 0 {
			out = append(out, x)
		}
	}
	return out
}

// rankUniversities — сначала вузы, где больше направлений, подходящих под
// предметы целиком, потом — частично: доли покрытия направлений вуза по
// убыванию сравниваются по очереди. При равенстве — где больше олимпиад
// дают льготу, потом порядок справочника (по названию).
func rankUniversities(unis []store.University, uniDirs map[string][]string, dirs []store.Direction, subjects []string) []store.University {
	fit := map[string][]float64{}
	for _, u := range unis {
		var cov []float64
		for _, id := range uniDirs[u.ID] {
			if i := slices.IndexFunc(dirs, func(x store.Direction) bool { return x.ID == id }); i >= 0 {
				if c := coverage(dirs[i], subjects); c > 0 {
					cov = append(cov, c)
				}
			}
		}
		slices.SortFunc(cov, func(a, b float64) int { return cmp.Compare(b, a) })
		fit[u.ID] = cov
	}
	out := slices.Clone(unis)
	slices.SortStableFunc(out, func(a, b store.University) int {
		if c := slices.Compare(fit[b.ID], fit[a.ID]); c != 0 {
			return c
		}
		return cmp.Compare(b.BenefitOlympiads, a.BenefitOlympiads)
	})
	return out
}
