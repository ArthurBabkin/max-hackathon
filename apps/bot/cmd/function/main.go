// Точка входа Cloud Functions для вебхука MAX.
// Entrypoint: apps/bot/cmd/function/main.Handler
//
// Пакет обязан быть main: рантайм собирает точку входа как Go-плагин
// (-buildmode=plugin), а плагин строится только из main-пакета.
package main

import (
	"net/http"
	"os"

	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"

	"github.com/ArthurBabkin/max-hackathon/apps/bot/internal/bot"
)

// Инициализация на уровне пакета выполняется один раз за холодный старт,
// а не на каждый вызов.
var handler = bot.New(
	os.Getenv("WEBHOOK_SECRET"),
	maxapi.New(
		config.Get("MAX_API_BASE", "https://platform-api2.max.ru"),
		os.Getenv("MAX_BOT_TOKEN"),
	),
)

// Handler — обработчик, который вызывает рантайм.
func Handler(rw http.ResponseWriter, req *http.Request) {
	handler.ServeHTTP(rw, req)
}

// main существует только чтобы пакет собирался локально обычным go build.
// В Cloud Functions он не вызывается: рантайм дёргает Handler напрямую.
func main() {}
