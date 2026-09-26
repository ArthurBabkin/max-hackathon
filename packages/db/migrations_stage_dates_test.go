package db_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

// Сроки 2026/27 (0023) на базе с придуманными датами: после миграции этапы
// ВсОШ и Московской олимпиады по информатике — ровно как в сиде, с
// источниками; событие «изменились сроки» — по каждому из этих профилей и ни
// по чему больше; повторный прогон событий не рождает.
func TestMigration_StageDates(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	raw := readMigration(t, "0023_stage_dates_2026_27.sql")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const touched = `(olympiad_profile_id LIKE 'vsosh-%' OR olympiad_profile_id = 'p669-37-informatika')`
	const rows = `SELECT s::text FROM stages s WHERE ` + touched
	const facts = `SELECT count(*) FROM stages WHERE olympiad_profile_id LIKE 'vsosh-%' AND NOT is_demo`

	// Как на проде до миграции.
	mustExec(t, tx.Exec, dbtest.DownSection(raw))
	if n := queryInt(t, tx, facts); n != 0 {
		t.Fatalf("до 0023 у ВсОШ %d фактических этапов, ожидали 0", n)
	}
	if n := queryInt(t, tx, `SELECT count(*) FROM sources WHERE url LIKE 'https://siriusolymp.ru/%'`); n != 0 {
		t.Fatalf("Down оставил источник «Сириуса»: %d", n)
	}
	mustExec(t, tx.Exec, `DELETE FROM content_changes`)

	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	all := queryInt(t, tx, `SELECT count(*) FROM stages WHERE olympiad_profile_id LIKE 'vsosh-%'`)
	if n := queryInt(t, tx, facts); n != all || all == 0 {
		t.Fatalf("после 0023 фактических этапов ВсОШ %d из %d", n, all)
	}
	const stage = `SELECT concat_ws(' ', to_char(starts_at AT TIME ZONE 'Europe/Moscow', 'YYYY-MM-DD'),
		to_char(ends_at AT TIME ZONE 'Europe/Moscow', 'YYYY-MM-DD'),
		CASE WHEN is_online THEN 'онлайн' ELSE 'очно' END, src.url)
		FROM stages st JOIN sources src ON src.id = st.source_id WHERE st.id = `
	for id, want := range map[string]string{
		// Школьный этап на «Сириусе» — окно, в которое регионы пишут тур.
		"vsosh-informatika:school:1": "2026-10-19 2026-10-23 онлайн https://siriusolymp.ru/school2026/about",
		// Предмет не на «Сириусе» — только крайний срок Порядка.
		"vsosh-ekonomika:school:1":  "2026-11-01 очно https://vserosolimp.edsoo.ru/",
		"vsosh-informatika:final:1": "2027-04-30 очно https://vserosolimp.edsoo.ru/",
		// Московская олимпиада: туры 10–11 классов, финал ещё не объявлен.
		"p669-37-informatika:qualifying:2": "2027-02-28 2027-02-28 очно https://mos.olimpiada.ru/schedule",
	} {
		if got := queryStrings(t, tx, stage+"'"+id+"'"); !slices.Equal(got, []string{want}) {
			t.Errorf("%s: %v, ожидали %q", id, got, want)
		}
	}
	changed := queryInt(t, tx, `SELECT count(DISTINCT entity_id) FROM content_changes
		WHERE entity = 'olympiad_profile' AND `+strings.ReplaceAll(touched, "olympiad_profile_id", "entity_id"))
	other := queryInt(t, tx, `SELECT count(*) FROM content_changes
		WHERE entity <> 'olympiad_profile' OR NOT `+strings.ReplaceAll(touched, "olympiad_profile_id", "entity_id"))
	profiles := queryInt(t, tx, `SELECT count(DISTINCT olympiad_profile_id) FROM stages WHERE `+touched)
	if changed != profiles || other != 0 {
		t.Fatalf("события 0023: профилей %d из %d, других %d", changed, profiles, other)
	}
	// Отдельной регистрации в расписании нет — демо-этап уходит.
	if n := queryInt(t, tx, `SELECT count(*) FROM stages WHERE id = 'p669-37-informatika:registration:1'`); n != 0 {
		t.Fatal("демо-регистрация Московской олимпиады по информатике осталась")
	}

	mustExec(t, tx.Exec, `DELETE FROM content_changes`)
	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	if n := queryInt(t, tx, `SELECT count(*) FROM content_changes`); n != 0 {
		t.Fatalf("повторный прогон 0023 породил %d событий", n)
	}

	// Этапы после миграции — ровно сид 0003.
	got := queryStrings(t, tx, rows)
	mustExec(t, tx.Exec, dbtest.UpSection(readMigration(t, "0003_seed_content.sql")))
	if want := queryStrings(t, tx, rows); !slices.Equal(got, want) {
		t.Fatalf("этапы после 0023 не как в сиде:\nлишняя:   %v\nнедостаёт: %v",
			first(diffSorted(got, want)), first(diffSorted(want, got)))
	}
}
