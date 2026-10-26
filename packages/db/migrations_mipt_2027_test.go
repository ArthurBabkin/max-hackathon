package db_test

import (
	"context"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

// МФТИ объявил правила приёма 2027 (0031, #91): у льгот МФТИ — оговорки
// «С приёма 2027 года …», по событию с текстом объявления на льготу; семь
// олимпиад РСОШ и ВсОШ БВИ не теряют. Повторный прогон событий не пишет,
// откат убирает и оговорки, и события.
func TestMigration_Mipt2027Rules(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	raw := readMigration(t, "0031_mipt_2027_rules.sql")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const bvi = `note LIKE '%С приёма 2027 года БВИ дают только семь олимпиад РСОШ%'`
	const seven = `(SELECT id FROM olympiad_profiles WHERE olympiad_id IN
		('p669-14', 'p669-37', 'p669-43', 'p669-50', 'p669-54', 'p669-70', 'p669-81') OR olympiad_id NOT LIKE 'p669-%')`
	for _, table := range []string{"benefits", "direction_benefits"} {
		if n := queryInt(t, tx, `SELECT count(*) FROM `+table+` WHERE `+bvi+` AND olympiad_profile_id IN `+seven); n != 0 {
			t.Fatalf("%s: семь олимпиад и ВсОШ с оговоркой о БВИ: %d", table, n)
		}
		if n := queryInt(t, tx, `SELECT count(*) FROM `+table+` WHERE university_id = 'mipt' AND benefit IN ('bvi', 'bvi_winners')
			AND NOT `+bvi+` AND olympiad_profile_id NOT IN `+seven); n != 0 {
			t.Fatalf("%s: БВИ МФТИ по другим олимпиадам РСОШ без оговорки: %d", table, n)
		}
		if n := queryInt(t, tx, `SELECT count(*) FROM `+table+` WHERE university_id <> 'mipt' AND note LIKE '%С приёма 2027%'`); n != 0 {
			t.Fatalf("%s: оговорка МФТИ у другого вуза: %d", table, n)
		}
	}
	// Физтех по физике у МФТИ — БВИ победителю остаётся, оговорка только о пороге.
	if n := queryInt(t, tx, `SELECT count(*) FROM benefits WHERE university_id = 'mipt' AND olympiad_profile_id = 'p669-54-fizika'
		AND benefit = 'bvi_winners' AND NOT `+bvi+` AND note LIKE '%порог ЕГЭ по профильному предмету — 85%'`); n != 1 {
		t.Fatalf("Физтех по физике у МФТИ: %d", n)
	}

	// Как на проде: база до 0031, событий нет.
	const events = `SELECT count(*) FROM content_changes WHERE entity = 'benefit' AND detail->>'text' LIKE 'МФТИ объявил правила приёма 2027 года%'`
	noted := queryInt(t, tx, `SELECT count(*) FROM benefits WHERE university_id = 'mipt' AND note LIKE '%С приёма 2027 года%'`)
	mustExec(t, tx.Exec, dbtest.DownSection(raw))
	mustExec(t, tx.Exec, `DELETE FROM content_changes`)
	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	if n := queryInt(t, tx, `SELECT count(*) FROM content_changes`); n != noted || n == 0 {
		t.Fatalf("событий объявления %d, льгот МФТИ с оговоркой %d", n, noted)
	}
	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	if n := queryInt(t, tx, events); n != noted {
		t.Fatalf("повторный прогон: событий %d, ожидали %d", n, noted)
	}

	mustExec(t, tx.Exec, dbtest.DownSection(raw))
	if n := queryInt(t, tx, events); n != 0 {
		t.Fatalf("после отката событий объявления: %d", n)
	}
	for _, table := range []string{"benefits", "direction_benefits"} {
		if n := queryInt(t, tx, `SELECT count(*) FROM `+table+` WHERE note LIKE '%С приёма 2027%'`); n != 0 {
			t.Fatalf("%s: после отката оговорок: %d", table, n)
		}
	}
}
