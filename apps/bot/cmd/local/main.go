// Локальный запуск вебхука обычным http.Server — для docker-compose.
// В продакшене тот же обработчик живёт в Cloud Functions (cmd/function).
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"

	"github.com/ArthurBabkin/max-hackathon/apps/bot/internal/bot"
)

func main() {
	handler := bot.New(
		os.Getenv("WEBHOOK_SECRET"),
		maxapi.New(
			config.Get("MAX_API_BASE", "https://platform-api2.max.ru"),
			os.Getenv("MAX_BOT_TOKEN"),
		),
	)

	mux := http.NewServeMux()
	mux.Handle("/", handler)
	// Healthcheck для docker-compose: сервис считается живым, только когда отвечает.
	mux.HandleFunc("/health", func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("ok"))
	})

	addr := ":" + config.Get("PORT", "8080")
	log.Printf("bot слушает %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
