// Package notifier — уведомления об изменениях контента (F33, ТЗ §6.5).
// Timer-триггер раз в сутки забирает события из content_changes (их пишут
// триггеры базы при настоящем изменении сроков и льгот) и отправляет каждой
// затронутой траектории одно сообщение со всеми изменениями.
//
// Доставка «не меньше одного раза» и без дублей одновременно. Обе гарантии
// держит content_change_deliveries: строка на пару (изменение, получатель) —
// это захват, а done_at — уже результат. Занять пару может только один
// запуск, поэтому дубля нет; notified_at на самом изменении ставится лишь
// после того, как обслужены все получатели, поэтому недоставленное остаётся
// в очереди и уходит следующим запуском.
//
// Раньше было «не больше одного раза»: события помечались разосланными до
// отправки. Дублей это не давало, но таймаут или отказ базы сжигали всю пачку
// целиком — восстанавливаться было нечем, состояния на получателя не
// существовало, а один флаг на изменение не способен описать «Ольге ушло,
// Артёму нет».
//
// Отсюда правило запуска: сбой на одной траектории не прерывает обход.
// Соседние траектории ни в чём не виноваты.
package notifier

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

const (
	// batch — сколько изменений за запуск. Остаток заберёт следующий.
	batch = 300
	// giveUpTTL — сколько пытаться доставить, прежде чем сдаться. Недельная
	// правка сроков ещё актуальна, а очередь не растёт бесконечно.
	giveUpTTL = 7 * 24 * time.Hour
	// purgeTTL — когда стирать закрытые изменения вместе с их доставками.
	purgeTTL = 30 * 24 * time.Hour
	// staleClaimTTL — после какого возраста незакрытый захват считается
	// брошенным. Заведомо больше таймаута функции (60 секунд) и заведомо
	// меньше giveUpTTL: брошенное должно залечиться следующим суточным
	// запуском, а не попасть в отчёт о потерях неделю спустя.
	staleClaimTTL = time.Hour
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
	// Postponed — изменения, отложенные до следующего запуска: кому-то не
	// ушло или пара занята соседним запуском. Это не потеря.
	Postponed int `json:"postponed"`
	// Dropped — изменения, доставить которые за giveUpTTL так и не удалось.
	// Вот это потеря, и она обязана быть видна.
	Dropped int `json:"dropped"`
	// Stale — захваты, брошенные прошлыми запусками и освобождённые сейчас.
	Stale int `json:"stale"`
}

func (w *Worker) Run(ctx context.Context) (Result, error) {
	var res Result
	if w.max == nil {
		slog.Warn("MAX_BOT_TOKEN пуст — уведомления об изменениях ждут токена")
		return res, nil
	}
	now := w.now()
	// До выборки: освобождённое должно уйти этим же запуском.
	if n, err := w.store.ReapStaleChangeDeliveries(ctx, now.Add(-staleClaimTTL)); err != nil {
		slog.Warn("освобождение брошенных захватов", "err", err)
	} else if n > 0 {
		res.Stale = int(n)
		slog.Warn("захваты доставки брошены прошлым запуском и освобождены — "+
			"результат отправки не был записан, возможен повтор", "deliveries", n)
	}
	ids, items, err := w.store.PendingContentChanges(ctx, batch)
	if err != nil {
		return res, err
	}
	res.Changes = len(ids)
	// Изменение считается обслуженным, пока не доказано обратное: никто из
	// получателей не остался без сообщения и без чужого незавершённого захвата.
	served := make(map[string]bool, len(ids))
	for _, id := range ids {
		served[id] = true
	}
	groups := byTrajectory(items)
	for i, group := range groups {
		if ctx.Err() != nil {
			// Ничего не потеряно: непомеченное заберёт следующий запуск.
			slog.Warn("таймаут: часть уведомлений отложена", "trajectories", len(groups)-i)
			for _, g := range groups[i:] {
				markUnserved(served, g)
			}
			break
		}
		w.send(ctx, group, served, &res)
	}

	var done []string
	for _, id := range ids {
		if served[id] {
			done = append(done, id)
		}
	}
	res.Postponed = len(ids) - len(done)
	// Подведение итогов переживает отмену контекста: сюда мы приходим и по
	// таймауту, а обслуженное уже обслужено — не записать этого нельзя.
	bctx, cancel := notify.Detached(ctx)
	defer cancel()
	if _, err := w.store.MarkChangesNotified(bctx, done, now); err != nil {
		return res, err
	}
	if n, err := w.store.GiveUpOldChanges(bctx, now, now.Add(-giveUpTTL)); err != nil {
		slog.Warn("изменения: отказ от старых", "err", err)
	} else if n > 0 {
		res.Dropped = int(n)
		slog.Warn("изменения так и не доставлены, перестаём пытаться", "changes", n, "срок", giveUpTTL)
	}
	if _, err := w.store.PurgeContentChanges(bctx, now.Add(-purgeTTL)); err != nil {
		slog.Warn("очистка изменений контента", "err", err)
	}
	return res, nil
}

// send — сводка одной траектории всем её участникам в их голосе. Ошибок
// наружу не отдаёт: см. правило запуска в комментарии к пакету. Всё, что
// обслужить не удалось, помечается в served, и изменение остаётся в очереди.
func (w *Worker) send(ctx context.Context, items []store.ChangeItem, served map[string]bool, res *Result) {
	id := items[0].TrajectoryID
	t, err := w.store.Trajectory(ctx, id)
	if err != nil {
		slog.Warn("изменения: траектория", "trajectory", id, "err", err)
		markUnserved(served, items)
		return
	}
	rs, err := w.store.ActiveRecipients(ctx, id, "")
	if err != nil {
		slog.Warn("изменения: получатели", "trajectory", id, "err", err)
		markUnserved(served, items)
		return
	}
	res.Trajectories++
	all := changeIDs(items)
	for _, r := range rs {
		claimed, unfinished, err := w.store.ClaimChangeDeliveries(ctx, all, r.MemberID)
		if err != nil {
			slog.Warn("изменения: захват доставки", "trajectory", id, "err", err)
			markUnserved(served, items)
			return
		}
		// Занято соседним запуском, который ещё не закончил: обслуженным это
		// не считается, иначе изменение закрылось бы до отправки.
		markIDsUnserved(served, unfinished)
		if len(claimed) == 0 {
			continue
		}
		mid, sendErr := w.max.Send(ctx, r.MaxUserID, w.notify.Changes(forClaimed(items, claimed), r, t))
		// Итог записываем контекстом, переживающим таймаут запуска: сообщение
		// уже ушло, и потерять этот факт нельзя — захват остался бы открытым.
		rctx, cancel := notify.Detached(ctx)
		switch {
		case maxapi.IsBlocked(sendErr):
			res.Skipped++
			err = w.store.FinishChangeDeliveries(rctx, claimed, r.MemberID, "")
		case sendErr != nil:
			slog.Warn("отправка уведомления об изменениях", "trajectory", id, "err", sendErr)
			res.Failed++
			markIDsUnserved(served, claimed)
			err = w.store.ReleaseChangeDeliveries(rctx, claimed, r.MemberID)
		default:
			res.Sent++
			err = w.store.FinishChangeDeliveries(rctx, claimed, r.MemberID, mid)
		}
		cancel()
		if err != nil {
			slog.Warn("изменения: запись результата доставки", "trajectory", id, "err", err)
			markIDsUnserved(served, claimed)
		}
	}
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

// changeIDs — объединение изменений группы: их и занимаем под получателя.
func changeIDs(items []store.ChangeItem) []string {
	seen := map[string]bool{}
	var out []string
	for _, it := range items {
		for _, id := range it.ChangeIDs {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// forClaimed — пункты, покрытые занятыми изменениями. Обычно это вся группа;
// подмножество выходит, когда часть изменений занял соседний запуск.
func forClaimed(items []store.ChangeItem, claimed []string) []store.ChangeItem {
	ok := make(map[string]bool, len(claimed))
	for _, id := range claimed {
		ok[id] = true
	}
	var out []store.ChangeItem
	for _, it := range items {
		if slices.ContainsFunc(it.ChangeIDs, func(id string) bool { return ok[id] }) {
			out = append(out, it)
		}
	}
	return out
}

func markUnserved(served map[string]bool, items []store.ChangeItem) {
	for _, it := range items {
		markIDsUnserved(served, it.ChangeIDs)
	}
}

func markIDsUnserved(served map[string]bool, ids []string) {
	for _, id := range ids {
		served[id] = false
	}
}
