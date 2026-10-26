package db_test

import (
	"context"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

// Подробности изменения (0030): правка этапа и льготы оставляет в событии,
// какими они были и какими стали; откат возвращает триггеры 0006 и убирает
// столбец, повторный накат — снова пишет подробности.
func TestMigration_ContentChangeDetails(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	raw := readMigration(t, "0030_content_change_details.sql")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var stage string
	if err := tx.QueryRow(ctx, `SELECT id FROM stages WHERE olympiad_profile_id = 'p669-8-informatika'
		AND kind = 'final' ORDER BY id LIMIT 1`).Scan(&stage); err != nil {
		t.Fatal(err)
	}
	var uni string
	if err := tx.QueryRow(ctx, `SELECT university_id FROM benefits WHERE olympiad_profile_id = 'p669-8-informatika'
		ORDER BY university_id LIMIT 1`).Scan(&uni); err != nil {
		t.Fatal(err)
	}
	edit := func() {
		t.Helper()
		mustExec(t, tx.Exec, `DELETE FROM content_changes`)
		mustExec(t, tx.Exec, `UPDATE stages SET starts_at = starts_at + interval '1 day', ends_at = ends_at + interval '1 day'
			WHERE id = '`+stage+`'`)
		mustExec(t, tx.Exec, `UPDATE stages SET starts_at = starts_at + interval '1 day', ends_at = ends_at + interval '1 day'
			WHERE id = '`+stage+`'`)
		mustExec(t, tx.Exec, `UPDATE benefits SET ege_min = coalesce(ege_min, 90) + 1 WHERE olympiad_profile_id = 'p669-8-informatika'
			AND university_id = '`+uni+`'`)
	}

	edit()
	if n := queryInt(t, tx, `SELECT count(*) FROM content_changes WHERE entity = 'olympiad_profile'
		AND detail->>'title' <> '' AND detail->>'kind' = 'final' AND detail ? 'old' AND detail ? 'new'`); n != 2 {
		t.Fatalf("событий этапа с подробностями %d, ожидали 2", n)
	}
	// Вторая правка — позже первой, хоть обе в одной транзакции.
	if n := queryInt(t, tx, `SELECT count(DISTINCT created_at) FROM content_changes WHERE entity = 'olympiad_profile'`); n != 2 {
		t.Fatalf("у правок этапа одно время: %d", n)
	}
	if n := queryInt(t, tx, `SELECT count(*) FROM content_changes WHERE entity = 'benefit'
		AND entity_id = 'p669-8-informatika@`+uni+`'
		AND (detail->'new'->>'ege_min')::int = coalesce((detail->'old'->>'ege_min')::int, 90) + 1`); n != 1 {
		t.Fatalf("событий льготы с новым порогом %d, ожидали 1", n)
	}

	mustExec(t, tx.Exec, dbtest.DownSection(raw))
	if n := queryInt(t, tx, `SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'content_changes' AND column_name = 'detail'`); n != 0 {
		t.Fatal("после отката столбец detail остался")
	}
	edit()
	if n := queryInt(t, tx, `SELECT count(*) FROM content_changes`); n != 3 {
		t.Fatalf("после отката событий %d, ожидали 3 — триггеры 0006 должны работать", n)
	}

	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	edit()
	if n := queryInt(t, tx, `SELECT count(*) FROM content_changes WHERE detail IS NOT NULL`); n != 3 {
		t.Fatalf("после повторного наката событий с подробностями %d, ожидали 3", n)
	}
}
