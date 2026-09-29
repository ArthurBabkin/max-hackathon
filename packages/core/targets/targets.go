// Package targets — правило «мои цели»: по каким направлениям вуза ученик
// смотрит льготы. Выбор ученика важнее цели онбординга, цель важнее вуза
// целиком. Пакет чистый: направления вуза, выбор и цель передаются готовыми.
package targets

import (
	"slices"
	"strings"
)

// Covers — направление a покрывает b: коды равны, или один из них —
// укрупнённая группа XX.00.00 с той же группой XX (вуз, у которого в
// источнике только группа, — так было у Иннополиса до #87).
func Covers(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if isGroup(b) {
		a, b = b, a
	}
	return isGroup(a) && strings.HasPrefix(b, a[:3])
}

// isGroup — код укрупнённой группы: XX.00.00.
func isGroup(code string) bool {
	return len(code) == len("09.00.00") && strings.HasSuffix(code, ".00.00")
}

// Basis — на чём держится цель в вузе.
type Basis string

const (
	Chosen     Basis = "chosen"     // ученик сам выбрал направления вуза
	Goal       Basis = "goal"       // направления вуза, покрывающие цель онбординга
	University Basis = "university" // вуз целиком: ни выбора, ни совпадения с целью
)

// Offered — направление вуза: Status offered (льготы проверены) или to_check.
type Offered struct{ DirectionID, Code, Status string }

type Target struct {
	UniversityID string
	Basis        Basis
	DirectionIDs []string // пусто при University
	Unverified   bool     // все направления цели в этом вузе — to_check: «льготы уточняются»
}

// Resolve: 1) у вуза явно выбраны направления (chosen — id) → они, Chosen;
// 2) иначе направления вуза, которые покрывают хотя бы один код цели (goalCodes) → Goal;
// 3) иначе вуз целиком → University. Порядок DirectionIDs — как в offered.
//
// Выбранное направление, которого у вуза нет (выбор устарел после обновления
// данных), не считается: если не осталось ни одного, цель ищется дальше.
func Resolve(universityID string, offered []Offered, chosen []string, goalCodes []string) Target {
	t := Target{UniversityID: universityID, Basis: University}
	var picked []Offered
	for _, o := range offered {
		if slices.Contains(chosen, o.DirectionID) {
			picked = append(picked, o)
		}
	}
	if len(picked) > 0 {
		t.Basis = Chosen
	} else {
		for _, o := range offered {
			if slices.ContainsFunc(goalCodes, func(g string) bool { return Covers(o.Code, g) }) {
				picked = append(picked, o)
			}
		}
		if len(picked) > 0 {
			t.Basis = Goal
		}
	}
	if len(picked) == 0 {
		return t
	}
	t.Unverified = true
	for _, o := range picked {
		t.DirectionIDs = append(t.DirectionIDs, o.DirectionID)
		if o.Status != "to_check" {
			t.Unverified = false
		}
	}
	return t
}
