// Package notifier — уведомления об изменениях контента (F33, ТЗ §6.5).
// Timer-триггер раз в сутки забирает события из content_changes (их пишут
// триггеры базы при настоящем изменении сроков и льгот) и отправляет каждой
// затронутой траектории одно сообщение со всеми изменениями.
//
// Доставка «не больше одного раза»: события помечаются разосланными до
// отправки. Упавший на середине запуск не пришлёт никому дубль завтра;
// недоставленное — редкая потеря, а не ежедневный спам.
package notifier

import (
	"context"
	"log/slog"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

type Worker struct {
	store  *store.Store
	max    maxapi.Sender
	notify *notify.Notifier
	now    func() time.Time
}

// New — max == nil означает запуск без токена бота: события копятся и
// уйдут первым запуском с токеном.
func New(st *store.Store, max maxapi.Sender, bot config.Bot) *Worker {
	return &Worker{store: st, max: max, now: time.Now,
		notify: &notify.Notifier{Store: st, Max: max, BotName: bot.Name, BotID: bot.ID}}
}

// Result возвращается триггеру; попадает в логи выполнения.
type Result struct {
	Changes      int `json:"changes"`
	Trajectories int `json:"trajectories"`
	Sent         int `json:"sent"`
	Failed       int `json:"failed"`
	// Skipped — получатель остановил бота.
	Skipped int `json:"skipped"`
}

func (w *Worker) Run(ctx context.Context) (Result, error) {
	var res Result
	if w.max == nil {
		slog.Warn("MAX_BOT_TOKEN пуст — уведомления об изменениях ждут токена")
		return res, nil
	}
	changes, items, err := w.store.TakeContentChanges(ctx, w.now())
	if err != nil {
		return res, err
	}
	res.Changes = changes
	groups := byTrajectory(items)
	for i, group := range groups {
		if ctx.Err() != nil {
			slog.Warn("таймаут: часть уведомлений об изменениях не отправлена", "trajectories", len(groups)-i)
			break
		}
		if err := w.send(ctx, group, &res); err != nil {
			return res, err
		}
	}
	return res, nil
}

// send — сводка одной траектории всем её участникам в их голосе.
func (w *Worker) send(ctx context.Context, items []store.ChangeItem, res *Result) error {
	id := items[0].TrajectoryID
	t, err := w.store.Trajectory(ctx, id)
	if err != nil {
		slog.Warn("изменения: траектория", "trajectory", id, "err", err)
		return nil
	}
	rs, err := w.store.ActiveRecipients(ctx, id, "")
	if err != nil {
		return err
	}
	res.Trajectories++
	for _, r := range rs {
		_, err := w.max.Send(ctx, r.MaxUserID, w.notify.Changes(items, r, t))
		switch {
		case maxapi.IsBlocked(err):
			res.Skipped++
		case err != nil:
			slog.Warn("отправка уведомления об изменениях", "trajectory", id, "err", err)
			res.Failed++
		default:
			res.Sent++
		}
	}
	return nil
}

// byTrajectory режет упорядоченный по траектории список на группы.
func byTrajectory(items []store.ChangeItem) [][]store.ChangeItem {
	var out [][]store.ChangeItem
	for i, it := range items {
		if i == 0 || it.TrajectoryID != items[i-1].TrajectoryID {
			out = append(out, nil)
		}
		out[len(out)-1] = append(out[len(out)-1], it)
	}
	return out
}
