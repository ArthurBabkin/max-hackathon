package schedule

import (
	"testing"
	"time"
)

var (
	moscow, _      = time.LoadLocation("Europe/Moscow")
	vladivostok, _ = time.LoadLocation("Asia/Vladivostok")
	// Срок регистрации «до 22 октября» — последний день, 23:59:59 по Москве.
	deadline = time.Date(2026, 10, 22, 23, 59, 59, 0, moscow)
)

func at(loc *time.Location, month time.Month, day, hour int) time.Time {
	return time.Date(2026, month, day, hour, 0, 0, 0, loc)
}

func offsets(ts []Threshold) []int {
	out := []int{}
	for _, t := range ts {
		out = append(out, t.Offset)
	}
	return out
}

func TestThresholds_TenAMLocalTimeNDaysBefore(t *testing.T) {
	got := Thresholds(deadline, moscow, 10)
	want := []Threshold{
		{30, at(moscow, 9, 22, 10)}, {7, at(moscow, 10, 15, 10)}, {3, at(moscow, 10, 19, 10)}, {1, at(moscow, 10, 21, 10)},
	}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i].Offset != want[i].Offset || !got[i].FireAt.Equal(want[i].FireAt) {
			t.Fatalf("порог %d: %v, ждали %v", i, got[i], want[i])
		}
	}
}

func TestThresholds_StudentTimeZone(t *testing.T) {
	// 22.10 23:59 по Москве — это 23.10 06:59 во Владивостоке: «за день» —
	// 22.10 в 10:00 по Владивостоку.
	got := Thresholds(deadline, vladivostok, 10)
	if !got[3].FireAt.Equal(at(vladivostok, 10, 22, 10)) {
		t.Fatalf("за день по зоне ученика: %v", got[3].FireAt)
	}
}

func TestUpcoming(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want []int
	}{
		{"все пороги впереди", at(moscow, 9, 1, 12), []int{30, 7, 3, 1}},
		{"прошедшие пороги не ставятся", at(moscow, 10, 17, 12), []int{3, 1}},
		{"все прошли, но 10:00 до срока ещё будет", at(moscow, 10, 21, 12), []int{1}},
		{"следующие 10:00 уже после срока", at(moscow, 10, 22, 12), []int{}},
		{"срок прошёл", at(moscow, 10, 23, 12), []int{}},
	}
	for _, c := range cases {
		got := Upcoming(deadline, moscow, 10, c.now)
		if len(got) != len(c.want) {
			t.Fatalf("%s: %v", c.name, offsets(got))
		}
		for i := range c.want {
			if got[i].Offset != c.want[i] || !got[i].FireAt.After(c.now) || !got[i].FireAt.Before(deadline) {
				t.Fatalf("%s: %v", c.name, got)
			}
		}
	}
	// Одно «догоняющее» напоминание — в ближайшие 10:00.
	if got := Upcoming(deadline, moscow, 10, at(moscow, 10, 21, 12)); !got[0].FireAt.Equal(at(moscow, 10, 22, 10)) {
		t.Fatalf("догоняющее — в 10:00 22.10: %v", got[0].FireAt)
	}
}

func TestTomorrow_NotLaterThanDeadline(t *testing.T) {
	got, ok := Tomorrow(deadline, moscow, 10, at(moscow, 10, 20, 15))
	if !ok || !got.Equal(at(moscow, 10, 21, 10)) {
		t.Fatalf("завтра в 10:00: %v %v", got, ok)
	}
	// Утром 22.10: завтрашние 10:00 — уже после срока.
	if _, ok := Tomorrow(deadline, moscow, 10, at(moscow, 10, 22, 9)); ok {
		t.Fatal("напомнить завтра после срока нельзя")
	}
}

// Итог этапа спрашиваем на следующий день после окончания в 10:00 по зоне
// ученика и повторяем через неделю. У Владивостока московский вечер —
// уже следующий день, поэтому «завтра» там наступает на день позже.
func TestAfter_NextDayAndWeekLater(t *testing.T) {
	got := After(deadline, moscow, 10)
	want := []Threshold{{-1, at(moscow, 10, 23, 10)}, {-8, at(moscow, 10, 30, 10)}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("After = %v, ожидали %v", got, want)
	}
	if vl := After(deadline, vladivostok, 10); !vl[0].FireAt.Equal(at(vladivostok, 10, 24, 10)) {
		t.Fatalf("Владивосток: %v", vl)
	}
}
