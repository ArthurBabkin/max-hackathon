package db

import (
	"context"
	"testing"
	"time"
)

func TestNewPool_DoesNotDialUntilFirstQuery(t *testing.T) {
	// Порт 1 заведомо закрыт. Если бы пул дозванивался на старте,
	// NewPool вернул бы ошибку или завис на таймауте подключения.
	start := time.Now()
	p, err := NewPool(context.Background(), "postgres://u:p@127.0.0.1:1/db?sslmode=disable", 2)
	if err != nil {
		t.Fatalf("NewPool не должен подключаться: %v", err)
	}
	defer p.Close()
	if time.Since(start) > time.Second {
		t.Fatalf("NewPool ждал сеть: %v", time.Since(start))
	}
	if p.Config().MaxConns != 2 || p.Config().MinConns != 0 {
		t.Fatalf("неожиданные лимиты пула: max=%d min=%d", p.Config().MaxConns, p.Config().MinConns)
	}
	if err := p.Ping(context.Background()); err == nil {
		t.Fatal("ping до закрытого порта должен падать")
	}
}

func TestNewPool_RejectsBrokenURL(t *testing.T) {
	if _, err := NewPool(context.Background(), "::не url::", 2); err == nil {
		t.Fatal("ожидалась ошибка разбора строки подключения")
	}
}
