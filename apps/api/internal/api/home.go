package api

import (
	"context"
	"net/http"
	"time"
)

type nextStep struct {
	TrackerItemID     string     `json:"tracker_item_id"`
	OlympiadProfileID string     `json:"olympiad_profile_id"`
	OlympiadName      string     `json:"olympiad_name"`
	StageTitle        string     `json:"stage_title"`
	StageKind         string     `json:"stage_kind"`
	DeadlineAt        *time.Time `json:"deadline_at"`
}

type homeResponse struct {
	Trajectory            trajectorySummary `json:"trajectory"`
	TrackerCount          int               `json:"tracker_count"`
	RegisteredCount       int               `json:"registered_count"`
	UniversitiesCount     int               `json:"universities_count"`
	PendingProposalsCount int               `json:"pending_proposals_count"`
	NextStep              *nextStep         `json:"next_step"`
	Upcoming              []trackerItem     `json:"upcoming"`
}

// trackerItems — пункты трекера траектории с текущим этапом, по сроку.
func (s *Server) trackerItems(ctx context.Context, trajectoryID string) ([]trackerItem, error) {
	rows, err := s.store.TrackerItems(ctx, trajectoryID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ProfileID
	}
	st, err := s.store.StagesFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	now := s.now()
	items := make([]trackerItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, trackerItemOf(r, st[r.ProfileID], now))
	}
	sortByDeadline(items)
	return items, nil
}

// home — GET /home (F34, F47).
func (s *Server) home(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	t, err := s.store.Trajectory(ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	items, err := s.trackerItems(ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	unis, err := s.store.TrajectoryUniversities(ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	pending, err := s.store.PendingProposalsCount(ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	out := homeResponse{
		Trajectory: summaryOf(t), TrackerCount: len(items), UniversitiesCount: len(unis),
		PendingProposalsCount: pending, Upcoming: []trackerItem{},
	}
	for _, it := range items {
		if it.RegisteredAt != nil {
			out.RegisteredCount++
		}
	}
	// Следующий шаг (ТЗ §6.6) — ближайший по сроку пункт без отметки о
	// регистрации, у которого ещё есть что делать. Всё отмечено — null.
	for _, it := range items {
		if it.RegisteredAt == nil && it.NextStageTitle != nil {
			out.NextStep = &nextStep{
				TrackerItemID: it.ID, OlympiadProfileID: it.OlympiadProfileID, OlympiadName: it.OlympiadName,
				StageTitle: *it.NextStageTitle, StageKind: it.nextKind, DeadlineAt: it.DeadlineAt,
			}
			break
		}
	}
	for _, it := range items {
		if it.DeadlineAt != nil && len(out.Upcoming) < 3 {
			out.Upcoming = append(out.Upcoming, it)
		}
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
