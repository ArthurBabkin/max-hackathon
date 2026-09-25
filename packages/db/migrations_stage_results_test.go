package db_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
	"github.com/jackc/pgx/v5"
)

// mustFail — оператор должен упасть; транзакция живёт дальше благодаря
// точке сохранения.
func mustFail(t *testing.T, tx pgx.Tx, sql string) {
	t.Helper()
	ctx := context.Background()
	mustExec(t, tx.Exec, `SAVEPOINT must_fail`)
	if _, err := tx.Exec(ctx, sql); err == nil {
		t.Fatalf("оператор прошёл, а должен был упасть:\n%s", sql)
	}
	mustExec(t, tx.Exec, `ROLLBACK TO SAVEPOINT must_fail`)
}

// Отметки этапов и вопросы бота об итоге (0019) ложатся на базу с живыми
// напоминаниями, а откат убирает только новое: плановые напоминания
// остаются, вопросы об итоге (отрицательные смещения) удаляются.
func TestMigration_TrackerStageResults(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	raw, err := os.ReadFile(filepath.Join(dbtest.MigrationsDir(), "0019_tracker_stage_results.sql"))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	mustExec(t, tx.Exec, dbtest.DownSection(string(raw)))
	mustExec(t, tx.Exec, `INSERT INTO users (id, max_user_id, first_name) VALUES
		('00000000-0000-4000-8000-00000000e001', 900000021, 'Артём')`)
	mustExec(t, tx.Exec, `INSERT INTO trajectories (id, student_name, grade, region_code, goal_status) VALUES
		('00000000-0000-4000-8000-00000000e010', 'Артём', 10, '16', 'exploring')`)
	mustExec(t, tx.Exec, `INSERT INTO members (id, trajectory_id, user_id, role, is_creator) VALUES
		('00000000-0000-4000-8000-00000000e020', '00000000-0000-4000-8000-00000000e010', '00000000-0000-4000-8000-00000000e001', 'kid', true)`)
	mustExec(t, tx.Exec, `INSERT INTO tracker_items (id, trajectory_id, olympiad_profile_id, registered_at) VALUES
		('00000000-0000-4000-8000-00000000e030', '00000000-0000-4000-8000-00000000e010', 'p669-14-biologiya', now())`)
	mustExec(t, tx.Exec, `INSERT INTO reminders (tracker_item_id, stage_id, offset_days, fire_at) VALUES
		('00000000-0000-4000-8000-00000000e030', 'p669-14-biologiya:qualifying:1', 7, now())`)

	mustExec(t, tx.Exec, dbtest.UpSection(string(raw)))

	const item = `'00000000-0000-4000-8000-00000000e030'`
	mustExec(t, tx.Exec, `INSERT INTO tracker_stage_results (tracker_item_id, stage_id, result, set_by_member_id) VALUES
		(`+item+`, 'p669-14-biologiya:qualifying:1', 'passed', '00000000-0000-4000-8000-00000000e020')`)
	mustExec(t, tx.Exec, `INSERT INTO tracker_stage_results (tracker_item_id, stage_id, registered) VALUES
		(`+item+`, 'p669-14-biologiya:registration:2', true)`)
	mustFail(t, tx, `INSERT INTO tracker_stage_results (tracker_item_id, stage_id, result) VALUES
		(`+item+`, 'p669-14-biologiya:final:1', 'champion')`)
	mustFail(t, tx, `INSERT INTO tracker_stage_results (tracker_item_id, stage_id) VALUES
		(`+item+`, 'p669-14-biologiya:final:1')`)

	// Вопрос об итоге — на следующий день и через неделю после этапа.
	mustExec(t, tx.Exec, `INSERT INTO reminders (tracker_item_id, stage_id, offset_days, fire_at) VALUES
		(`+item+`, 'p669-14-biologiya:qualifying:1', -1, now()),
		(`+item+`, 'p669-14-biologiya:qualifying:1', -8, now())`)
	mustFail(t, tx, `INSERT INTO reminders (tracker_item_id, stage_id, offset_days, fire_at) VALUES
		(`+item+`, 'p669-14-biologiya:qualifying:1', -1, now())`)
	mustFail(t, tx, `INSERT INTO reminders (tracker_item_id, stage_id, offset_days, fire_at) VALUES
		(`+item+`, 'p669-14-biologiya:qualifying:1', -2, now())`)
	// Прежний ON CONFLICT плановых напоминаний работает — старый код
	// функций живёт рядом с новой схемой, пока идёт выкладка.
	mustExec(t, tx.Exec, `INSERT INTO reminders (tracker_item_id, stage_id, offset_days, fire_at) VALUES
		(`+item+`, 'p669-14-biologiya:qualifying:1', 7, now())
		ON CONFLICT (tracker_item_id, stage_id, offset_days) WHERE offset_days > 0 DO NOTHING`)

	var marks int
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM tracker_stage_results WHERE tracker_item_id = `+item).Scan(&marks)
	if marks != 2 {
		t.Fatalf("отметок %d, ожидали 2", marks)
	}

	mustExec(t, tx.Exec, dbtest.DownSection(string(raw)))
	var planned, asks int
	_ = tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE offset_days > 0), count(*) FILTER (WHERE offset_days < 0)
		FROM reminders WHERE tracker_item_id = `+item).Scan(&planned, &asks)
	if planned != 1 || asks != 0 {
		t.Fatalf("после отката: плановых %d (ожидали 1), вопросов %d (ожидали 0)", planned, asks)
	}
	mustExec(t, tx.Exec, dbtest.UpSection(string(raw)))

	// Пункт убрали из трекера — отметки уходят с ним.
	mustExec(t, tx.Exec, `INSERT INTO tracker_stage_results (tracker_item_id, stage_id, result) VALUES
		(`+item+`, 'p669-14-biologiya:qualifying:1', 'failed')`)
	mustExec(t, tx.Exec, `DELETE FROM tracker_items WHERE id = `+item)
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM tracker_stage_results WHERE tracker_item_id = `+item).Scan(&marks)
	if marks != 0 {
		t.Fatalf("отметки пережили пункт трекера: %d", marks)
	}
}
