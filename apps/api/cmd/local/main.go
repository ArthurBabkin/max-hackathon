// Локальный запуск REST-сервера для docker-compose.
package main

import (
	"log"
	"net/http"

	"github.com/ArthurBabkin/max-hackathon/apps/api/internal/api"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
)

func main() {
	addr := ":" + config.Get("PORT", "8081")
	log.Printf("api слушает %s", addr)
	if err := http.ListenAndServe(addr, api.New()); err != nil {
		log.Fatal(err)
	}
}
