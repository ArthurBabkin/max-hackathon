package voice

import (
	"testing"
	"time"
)

func TestPlural(t *testing.T) {
	for n, want := range map[int]string{1: "1 день", 2: "2 дня", 4: "4 дня", 5: "5 дней", 11: "11 дней", 21: "21 день", 112: "112 дней"} {
		if got := Days(n); got != want {
			t.Errorf("Days(%d) = %q, ждали %q", n, got, want)
		}
	}
	if Olympiads(1) != "1 олимпиада" || Olympiads(3) != "3 олимпиады" || Olympiads(7) != "7 олимпиад" {
		t.Fatal("олимпиады")
	}
}

func TestDaysUntil_CalendarDaysInZone(t *testing.T) {
	msk, _ := time.LoadLocation("Europe/Moscow")
	vl, _ := time.LoadLocation("Asia/Vladivostok")
	deadline := time.Date(2026, 10, 22, 23, 59, 59, 0, msk)
	now := time.Date(2026, 10, 21, 10, 0, 0, 0, msk)
	if d := DaysUntil(deadline, now, msk); d != 1 {
		t.Fatalf("по Москве до срока — завтра: %d", d)
	}
	// Во Владивостоке 22.10 23:59 по Москве — уже 23.10.
	if d := DaysUntil(deadline, time.Date(2026, 10, 21, 10, 0, 0, 0, vl), vl); d != 2 {
		t.Fatalf("по Владивостоку — послезавтра: %d", d)
	}
}
