// Локальный аналог: ручной запуск воркера по HTTP, чтобы не ждать таймер.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/ArthurBabkin/max-hackathon/apps/reminders/internal/reminders"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
)

func main() {
	worker, err := reminders.FromEnv(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /run", func(rw http.ResponseWriter, req *http.Request) {
		res, err := worker.Run(req.Context())
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(res)
	})

	addr := ":" + config.Get("PORT", "8082")
	log.Printf("reminders слушает %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
