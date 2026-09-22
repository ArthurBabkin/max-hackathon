package db_test

import (
	"context"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

// Реальные данные: у одной олимпиады бывает несколько профилей с одним
// школьным предметом (у НТО — 15 профилей с предметом «Информатика»).
// Схема обязана это принимать, а уникальность держится на слаге профиля.
func TestSchema_TwoProfilesOfOneOlympiadMayShareSubject(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	mustExec(t, tx.Exec, `INSERT INTO subjects (code, name) VALUES ('zz-test', 'Тестовый предмет')`)
	mustExec(t, tx.Exec, `INSERT INTO olympiads (id, name, kind) VALUES ('zz-olymp', 'Тест', 'perechen')`)
	mustExec(t, tx.Exec, `INSERT INTO olympiad_profiles
		(id, olympiad_id, subject_code, profile_slug, profile_name, level, school_year, grades_from, grades_to)
		VALUES ('zz-a', 'zz-olymp', 'zz-test', 'ai', 'искусственный интеллект', 'III', '2026/27', 8, 11),
		       ('zz-b', 'zz-olymp', 'zz-test', 'infosec', 'информационная безопасность', 'III', '2026/27', 8, 11)`)

	if _, err := tx.Exec(ctx, `INSERT INTO olympiad_profiles
		(id, olympiad_id, subject_code, profile_slug, level, school_year, grades_from, grades_to)
		VALUES ('zz-c', 'zz-olymp', 'zz-test', 'ai', 'III', '2026/27', 8, 11)`); err == nil {
		t.Fatal("два профиля с одним слагом в одной олимпиаде и году должны быть запрещены")
	}
}

func TestSchema_StagesCarryDemoFlagAndSource(t *testing.T) {
	pool := dbtest.Open(t)
	var cols int
	err := pool.QueryRow(context.Background(), `SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'stages' AND column_name IN ('is_demo', 'source_id', 'title')`).Scan(&cols)
	if err != nil {
		t.Fatal(err)
	}
	if cols != 3 {
		t.Fatalf("у stages должны быть is_demo, source_id и title, найдено %d", cols)
	}
}

func TestSchema_BotDialogTablesExist(t *testing.T) {
	pool := dbtest.Open(t)
	for _, name := range []string{"bot_dialogs", "bot_updates_seen"} {
		var ok bool
		if err := pool.QueryRow(context.Background(), "SELECT to_regclass($1) IS NOT NULL", name).Scan(&ok); err != nil || !ok {
			t.Fatalf("нет таблицы %s (err=%v)", name, err)
		}
	}
}

func mustExec[T any](t *testing.T, exec func(context.Context, string, ...any) (T, error), sql string) {
	t.Helper()
	if _, err := exec(context.Background(), sql); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
}
