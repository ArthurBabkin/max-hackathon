package voice

import (
	"fmt"
	"time"
)

// Plural — форма слова по числу: 1 день, 2 дня, 5 дней. Та же логика, что
// plural в apps/web/src/lib/deadline.ts.
func Plural(n int, one, few, many string) string {
	hundreds := n % 100
	if hundreds < 0 {
		hundreds = -hundreds
	}
	units := hundreds % 10
	switch {
	case hundreds > 10 && hundreds < 20:
		return many
	case units > 1 && units < 5:
		return few
	case units == 1:
		return one
	}
	return many
}

// Days — «4 дня», «1 день», «10 дней».
func Days(n int) string { return fmt.Sprintf("%d %s", n, Plural(n, "день", "дня", "дней")) }

// Olympiads — «4 олимпиады», «1 олимпиада», «5 олимпиад».
func Olympiads(n int) string {
	return fmt.Sprintf("%d %s", n, Plural(n, "олимпиада", "олимпиады", "олимпиад"))
}

// DaysUntil — календарных дней от now до t в зоне loc: «завтра» — это
// завтра, даже если до срока 15 часов.
func DaysUntil(t, now time.Time, loc *time.Location) int {
	day := func(x time.Time) time.Time {
		l := x.In(loc)
		return time.Date(l.Year(), l.Month(), l.Day(), 12, 0, 0, 0, time.UTC)
	}
	return int(day(t).Sub(day(now)).Hours() / 24)
}
