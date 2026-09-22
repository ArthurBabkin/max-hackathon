package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestWrap_MapsDriverErrorsToDomain(t *testing.T) {
	if !errors.Is(wrap(pgx.ErrNoRows), ErrNotFound) {
		t.Error("ErrNoRows должен стать ErrNotFound")
	}
	uniq := wrap(&pgconn.PgError{Code: "23505", ConstraintName: "members_single_active_kid_uniq"})
	if !errors.Is(uniq, ErrConflict) || !IsConflictOn(uniq, "members_single_active_kid_uniq") {
		t.Errorf("23505 должен стать ConflictError с именем ограничения, получили %v", uniq)
	}
	if IsConflictOn(uniq, "другое") {
		t.Error("IsConflictOn не должен совпадать с чужим ограничением")
	}
	if !errors.Is(wrap(&pgconn.PgError{Code: "23503"}), ErrNotFound) {
		t.Error("нарушение внешнего ключа — ссылка на несуществующее, ErrNotFound")
	}
	other := errors.New("сеть")
	if wrap(other) != other || wrap(nil) != nil {
		t.Error("прочие ошибки проходят как есть")
	}
}

func TestTx_RollsBackOnErrorAndCommitsOnSuccess(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := context.Background()
	s := New(pool)
	count := func() (n int) {
		t.Helper()
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	boom := errors.New("сценарий упал")
	err := s.Tx(ctx, func(tx *Store) error {
		if _, err := tx.db.Exec(ctx, "INSERT INTO users (max_user_id, first_name) VALUES (1, 'А')"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) || count() != 0 {
		t.Fatalf("ошибка сценария должна откатить вставку: err=%v, строк=%d", err, count())
	}

	func() {
		defer func() { _ = recover() }()
		_ = s.Tx(ctx, func(tx *Store) error {
			_, _ = tx.db.Exec(ctx, "INSERT INTO users (max_user_id, first_name) VALUES (2, 'Б')")
			panic("паника в сценарии")
		})
	}()
	if count() != 0 {
		t.Fatal("паника должна откатить вставку")
	}

	if err := s.Tx(ctx, func(tx *Store) error {
		_, err := tx.db.Exec(ctx, "INSERT INTO users (max_user_id, first_name) VALUES (3, 'В')")
		return err
	}); err != nil || count() != 1 {
		t.Fatalf("успешный сценарий фиксируется: err=%v, строк=%d", err, count())
	}
}
