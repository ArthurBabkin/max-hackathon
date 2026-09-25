package stages

import (
	"errors"
	"maps"
	"slices"
	"time"
)

// Итоги этапа. «Прошёл» ведёт дальше, остальные заканчивают участие и
// закрывают олимпиаду: следующие этапы серые, напоминаний больше нет.
const (
	Passed      = "passed"
	Failed      = "failed"
	Winner      = "winner"
	Prizer      = "prizer"
	Participant = "participant" // финал без диплома
)

// Статус пункта трекера и итог участия.
const (
	StatusOpen     = "open"     // нужно зарегистрироваться
	StatusActive   = "active"   // участвует
	StatusFinished = "finished" // участие закончено

	OutcomeMissed  = "missed"  // регистрация закрылась без отметки
	OutcomeUnknown = "unknown" // сезон прошёл, итог не отмечен
)

var (
	ErrUnknownStage = errors.New("этап не из этой олимпиады")
	ErrNotAllowed   = errors.New("такую отметку у этапа поставить нельзя")
	ErrConflict     = errors.New("отметка противоречит другим отметкам")
)

// Mark — отметка ученика у одного этапа.
type Mark struct {
	Registered bool
	Result     string
}

// Progress — отметки по олимпиаде. Первую регистрацию хранит сам пункт
// трекера (registered_at): её же ставит галочка «Я зарегистрировался» и
// кнопка в напоминании. У олимпиады без этапа-регистрации это «участвую».
// Остальные отметки — по этапам.
type Progress struct {
	Registered bool
	Marks      map[string]Mark
}

func (p Progress) mark(s Stage) Mark { return p.Marks[s.ID] }

// FirstRegistration — индекс первого этапа-регистрации, -1 — такого нет.
// Этапы должны быть отсортированы (Sort).
func FirstRegistration(st []Stage) int {
	return slices.IndexFunc(st, func(s Stage) bool { return RegistrationLike(s.Kind) })
}

// RegisteredOn — отмечена ли регистрация на этап i.
func RegisteredOn(st []Stage, p Progress, i int) bool {
	if i == FirstRegistration(st) {
		return p.Registered
	}
	return RegistrationLike(st[i].Kind) && p.mark(st[i]).Registered
}

// Results — какие итоги бывают у этапа i. У регистрации итогов нет;
// последний заключительный заканчивается дипломом или без него; остальные
// — «прошёл дальше» или «не прошёл». Если финала в данных ещё нет,
// последний отборочный тоже ведёт дальше.
func Results(st []Stage, i int) []string {
	switch {
	case st[i].Kind == "registration":
		return nil
	case st[i].Kind == "final" && !slices.ContainsFunc(st[i+1:], func(s Stage) bool { return s.Kind == "final" }):
		return []string{Winner, Prizer, Participant}
	default:
		return []string{Passed, Failed}
	}
}

// Closing — итог заканчивает участие.
func Closing(result string) bool { return result != "" && result != Passed }

// ClosedAt — индекс этапа с закрывающим итогом, -1 — олимпиада не закрыта.
func ClosedAt(st []Stage, p Progress) int {
	return slices.IndexFunc(st, func(s Stage) bool { return Closing(p.mark(s).Result) })
}

// Locked — этап после закрывающего итога: отмечать его нельзя.
func Locked(st []Stage, p Progress, i int) bool {
	c := ClosedAt(st, p)
	return c >= 0 && i > c
}

// done — с этапом всё: регистрация отмечена или итог известен.
func done(st []Stage, p Progress, i int) bool {
	return p.mark(st[i]).Result != "" || (RegistrationLike(st[i].Kind) && RegisteredOn(st, p, i))
}

// Settled — об этапе больше не напоминать и не показывать его срок в
// календаре: он отмечен или идёт после закрывающего итога.
func Settled(st []Stage, p Progress, i int) bool {
	return done(st, p, i) || Locked(st, p, i)
}

// End — когда этап заканчивается: конец окна, иначе срок, иначе начало.
func End(s Stage) *time.Time {
	switch {
	case s.EndsAt != nil:
		return s.EndsAt
	case s.DeadlineAt != nil:
		return s.DeadlineAt
	default:
		return s.StartsAt
	}
}

// NeedsResult — ждём ли итог этапа i: ученик участвует, олимпиада не
// закрыта, итога у этапа нет и позже итогов тоже нет (иначе этот уже
// неважен). Время не учитывается — по нему планируется вопрос бота.
func NeedsResult(st []Stage, p Progress, i int) bool {
	if !p.Registered || len(Results(st, i)) == 0 || ClosedAt(st, p) >= 0 {
		return false
	}
	return !slices.ContainsFunc(st[i:], func(s Stage) bool { return p.mark(s).Result != "" })
}

// Asking — этап, итог которого пора отметить: последний закончившийся из
// тех, что ждут итога. -1 — спрашивать не о чем.
func Asking(st []Stage, p Progress, now time.Time) int {
	for i := len(st) - 1; i >= 0; i-- {
		if end := End(st[i]); end != nil && end.Before(now) && NeedsResult(st, p, i) {
			return i
		}
	}
	return -1
}

// Status — где ученик в олимпиаде и чем она кончилась.
func Status(st []Stage, p Progress, now time.Time) (status, outcome string) {
	if c := ClosedAt(st, p); c >= 0 {
		return StatusFinished, p.mark(st[c]).Result
	}
	if !p.Registered {
		if Joinable(firstFrom(st, FirstRegistration(st)), now) {
			return StatusOpen, ""
		}
		return StatusFinished, OutcomeMissed
	}
	// Этапов в данных нет — отмеченное участие продолжается.
	if len(st) == 0 || Current(st, p, now) >= 0 {
		return StatusActive, ""
	}
	// Прошёл дальше последний известный этап — следующий ещё не опубликован.
	if len(st) > 0 && p.mark(st[len(st)-1]).Result == Passed {
		return StatusActive, ""
	}
	return StatusFinished, OutcomeUnknown
}

// firstFrom — этапы начиная с первой регистрации, а без неё — все.
func firstFrom(st []Stage, i int) []Stage {
	if i < 0 {
		return st
	}
	return st[i:]
}

// started — этап уже идёт или прошёл: итог раньше не отметить.
func started(s Stage, now time.Time) bool {
	return s.StartsAt == nil || !s.StartsAt.After(now)
}

// Apply ставит отметку m этапу stageID и возвращает новые отметки; вход не
// меняется. Итог подразумевает регистрацию. Противоречия — ErrConflict:
// отметка после закрывающего итога, закрывающий итог при отметках дальше,
// снятие регистрации, на которой держатся другие отметки.
func Apply(st []Stage, p Progress, stageID string, m Mark, now time.Time) (Progress, error) {
	i := slices.IndexFunc(st, func(s Stage) bool { return s.ID == stageID })
	if i < 0 {
		return p, ErrUnknownStage
	}
	if m.Result != "" && (!slices.Contains(Results(st, i), m.Result) || !started(st[i], now)) {
		return p, ErrNotAllowed
	}
	if Locked(st, p, i) {
		return p, ErrConflict
	}
	later := slices.ContainsFunc(st[i+1:], func(s Stage) bool {
		x := p.mark(s)
		return x.Registered || x.Result != ""
	})
	if Closing(m.Result) && later {
		return p, ErrConflict
	}
	first := FirstRegistration(st)
	registered := RegistrationLike(st[i].Kind) && m.Registered
	if RegisteredOn(st, p, i) && !registered && m.Result == "" {
		anyResult := slices.ContainsFunc(st, func(s Stage) bool { return s.ID != stageID && p.mark(s).Result != "" })
		if later || (i == first && anyResult) {
			return p, ErrConflict
		}
	}

	out := Progress{Registered: p.Registered, Marks: maps.Clone(p.Marks)}
	if out.Marks == nil {
		out.Marks = map[string]Mark{}
	}
	if i == first {
		out.Registered = registered || m.Result != ""
		registered = false // первая регистрация живёт в пункте трекера
	}
	// Итог и регистрация на следующий этап подразумевают участие.
	if m.Result != "" || registered {
		out.Registered = true
	}
	if x := (Mark{Registered: registered, Result: m.Result}); x == (Mark{}) {
		delete(out.Marks, stageID)
	} else {
		out.Marks[stageID] = x
	}
	return out, nil
}
