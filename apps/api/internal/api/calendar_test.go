package api

import (
	"context"
	"testing"
)

func TestCalendar(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	nto := e.track(f, "p669-5-iskusstvennyy-intellekt", false) // регистрация до 22.10, первый этап до 23.10
	e.track(f, "vsosh-informatika", false)                     // школьный этап до 28.10
	token := e.login(900000002, "Ольга")

	r := e.do("GET", "/api/v1/calendar?month=2026-10", token, nil)
	if r.code != 200 || r.body["month"] != "2026-10" {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	type entry struct{ date, profile, stage string }
	var got []entry
	for _, d := range list(t, r.body["days"]) {
		for _, it := range list(t, d["items"]) {
			got = append(got, entry{d["date"].(string), it["olympiad_profile_id"].(string), it["next_stage_title"].(string)})
		}
	}
	want := []entry{
		{"2026-10-22", "p669-5-iskusstvennyy-intellekt", "Регистрация"},
		{"2026-10-23", "p669-5-iskusstvennyy-intellekt", "Первый (индивидуальный) этап"},
		{"2026-10-28", "vsosh-informatika", "Школьный этап"},
	}
	if len(got) != len(want) {
		t.Fatalf("два срока одного пункта — две записи: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("запись %d: %v, ждали %v", i, got[i], want[i])
		}
	}

	// После отметки срок регистрации из календаря уходит.
	if _, err := e.pool.Exec(context.Background(), `UPDATE tracker_items SET registered_at = now() WHERE id = $1`, nto); err != nil {
		t.Fatal(err)
	}
	days := list(t, e.do("GET", "/api/v1/calendar?month=2026-10", token, nil).body["days"])
	if len(days) != 2 || days[0]["date"] != "2026-10-23" {
		t.Fatalf("после отметки: %v", days)
	}

	if r := e.do("GET", "/api/v1/calendar?month=2026-08", token, nil); string(r.raw) != `{"month":"2026-08","days":[]}`+"\n" {
		t.Fatalf("пустой месяц — days: []: %s", r.raw)
	}
	for _, bad := range []string{"", "?month=2026-13", "?month=2026-1", "?month=октябрь"} {
		if r := e.do("GET", "/api/v1/calendar"+bad, token, nil); r.code != 400 || r.errCode() != "BAD_REQUEST" {
			t.Fatalf("%q — 400: %d %s", bad, r.code, r.raw)
		}
	}
}

// Отмеченный итог и закрывающая отметка убирают сроки из календаря: после
// «не прошёл» у олимпиады больше нет дат, которые нужно помнить.
func TestCalendar_SkipsSettledStages(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	bio := e.track(f, bioProfile, true)
	token := e.login(artemMax, "Артём")
	month := func(m string) int {
		n := 0
		for _, d := range list(t, e.do("GET", "/api/v1/calendar?month="+m, token, nil).body["days"]) {
			n += len(list(t, d["items"]))
		}
		return n
	}
	if month("2026-11") != 1 || month("2027-03") != 2 {
		t.Fatalf("до отметок: отборочный в ноябре, регистрация и финал в марте: %d %d", month("2026-11"), month("2027-03"))
	}
	if _, err := e.pool.Exec(context.Background(), `INSERT INTO tracker_stage_results (tracker_item_id, stage_id, result)
		VALUES ($1, $2, 'failed')`, bio, bioQual); err != nil {
		t.Fatal(err)
	}
	if month("2026-11") != 0 || month("2027-03") != 0 {
		t.Fatalf("не прошёл — сроков больше нет: %d %d", month("2026-11"), month("2027-03"))
	}
}
