// Локальный запуск бота. BOT_MODE=webhook (по умолчанию) — обычный
// http.Server для docker-compose, тот же обработчик, что в Cloud Functions.
// BOT_MODE=poll — long polling: бот отлаживается в настоящем MAX без
// HTTPS-адреса (пока у бота нет подписки на вебхук).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ArthurBabkin/max-hackathon/apps/bot/internal/bot"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	b, client, err := bot.FromEnv(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if os.Getenv("MAX_BOT_TOKEN") == "" {
		log.Printf("WARN: MAX_BOT_TOKEN пуст — бот примет вебхук, но ответить в MAX не сможет")
	}
	if config.Get("BOT_MODE", "webhook") == "poll" {
		log.Printf("bot: long polling")
		if err := b.Poll(ctx, client); err != nil && ctx.Err() == nil {
			log.Fatal(err)
		}
		return
	}

	mux := http.NewServeMux()
	mux.Handle("/", bot.NewHandler(os.Getenv("WEBHOOK_SECRET"), b))
	// Healthcheck для docker-compose: сервис считается живым, только когда отвечает.
	mux.HandleFunc("/health", func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("ok"))
	})
	addr := ":" + config.Get("PORT", "8080")
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	log.Printf("bot слушает %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
