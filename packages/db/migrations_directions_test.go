package db_test

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
	"github.com/jackc/pgx/v5"
)

// Вузы и направления (0020 — схема, 0021 — контент из датасетов A и B):
// льгота уточняется до направления вуза, ученик выбирает направления в вузе.

func readMigration(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dbtest.MigrationsDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func queryInt(t *testing.T, q rowQuerier, sql string) int {
	t.Helper()
	var n int
	if err := q.QueryRow(context.Background(), sql).Scan(&n); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
	return n
}

func queryStrings(t *testing.T, q rowQuerier, sql string) []string {
	t.Helper()
	var out []string
	if err := q.QueryRow(context.Background(), `SELECT coalesce(array_agg(x ORDER BY x), '{}') FROM (`+sql+`) s(x)`).Scan(&out); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
	return out
}

// Счётчики контента 0021 — числа из датасетов на 21.09.2026 с исправленной
// привязкой льгот к программам (0022), без филиалов (0024) и с направлениями
// Иннополиса из правил приёма (0028).
func universityDirectionCounts(t *testing.T, q rowQuerier) map[string]int {
	t.Helper()
	return map[string]int{
		"directions":   queryInt(t, q, `SELECT count(*) FROM directions`),
		"onboarding":   queryInt(t, q, `SELECT count(*) FROM directions WHERE onboarding`),
		"no_groups":    queryInt(t, q, `SELECT count(*) FROM directions WHERE groups = '{}'`),
		"pairs":        queryInt(t, q, `SELECT count(*) FROM university_directions`),
		"to_check":     queryInt(t, q, `SELECT count(*) FROM university_directions WHERE status = 'to_check'`),
		"benefits":     queryInt(t, q, `SELECT count(*) FROM direction_benefits`),
		"varies":       queryInt(t, q, `SELECT count(*) FROM direction_benefits WHERE varies`),
		"user_choices": queryInt(t, q, `SELECT count(*) FROM trajectory_university_directions`),
	}
}

var wantUniversityDirections = map[string]int{
	"directions": 67, "onboarding": 16, "no_groups": 0,
	"pairs": 164, "to_check": 0,
	"benefits": 11519, "varies": 841,
	"user_choices": 0,
}

// То же до 0028 — у Иннополиса одна укрупнённая группа 09.00.00.
var wantBefore0028 = map[string]int{
	"directions": 68, "onboarding": 16, "no_groups": 0,
	"pairs": 163, "to_check": 0,
	"benefits": 11411, "varies": 841,
	"user_choices": 0,
}

// То же до 0024 — с программами филиалов, как на проде после 0022.
var wantWithBranches = map[string]int{
	"directions": 72, "onboarding": 16, "no_groups": 0,
	"pairs": 173, "to_check": 0,
	"benefits": 12769, "varies": 1631,
	"user_choices": 0,
}

func TestMigration_UniversityDirections_Content(t *testing.T) {
	pool := dbtest.Open(t)
	if got := universityDirectionCounts(t, pool); !maps.Equal(got, wantUniversityDirections) {
		t.Fatalf("контент 0021:\n%v\nожидали:\n%v", got, wantUniversityDirections)
	}
	// Пар «льготы уточняются» в сиде нет: ВШЭ — 40.03.01 держалась на
	// очно-заочной программе, её в A больше нет (#79).
	if n := queryInt(t, pool, `SELECT count(*) FROM university_directions WHERE status = 'to_check'`); n != 0 {
		t.Fatalf("to_check: %d, ожидали 0", n)
	}
	// Иннополис: направления из правил приёма, укрупнённой группы нет (0028).
	if got := queryStrings(t, pool, `SELECT direction_id FROM university_directions
		WHERE university_id = 'innopolis' AND status = 'offered'`); !slices.Equal(got, []string{"napr-09-03-01", "napr-15-03-06"}) {
		t.Fatalf("направления Иннополиса: %v", got)
	}
	if n := queryInt(t, pool, `SELECT count(*) FROM university_directions
		WHERE programs < 1 OR cardinality(program_names) < 1`); n != 0 {
		t.Fatalf("пар без программ: %d", n)
	}
}

func TestMigration_UniversityDirections_CodeIsGenerated(t *testing.T) {
	pool := dbtest.Open(t)
	var code string
	if err := pool.QueryRow(context.Background(), `SELECT code FROM directions WHERE id = 'napr-09-03-04'`).Scan(&code); err != nil || code != "09.03.04" {
		t.Fatalf("code napr-09-03-04: %q (err=%v)", code, err)
	}
	if n := queryInt(t, pool, `SELECT count(*) FROM directions WHERE code !~ '^\d\d\.\d\d\.\d\d$'`); n != 0 {
		t.Fatalf("направлений с кривым кодом: %d", n)
	}
}

// Льгота по направлению — уточнение льготы вуза: у каждой строки
// direction_benefits есть строка benefits того же профиля, вуза и года, и
// наоборот (кроме баллов вне перечня — у них направлений нет).
func TestMigration_UniversityDirections_BenefitsRefineUniversityBenefits(t *testing.T) {
	pool := dbtest.Open(t)
	orphans := queryInt(t, pool, `SELECT count(*) FROM direction_benefits d WHERE NOT EXISTS (
		SELECT 1 FROM benefits b WHERE (b.olympiad_profile_id, b.university_id, b.admission_year)
		                              = (d.olympiad_profile_id, d.university_id, d.admission_year))`)
	uncovered := queryInt(t, pool, `SELECT count(*) FROM benefits b WHERE b.benefit <> 'extra_points' AND NOT EXISTS (
		SELECT 1 FROM direction_benefits d WHERE (b.olympiad_profile_id, b.university_id, b.admission_year)
		                                       = (d.olympiad_profile_id, d.university_id, d.admission_year))`)
	if orphans != 0 || uncovered != 0 {
		t.Fatalf("льготы по направлениям без льготы вуза: %d, льготы вуза без направлений: %d", orphans, uncovered)
	}
	// «Зависит от программы» — ровно у строк с varies.
	mismatch := queryInt(t, pool, `SELECT count(*) FROM direction_benefits
		WHERE varies <> coalesce(note LIKE 'Зависит от программы: %', false)`)
	if mismatch != 0 {
		t.Fatalf("строк, где varies не совпадает с пояснением: %d", mismatch)
	}
}

// Шестнадцать направлений онбординга — те же, что в сиде 0003: 0021 им
// только проставляет группы.
func TestMigration_UniversityDirections_OnboardingDirectionsAsInSeed(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	onboarding := func() []string {
		return queryStrings(t, tx, `SELECT id || ' ' || name || ' ' || subject_codes::text || ' ' || code
			FROM directions WHERE onboarding`)
	}
	before := onboarding()
	// Строка направлений из сида: повторный прогон вернул бы имя и предметы,
	// если бы 0021 их поменяла.
	seed := readMigration(t, "0003_seed_content.sql")
	m := regexp.MustCompile(`(?s)INSERT INTO directions .*?;\n`).FindString(seed)
	if m == "" {
		t.Fatal("в 0003 нет вставки направлений")
	}
	mustExec(t, tx.Exec, m)
	if after := onboarding(); len(before) != 16 || !slices.Equal(before, after) {
		t.Fatalf("направления онбординга после 0021:\n%v\nв сиде 0003:\n%v", before, after)
	}
	if n := queryInt(t, tx, `SELECT count(*) FROM directions WHERE onboarding AND groups = '{}'`); n != 0 {
		t.Fatalf("направлений онбординга без групп: %d", n)
	}
}

// Новые таблицы без триггеров: триггер 0006 на benefits рассылает семьям
// «изменились льготы», а загрузка льгот по направлениям — не изменение.
func TestMigration_UniversityDirections_NoContentChanges(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	triggers := queryInt(t, pool, `SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal AND tgrelid IN (
		'university_directions'::regclass, 'direction_benefits'::regclass, 'trajectory_university_directions'::regclass)`)
	if triggers != 0 {
		t.Fatalf("триггеров на новых таблицах: %d", triggers)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	content := readMigration(t, "0021_university_directions_content.sql")
	mustExec(t, tx.Exec, dbtest.DownSection(content))
	mustExec(t, tx.Exec, dbtest.UpSection(content))
	if n := queryInt(t, tx, `SELECT count(*) FROM content_changes`); n != 0 {
		t.Fatalf("0021 породила %d событий изменения контента", n)
	}
}

// Откат удаляет контент и каскадом — выбор пользователей по новым
// направлениям; повторный накат возвращает всё как было, а повторный
// прогон контента ничего не меняет.
func TestMigration_UniversityDirections_DownUp(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	schema := readMigration(t, "0020_university_directions.sql")
	content := readMigration(t, "0021_university_directions_content.sql")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const tid = "00000000-0000-4000-8000-00000000f010"
	mustExec(t, tx.Exec, `INSERT INTO trajectories (id, student_name, grade, region_code, goal_status) VALUES
		('`+tid+`', 'Артём', 10, '16', 'known')`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_directions (trajectory_id, direction_id, position) VALUES
		('`+tid+`', 'napr-09-03-04', 0), ('`+tid+`', 'napr-15-03-06', 1)`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_universities (trajectory_id, university_id) VALUES
		('`+tid+`', 'innopolis'), ('`+tid+`', 'msu')`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_university_directions (trajectory_id, university_id, direction_id) VALUES
		('`+tid+`', 'innopolis', 'napr-15-03-06'), ('`+tid+`', 'msu', 'napr-01-03-02')`)
	want := maps.Clone(wantUniversityDirections)
	want["user_choices"] = 2
	if got := universityDirectionCounts(t, tx); !maps.Equal(got, want) {
		t.Fatalf("до отката:\n%v\nожидали:\n%v", got, want)
	}
	mustExec(t, tx.Exec, dbtest.DownSection(content))
	if n := queryInt(t, tx, `SELECT count(*) FROM directions`); n != 16 {
		t.Fatalf("после отката 0021 направлений %d, ожидали 16", n)
	}
	if got := queryStrings(t, tx, `SELECT direction_id FROM trajectory_directions WHERE trajectory_id = '`+tid+`'`); !slices.Equal(got, []string{"napr-09-03-04"}) {
		t.Fatalf("направления траектории после отката: %v", got)
	}
	if n := queryInt(t, tx, `SELECT count(*) FROM trajectory_university_directions`); n != 0 {
		t.Fatalf("выбор направлений в вузах после отката: %d", n)
	}
	if n := queryInt(t, tx, `SELECT count(*) FROM trajectory_universities WHERE trajectory_id = '`+tid+`'`); n != 2 {
		t.Fatalf("вузы траектории после отката: %d, ожидали 2", n)
	}
	// Источники, которые появились только в 0021 (их перечисляет её Down),
	// уходят вместе с ней. Источники 0003 остаются, даже без ссылок: сид
	// держит те, на которые ссылаются миграции после него (0018, 0022).
	own := regexp.MustCompile(`src-[0-9a-f]{12}`).FindAllString(dbtest.DownSection(content), -1)
	if len(own) == 0 {
		t.Fatal("Down 0021 не удаляет своих источников")
	}
	if n := queryInt(t, tx, `SELECT count(*) FROM sources WHERE id IN ('`+strings.Join(own, "', '")+`')`); n != 0 {
		t.Fatalf("после отката 0021 осталось %d её источников", n)
	}

	mustExec(t, tx.Exec, dbtest.DownSection(schema))
	var gone bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('university_directions') IS NULL
		AND to_regclass('direction_benefits') IS NULL
		AND to_regclass('trajectory_university_directions') IS NULL
		AND NOT EXISTS (SELECT 1 FROM information_schema.columns
		                WHERE table_name = 'directions' AND column_name IN ('code', 'groups', 'onboarding'))`).Scan(&gone); err != nil || !gone {
		t.Fatalf("после отката 0020 схема не прежняя (err=%v)", err)
	}

	mustExec(t, tx.Exec, dbtest.UpSection(schema))
	mustExec(t, tx.Exec, dbtest.UpSection(content))
	if got := universityDirectionCounts(t, tx); !maps.Equal(got, wantUniversityDirections) {
		t.Fatalf("после повторного наката:\n%v\nожидали:\n%v", got, wantUniversityDirections)
	}
	mustExec(t, tx.Exec, dbtest.UpSection(content))
	if got := universityDirectionCounts(t, tx); !maps.Equal(got, wantUniversityDirections) {
		t.Fatalf("повторный прогон 0021 изменил контент:\n%v\nожидали:\n%v", got, wantUniversityDirections)
	}
}

// Выбор направления в вузе живёт, пока вуз выбран: убрали вуз из
// траектории — ушёл и выбор; без выбранного вуза выбрать направление нельзя.
func TestMigration_UniversityDirections_ChoiceGoesWithUniversity(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const tid = "00000000-0000-4000-8000-00000000f020"
	mustExec(t, tx.Exec, `INSERT INTO trajectories (id, student_name, grade, region_code, goal_status) VALUES
		('`+tid+`', 'Артём', 10, '16', 'known')`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_universities (trajectory_id, university_id) VALUES ('`+tid+`', 'msu')`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_university_directions (trajectory_id, university_id, direction_id) VALUES
		('`+tid+`', 'msu', 'napr-01-03-02')`)

	sp, err := tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sp.Exec(ctx, `INSERT INTO trajectory_university_directions (trajectory_id, university_id, direction_id) VALUES
		('`+tid+`', 'innopolis', 'napr-09-03-01')`)
	if err == nil || !strings.Contains(err.Error(), "foreign key") {
		t.Fatalf("направление в невыбранном вузе должно быть запрещено, err=%v", err)
	}
	_ = sp.Rollback(ctx)

	mustExec(t, tx.Exec, `DELETE FROM trajectory_universities WHERE trajectory_id = '`+tid+`' AND university_id = 'msu'`)
	if n := queryInt(t, tx, `SELECT count(*) FROM trajectory_university_directions WHERE trajectory_id = '`+tid+`'`); n != 0 {
		t.Fatalf("выбор направлений пережил вуз: %d", n)
	}
}

// 0022: льготы, которые парсер приписывал чужим программам (#68). До миграции
// ВсОШ по биологии давала льготу направлению ПМИ МГУ (через филиал в
// Севастополе), после — нет; контент совпадает со свежим сидом, события
// «изменились льготы» — только по парам профиль@вуз, где льгота правда
// поменялась, а повторный прогон ничего не меняет.
func TestMigration_BenefitLinking(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	raw := readMigration(t, "0022_benefit_linking.sql")
	branches := readMigration(t, "0024_drop_branches.sql")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	// 0022 проверяется в том состоянии, в котором её накатывали: до 0024 и 0028.
	mustExec(t, tx.Exec, dbtest.DownSection(readMigration(t, "0028_innopolis_directions.sql")))
	mustExec(t, tx.Exec, dbtest.DownSection(branches))

	const bioOnPmi = `SELECT count(*) FROM direction_benefits WHERE olympiad_profile_id = 'vsosh-biologiya'
		AND university_id = 'msu' AND direction_id = 'napr-01-03-02'`
	benefits := queryInt(t, tx, `SELECT count(*) FROM benefits`)

	// Как на проде до миграции.
	mustExec(t, tx.Exec, dbtest.DownSection(raw))
	if n := queryInt(t, tx, bioOnPmi); n != 1 {
		t.Fatalf("до 0022 ВсОШ по биологии на ПМИ МГУ: %d строк, ожидали 1", n)
	}
	// Ученик выбрал в ВШЭ «Юриспруденцию» — пару, которая держалась на
	// очно-заочной программе (#79).
	const tid = "00000000-0000-4000-8000-00000000f022"
	mustExec(t, tx.Exec, `INSERT INTO trajectories (id, student_name, grade, region_code, goal_status) VALUES
		('`+tid+`', 'Артём', 10, '16', 'known')`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_directions (trajectory_id, direction_id, position) VALUES ('`+tid+`', 'napr-40-03-01', 0)`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_universities (trajectory_id, university_id) VALUES ('`+tid+`', 'hse')`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_university_directions (trajectory_id, university_id, direction_id) VALUES
		('`+tid+`', 'hse', 'napr-40-03-01')`)
	mustExec(t, tx.Exec, `DELETE FROM content_changes`)

	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	if n := queryInt(t, tx, bioOnPmi); n != 0 {
		t.Fatalf("после 0022 ВсОШ по биологии на ПМИ МГУ: %d строк", n)
	}
	// Пары больше нет — её выбор снят каскадом (user_choices ниже — 0), а
	// направление и вуз в цели ученика остались.
	if n := queryInt(t, tx, `SELECT count(*) FROM trajectory_directions WHERE trajectory_id = '`+tid+`'`) +
		queryInt(t, tx, `SELECT count(*) FROM trajectory_universities WHERE trajectory_id = '`+tid+`'`); n != 2 {
		t.Fatalf("цель ученика после 0022: направлений и вузов %d, ожидали 2", n)
	}
	if got := universityDirectionCounts(t, tx); !maps.Equal(got, wantWithBranches) {
		t.Fatalf("после 0022:\n%v\nожидали:\n%v", got, wantWithBranches)
	}
	if n := queryInt(t, tx, `SELECT count(*) FROM benefits`); n != benefits {
		t.Fatalf("льгот вузов после 0022: %d, в сиде %d", n, benefits)
	}
	changed := queryInt(t, tx, `SELECT count(DISTINCT entity_id) FROM content_changes WHERE entity = 'benefit'`)
	other := queryInt(t, tx, `SELECT count(*) FROM content_changes WHERE entity <> 'benefit'`)
	if changed != 1126 || other != 0 {
		t.Fatalf("события 0022: льгот %d (ожидали 1126), других %d", changed, other)
	}

	mustExec(t, tx.Exec, `DELETE FROM content_changes`)
	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	if n := queryInt(t, tx, `SELECT count(*) FROM content_changes`); n != 0 {
		t.Fatalf("повторный прогон 0022 породил %d событий", n)
	}
}

// 0024: филиалы — отдельные вузы. До миграции (как на проде) Всесибирская по
// физике давала ВШЭ на 01.03.01 БВИ — это нижегородская программа, после —
// 100 баллов победителю, как в Москве. Пары, которые держались на филиалах,
// уходят вместе с выбором учеников; направление без программ удаляется, только
// если его никто не выбрал. События — только по льготам, повторный прогон
// ничего не меняет.
func TestMigration_DropBranches(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	raw := readMigration(t, "0024_drop_branches.sql")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const fizika = `SELECT benefit FROM direction_benefits WHERE olympiad_profile_id = 'p669-14-fizika'
		AND university_id = 'hse' AND direction_id = 'napr-01-03-01'`
	const branchDirections = `SELECT count(*) FROM universities
		WHERE id = 'hse' AND 'Государственное и муниципальное управление' = ANY(directions)`
	benefit := func() string {
		var b string
		if err := tx.QueryRow(ctx, fizika).Scan(&b); err != nil {
			t.Fatal(err)
		}
		return b
	}

	// Как на проде до миграции: без 0028 и 0024.
	mustExec(t, tx.Exec, dbtest.DownSection(readMigration(t, "0028_innopolis_directions.sql")))
	mustExec(t, tx.Exec, dbtest.DownSection(raw))
	if got := universityDirectionCounts(t, tx); !maps.Equal(got, wantWithBranches) {
		t.Fatalf("до 0024:\n%v\nожидали:\n%v", got, wantWithBranches)
	}
	if b := benefit(); b != "bvi" || queryInt(t, tx, branchDirections) != 1 {
		t.Fatalf("до 0024: льгота %q, направление филиала у ВШЭ %d", b, queryInt(t, tx, branchDirections))
	}
	// Ученик выбрал «Психологию» в ВШЭ (Нижний Новгород) и цель «Социология».
	const tid = "00000000-0000-4000-8000-00000000f024"
	mustExec(t, tx.Exec, `INSERT INTO trajectories (id, student_name, grade, region_code, goal_status) VALUES
		('`+tid+`', 'Артём', 10, '52', 'known')`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_directions (trajectory_id, direction_id, position) VALUES
		('`+tid+`', 'napr-37-03-01', 0), ('`+tid+`', 'napr-39-03-01', 1)`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_universities (trajectory_id, university_id) VALUES ('`+tid+`', 'hse')`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_university_directions (trajectory_id, university_id, direction_id) VALUES
		('`+tid+`', 'hse', 'napr-37-03-01')`)
	mustExec(t, tx.Exec, `DELETE FROM content_changes`)

	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	if b := benefit(); b != "score100" || queryInt(t, tx, branchDirections) != 0 {
		t.Fatalf("после 0024: льгота %q, направление филиала у ВШЭ %d", b, queryInt(t, tx, branchDirections))
	}
	// Выбранные учеником направления остались, остальные без программ ушли.
	if got := queryStrings(t, tx, `SELECT id FROM directions WHERE id IN
		('napr-37-03-01', 'napr-39-03-01', 'napr-44-03-01', 'napr-44-03-04')`); !slices.Equal(got, []string{"napr-37-03-01", "napr-39-03-01"}) {
		t.Fatalf("направления без программ после 0024: %v", got)
	}
	if n := queryInt(t, tx, `SELECT count(*) FROM trajectory_directions WHERE trajectory_id = '`+tid+`'`) +
		queryInt(t, tx, `SELECT count(*) FROM trajectory_universities WHERE trajectory_id = '`+tid+`'`); n != 3 {
		t.Fatalf("цель ученика после 0024: направлений и вузов %d, ожидали 3", n)
	}
	want := maps.Clone(wantBefore0028)
	want["directions"] += 2 // выбранные учеником
	if got := universityDirectionCounts(t, tx); !maps.Equal(got, want) {
		t.Fatalf("после 0024:\n%v\nожидали:\n%v", got, want)
	}
	changed := queryInt(t, tx, `SELECT count(DISTINCT entity_id) FROM content_changes WHERE entity = 'benefit'`)
	other := queryInt(t, tx, `SELECT count(*) FROM content_changes WHERE entity <> 'benefit'`)
	if changed == 0 || other != 0 {
		t.Fatalf("события 0024: льгот %d, других %d", changed, other)
	}

	mustExec(t, tx.Exec, `DELETE FROM content_changes`)
	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	if n := queryInt(t, tx, `SELECT count(*) FROM content_changes`); n != 0 {
		t.Fatalf("повторный прогон 0024 породил %d событий", n)
	}
}

// На чистой базе 0017 и 0018 перезаписывают строки нового сида 0003, а 0022 и
// 0024 обязаны вернуть их к сиду: после всех миграций льготы — ровно 0003 и
// 0021, без устаревших строк.
func TestMigration_BenefitLinkingMatchesSeed(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const rows = `SELECT 'benefits ' || b::text FROM benefits b
		UNION ALL SELECT 'direction_benefits ' || d::text FROM direction_benefits d
		UNION ALL SELECT 'university_directions ' || u::text FROM university_directions u`
	got := queryStrings(t, tx, rows)
	mustExec(t, tx.Exec, `DELETE FROM direction_benefits; DELETE FROM benefits`)
	mustExec(t, tx.Exec, dbtest.UpSection(readMigration(t, "0003_seed_content.sql")))
	mustExec(t, tx.Exec, dbtest.UpSection(readMigration(t, "0021_university_directions_content.sql")))
	want := queryStrings(t, tx, rows)
	if slices.Equal(got, want) {
		return
	}
	extra, missing := diffSorted(got, want), diffSorted(want, got)
	t.Fatalf("после всех миграций лишних строк %d, недостающих %d; например\nлишняя:   %v\nнедостаёт: %v",
		len(extra), len(missing), first(extra), first(missing))
}

// diffSorted — строки a, которых нет в b (оба отсортированы, с повторами).
func diffSorted(a, b []string) []string {
	var out []string
	i := 0
	for _, x := range a {
		for i < len(b) && b[i] < x {
			i++
		}
		if i < len(b) && b[i] == x {
			i++
			continue
		}
		out = append(out, x)
	}
	return out
}

func first(xs []string) string {
	if len(xs) == 0 {
		return "—"
	}
	return xs[0]
}

// 0028: у Иннополиса вместо укрупнённой группы 09.00.00 — направления из
// правил приёма, 09.03.01 и 15.03.06. Выбор 09.00.00 в вузе и цель 09.00.00
// переходят на 09.03.01, откат возвращает выбор в вузе. Событие «изменились
// льготы» — одно: «Юниор» (инженерные науки) даёт в Иннополисе БВИ на
// робототехнику. Повторный прогон ничего не меняет.
func TestMigration_InnopolisDirections(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	raw := readMigration(t, "0028_innopolis_directions.sql")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const innopolis = `SELECT direction_id FROM university_directions WHERE university_id = 'innopolis'`
	const tid = "00000000-0000-4000-8000-00000000f028"
	choices := func() []string {
		return queryStrings(t, tx, `SELECT direction_id FROM trajectory_university_directions WHERE trajectory_id = '`+tid+`'`)
	}

	// Как на проде до миграции.
	mustExec(t, tx.Exec, dbtest.DownSection(raw))
	if got := queryStrings(t, tx, innopolis); !slices.Equal(got, []string{"napr-09-00-00"}) {
		t.Fatalf("до 0028 направления Иннополиса: %v", got)
	}
	mustExec(t, tx.Exec, `INSERT INTO trajectories (id, student_name, grade, region_code, goal_status) VALUES
		('`+tid+`', 'Артём', 10, '16', 'known')`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_directions (trajectory_id, direction_id, position) VALUES
		('`+tid+`', 'napr-09-03-04', 0), ('`+tid+`', 'napr-09-00-00', 1)`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_universities (trajectory_id, university_id) VALUES ('`+tid+`', 'innopolis')`)
	mustExec(t, tx.Exec, `INSERT INTO trajectory_university_directions (trajectory_id, university_id, direction_id) VALUES
		('`+tid+`', 'innopolis', 'napr-09-00-00')`)
	mustExec(t, tx.Exec, `DELETE FROM content_changes`)

	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	if got := queryStrings(t, tx, innopolis); !slices.Equal(got, []string{"napr-09-03-01", "napr-15-03-06"}) {
		t.Fatalf("после 0028 направления Иннополиса: %v", got)
	}
	if got := choices(); !slices.Equal(got, []string{"napr-09-03-01"}) {
		t.Fatalf("выбор в Иннополисе после 0028: %v", got)
	}
	if got := queryStrings(t, tx, `SELECT direction_id FROM trajectory_directions WHERE trajectory_id = '`+tid+`'`); !slices.Equal(got, []string{"napr-09-03-01", "napr-09-03-04"}) {
		t.Fatalf("цель после 0028: %v", got)
	}
	want := maps.Clone(wantUniversityDirections)
	want["user_choices"] = 1
	if got := universityDirectionCounts(t, tx); !maps.Equal(got, want) {
		t.Fatalf("после 0028:\n%v\nожидали:\n%v", got, want)
	}
	if got := queryStrings(t, tx, `SELECT DISTINCT entity_id FROM content_changes`); !slices.Equal(got, []string{"p669-13-inzhenernye-nauki@innopolis"}) {
		t.Fatalf("события 0028: %v", got)
	}

	mustExec(t, tx.Exec, `DELETE FROM content_changes`)
	mustExec(t, tx.Exec, dbtest.UpSection(raw))
	if n := queryInt(t, tx, `SELECT count(*) FROM content_changes`); n != 0 {
		t.Fatalf("повторный прогон 0028 породил %d событий", n)
	}

	mustExec(t, tx.Exec, dbtest.DownSection(raw))
	if got := choices(); !slices.Equal(got, []string{"napr-09-00-00"}) {
		t.Fatalf("выбор в Иннополисе после отката 0028: %v", got)
	}
	want = maps.Clone(wantBefore0028)
	want["user_choices"] = 1
	if got := universityDirectionCounts(t, tx); !maps.Equal(got, want) {
		t.Fatalf("после отката 0028:\n%v\nожидали:\n%v", got, want)
	}
}
