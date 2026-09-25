package stages

import (
	"testing"
	"time"
)

var msk = time.FixedZone("MSK", 3*3600)

func at(y int, m time.Month, d, h int) *time.Time {
	t := time.Date(y, m, d, h, 0, 0, 0, msk)
	return &t
}

// Календарь НТО из сида: регистрация до 22.10, два отборочных онлайн-окна.
func nto() []Stage {
	return []Stage{
		{ID: "r", Kind: "registration", Title: "Регистрация", StartsAt: at(2026, 8, 26, 0),
			EndsAt: at(2026, 10, 22, 23), DeadlineAt: at(2026, 10, 22, 23), IsOnline: true},
		{ID: "q1", Kind: "qualifying", Title: "Первый этап", StartsAt: at(2026, 9, 17, 0),
			EndsAt: at(2026, 10, 23, 23), DeadlineAt: at(2026, 10, 23, 23), IsOnline: true},
		{ID: "f", Kind: "final", Title: "Заключительный этап", StartsAt: at(2027, 2, 20, 0),
			EndsAt: at(2027, 2, 22, 23), DeadlineAt: at(2027, 2, 20, 0)},
	}
}

func TestCurrent_FirstNotPassed(t *testing.T) {
	now := *at(2026, 9, 22, 12)
	if got := Current(nto(), Progress{}, now); got != 0 {
		t.Fatalf("до конца регистрации текущая — регистрация, получили %d", got)
	}
	if got := Current(nto(), Progress{Registered: true}, now); got != 1 {
		t.Fatalf("после отметки о регистрации текущий — следующий этап, получили %d", got)
	}
	if got := Current(nto(), Progress{}, *at(2026, 10, 23, 1)); got != 1 {
		t.Fatalf("регистрация прошла — текущий отборочный, получили %d", got)
	}
	if got := Current(nto(), Progress{}, *at(2027, 3, 1, 0)); got != -1 {
		t.Fatalf("всё в прошлом — -1, получили %d", got)
	}
	if got := Current(nil, Progress{}, now); got != -1 {
		t.Fatalf("без этапов — -1, получили %d", got)
	}
}

func TestStates(t *testing.T) {
	now := *at(2026, 10, 23, 1)
	got := States(nto(), Progress{}, now)
	want := []string{"past", "current", "future"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("состояния %v, ожидали %v", got, want)
		}
	}
	all := States(nto(), Progress{}, *at(2027, 3, 1, 0))
	for _, s := range all {
		if s != "past" {
			t.Fatalf("всё прошло — всё past: %v", all)
		}
	}
}

func TestAllPassed(t *testing.T) {
	if AllPassed(nil, *at(2026, 9, 22, 0)) {
		t.Fatal("профиль без этапов не считается прошедшим — он остаётся в подборе")
	}
	if !AllPassed(nto(), *at(2027, 3, 1, 0)) || AllPassed(nto(), *at(2026, 9, 22, 0)) {
		t.Fatal("AllPassed")
	}
}

// Вступить можно, пока не прошёл срок первого этапа: регистрации, а если
// её нет — отборочного или школьного. Порядок этапов на входе не важен.
func TestJoinable(t *testing.T) {
	if !Joinable(nto(), *at(2026, 10, 22, 12)) {
		t.Fatal("в последний день регистрации вступить ещё можно")
	}
	if Joinable(nto(), *at(2026, 10, 23, 1)) {
		t.Fatal("регистрация закрылась — вступить нельзя, хотя отборочный ещё идёт")
	}
	reversed := nto()
	reversed[0], reversed[2] = reversed[2], reversed[0]
	if Joinable(reversed, *at(2026, 10, 23, 1)) {
		t.Fatal("первый этап ищется по датам, а не по порядку в срезе")
	}
	noReg := nto()[1:]
	if !Joinable(noReg, *at(2026, 10, 23, 1)) || Joinable(noReg, *at(2026, 10, 24, 0)) {
		t.Fatal("без регистрации вступают до конца отборочного")
	}
	if !Joinable(nil, *at(2027, 3, 1, 0)) {
		t.Fatal("профиль без этапов не исключается: о сроках просто ничего не известно")
	}
	undated := []Stage{{Kind: "registration"}, {Kind: "final"}}
	if !Joinable(undated, *at(2026, 10, 1, 0)) {
		t.Fatal("этапы без дат не закрываются")
	}
}

func TestRegistrationLike(t *testing.T) {
	for kind, want := range map[string]bool{"registration": true, "school": true, "qualifying": false, "final": false} {
		if RegistrationLike(kind) != want {
			t.Errorf("%s: %v", kind, !want)
		}
	}
}

func TestSubtitle(t *testing.T) {
	st := nto()
	cases := []struct {
		s    Stage
		want string
	}{
		{st[0], "до 22 октября, онлайн"},
		{st[2], "20–22 февраля"},
		{Stage{Kind: "final", StartsAt: at(2026, 11, 28, 0), EndsAt: at(2026, 12, 3, 23), DeadlineAt: at(2026, 11, 28, 0)}, "28 ноября — 3 декабря"},
		{Stage{Kind: "municipal", StartsAt: at(2026, 11, 28, 0), EndsAt: at(2026, 11, 28, 23), DeadlineAt: at(2026, 11, 28, 0)}, "28 ноября"},
		{Stage{Kind: "qualifying", StartsAt: at(2026, 11, 5, 0), EndsAt: at(2026, 12, 11, 23), DeadlineAt: at(2026, 12, 11, 23), IsOnline: true}, "до 11 декабря, онлайн"},
		{Stage{Kind: "final"}, ""},
		// «Финал — не позднее 31 марта»: организатор назвал только крайний день.
		{Stage{Kind: "final", EndsAt: at(2027, 3, 31, 23), DeadlineAt: at(2027, 3, 31, 23)}, "до 31 марта"},
	}
	for _, c := range cases {
		if got := Subtitle(c.s, msk); got != c.want {
			t.Errorf("Subtitle(%s) = %q, ожидали %q", c.s.Kind, got, c.want)
		}
	}
}

func TestSort_ByStartThenDeadline(t *testing.T) {
	st := nto()
	shuffled := []Stage{st[2], st[0], st[1]}
	Sort(shuffled)
	if shuffled[0].ID != "r" || shuffled[1].ID != "q1" || shuffled[2].ID != "f" {
		t.Fatalf("порядок: %s %s %s", shuffled[0].ID, shuffled[1].ID, shuffled[2].ID)
	}
}

// У «Росатома» регистрация и отборочный тур идут в одном окне: при равных
// датах регистрация первая, иначе отметка «зарегистрирован» встала бы не на
// тот этап. Порядок не зависит от порядка строк в базе.
func TestSort_TieRegistrationFirst(t *testing.T) {
	q := Stage{ID: "x:qualifying:1", Kind: "qualifying", StartsAt: at(2026, 10, 5, 0), DeadlineAt: at(2027, 1, 13, 23)}
	r := Stage{ID: "x:registration:1", Kind: "registration", StartsAt: at(2026, 10, 5, 0), DeadlineAt: at(2027, 1, 13, 23)}
	for _, in := range [][]Stage{{q, r}, {r, q}} {
		Sort(in)
		if in[0].Kind != "registration" {
			t.Fatalf("при равных датах первой идёт регистрация: %s, %s", in[0].ID, in[1].ID)
		}
	}
}
