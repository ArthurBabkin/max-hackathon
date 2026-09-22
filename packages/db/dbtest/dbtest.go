// Package dbtest поднимает настоящую базу для тестов.
//
// Схема и сид контента накатываются один раз в шаблонную базу, имя которой
// зависит от содержимого миграций: поменялась миграция — собирается новый
// шаблон. Каждый тестовый пакет (это отдельный процесс go test) получает
// свою копию шаблона, так что пакеты не мешают друг другу при параллельном
// запуске. Перед каждым тестом пользовательские таблицы очищаются, а
// справочники контента остаются.
//
// Без TEST_DATABASE_URL тесты с базой пропускаются: `go test ./...` на
// машине без Postgres остаётся зелёным.
package dbtest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// userTables очищаются перед каждым тестом. CASCADE снимает и зависимые:
// участников, трекер, напоминания, историю помощника.
var userTables = []string{
	"users", "trajectories", "content_changes", "audit_log",
	"bot_dialogs", "bot_updates_seen",
}

var (
	once    sync.Once
	shared  *pgxpool.Pool
	openErr error
)

// Open возвращает пул к чистой копии базы. Вызывать в начале каждого теста.
func Open(t testing.TB) *pgxpool.Pool {
	t.Helper()
	admin := os.Getenv("TEST_DATABASE_URL")
	if admin == "" {
		t.Skip("TEST_DATABASE_URL не задан — тест с базой пропущен")
	}
	once.Do(func() { shared, openErr = setup(admin) })
	if openErr != nil {
		t.Fatalf("тестовая база: %v", openErr)
	}
	ctx := context.Background()
	tables := strings.Join(existing(ctx, shared, userTables), ", ")
	if tables != "" {
		if _, err := shared.Exec(ctx, "TRUNCATE "+tables+" RESTART IDENTITY CASCADE"); err != nil {
			t.Fatalf("очистка таблиц: %v", err)
		}
	}
	return shared
}

// existing отбрасывает таблицы, которых ещё нет в схеме: харнесс должен
// работать и на срезе, где часть миграций ещё не написана.
func existing(ctx context.Context, p *pgxpool.Pool, names []string) []string {
	var out []string
	for _, n := range names {
		var ok bool
		_ = p.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", n).Scan(&ok)
		if ok {
			out = append(out, n)
		}
	}
	return out
}

func setup(adminURL string) (*pgxpool.Pool, error) {
	ctx := context.Background()
	files, sum, err := migrationFiles()
	if err != nil {
		return nil, err
	}
	tpl := "traektoria_tpl_" + sum[:12]
	db := "traektoria_t_" + packageKey()

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return nil, fmt.Errorf("подключение к %s: %w", redact(adminURL), err)
	}
	defer admin.Close(ctx)

	// Шаблон собирается и клонируется под рекомендательной блокировкой:
	// параллельные процессы go test иначе столкнутся на CREATE DATABASE.
	if _, err := admin.Exec(ctx, "SELECT pg_advisory_lock(727001)"); err != nil {
		return nil, err
	}
	defer admin.Exec(ctx, "SELECT pg_advisory_unlock(727001)") //nolint:errcheck

	var exists bool
	if err := admin.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", tpl).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		if err := dropStale(ctx, admin, "traektoria_tpl_%"); err != nil {
			return nil, err
		}
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{tpl}.Sanitize()); err != nil {
			return nil, fmt.Errorf("создание шаблона: %w", err)
		}
		if err := migrate(ctx, withDB(adminURL, tpl), files); err != nil {
			_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{tpl}.Sanitize())
			return nil, err
		}
	}
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{db}.Sanitize()+" WITH (FORCE)"); err != nil {
		return nil, err
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{db}.Sanitize()+" TEMPLATE "+pgx.Identifier{tpl}.Sanitize()); err != nil {
		return nil, fmt.Errorf("копия шаблона: %w", err)
	}
	cfg, err := pgxpool.ParseConfig(withDB(adminURL, db))
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 4
	return pgxpool.NewWithConfig(ctx, cfg)
}

// migrate накатывает секции «-- +goose Up» по порядку — так же, как goose в
// compose. Каждый файл — одна транзакция.
func migrate(ctx context.Context, url string, files []string) error {
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		up := UpSection(string(raw))
		// Простой протокол: несколько выражений в одном запросе.
		if _, err := conn.PgConn().Exec(ctx, "BEGIN;\n"+up+"\nCOMMIT;").ReadAll(); err != nil {
			return fmt.Errorf("миграция %s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

// UpSection вырезает из файла goose секцию Up.
func UpSection(sql string) string {
	start := strings.Index(sql, "-- +goose Up")
	if start < 0 {
		return sql
	}
	sql = sql[start+len("-- +goose Up"):]
	if end := strings.Index(sql, "-- +goose Down"); end >= 0 {
		sql = sql[:end]
	}
	sql = strings.ReplaceAll(sql, "-- +goose StatementBegin", "")
	return strings.ReplaceAll(sql, "-- +goose StatementEnd", "")
}

// MigrationsDir — каталог миграций относительно этого файла, чтобы тесты
// работали из любого пакета.
func MigrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "migrations")
}

func migrationFiles() ([]string, string, error) {
	files, err := filepath.Glob(filepath.Join(MigrationsDir(), "*.sql"))
	if err != nil || len(files) == 0 {
		return nil, "", fmt.Errorf("миграции не найдены в %s", MigrationsDir())
	}
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, "", err
		}
		h.Write([]byte(filepath.Base(f)))
		h.Write(b)
	}
	return files, hex.EncodeToString(h.Sum(nil)), nil
}

// packageKey — короткий стабильный ключ тестового пакета: имя базы должно
// различаться у пакетов, которые go test гоняет параллельно.
func packageKey() string {
	wd, _ := os.Getwd()
	h := sha256.Sum256([]byte(wd))
	return hex.EncodeToString(h[:])[:10]
}

func dropStale(ctx context.Context, admin *pgx.Conn, pattern string) error {
	rows, err := admin.Query(ctx, "SELECT datname FROM pg_database WHERE datname LIKE $1", pattern)
	if err != nil {
		return err
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, n := range names {
		if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{n}.Sanitize()+" WITH (FORCE)"); err != nil {
			return err
		}
	}
	return nil
}

func withDB(raw, name string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Path = "/" + name
	return u.String()
}

func redact(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<url>"
	}
	return u.Redacted()
}
