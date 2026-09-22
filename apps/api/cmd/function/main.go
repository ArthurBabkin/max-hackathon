// Точка входа Cloud Functions для REST мини-приложения.
// Entrypoint: apps/api/cmd/function/main.Handler
//
// Пакет обязан быть main: рантайм собирает точку входа как Go-плагин.
package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/ArthurBabkin/max-hackathon/apps/api/internal/api"
	"github.com/ArthurBabkin/max-hackathon/packages/db"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
)

var router http.Handler

// init проверяет конфиг при загрузке плагина: инстанс с опасной комбинацией
// (дев-обход подписи в production, слабый JWT_SECRET) не должен обслужить ни
// одного запроса. Пул создаётся здесь же и переживает вызовы в тёплом
// инстансе, но в базу не ходит, пока не придёт первый запрос.
func init() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("конфиг: %v", err)
	}
	pool, err := db.Pool(context.Background())
	if err != nil {
		log.Fatalf("база: %v", err)
	}
	router = api.New(api.Deps{
		Store:       store.New(pool),
		Config:      cfg,
		CORSOrigins: strings.Split(config.Get("CORS_ALLOWED_ORIGINS", ""), ","),
	})
}

func Handler(rw http.ResponseWriter, req *http.Request) {
	router.ServeHTTP(rw, req)
}

// main нужен только для локальной сборки, рантайм его не вызывает.
func main() {}
