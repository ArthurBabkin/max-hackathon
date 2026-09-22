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
	"log/slog"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

const (
	// batch — сколько напоминаний за запуск. При 25 сообщениях в секунду
	// (лимит клиента) пачка укладывается в таймаут функции; остаток заберёт
	// следующий запуск через 15 минут.
	batch = 300
	// seenTTL — дольше MAX обновление не передоставляет.
	seenTTL = 24 * time.Hour
	// deletedTTL — удалённая траектория стирается физически, когда
	// уведомления об удалении уже разосланы.
	deletedTTL = time.Hour
)

type Worker struct {
	store  *store.Store
	max    maxapi.Sender
	notify *notify.Notifier
	hour   int
	now    func() time.Time
}

// New — max == nil означает запуск без токена бота: план напоминаний
// обслуживается, но ничего не отправляется.
func New(st *store.Store, max maxapi.Sender, bot config.Bot, hour int) *Worker {
	return &Worker{store: st, max: max, hour: hour, now: time.Now,
		notify: &notify.Notifier{Store: st, Max: max, BotName: bot.Name, BotID: bot.ID, ReminderHour: hour}}
}

// Result возвращается триггеру; попадает в логи выполнения.
type Result struct {
	Processed int `json:"processed"`
	Sent      int `json:"sent"`
	Failed    int `json:"failed"`
	// Skipped — получатель остановил бота: повторять бессмысленно.
	Skipped int `json:"skipped"`
	Expired int `json:"expired"`
}

func (w *Worker) Run(ctx context.Context) (Result, error) {
	now := w.now()
	var res Result
	// Страховка к пересчёту после каждого изменения в API и боте: новые
	// этапы и сдвинутые даты попадают в план, даже если никто ничего не менял.
	if err := w.store.SyncAllReminders(ctx, w.hour, now); err != nil {
		return res, err
	}
	expired, err := w.store.ExpireReminders(ctx, now)
	if err != nil {
		return res, err
	}
	res.Expired = int(expired)
	if w.max != nil {
		if err := w.deliver(ctx, now, &res); err != nil {
			return res, err
		}
	}
	if _, err := w.store.PurgeSeenUpdates(ctx, now.Add(-seenTTL)); err != nil {
		slog.Warn("очистка bot_updates_seen", "err", err)
	}
	if _, err := w.store.PurgeDeletedTrajectories(ctx, now.Add(-deletedTTL)); err != nil {
		slog.Warn("очистка удалённых траекторий", "err", err)
	}
	return res, nil
}

func (w *Worker) deliver(ctx context.Context, now time.Time, res *Result) error {
	due, err := w.store.DueReminders(ctx, now, batch)
	if err != nil {
		return err
	}
	if len(due) == batch {
		slog.Warn("напоминаний больше, чем пачка: остаток уйдёт следующим запуском", "batch", batch)
	}
	trajectories := map[string]store.Trajectory{}
	for _, d := range due {
		if ctx.Err() != nil {
			// Таймаут функции: доставленное отмечено, остальное — следующим запуском.
			return nil
		}
		t, ok := trajectories[d.TrajectoryID]
		if !ok {
			if t, err = w.store.Trajectory(ctx, d.TrajectoryID); err != nil {
				slog.Warn("напоминание без траектории", "reminder", d.ID, "err", err)
				continue
			}
			trajectories[d.TrajectoryID] = t
		}
		recipients, err := w.store.ReminderRecipients(ctx, d)
		if err != nil {
			return err
		}
		res.Processed++
		complete := true
		for _, r := range recipients {
			ok, err := w.send(ctx, d, r, t, now, res)
			if err != nil {
				return err
			}
			complete = complete && ok
		}
		// Сбой отправки оставляет напоминание в плане: следующий запуск
		// дошлёт тем, кому не ушло, — занятые пары отсекает ClaimDelivery.
		if complete {
			if err := w.store.MarkReminderSent(ctx, d.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// send — одно напоминание одному получателю. false — не доставлено и
// стоит повторить; ошибка — сбой базы, запуск прерывается.
func (w *Worker) send(ctx context.Context, d store.DueReminder, r store.Recipient, t store.Trajectory, now time.Time, res *Result) (bool, error) {
	claimed, err := w.store.ClaimDelivery(ctx, d.ID, r.MemberID)
	if err != nil || !claimed {
		return true, err
	}
	mid, err := w.max.Send(ctx, r.MaxUserID, w.notify.Reminder(d, r, t, now))
	switch {
	case maxapi.IsBlocked(err):
		res.Skipped++
		return true, nil
	case err != nil:
		slog.Warn("отправка напоминания", "reminder", d.ID, "err", err)
		res.Failed++
		// Контекст запуска мог истечь — освобождаем пару без него.
		rctx, cancel := notify.Detached(ctx)
		defer cancel()
		return false, w.store.ReleaseDelivery(rctx, d.ID, r.MemberID)
	}
	res.Sent++
	return true, w.store.SetDeliveryMessage(ctx, d.ID, r.MemberID, mid)
}
