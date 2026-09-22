// Локальный запуск REST-сервера для docker-compose.
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

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("конфиг: %v", err)
	}
	if cfg.DevUnsignedInitData {
		log.Printf("WARN: включён дев-обход подписи initData (APP_ENV=%s) — только для локальной отладки", cfg.AppEnv)
	}
	pool, err := db.Pool(context.Background())
	if err != nil {
		log.Fatalf("база: %v", err)
	}
	h := api.New(api.Deps{
		Store:       store.New(pool),
		Config:      cfg,
		CORSOrigins: strings.Split(config.Get("CORS_ALLOWED_ORIGINS", ""), ","),
		Max:         api.Sender(cfg),
	})
	addr := ":" + config.Get("PORT", "8081")
	log.Printf("api слушает %s, %s", addr, cfg)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}
