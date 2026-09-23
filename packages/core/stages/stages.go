// Package stages — правила этапов олимпиады, общие для карточки, трекера,
// календаря, главной и напоминаний: какой этап текущий, что уже прошло и
// как подписать срок по-русски.
package stages

import (
	"fmt"
	"sort"
	"time"
)

type Stage struct {
	ID         string
	Kind       string // registration | qualifying | final | school | municipal | regional
	Title      string
	StartsAt   *time.Time
	EndsAt     *time.Time
	DeadlineAt *time.Time
	IsOnline   bool
	IsDemo     bool
}

// RegistrationLike — этапы, которые закрывает отметка «зарегистрирован»:
// регистрация перечневой олимпиады и школьный этап ВсОШ, на который
// записываются в школе. После отметки их напоминания отменяются (ТЗ §6.3).
func RegistrationLike(kind string) bool {
	return kind == "registration" || kind == "school"
}

// Sort упорядочивает этапы по началу, затем по сроку. Этапы без дат — в конце.
func Sort(st []Stage) {
	key := func(s Stage) (time.Time, bool) {
		if s.StartsAt != nil {
			return *s.StartsAt, true
		}
		if s.DeadlineAt != nil {
			return *s.DeadlineAt, true
		}
		return time.Time{}, false
	}
	sort.SliceStable(st, func(i, j int) bool {
		a, okA := key(st[i])
		b, okB := key(st[j])
		if okA != okB {
			return okA
		}
		if !a.Equal(b) {
			return a.Before(b)
		}
		return deadlineOrZero(st[i]).Before(deadlineOrZero(st[j]))
	})
}

func deadlineOrZero(s Stage) time.Time {
	if s.DeadlineAt == nil {
		return time.Time{}
	}
	return *s.DeadlineAt
}

// passed — срок этапа уже в прошлом. Этап без срока не проходит никогда:
// о нём просто нечего сказать.
func passed(s Stage, now time.Time) bool {
	return s.DeadlineAt != nil && s.DeadlineAt.Before(now)
}

// Current — индекс текущего этапа: первого, срок которого ещё не прошёл.
// После отметки «зарегистрирован» этапы-регистрации пропускаются — иначе
// карточка писала бы «Дальше: Регистрация» уже после регистрации.
// -1 — впереди ничего нет. Этапы должны быть отсортированы (Sort).
func Current(st []Stage, registered bool, now time.Time) int {
	for i, s := range st {
		if registered && RegistrationLike(s.Kind) {
			continue
		}
		if !passed(s, now) {
			return i
		}
	}
	return -1
}

// States — состояние каждого этапа для таймлайна (F21): до текущего — past,
// текущий — current, после — future. Если текущего нет, всё past.
func States(st []Stage, registered bool, now time.Time) []string {
	cur := Current(st, registered, now)
	out := make([]string, len(st))
	for i := range st {
		switch {
		case cur == -1 || i < cur:
			out[i] = "past"
		case i == cur:
			out[i] = "current"
		default:
			out[i] = "future"
		}
	}
	return out
}

// AllPassed — у профиля есть этапы, и все они в прошлом. Такой профиль
// исключается из подбора (ТЗ §6.1); профиль без этапов — нет.
func AllPassed(st []Stage, now time.Time) bool {
	if len(st) == 0 {
		return false
	}
	for _, s := range st {
		if !passed(s, now) {
			return false
		}
	}
	return true
}

var monthsGen = [...]string{"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря"}

// Day — «22 октября».
func Day(t time.Time) string { return fmt.Sprintf("%d %s", t.Day(), monthsGen[t.Month()-1]) }

// Subtitle — подпись под этапом: «до 22 октября, онлайн» для окна с
// последним днём, «20–22 февраля» для очного этапа. Этап, у которого
// организатор назвал только крайний день, тоже «до …». Даты — в зоне loc.
func Subtitle(s Stage, loc *time.Location) string {
	var text string
	switch {
	case (s.Kind == "registration" || s.Kind == "qualifying" || s.Kind == "school") && s.DeadlineAt != nil:
		text = "до " + Day(s.DeadlineAt.In(loc))
	case s.StartsAt == nil && s.EndsAt != nil:
		text = "до " + Day(s.EndsAt.In(loc))
	case s.StartsAt != nil:
		start := s.StartsAt.In(loc)
		end := start
		if s.EndsAt != nil {
			end = s.EndsAt.In(loc)
		}
		switch {
		case start.Year() == end.Year() && start.YearDay() == end.YearDay():
			text = Day(start)
		case start.Month() == end.Month() && start.Year() == end.Year():
			text = fmt.Sprintf("%d–%d %s", start.Day(), end.Day(), monthsGen[start.Month()-1])
		default:
			text = Day(start) + " — " + Day(end)
		}
	default:
		return ""
	}
	if s.IsOnline {
		text += ", онлайн"
	}
	return text
}
