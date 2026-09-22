// Package notifier — уведомления об изменениях контента: раз в сутки
// собирает записи из content_changes и рассылает тем, кого это касается.
package notifier

import (
	"context"
	"log"
)

type Worker struct{}

func New() *Worker { return &Worker{} }

type Result struct {
	Changes int `json:"changes"`
	Sent    int `json:"sent"`
}

func (w *Worker) Run(_ context.Context) (Result, error) {
	// Здесь будет: выбрать content_changes с notified_at is null,
	// найти затронутые траектории через tracker_items, разослать, проставить notified_at.
	log.Printf("воркер уведомлений вызван, логика ещё не реализована")
	return Result{}, nil
}
