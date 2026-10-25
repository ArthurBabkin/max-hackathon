package db_test

import (
	"context"
	"slices"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

// Сроки двенадцати олимпиад, сверенные 26.10.2026 (0029), на базе, где они как на
// проде до миграции: после неё этапы с часами организатора, классы и даты
// сверки — ровно как в сиде; событие «изменились сроки» — по профилям, где
// сроки правда сдвинулись; повторный прогон событий не рождает.
func TestMigration_VerifiedStageDates(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	raw := readMigration(t, "0029_verified_stage_dates.sql")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const olympiads = `('p669-11', 'p669-12', 'p669-19', 'p669-22', 'p669-26', 'p669-31', 'p669-34', 'p669-49',
		'p669-54', 'p669-55', 'p669-69', 'p669-83')`
	const rows = `SELECT x FROM (
		SELECT p::text AS x FROM olympiad_profiles p WHERE olympiad_id IN ` + olympiads + `
		UNION ALL SELECT s::text FROM stages s JOIN olympiad_profiles p ON p.id = s.olympiad_profile_id
			WHERE p.olympiad_id IN ` + olympiads + `
		UNION ALL SELECT src::text FROM sources src WHERE src.id IN (
			SELECT s.source_id FROM stages s JOIN olympiad_profiles p ON p.id = s.olympiad_profile_id
			WHERE p.olympiad_id IN ` + olympiads + `)) r`
	const stage = `SELECT concat_ws(' ', to_char(starts_at AT TIME ZONE 'Europe/Moscow', 'YYYY-MM-DD HH24:MI'),
		to_char(ends_at AT TIME ZONE 'Europe/Moscow', 'YYYY-MM-DD HH24:MI')) FROM stages WHERE id = $1`
	at := func(id string) string {
		t.Helper()
		var s string
		if err := tx.QueryRow(ctx, stage, id).Scan(&s); err != nil {
			t.Fatalf("этап %s: %v", id, err)
		}
		return s
	}

	// Как на проде до миграции.
	mustExec(t, tx.Exec, dbtest.DownSection(raw))
	if got := at("p669-22-iskusstvennyy-intellekt:qualifying:2"); got != "2027-01-16 00:00 2027-01-17 23:59" {
		t.Fatalf("Down: 2-й тур ИИ %s", got)
	}
	if n := queryInt(t, tx, `SELECT count(*) FROM stages WHERE olympiad_profile_id = 'p669-49-estestvennye-nauki'
		OR id IN ('p669-22-informatika:registration:2', 'p669-22-matematika:registration:2')`); n != 0 {
		t.Fatalf("Down оставил %d новых этапов", n)
	}
	// У Физтеха на проде у всех профилей одни даты, и у всех — регистрация.
	if n := queryInt(t, tx, `SELECT count(*) FROM stages WHERE id LIKE 'p669-54-%:registration:1'
		AND ends_at = '2026-09-07 23:59:59+03'`); n != 7 {
		t.Fatalf("Down: регистрация 7 сентября у %d профилей Физтеха из 7", n)
	}
	mustExec(t, tx.Exec, `DELETE FROM content_changes`)

	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	if n := queryInt(t, tx, `SELECT count(*) FROM stages s JOIN olympiad_profiles p ON p.id = s.olympiad_profile_id
		WHERE p.olympiad_id IN `+olympiads+` AND s.is_demo`); n != 0 {
		t.Fatalf("после 0029 у сверенных олимпиад %d демо-этапов", n)
	}
	for id, want := range map[string]string{
		// ИИ и ИБ были перепутаны; второй приём заявок — с часами организатора.
		"p669-22-iskusstvennyy-intellekt:qualifying:2":     "2027-01-23 00:00 2027-01-24 23:59",
		"p669-22-informacionnaya-bezopasnost:qualifying:2": "2027-01-17 00:00 2027-01-17 23:59",
		"p669-22-robototehnika:registration:1":             "2026-11-10 15:00",
		"p669-22-informatika:registration:2":               "2026-10-19 10:00 2026-11-18 12:00",
		"p669-22-matematika:registration:2":                "2026-10-26 10:00 2026-11-23 15:00",
		// 23:59 по Екатеринбургу, 17:00 по Красноярску.
		"p669-26-fizika:qualifying:1":               "2026-09-01 00:00 2027-01-19 21:59",
		"p669-83-himiya:qualifying:1":               "2026-10-01 00:00 2027-01-12 13:00",
		"p669-34-informatika:qualifying:1":          "2026-10-26 10:00 2026-11-23 23:59",
		"p669-49-estestvennye-nauki:registration:2": "2027-01-15 00:00 2027-02-05 10:00",
		"p669-49-estestvennye-nauki:final:1":        "2027-02-13 00:00 2027-02-13 23:59",
		// Физтех: туры онлайн-этапа с часами, регистрация до утра накануне.
		"p669-54-fizika:registration:2":   "2026-10-24 08:00",
		"p669-54-fizika:qualifying:2":     "2026-10-25 08:00 2026-10-25 13:00",
		"p669-54-matematika:qualifying:1": "2026-10-11 10:00 2026-10-11 15:00",
		"p669-54-matematika:final:1":      "2027-02-14 00:00 2027-02-14 23:59",
		"p669-54-himiya:qualifying:1":     "2026-12-15 00:00 2027-01-10 23:59",
		// Финал объявлен, день — нет: демо-даты уходят.
		"p669-54-inzhenernoe-delo:final:1":                          "",
		"p669-11-himiya:final:1":                                    "",
		"p669-69-inzhenernoe-delo:final:1":                          "",
		"p669-12-pravo:qualifying:1":                                "2026-12-01 09:00 2027-01-01 20:59",
		"p669-69-inzhenernoe-delo:registration:1":                   "2026-10-10 00:00 2027-01-15 23:59",
		"p669-55-kompyuternoe-modelirovanie-i-grafika:qualifying:2": "2026-11-16 00:00 2026-11-22 23:59",
	} {
		if got := at(id); got != want {
			t.Errorf("этап %s после 0029: %s, ожидали %s", id, got, want)
		}
	}
	if got := queryStrings(t, tx, `SELECT concat_ws(' ', id, grades_from, grades_to) FROM olympiad_profiles
		WHERE id IN ('p669-31-matematika', 'p669-34-informatika', 'p669-49-estestvennye-nauki',
			'p669-49-himiya', 'p669-49-informatika', 'p669-54-himiya', 'p669-54-informatika-i-programmirovanie',
			'p669-54-inzhenernoe-delo')`); !slices.Equal(got, []string{
		"p669-31-matematika 9 11", "p669-34-informatika 10 11", "p669-49-estestvennye-nauki 9 11",
		"p669-49-himiya 9 11", "p669-49-informatika 9 11", "p669-54-himiya 7 11",
		"p669-54-informatika-i-programmirovanie 7 11", "p669-54-inzhenernoe-delo 8 11",
	}) {
		t.Fatalf("классы после 0029: %v", got)
	}
	// Интернет-олимпиаду по физике сверили, сроки те же — новая только дата сверки.
	if n := queryInt(t, tx, `SELECT count(*) FROM sources WHERE id IN (
		SELECT source_id FROM stages WHERE olympiad_profile_id LIKE ANY (ARRAY['p669-19-%', 'p669-31-%']))
		AND verified_at = '2026-10-26'`); n != 3 {
		t.Fatalf("дата сверки обновилась у %d источников Интернет-олимпиады и Верченко из 3", n)
	}
	if got := queryStrings(t, tx, `SELECT DISTINCT entity_id FROM content_changes`); !slices.Equal(got, []string{
		"p669-11-biologiya", "p669-11-himiya", "p669-12-obschestvoznanie", "p669-12-pravo",
		"p669-22-informacionnaya-bezopasnost", "p669-22-informatika", "p669-22-iskusstvennyy-intellekt",
		"p669-22-matematika", "p669-22-robototehnika",
		"p669-26-fizika", "p669-26-himiya", "p669-26-informatika", "p669-26-matematika", "p669-26-obschestvoznanie",
		"p669-34-fizika", "p669-34-himiya", "p669-34-informatika",
		"p669-49-estestvennye-nauki", "p669-49-himiya", "p669-49-informatika",
		"p669-54-biologiya", "p669-54-fizika", "p669-54-himiya", "p669-54-informatika-i-programmirovanie",
		"p669-54-inzhenernoe-delo", "p669-54-matematika", "p669-54-nauchno-tehnicheskiy",
		"p669-55-fizika", "p669-55-himiya", "p669-55-informatika", "p669-55-inzhenernoe-delo",
		"p669-55-kompyuternoe-modelirovanie-i-grafika", "p669-55-matematika",
		"p669-69-informacionnye-i-kommunikacionnye-tehnologii", "p669-69-inzhenernoe-delo",
		"p669-83-fizika", "p669-83-himiya", "p669-83-informatika", "p669-83-matematika",
	}) {
		t.Fatalf("события 0029: %v", got)
	}

	mustExec(t, tx.Exec, `DELETE FROM content_changes`)
	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	if n := queryInt(t, tx, `SELECT count(*) FROM content_changes`); n != 0 {
		t.Fatalf("повторный прогон 0029 породил %d событий", n)
	}

	// Профили, этапы и их источники после миграции — ровно сид 0003.
	got := queryStrings(t, tx, rows)
	mustExec(t, tx.Exec, dbtest.UpSection(readMigration(t, "0003_seed_content.sql")))
	if want := queryStrings(t, tx, rows); !slices.Equal(got, want) {
		t.Fatalf("после 0029 не как в сиде:\nлишняя:   %v\nнедостаёт: %v",
			first(diffSorted(got, want)), first(diffSorted(want, got)))
	}
}
