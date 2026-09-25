package pick

import (
	"testing"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/match"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

// Стартовая олимпиада (SPEC 9): лучшая по скору, до дедлайна которой больше
// суток; новичку — не ВсОШ, если есть олимпиада перечня.
func TestStart(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { x := now.Add(d); return &x }
	item := func(id, kind string, score float64, deadline *time.Time) match.Result {
		return match.Result{Candidate: match.Candidate{ProfileID: id, Kind: kind}, Score: score, Deadline: deadline}
	}
	items := []match.Result{
		item("soon", "perechen", 0.99, at(12*time.Hour)),
		item("vsosh", "vsosh", 0.9, at(10*24*time.Hour)),
		item("none", "perechen", 0.95, nil),
		item("list", "perechen", 0.7, at(5*24*time.Hour)),
	}
	res := func(exp string, items ...match.Result) Result {
		return Result{Items: items, Set: Set{Now: now, Trajectory: store.Trajectory{Experience: exp}}}
	}
	for _, tc := range []struct {
		exp  string
		want string
	}{{"", "list"}, {"none", "list"}, {"school", "vsosh"}, {"region", "vsosh"}} {
		if got := Start(res(tc.exp, items...)); got == nil || got.ProfileID != tc.want {
			t.Errorf("опыт %q: %v, ждали %s", tc.exp, got, tc.want)
		}
	}
	if got := Start(res("none", items[1])); got == nil || got.ProfileID != "vsosh" {
		t.Errorf("новичку без перечня — ВсОШ: %v", got)
	}
	if got := Start(res("none", items[0], items[2])); got != nil {
		t.Errorf("нет подходящей: %v", got.ProfileID)
	}
}
