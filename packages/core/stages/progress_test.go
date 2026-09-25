package stages

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// Олимпиада с двумя регистрациями, как Всесибирская по биологии в сиде:
// регистрация на отборочный, отборочный, регистрация на заключительный, финал.
func twoRegs() []Stage {
	return []Stage{
		{ID: "r1", Kind: "registration", Title: "Регистрация на отборочный этап", StartsAt: at(2026, 9, 1, 0),
			EndsAt: at(2026, 10, 30, 23), DeadlineAt: at(2026, 10, 30, 23)},
		{ID: "q", Kind: "qualifying", Title: "Отборочный этап", StartsAt: at(2026, 11, 1, 0),
			EndsAt: at(2026, 11, 1, 23), DeadlineAt: at(2026, 11, 1, 0)},
		{ID: "r2", Kind: "registration", Title: "Регистрация на заключительный этап", StartsAt: at(2027, 2, 1, 0),
			EndsAt: at(2027, 3, 5, 23), DeadlineAt: at(2027, 3, 5, 23)},
		{ID: "f", Kind: "final", Title: "Заключительный этап", StartsAt: at(2027, 3, 7, 0),
			EndsAt: at(2027, 3, 7, 23), DeadlineAt: at(2027, 3, 7, 0)},
	}
}

// ВсОШ: на школьный этап записываются в школе, он же первая «регистрация».
func vsosh() []Stage {
	return []Stage{
		{ID: "s", Kind: "school", StartsAt: at(2026, 9, 15, 0), EndsAt: at(2026, 10, 20, 23), DeadlineAt: at(2026, 10, 20, 23)},
		{ID: "m", Kind: "municipal", StartsAt: at(2026, 11, 20, 0), EndsAt: at(2026, 11, 20, 23), DeadlineAt: at(2026, 11, 20, 0)},
		{ID: "rg", Kind: "regional", StartsAt: at(2027, 1, 20, 0), EndsAt: at(2027, 1, 21, 23), DeadlineAt: at(2027, 1, 20, 0)},
		{ID: "f", Kind: "final", StartsAt: at(2027, 4, 1, 0), EndsAt: at(2027, 4, 7, 23), DeadlineAt: at(2027, 4, 1, 0)},
	}
}

func marks(kv ...any) Progress {
	p := Progress{Registered: true, Marks: map[string]Mark{}}
	for i := 0; i < len(kv); i += 2 {
		p.Marks[kv[i].(string)] = kv[i+1].(Mark)
	}
	return p
}

func TestResults_ByStageKind(t *testing.T) {
	st := twoRegs()
	if r := Results(st, 0); len(r) != 0 {
		t.Fatalf("у регистрации итогов нет, получили %v", r)
	}
	if r := Results(st, 1); !slices.Equal(r, []string{Passed, Failed}) {
		t.Fatalf("отборочный: прошёл / не прошёл, получили %v", r)
	}
	if r := Results(st, 3); !slices.Equal(r, []string{Winner, Prizer, Participant}) {
		t.Fatalf("последний финал: победитель / призёр / без диплома, получили %v", r)
	}
	if r := Results(vsosh(), 0); !slices.Equal(r, []string{Passed, Failed}) {
		t.Fatalf("школьный этап ВсОШ — и регистрация, и итог, получили %v", r)
	}
	// У химии МГУ два «заключительных»: отборочный тур финала и финальные туры.
	twoFinals := []Stage{{ID: "a", Kind: "final", StartsAt: at(2026, 12, 19, 0)}, {ID: "b", Kind: "final", StartsAt: at(2027, 3, 3, 0)}}
	if r := Results(twoFinals, 0); !slices.Equal(r, []string{Passed, Failed}) {
		t.Fatalf("не последний финал — прошёл дальше или нет, получили %v", r)
	}
	// У НТО финала в данных ещё нет: после отборочного — «прошёл дальше».
	if r := Results(nto()[:2], 1); !slices.Equal(r, []string{Passed, Failed}) {
		t.Fatalf("последний этап не финал — прошёл / не прошёл, получили %v", r)
	}
}

func TestFirstRegistration(t *testing.T) {
	if FirstRegistration(twoRegs()) != 0 || FirstRegistration(vsosh()) != 0 {
		t.Fatal("первая регистрация — первый этап-регистрация по порядку")
	}
	if FirstRegistration(twoRegs()[1:]) != 1 {
		t.Fatal("регистрация на финал становится первой, если раньше регистраций нет")
	}
	if FirstRegistration([]Stage{{Kind: "qualifying"}, {Kind: "final"}}) != -1 {
		t.Fatal("без регистраций — -1")
	}
}

func TestRegisteredOn(t *testing.T) {
	st := twoRegs()
	p := marks("r2", Mark{Registered: true})
	if !RegisteredOn(st, p, 0) || !RegisteredOn(st, p, 2) {
		t.Fatal("первая регистрация — registered_at, вторая — отметка этапа")
	}
	p = Progress{Marks: map[string]Mark{"r1": {Registered: true}}}
	if RegisteredOn(st, p, 0) {
		t.Fatal("первая регистрация берётся только из registered_at")
	}
}

func TestCurrent_SkipsMarkedStages(t *testing.T) {
	st := twoRegs()
	now := *at(2026, 10, 1, 12)
	if got := Current(st, Progress{}, now); got != 0 {
		t.Fatalf("без отметок текущая — регистрация, получили %d", got)
	}
	if got := Current(st, marks(), now); got != 1 {
		t.Fatalf("после регистрации — отборочный, получили %d", got)
	}
	later := *at(2027, 2, 10, 12)
	if got := Current(st, marks("q", Mark{Result: Passed}), later); got != 2 {
		t.Fatalf("прошёл отборочный — регистрация на финал, получили %d", got)
	}
	if got := Current(st, marks("q", Mark{Result: Passed}, "r2", Mark{Registered: true}), later); got != 3 {
		t.Fatalf("зарегистрирован на финал — финал, получили %d", got)
	}
	if got := Current(st, marks("q", Mark{Result: Failed}), later); got != -1 {
		t.Fatalf("не прошёл — олимпиада закрыта, текущего этапа нет, получили %d", got)
	}
}

func TestStates_ClosedIsAllPast(t *testing.T) {
	got := States(twoRegs(), marks("q", Mark{Result: Failed}), *at(2027, 2, 10, 12))
	for _, s := range got {
		if s != "past" {
			t.Fatalf("после закрывающего итога всё серое: %v", got)
		}
	}
}

func TestClosedAtAndLocked(t *testing.T) {
	st := twoRegs()
	p := marks("q", Mark{Result: Failed})
	if ClosedAt(st, p) != 1 {
		t.Fatalf("закрывает этап с итогом «не прошёл», получили %d", ClosedAt(st, p))
	}
	if Locked(st, p, 1) || !Locked(st, p, 2) || !Locked(st, p, 3) {
		t.Fatal("заблокированы только этапы после закрывающего")
	}
	if ClosedAt(st, marks("q", Mark{Result: Passed})) != -1 {
		t.Fatal("«прошёл» не закрывает")
	}
	for _, r := range []string{Failed, Winner, Prizer, Participant} {
		if !Closing(r) {
			t.Errorf("%s закрывает олимпиаду", r)
		}
	}
	if Closing(Passed) || Closing("") {
		t.Error("«прошёл» и пустой итог не закрывают")
	}
}

func TestSettled(t *testing.T) {
	st := twoRegs()
	p := marks("q", Mark{Result: Passed})
	if !Settled(st, p, 0) || !Settled(st, p, 1) || Settled(st, p, 2) || Settled(st, p, 3) {
		t.Fatal("отмеченные этапы закрыты, остальные — нет")
	}
	p = marks("q", Mark{Result: Failed})
	if !Settled(st, p, 2) || !Settled(st, p, 3) {
		t.Fatal("после закрывающего итога напоминать не о чем")
	}
	if Settled(st, Progress{}, 0) {
		t.Fatal("регистрация без отметки ждёт")
	}
}

func TestEnd(t *testing.T) {
	s := Stage{StartsAt: at(2026, 11, 1, 0), DeadlineAt: at(2026, 11, 2, 0), EndsAt: at(2026, 11, 3, 0)}
	if !End(s).Equal(*at(2026, 11, 3, 0)) {
		t.Fatal("конец этапа — ends_at")
	}
	s.EndsAt = nil
	if !End(s).Equal(*at(2026, 11, 2, 0)) {
		t.Fatal("без ends_at — срок")
	}
	s.DeadlineAt = nil
	if !End(s).Equal(*at(2026, 11, 1, 0)) {
		t.Fatal("без срока — начало")
	}
	if End(Stage{}) != nil {
		t.Fatal("без дат — nil")
	}
}

func TestNeedsResultAndAsking(t *testing.T) {
	st := twoRegs()
	if NeedsResult(st, Progress{}, 1) {
		t.Fatal("без регистрации итог не спрашиваем")
	}
	if !NeedsResult(st, marks(), 1) || NeedsResult(st, marks(), 0) {
		t.Fatal("итог ждём у отборочного, у регистрации — нет")
	}
	if NeedsResult(st, marks("q", Mark{Result: Passed}), 1) {
		t.Fatal("отмеченный итог не спрашиваем")
	}
	if NeedsResult(st, marks("f", Mark{Result: Prizer}), 1) {
		t.Fatal("есть итог позже — про ранний этап не спрашиваем")
	}
	if NeedsResult(st, marks("q", Mark{Result: Failed}), 3) {
		t.Fatal("после закрывающего итога не спрашиваем")
	}

	if got := Asking(st, marks(), *at(2026, 11, 1, 12)); got != -1 {
		t.Fatalf("отборочный ещё идёт — спрашивать рано, получили %d", got)
	}
	if got := Asking(st, marks(), *at(2026, 11, 2, 10)); got != 1 {
		t.Fatalf("отборочный закончился — ждём его итог, получили %d", got)
	}
	if got := Asking(st, marks(), *at(2027, 3, 8, 10)); got != 3 {
		t.Fatalf("прошло всё — спрашиваем о последнем этапе, получили %d", got)
	}
}

func TestStatus(t *testing.T) {
	st := twoRegs()
	cases := []struct {
		name            string
		p               Progress
		now             *time.Time
		status, outcome string
	}{
		{"регистрация открыта", Progress{}, at(2026, 10, 1, 0), StatusOpen, ""},
		{"регистрация закрылась без отметки", Progress{}, at(2026, 11, 5, 0), StatusFinished, OutcomeMissed},
		{"участвует", marks(), at(2026, 11, 5, 0), StatusActive, ""},
		{"не прошёл", marks("q", Mark{Result: Failed}), at(2026, 12, 1, 0), StatusFinished, Failed},
		{"призёр", marks("q", Mark{Result: Passed}, "f", Mark{Result: Prizer}), at(2027, 3, 8, 0), StatusFinished, Prizer},
		{"сезон прошёл, итог не отмечен", marks(), at(2027, 4, 1, 0), StatusFinished, OutcomeUnknown},
	}
	for _, c := range cases {
		status, outcome := Status(st, c.p, *c.now)
		if status != c.status || outcome != c.outcome {
			t.Errorf("%s: %s/%s, ожидали %s/%s", c.name, status, outcome, c.status, c.outcome)
		}
	}
	// У НТО финала в данных пока нет: «прошёл» после последнего отборочного —
	// участие продолжается, а не «итог не отмечен».
	status, _ := Status(nto()[:2], marks("q1", Mark{Result: Passed}), *at(2026, 12, 1, 0))
	if status != StatusActive {
		t.Fatalf("прошёл дальше, а даты финала не опубликованы — участвует, получили %s", status)
	}
	// Олимпиада без регистрации: участие отмечается той же галочкой.
	noReg := twoRegs()[1:2]
	if status, _ := Status(noReg, Progress{}, *at(2026, 10, 1, 0)); status != StatusOpen {
		t.Fatalf("без регистрации до отборочного — «нужно записаться», получили %s", status)
	}
}

func TestApply(t *testing.T) {
	st := twoRegs()
	now := *at(2026, 11, 5, 12)

	if _, err := Apply(st, Progress{}, "nope", Mark{Registered: true}, now); !errors.Is(err, ErrUnknownStage) {
		t.Fatalf("чужой этап — ErrUnknownStage, получили %v", err)
	}
	if _, err := Apply(st, marks(), "q", Mark{Result: Winner}, now); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("диплом у отборочного — ErrNotAllowed, получили %v", err)
	}
	if _, err := Apply(st, marks(), "r1", Mark{Registered: true, Result: Passed}, now); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("итог у регистрации — ErrNotAllowed, получили %v", err)
	}
	if _, err := Apply(st, marks(), "f", Mark{Result: Winner}, now); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("итог этапа, который ещё не начался, — ErrNotAllowed, получили %v", err)
	}

	p, err := Apply(st, Progress{}, "q", Mark{Result: Passed}, now)
	if err != nil || !p.Registered || p.Marks["q"].Result != Passed {
		t.Fatalf("итог подразумевает регистрацию: %+v, %v", p, err)
	}
	p, err = Apply(st, p, "r2", Mark{Registered: true}, now)
	if err != nil || !p.Marks["r2"].Registered {
		t.Fatalf("регистрация на финал — отметка этапа: %+v, %v", p, err)
	}
	if _, err := Apply(st, p, "q", Mark{Result: Failed}, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("закрывающий итог при отметках дальше — ErrConflict, получили %v", err)
	}
	if _, err := Apply(st, p, "r1", Mark{}, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("снять первую регистрацию при отметках — ErrConflict, получили %v", err)
	}

	closed, err := Apply(st, marks(), "q", Mark{Result: Failed}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(st, closed, "r2", Mark{Registered: true}, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("отметка после закрывающей — ErrConflict, получили %v", err)
	}
	reopened, err := Apply(st, closed, "q", Mark{}, now)
	if err != nil || ClosedAt(st, reopened) != -1 {
		t.Fatalf("закрывающий итог снимается, и всё возвращается: %v", err)
	}
	if _, ok := reopened.Marks["q"]; ok {
		t.Fatal("пустая отметка удаляется")
	}
	if reopened.Registered != true {
		t.Fatal("снятие итога не снимает регистрацию")
	}

	p, err = Apply(st, marks(), "q", Mark{Registered: true, Result: Passed}, now)
	if err != nil || p.Marks["q"].Registered {
		t.Fatalf("«зарегистрирован» у этапа без регистрации не хранится: %+v, %v", p.Marks["q"], err)
	}
	orig := marks()
	if _, err := Apply(st, orig, "q", Mark{Result: Passed}, now); err != nil || len(orig.Marks) != 0 {
		t.Fatal("Apply не меняет входные отметки")
	}
}
