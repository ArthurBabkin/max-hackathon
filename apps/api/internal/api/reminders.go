package api

import (
	"log/slog"
	"net/http"
)

// replan пересчитывает план напоминаний траектории после изменения трекера,
// отметки или региона. Само изменение уже сохранено, поэтому сбой пересчёта
// не валит запрос: воркер каждые 15 минут прогоняет ту же синхронизацию.
func (s *Server) replan(r *http.Request, trajectoryID string) {
	if err := s.store.SyncReminders(r.Context(), trajectoryID, s.cfg.ReminderHour, s.now()); err != nil {
		slog.Warn("пересчёт напоминаний", "request_id", reqID(r.Context()), "err", err)
	}
}
