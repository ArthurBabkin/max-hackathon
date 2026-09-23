package api

import (
	"context"
	"testing"
)

// План напоминаний пересчитывается сразу, а не ждёт воркера: отметили
// регистрацию — о ней больше не напомнят, сменили регион — 10:00 по новой зоне.
func TestReminders_ReplannedAfterTrackerChanges(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	kid := e.login(900000001, "Артём")
	ctx := context.Background()
	count := func(where string) int {
		t.Helper()
		var n int
		if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM reminders r JOIN stages st ON st.id = r.stage_id
			WHERE `+where).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Турнир Ломоносова: регистрация до 2.10, осенний тур 4.10, финал 13.03; сейчас 22.09.
	r := e.do("POST", "/api/v1/tracker", kid, map[string]any{"olympiad_profile_id": "p669-82-fizika"})
	if r.code != 201 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	id := r.body["id"].(string)
	// «За 30 дней» до регистрации и тура было в начале сентября — уже прошло: 3 + 3 + 4.
	if n := count(`r.status = 'planned'`); n != 10 {
		t.Fatalf("прошедшие пороги не планируются, получили %d", n)
	}

	if r := e.do("PUT", "/api/v1/tracker/"+id+"/registered", kid, nil); r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	if n := count(`r.status = 'planned' AND st.kind = 'registration'`); n != 0 {
		t.Fatalf("после отметки о регистрации не напоминают: %d", n)
	}
	if r := e.do("DELETE", "/api/v1/tracker/"+id+"/registered", kid, nil); r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	if n := count(`r.status = 'planned' AND st.kind = 'registration'`); n != 3 {
		t.Fatalf("сняли отметку — напоминания вернулись: %d", n)
	}

	if r := e.do("PATCH", "/api/v1/profile", kid, map[string]any{"region_code": "25"}); r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	var hour int
	_ = e.pool.QueryRow(ctx, `SELECT DISTINCT extract(hour FROM fire_at AT TIME ZONE 'Asia/Vladivostok')::int
		FROM reminders WHERE status = 'planned'`).Scan(&hour)
	if hour != 10 {
		t.Fatalf("после переезда в Приморье — 10:00 по Владивостоку, получили %d", hour)
	}
}
