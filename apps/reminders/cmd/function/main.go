// Точка входа Cloud Functions для воркера напоминаний.
// Вызывается Timer-триггером, наружу не смотрит: публичный доступ ему не нужен.
// Entrypoint: apps/reminders/cmd/function/main.Handler
package main

import (
	"context"
	"log"

	"github.com/ArthurBabkin/max-hackathon/apps/reminders/internal/reminders"
)

var worker *reminders.Worker

// init — один раз за холодный старт; пул живёт между вызовами таймера.
func init() {
	w, err := reminders.FromEnv(context.Background())
	if err != nil {
		log.Fatalf("воркер напоминаний: %v", err)
	}
	worker = w
}

// Handler здесь без http: таймер вызывает функцию напрямую, HTTP-обвязка не нужна.
func Handler(ctx context.Context) (reminders.Result, error) {
	return worker.Run(ctx)
}

func main() {}
