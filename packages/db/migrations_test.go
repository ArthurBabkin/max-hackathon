package db_test

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"strings"
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

// «Напомнить завтра» адресно: у разового напоминания (offset 0) есть тот,
// кто его попросил, у плановых — нет, они уходят всем по их порогам.
func TestSchema_OneOffReminderHasRequester(t *testing.T) {
	pool := dbtest.Open(t)
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pg_constraint
		WHERE conname = 'reminders_requester_only_once'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("нет CHECK reminders_requester_only_once (n=%d, err=%v)", n, err)
	}
	var col int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'reminders' AND column_name = 'requested_by_member_id'`).Scan(&col)
	if col != 1 {
		t.Fatal("нет reminders.requested_by_member_id")
	}
}

func mustExec[T any](t *testing.T, exec func(context.Context, string, ...any) (T, error), sql string) {
	t.Helper()
	if _, err := exec(context.Background(), sql); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
}

// Сид контента — сгенерированная миграция 0003. Она должна лечь на настоящую
// схему со всеми CHECK и внешними ключами, а повторный прогон — ничего не
// менять: генератор пишет ON CONFLICT DO UPDATE по текстовым ключам.
func TestSeed_AppliesAndIsIdempotent(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	counts := func() map[string]int {
		t.Helper()
		out := map[string]int{}
		for _, table := range []string{"subjects", "directions", "universities", "sources",
			"olympiads", "olympiad_profiles", "stages", "benefits"} {
			var n int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
				t.Fatal(err)
			}
			out[table] = n
		}
		return out
	}
	before := counts()
	for table, n := range before {
		if n == 0 {
			t.Errorf("таблица %s пуста после сида", table)
		}
	}
	if before["olympiad_profiles"] < 192 {
		t.Errorf("профилей %d, ожидали все 192 из датасетов", before["olympiad_profiles"])
	}

	raw, err := os.ReadFile(filepath.Join(dbtest.MigrationsDir(), "0003_seed_content.sql"))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Conn().PgConn().Exec(ctx, dbtest.UpSection(string(raw))).ReadAll(); err != nil {
		t.Fatalf("повторный прогон сида: %v", err)
	}
	if after := counts(); !maps.Equal(before, after) {
		t.Fatalf("повторный прогон изменил число строк: было %v, стало %v", before, after)
	}
}

// Контракт: демо-этап не бывает «Фактом», а у ВсОШ ровно четыре этапа (F14).
func TestSeed_StagesAreHonest(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	var unsourcedFacts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM stages WHERE NOT is_demo AND source_id IS NULL`).Scan(&unsourcedFacts); err != nil {
		t.Fatal(err)
	}
	if unsourcedFacts != 0 {
		t.Errorf("%d этапов помечены фактом без источника", unsourcedFacts)
	}
	var wrong int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM (
		SELECT p.id FROM olympiad_profiles p JOIN olympiads o ON o.id = p.olympiad_id
		LEFT JOIN stages s ON s.olympiad_profile_id = p.id
		WHERE o.kind = 'vsosh' GROUP BY p.id HAVING count(s.id) <> 4) x`).Scan(&wrong); err != nil {
		t.Fatal(err)
	}
	if wrong != 0 {
		t.Errorf("%d профилей ВсОШ без ровно четырёх этапов", wrong)
	}
}

// Демо-траектория ссылается на профили из сида по id: если генератор сида
// переименует профиль, seed-demo упадёт у проверяющего. Ловим это здесь.
func TestDemoSeed_AppliesOnTopOfContent(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	raw, err := os.ReadFile(filepath.Join(dbtest.MigrationsDir(), "..", "migrations-demo", "0001_demo_trajectory.sql"))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Conn().PgConn().Exec(ctx, dbtest.UpSection(string(raw))).ReadAll(); err != nil {
		t.Fatalf("демо-траектория не легла: %v", err)
	}
	var items, pending int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM tracker_items").Scan(&items)
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM proposals WHERE status = 'pending'").Scan(&pending)
	if items != 2 || pending != 1 {
		t.Fatalf("трекер %d, ожидающих предложений %d", items, pending)
	}
}

// Описания олимпиад: у всех ВсОШ и у главных олимпиад перечня под
// IT и математику — они чаще всего попадают в подборку.
func TestContent_OlympiadDescriptions(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	var vsoshWithout, described int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM olympiads WHERE kind = 'vsosh' AND description IS NULL),
		(SELECT count(*) FROM olympiads WHERE description IS NOT NULL)`).Scan(&vsoshWithout, &described); err != nil {
		t.Fatal(err)
	}
	if vsoshWithout != 0 || described < 25 {
		t.Fatalf("ВсОШ без описания: %d, всего с описанием: %d", vsoshWithout, described)
	}
	for _, id := range []string{"p669-8", "p669-50", "p669-54", "p669-22", "p669-57", "p669-5"} {
		var d *string
		if err := pool.QueryRow(ctx, `SELECT description FROM olympiads WHERE id = $1`, id).Scan(&d); err != nil || d == nil {
			t.Fatalf("у %s нет описания (err=%v)", id, err)
		}
	}
	// Раньше вела на «Урок цифры».
	var url string
	if err := pool.QueryRow(ctx, `SELECT official_url FROM olympiads WHERE id = 'p669-37'`).Scan(&url); err != nil ||
		url != "https://mos.olimpiada.ru/" {
		t.Fatalf("сайт Московской олимпиады: %q (err=%v)", url, err)
	}
}

// Вымышленные олимпиады (F16) нужны только для локального показа: в проде
// пользователь принял бы их за настоящие.
func TestContent_NoFictionalOlympiads(t *testing.T) {
	pool := dbtest.Open(t)
	var olympiads, changes int
	if err := pool.QueryRow(context.Background(), `SELECT
		(SELECT count(*) FROM olympiads WHERE organizer = 'Вымышленный пример для демонстрации'),
		(SELECT count(*) FROM content_changes WHERE entity_id LIKE 'other-%')`).Scan(&olympiads, &changes); err != nil {
		t.Fatal(err)
	}
	if olympiads != 0 || changes != 0 {
		t.Fatalf("вымышленных олимпиад %d, событий изменения о них %d — ждали 0 и 0", olympiads, changes)
	}
}

// Если вымышленную олимпиаду успели взять в трекер, миграция её не трогает:
// каскад снёс бы пункт трекера пользователя.
func TestContent_FictionalOlympiadInTrackerSurvives(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	up, err := os.ReadFile(filepath.Join(dbtest.MigrationsDir(), "0009_hide_fictional_olympiads.sql"))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	mustExec(t, tx.Exec, `INSERT INTO olympiads (id, name, organizer, kind)
		VALUES ('other-tyk', 'Турнир юных программистов Казани', 'Вымышленный пример для демонстрации', 'other')`)
	mustExec(t, tx.Exec, `INSERT INTO olympiad_profiles (id, olympiad_id, subject_code, profile_slug, school_year, grades_from, grades_to)
		VALUES ('other-tyk-inf', 'other-tyk', 'inf', 'inf', '2026/27', 7, 11)`)
	mustExec(t, tx.Exec, `INSERT INTO trajectories (id, student_name, grade, region_code, goal_status)
		VALUES ('00000000-0000-4000-8000-00000000f001', 'Тест', 9, '16', 'known')`)
	mustExec(t, tx.Exec, `INSERT INTO tracker_items (trajectory_id, olympiad_profile_id)
		VALUES ('00000000-0000-4000-8000-00000000f001', 'other-tyk-inf')`)
	mustExec(t, tx.Exec, dbtest.UpSection(string(up)))

	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM tracker_items WHERE olympiad_profile_id = 'other-tyk-inf'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("пункт трекера пропал (n=%d, err=%v)", n, err)
	}
}

// migrations-demo катится и в прод, пока включён демо-режим в браузере, —
// вымышленных олимпиад там быть не должно: их увидели бы настоящие
// пользователи. Они живут в migrations-local, только на локальном стенде.
func TestDemoMigrations_HaveNoFictionalOlympiads(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(dbtest.MigrationsDir(), "..", "migrations-demo", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("нет демо-миграций: %v", err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if up := dbtest.UpSection(string(raw)); strings.Contains(up, "INSERT INTO olympiads") {
			t.Errorf("%s вставляет олимпиады — это прод-демо, а не локальный стенд", filepath.Base(f))
		}
	}
}

// Несколько чатов с помощником (F58): прежняя история участника становится
// его первым чатом. Название — день первой реплики в поясе траектории (F59):
// 21:30 UTC 20 сентября — это уже 21 сентября в Москве.
func TestMigration_AiChats_OldHistoryBecomesFirstChat(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	raw, err := os.ReadFile(filepath.Join(dbtest.MigrationsDir(), "0013_ai_chats.sql"))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Откат до схемы без чатов и история в старом виде: реплики только по участнику.
	mustExec(t, tx.Exec, dbtest.DownSection(string(raw)))
	mustExec(t, tx.Exec, `INSERT INTO users (id, max_user_id, first_name) VALUES
		('00000000-0000-4000-8000-00000000c001', 900000001, 'Артём'),
		('00000000-0000-4000-8000-00000000c002', 900000002, 'Ольга')`)
	mustExec(t, tx.Exec, `INSERT INTO trajectories (id, student_name, grade, region_code, tz, goal_status)
		VALUES ('00000000-0000-4000-8000-00000000c010', 'Артём', 9, '16', 'Europe/Moscow', 'known')`)
	mustExec(t, tx.Exec, `INSERT INTO members (id, trajectory_id, user_id, role, is_creator) VALUES
		('00000000-0000-4000-8000-00000000c101', '00000000-0000-4000-8000-00000000c010', '00000000-0000-4000-8000-00000000c001', 'kid', true),
		('00000000-0000-4000-8000-00000000c102', '00000000-0000-4000-8000-00000000c010', '00000000-0000-4000-8000-00000000c002', 'parent', false)`)
	mustExec(t, tx.Exec, `INSERT INTO ai_messages (member_id, role, text, created_at) VALUES
		('00000000-0000-4000-8000-00000000c101', 'user', 'первый', '2026-09-20 21:30:00+00'),
		('00000000-0000-4000-8000-00000000c101', 'assistant', 'ответ', '2026-09-20 21:30:00+00'),
		('00000000-0000-4000-8000-00000000c101', 'user', 'второй', '2026-09-22 08:00:00+00'),
		('00000000-0000-4000-8000-00000000c101', 'assistant', 'ответ', '2026-09-22 08:00:00+00')`)

	mustExec(t, tx.Exec, dbtest.UpSection(string(raw)))

	var chats int
	var member, title, created, last string
	err = tx.QueryRow(ctx, `SELECT count(*) OVER (), member_id::text, title,
		to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI'), to_char(last_message_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI')
		FROM ai_chats`).Scan(&chats, &member, &title, &created, &last)
	if err != nil {
		t.Fatal(err)
	}
	if chats != 1 || member != "00000000-0000-4000-8000-00000000c101" {
		t.Fatalf("чат должен появиться только у участника с историей: чатов %d, участник %s", chats, member)
	}
	if title != "Чат 21 сентября" || created != "2026-09-20 21:30" || last != "2026-09-22 08:00" {
		t.Fatalf("название %q, создан %s, последняя реплика %s", title, created, last)
	}
	var orphans int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM ai_messages WHERE chat_id IS NULL`).Scan(&orphans); err != nil || orphans != 0 {
		t.Fatalf("реплик без чата: %d (err=%v)", orphans, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ai_messages (member_id, role, text)
		VALUES ('00000000-0000-4000-8000-00000000c102', 'user', 'без чата')`); err == nil {
		t.Fatal("реплика без чата должна быть запрещена")
	}
}
