package db_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

// Олимпиады РГГУ, «Звезда» и Санкт-Петербургская астрономическая (0026) на
// базе, где у них не было ни сайта, ни сроков: после миграции сайт,
// организатор, классы и этапы — ровно как в сиде; событие «изменились сроки» —
// только по их профилям; повторный прогон событий не рождает.
func TestMigration_OlympiadSitesAndDates(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	raw := readMigration(t, "0026_olympiad_sites_and_dates.sql")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const olympiads = `('p669-48', 'p669-36', 'p669-74')`
	const profiles = `('p669-48-obschestvoznanie', 'p669-36-estestvennye-nauki', 'p669-36-tehnika-i-tehnologii',
		'p669-74-astronomiya')`
	const rows = `SELECT x FROM (
		SELECT o::text AS x FROM olympiads o WHERE id IN ` + olympiads + `
		UNION ALL SELECT p::text FROM olympiad_profiles p WHERE olympiad_id IN ` + olympiads + `
		UNION ALL SELECT s::text FROM stages s WHERE olympiad_profile_id IN ` + profiles + `) r`
	const rggu = "https://www.rsuh.ru/education/cdo/olimpiada-rggu-dlya-shkolnikov.php"
	const zvezda = "https://zv.susu.ru/index.php/mnogoprofilnaya-inzhenernaya-olimpiada-zvezda/otborochnyj-etap"

	// Как на проде до миграции.
	mustExec(t, tx.Exec, dbtest.DownSection(raw))
	if n := queryInt(t, tx, `SELECT count(*) FROM olympiads WHERE id IN `+olympiads+` AND official_url IS NULL`); n != 3 {
		t.Fatalf("Down оставил сайт у %d олимпиад из 3", 3-n)
	}
	if n := queryInt(t, tx, `SELECT count(*) FROM stages WHERE olympiad_profile_id IN `+profiles); n != 0 {
		t.Fatalf("Down оставил %d этапов", n)
	}
	if n := queryInt(t, tx, `SELECT count(*) FROM sources WHERE url IN ('`+rggu+`', '`+zvezda+`')`); n != 0 {
		t.Fatalf("Down оставил %d источников", n)
	}
	mustExec(t, tx.Exec, `DELETE FROM content_changes`)

	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	if got := queryStrings(t, tx, `SELECT concat_ws(' | ', id, organizer, official_url, coalesce(final_city, '—'))
		FROM olympiads WHERE id IN `+olympiads+` ORDER BY id`); !slices.Equal(got, []string{
		"p669-36 | ЮУрГУ | https://zv.susu.ru/ | —",
		"p669-48 | РГГУ | " + rggu + " | Москва",
		"p669-74 | Алфёровский университет | http://school.astro.spbu.ru/?q=olymp | —",
	}) {
		t.Fatalf("олимпиады после 0026: %v", got)
	}
	if got := queryStrings(t, tx, `SELECT concat_ws(' ', id, grades_from, grades_to) FROM olympiad_profiles
		WHERE id IN `+profiles+` ORDER BY id`); !slices.Equal(got, []string{
		"p669-36-estestvennye-nauki 6 11", "p669-36-tehnika-i-tehnologii 7 11",
		"p669-48-obschestvoznanie 9 11", "p669-74-astronomiya 5 11",
	}) {
		t.Fatalf("классы профилей после 0026: %v", got)
	}
	const stages = `SELECT concat_ws(' ', st.kind, to_char(st.starts_at AT TIME ZONE 'Europe/Moscow', 'YYYY-MM-DD'),
		to_char(st.ends_at AT TIME ZONE 'Europe/Moscow', 'YYYY-MM-DD'),
		CASE WHEN st.is_online THEN 'онлайн' ELSE 'очно' END, CASE WHEN st.is_demo THEN 'демо' ELSE src.url END)
		FROM stages st LEFT JOIN sources src ON src.id = st.source_id
		WHERE st.olympiad_profile_id = `
	for id, want := range map[string][]string{
		"p669-48-obschestvoznanie": {
			"registration 2026-12-01 2027-01-11 онлайн " + rggu,
			"qualifying 2027-01-13 2027-01-27 онлайн " + rggu,
			"final 2027-02-21 2027-02-21 очно " + rggu,
		},
		// Отбор — интернет-тур или очно на площадках; финал не объявлен.
		"p669-36-tehnika-i-tehnologii": {
			"qualifying 2026-11-09 2026-12-09 онлайн " + zvezda,
			"qualifying 2026-11-09 2026-12-20 очно " + zvezda,
			"final 2027-01-19 2027-01-21 очно демо",
		},
		// Сроков 2026/27 нет: примерные — по прошлому сезону через 52 недели.
		"p669-74-astronomiya": {
			"qualifying 2026-12-15 2027-01-19 онлайн демо",
			"final 2027-02-07 2027-02-07 очно демо",
			"final 2027-03-14 2027-03-14 очно демо",
		},
	} {
		slices.Sort(want) // queryStrings сортирует строки
		if got := queryStrings(t, tx, stages+"'"+id+"'"); !slices.Equal(got, want) {
			t.Errorf("этапы %s после 0026: %v", id, got)
		}
	}
	changed := queryStrings(t, tx, `SELECT DISTINCT entity || ':' || entity_id FROM content_changes ORDER BY 1`)
	want := []string{}
	for _, p := range strings.Split(strings.Trim(profiles, "()"), ",") {
		want = append(want, "olympiad_profile:"+strings.Trim(strings.TrimSpace(p), "'"))
	}
	slices.Sort(want)
	if !slices.Equal(changed, want) {
		t.Fatalf("события 0026: %v, ожидали %v", changed, want)
	}

	mustExec(t, tx.Exec, `DELETE FROM content_changes`)
	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	if n := queryInt(t, tx, `SELECT count(*) FROM content_changes`); n != 0 {
		t.Fatalf("повторный прогон 0026 породил %d событий", n)
	}

	// Олимпиады, профили и этапы после миграции — ровно сид 0003.
	got := queryStrings(t, tx, rows)
	mustExec(t, tx.Exec, dbtest.UpSection(readMigration(t, "0003_seed_content.sql")))
	if want := queryStrings(t, tx, rows); !slices.Equal(got, want) {
		t.Fatalf("после 0026 не как в сиде:\nлишняя:   %v\nнедостаёт: %v",
			first(diffSorted(got, want)), first(diffSorted(want, got)))
	}
}
