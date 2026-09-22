// Точка входа Cloud Functions для вебхука MAX.
// Entrypoint: apps/bot/cmd/function/main.Handler
//
// Пакет обязан быть main: рантайм собирает точку входа как Go-плагин
// (-buildmode=plugin), а плагин строится только из main-пакета.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/ArthurBabkin/max-hackathon/apps/bot/internal/bot"
)

var handler http.Handler

// init выполняется один раз за холодный старт. Инстанс без токена или с
// битым конфигом не должен обслужить ни одного обновления. Секрет вебхука
// обязателен: адрес функции публичный, и без секрета любой мог бы прислать
// обновление от имени чужого user_id — например, /delete.
func init() {
	if os.Getenv("MAX_BOT_TOKEN") == "" {
		log.Fatal("бот: MAX_BOT_TOKEN не задан")
	}
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("бот: WEBHOOK_SECRET не задан")
	}
	b, _, err := bot.FromEnv(context.Background())
	if err != nil {
		log.Fatalf("бот: %v", err)
	}
	handler = bot.NewHandler(secret, b)
}

// Handler — обработчик, который вызывает рантайм.
func Handler(rw http.ResponseWriter, req *http.Request) {
	handler.ServeHTTP(rw, req)
}

// main существует только чтобы пакет собирался локально обычным go build.
// В Cloud Functions он не вызывается: рантайм дёргает Handler напрямую.
func main() {}
