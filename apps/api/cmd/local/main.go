// Локальный запуск REST-сервера для docker-compose.
package main

import (
	"log"
	"net/http"

	"github.com/ArthurBabkin/max-hackathon/apps/api/internal/api"
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
	addr := ":" + config.Get("PORT", "8081")
	log.Printf("api слушает %s, %s", addr, cfg)
	if err := http.ListenAndServe(addr, api.New()); err != nil {
		log.Fatal(err)
	}
}
