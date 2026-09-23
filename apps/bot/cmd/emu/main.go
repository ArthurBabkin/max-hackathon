// Эмулятор MAX локально: веб-чат с ботом на http://localhost:9000.
// Запуск со всей обвязкой (база, миграции) — make emu.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/ArthurBabkin/max-hackathon/apps/bot/internal/emu"
	"github.com/ArthurBabkin/max-hackathon/packages/db"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
)

func main() {
	pool, err := db.Pool(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	e, err := emu.FromEnv(pool)
	if err != nil {
		log.Fatal(err)
	}
	addr := config.Get("EMU_ADDR", "localhost:9000")
	log.Printf("эмулятор MAX: http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, e))
}
