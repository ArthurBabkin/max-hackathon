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
	"slices"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
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
	// staleClaimTTL — после какого возраста незакрытый захват считается
	// брошенным. Заведомо больше таймаута функции (60 секунд), чтобы не
	// отнять пару у запуска, который ещё работает.
	staleClaimTTL = 15 * time.Minute
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
	// Stale — захваты, брошенные прошлыми запусками и освобождённые сейчас.
	Stale int `json:"stale"`
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
	// До доставки: освобождённое должно уйти этим же запуском.
	if n, err := w.store.ReapStaleDeliveries(ctx, now.Add(-staleClaimTTL)); err != nil {
		slog.Warn("освобождение брошенных захватов", "err", err)
	} else if n > 0 {
		res.Stale = int(n)
		slog.Warn("захваты доставки брошены прошлым запуском и освобождены — "+
			"результат отправки не был записан, возможен повтор", "deliveries", n)
	}
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
		var results []string
		if d.Ask() {
			var ok bool
			if results, ok, err = w.askResults(ctx, d); err != nil {
				return err
			}
			if !ok {
				if err := w.store.CancelReminder(ctx, d.ID); err != nil {
					return err
				}
				continue
			}
		}
		recipients, err := w.store.ReminderRecipients(ctx, d)
		if err != nil {
			return err
		}
		res.Processed++
		complete := true
		for _, r := range recipients {
			msg := w.notify.Reminder(d, r, t, now)
			if d.Ask() {
				msg = w.notify.ResultAsk(d, r, t, results)
			}
			ok, err := w.send(ctx, d, r, msg, res)
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

// send — одно напоминание одному получателю. Первый результат — обслужен
// ли получатель: false означает «повторить следующим запуском», и по нему
// напоминание остаётся в плане. Ошибка — сбой базы, запуск прерывается.
func (w *Worker) send(ctx context.Context, d store.DueReminder, r store.Recipient, msg maxapi.NewMessage, res *Result) (bool, error) {
	claimed, done, err := w.store.ClaimDelivery(ctx, d.ID, r.MemberID)
	if err != nil {
		return false, err
	}
	if !claimed {
		// Пару занял другой запуск. Обслуженной она считается, только если
		// он довёл отправку до конца: иначе напоминание пометилось бы
		// разосланным, пока сообщение ещё не ушло, и выпало бы из выборки
		// planned навсегда.
		return done, nil
	}
	mid, err := w.max.Send(ctx, r.MaxUserID, msg)
	// Итог отправки записываем контекстом, переживающим таймаут запуска:
	// сообщение уже ушло (или уже понятно, что не уйдёт), и потерять этот
	// факт нельзя — захват остался бы незакрытым.
	rctx, cancel := notify.Detached(ctx)
	defer cancel()
	switch {
	case maxapi.IsBlocked(err):
		res.Skipped++
		return true, w.store.MarkDeliveryDone(rctx, d.ID, r.MemberID)
	case err != nil:
		slog.Warn("отправка напоминания", "reminder", d.ID, "err", err)
		res.Failed++
		return false, w.store.ReleaseDelivery(rctx, d.ID, r.MemberID)
	}
	res.Sent++
	return true, w.store.SetDeliveryMessage(rctx, d.ID, r.MemberID, mid)
}

// askResults — какие итоги предложить в вопросе об этапе. ok = false — итог
// уже не нужен: отмечен, олимпиада закрыта или этапа больше нет.
func (w *Worker) askResults(ctx context.Context, d store.DueReminder) (results []string, ok bool, err error) {
	byProfile, err := w.store.StagesFor(ctx, []string{d.ProfileID})
	if err != nil {
		return nil, false, err
	}
	marks, err := w.store.StageMarks(ctx, []string{d.TrackerItemID})
	if err != nil {
		return nil, false, err
	}
	st := byProfile[d.ProfileID]
	p := stages.Progress{Registered: d.Registered, Marks: marks[d.TrackerItemID]}
	i := slices.IndexFunc(st, func(s stages.Stage) bool { return s.ID == d.StageID })
	if i < 0 || !stages.NeedsResult(st, p, i) {
		return nil, false, nil
	}
	return stages.Results(st, i), true, nil
}
