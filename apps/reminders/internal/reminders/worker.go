// Package reminders — воркер напоминаний. Запускается Timer-триггером
// раз в ~15 минут и сам решает, кому уже пора.
//
// Очереди (Redis/BullMQ) в архитектуре нет намеренно: serverless-функция
// не держит долгоживущего consumer'а, а периодический опрос БД закрывает
// ту же задачу проще. Дедупликация — уникальным индексом
// (reminder_id, member_id) в reminder_deliveries, а не состоянием в памяти.
package reminders

import (
	"context"
	"log"
)

type Worker struct{}

func New() *Worker { return &Worker{} }

// Result возвращается триггеру; попадает в логи выполнения.
type Result struct {
	Processed int `json:"processed"`
	Sent      int `json:"sent"`
	Failed    int `json:"failed"`
}

func (w *Worker) Run(_ context.Context) (Result, error) {
	// Здесь будет: выбрать reminders со status=planned и fire_at <= now(),
	// разослать участникам траектории, проставить reminder_deliveries.
	log.Printf("воркер напоминаний вызван, логика ещё не реализована")
	return Result{}, nil
}
