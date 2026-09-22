// Точка входа Cloud Functions для уведомлений об изменениях контента.
// Вызывается Timer-триггером раз в сутки.
// Entrypoint: apps/notifier/cmd/function/main.Handler
package main

import (
	"context"

	"github.com/ArthurBabkin/max-hackathon/apps/notifier/internal/notifier"
)

var worker = notifier.New()

func Handler(ctx context.Context) (notifier.Result, error) {
	return worker.Run(ctx)
}

func main() {}
