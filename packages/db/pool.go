// Package db открывает пул соединений с Postgres. Один пакет на все четыре
// сервиса: бот, API и оба воркера ходят в одну и ту же базу.
package db

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	once    sync.Once
	pool    *pgxpool.Pool
	poolErr error
)

// Pool возвращает пул по DATABASE_URL. Пул создаётся один раз на процесс:
// Yandex Cloud Functions переиспользует инстанс между вызовами, и тёплое
// соединение переживает холодный старт. Создаётся лениво — первый вызов
// функции, которому база не нужна (например, вебхук с чужим секретом), не
// ждёт подключения.
func Pool(ctx context.Context) (*pgxpool.Pool, error) {
	once.Do(func() {
		url := os.Getenv("DATABASE_URL")
		if url == "" {
			poolErr = fmt.Errorf("DATABASE_URL не задан")
			return
		}
		maxConns := 2
		if v, err := strconv.Atoi(os.Getenv("DB_MAX_CONNS")); err == nil && v > 0 {
			maxConns = v
		}
		pool, poolErr = NewPool(ctx, url, int32(maxConns))
	})
	return pool, poolErr
}

// NewPool создаёт пул без немедленного подключения: с MinConns=0 pgxpool
// дозванивается до базы только на первом запросе. Поэтому упавшая база
// превращается в ошибку конкретного запроса, а не в падение на старте.
func NewPool(ctx context.Context, url string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("разбор DATABASE_URL: %w", err)
	}
	// Инстанс функции обслуживает один запрос за раз: пул нужен ради
	// переиспользования соединения, а не ради параллелизма. Каждое лишнее
	// соединение умножается на число тёплых инстансов всех функций и
	// упирается в max_connections Managed PostgreSQL.
	cfg.MaxConns = maxConns
	cfg.MinConns = 0
	// Спящий инстанс не должен держать соединение: закрываем раньше, чем
	// Managed PostgreSQL или сеть оборвут его сами.
	cfg.MaxConnIdleTime = 2 * time.Minute
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	// Кеш подготовленных выражений безопасен: к Managed PostgreSQL ходим
	// напрямую, без пулера в режиме транзакций.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	return pgxpool.NewWithConfig(ctx, cfg)
}
