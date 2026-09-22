// Точка входа Cloud Functions для уведомлений об изменениях контента.
// Вызывается Timer-триггером раз в сутки.
// Entrypoint: apps/notifier/cmd/function/main.Handler
package main

import (
	"context"
	"log"

	"github.com/ArthurBabkin/max-hackathon/apps/notifier/internal/notifier"
)

var worker *notifier.Worker

// init — один раз за холодный старт; пул живёт между вызовами таймера.
func init() {
	w, err := notifier.FromEnv(context.Background())
	if err != nil {
		log.Fatalf("воркер уведомлений: %v", err)
	}
	worker = w
}

func Handler(ctx context.Context) (notifier.Result, error) {
	return worker.Run(ctx)
}

func main() {}
