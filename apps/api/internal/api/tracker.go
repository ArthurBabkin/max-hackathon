package api

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/names"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var errNoTrackerItem = notFound("Такой олимпиады в трекере нет.")

// trackerItemID — id пункта из пути; не uuid — такого пункта нет.
func trackerItemID(r *http.Request) (string, error) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		return "", errNoTrackerItem
	}
	return id, nil
}

// trackerItem — один пункт трекера с текущим этапом.
func (s *Server) trackerItem(ctx context.Context, trajectoryID, id string) (trackerItem, error) {
	row, err := s.store.TrackerItem(ctx, trajectoryID, id)
	if errors.Is(err, store.ErrNotFound) {
		return trackerItem{}, errNoTrackerItem
	}
	if err != nil {
		return trackerItem{}, err
	}
	st, err := s.store.StagesFor(ctx, []string{row.ProfileID})
	if err != nil {
		return trackerItem{}, err
	}
	progress, err := s.progressOf(ctx, []store.TrackerRow{row})
	if err != nil {
		return trackerItem{}, err
	}
	return trackerItemOf(row, st[row.ProfileID], progress[row.ID], s.now()), nil
}

type proposalDTO struct {
	badge
	ID                string      `json:"id"`
	OlympiadProfileID string      `json:"olympiad_profile_id"`
	OlympiadID        string      `json:"olympiad_id"`
	OlympiadName      string      `json:"olympiad_name"`
	DeadlineAt        *time.Time  `json:"deadline_at"`
	Status            string      `json:"status"`
	ProposedBy        memberBrief `json:"proposed_by"`
	CreatedAt         time.Time   `json:"created_at"`
	ResolvedAt        *time.Time  `json:"resolved_at"`
}

func (s *Server) proposalsOf(ctx context.Context, rows []store.ProposalRow) ([]proposalDTO, error) {
	ids := make([]string, len(rows))
	for i, p := range rows {
		ids[i] = p.ProfileID
	}
	st, err := s.store.StagesFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]proposalDTO, 0, len(rows))
	for _, p := range rows {
		// Без автора предложение не показать: контракт требует proposed_by.
		if p.ProposedBy == nil {
			continue
		}
		d := proposalDTO{
			ID: p.ID, OlympiadProfileID: p.ProfileID, OlympiadID: p.OlympiadID, OlympiadName: names.Olympiad(p.OlympiadName),
			Status: p.Status, ProposedBy: *briefOf(p.ProposedBy), CreatedAt: p.CreatedAt.UTC(), ResolvedAt: utc(p.ResolvedAt),
		}
		if cur := stages.Current(st[p.ProfileID], stages.Progress{}, now); cur >= 0 {
			d.DeadlineAt = utc(st[p.ProfileID][cur].DeadlineAt)
		}
		out = append(out, d)
	}
	return out, nil
}

type trackerResponse struct {
	Items     []trackerItem `json:"items"`
	Proposals []proposalDTO `json:"proposals"`
}

// getTracker — GET /tracker (F31, F45). Ученик видит предложения, которые
// ждут его ответа; родитель — свои, ждущие подтверждения.
func (s *Server) getTracker(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	items, err := s.trackerItems(ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	proposedBy := ""
	if m.Role != "kid" {
		proposedBy = m.MemberID
	}
	rows, err := s.store.PendingProposals(ctx, m.TrajectoryID, proposedBy)
	if err != nil {
		return err
	}
	props, err := s.proposalsOf(ctx, rows)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, trackerResponse{Items: items, Proposals: props})
	return nil
}

// addToTracker — POST /tracker (F28): 201 — добавлен, 200 — уже был.
func (s *Server) addToTracker(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	if !permissionsOf(m).AddToTracker {
		return forbidden("Добавлять олимпиады в трекер может ученик — предложите её.")
	}
	var body struct {
		OlympiadProfileID string `json:"olympiad_profile_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	if body.OlympiadProfileID == "" {
		return badRequest("Не указан профиль олимпиады.")
	}
	id, created, err := s.store.AddTrackerItem(ctx, m.TrajectoryID, body.OlympiadProfileID, m.MemberID)
	if errors.Is(err, store.ErrNotFound) {
		return notFound("Олимпиада не найдена.")
	}
	if err != nil {
		return err
	}
	if created {
		s.replan(r, m.TrajectoryID)
	}
	item, err := s.trackerItem(ctx, m.TrajectoryID, id)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, item)
	return nil
}

// removeFromTracker — DELETE /tracker/{id} (F28).
func (s *Server) removeFromTracker(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	if !permissionsOf(m).RemoveFromTracker {
		return forbidden("Убирать олимпиады из трекера может ученик.")
	}
	id, err := trackerItemID(r)
	if err != nil {
		return err
	}
	if err := s.store.DeleteTrackerItem(ctx, m.TrajectoryID, id, m.MemberID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errNoTrackerItem
		}
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// setRegistered — PUT и DELETE /tracker/{id}/registered (F46): отметку
// ставит и снимает любой участник.
func (s *Server) setRegistered(on bool) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx, m := r.Context(), me(r)
		if !permissionsOf(m).ToggleRegistered {
			return forbidden("Отмечать регистрацию нельзя.")
		}
		id, err := trackerItemID(r)
		if err != nil {
			return err
		}
		changed, err := s.store.SetRegistered(ctx, m.TrajectoryID, id, m.MemberID, on)
		if errors.Is(err, store.ErrNotFound) {
			return errNoTrackerItem
		}
		if errors.Is(err, store.ErrConflict) {
			return errMarksDependOnRegistration
		}
		if err != nil {
			return err
		}
		if changed {
			s.replan(r, m.TrajectoryID)
			if on {
				s.tell(r, func(ctx context.Context) { s.notify.Registered(ctx, m.TrajectoryID, id, m) })
			}
		}
		item, err := s.trackerItem(ctx, m.TrajectoryID, id)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, item)
		return nil
	}
}

// newOnStage — что у этапа появилось: регистрация, которой не было, или
// новый итог. Снятая отметка — не новость: семье о ней не пишем.
func newOnStage(before, after trackerItem, stageID string) stages.Mark {
	find := func(it trackerItem) trackerStage {
		for _, st := range it.Stages {
			if st.ID == stageID {
				return st
			}
		}
		return trackerStage{}
	}
	was, now := find(before), find(after)
	var news stages.Mark
	news.Registered = now.Registered && !was.Registered
	if now.Result != nil && (was.Result == nil || *was.Result != *now.Result) {
		news.Result = *now.Result
	}
	return news
}

var errMarksDependOnRegistration = conflict("Сначала снимите итоги этапов: без регистрации их не бывает.")

// setStageMark — PUT /tracker/{id}/stages/{stage_id}: регистрация на этап
// или его итог. Отмечать может любой участник, как и регистрацию (F46).
// Пустая отметка снимает прежнюю. Остальным уходит сообщение об отметке.
func (s *Server) setStageMark(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	if !permissionsOf(m).ToggleRegistered {
		return forbidden("Отмечать этапы нельзя.")
	}
	id, err := trackerItemID(r)
	if err != nil {
		return err
	}
	var body struct {
		Registered bool    `json:"registered"`
		Result     *string `json:"result"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	mark := stages.Mark{Registered: body.Registered}
	if body.Result != nil {
		mark.Result = *body.Result
	}
	stageID := r.PathValue("stage_id")
	before, err := s.trackerItem(ctx, m.TrajectoryID, id)
	if err != nil {
		return err
	}
	changed, err := s.store.SetStageMark(ctx, m.TrajectoryID, id, stageID, m.MemberID, mark, s.now())
	switch {
	case errors.Is(err, store.ErrNotFound):
		return errNoTrackerItem
	case errors.Is(err, stages.ErrUnknownStage):
		return notFound("Такого этапа у олимпиады нет.")
	case errors.Is(err, stages.ErrNotAllowed):
		return badRequest("Такую отметку у этого этапа поставить нельзя.")
	case errors.Is(err, stages.ErrConflict):
		return conflict("Отметка противоречит другим отметкам этапов — сначала снимите их.")
	case err != nil:
		return err
	}
	item, err := s.trackerItem(ctx, m.TrajectoryID, id)
	if err != nil {
		return err
	}
	if changed {
		s.replan(r, m.TrajectoryID)
		if news := newOnStage(before, item, stageID); news != (stages.Mark{}) {
			s.tell(r, func(ctx context.Context) { s.notify.StageMarked(ctx, m.TrajectoryID, id, stageID, news, m) })
		}
	}
	writeJSON(w, http.StatusOK, item)
	return nil
}
