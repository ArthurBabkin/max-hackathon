// Package store — все SQL-запросы приложения. Запросы написаны руками, без
// генератора: половина из них собирает вложенные структуры контракта, а
// самые тяжёлые (подбор, каталог, календарь) — динамические.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DB — то, что умеют и пул, и транзакция. Благодаря этому любой метод Store
// работает и сам по себе, и внутри Store.Tx без дублирования кода.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

type Store struct {
	db DB
}

func New(db DB) *Store {
	return &Store{db: db}
}

// Tx выполняет fn в одной транзакции. Вложенный вызов внутри fn открывает
// точку сохранения (pgx.Tx.Begin), так что сценарии можно составлять друг
// из друга. Ошибка или паника fn откатывают всё.
func (s *Store) Tx(ctx context.Context, fn func(*Store) error) (err error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("начало транзакции: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if err = fn(&Store{db: tx}); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("фиксация транзакции: %w", err)
	}
	return nil
}

var (
	// ErrNotFound — строки нет или ссылка ведёт на несуществующую запись.
	ErrNotFound = errors.New("не найдено")
	// ErrConflict — нарушена уникальность.
	ErrConflict = errors.New("конфликт")
)

// ConflictError уточняет ErrConflict именем ограничения: по нему сценарий
// понимает, что именно случилось (например, в траектории уже есть ученик).
type ConflictError struct {
	Constraint string
	err        error
}

func (e *ConflictError) Error() string { return "конфликт: " + e.Constraint }
func (e *ConflictError) Unwrap() []error {
	return []error{ErrConflict, e.err}
}

// IsConflictOn сообщает, что err — нарушение конкретного ограничения.
func IsConflictOn(err error, constraint string) bool {
	var ce *ConflictError
	return errors.As(err, &ce) && ce.Constraint == constraint
}

// wrap переводит ошибки драйвера в ошибки предметной области.
func wrap(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return &ConflictError{Constraint: pgErr.ConstraintName, err: err}
		case "23503": // foreign_key_violation: ссылка на то, чего нет
			return fmt.Errorf("%w: %s", ErrNotFound, pgErr.ConstraintName)
		}
	}
	return err
}

type rowScanner interface{ Scan(...any) error }

// collect читает все строки и закрывает курсор. Пустой результат — пустой
// срез, а не nil: в JSON он станет [], как требует контракт.
func collect[T any](rows pgx.Rows, scan func(rowScanner) (T, error)) ([]T, error) {
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, wrap(err)
		}
		out = append(out, v)
	}
	return out, wrap(rows.Err())
}
