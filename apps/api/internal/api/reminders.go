package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
)

// replan пересчитывает план напоминаний траектории после изменения трекера,
// отметки или региона. Само изменение уже сохранено, поэтому сбой пересчёта
// не валит запрос: воркер каждые 15 минут прогоняет ту же синхронизацию.
func (s *Server) replan(r *http.Request, trajectoryID string) {
	if err := s.store.SyncReminders(r.Context(), trajectoryID, s.cfg.ReminderHour, s.now()); err != nil {
		slog.Warn("пересчёт напоминаний", "request_id", reqID(r.Context()), "err", err)
	}
}

// tell рассылает уведомление семье после сохранённого изменения. Синхронно:
// в Cloud Functions инстанс замораживается сразу после ответа, и горутина
// «после» могла бы не выполниться. Отмена запроса клиентом рассылку не
// обрывает, сбой отправки — только в лог.
func (s *Server) tell(r *http.Request, send func(ctx context.Context)) {
	ctx, cancel := notify.Detached(r.Context())
	defer cancel()
	send(ctx)
}
