package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

var errNoProposal = notFound("Предложение не найдено.")

func (s *Server) proposal(ctx context.Context, trajectoryID, id string) (proposalDTO, error) {
	row, err := s.store.Proposal(ctx, trajectoryID, id)
	if errors.Is(err, store.ErrNotFound) {
		return proposalDTO{}, errNoProposal
	}
	if err != nil {
		return proposalDTO{}, err
	}
	out, err := s.proposalsOf(ctx, []store.ProposalRow{row})
	if err != nil {
		return proposalDTO{}, err
	}
	if len(out) == 0 {
		return proposalDTO{}, errNoProposal
	}
	return out[0], nil
}

// propose — POST /proposals (F45): родитель, когда ученик в траектории.
func (s *Server) propose(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	if !permissionsOf(m).Propose {
		return forbidden("Предлагать олимпиады может родитель, когда ученик уже в траектории.")
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
	id, created, err := s.store.CreateProposal(ctx, m.TrajectoryID, body.OlympiadProfileID, m.MemberID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return notFound("Олимпиада не найдена.")
	case errors.Is(err, store.ErrAlreadyInTracker):
		return conflict("Эта олимпиада уже в трекере.")
	case err != nil:
		return err
	}
	p, err := s.proposal(ctx, m.TrajectoryID, id)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
		s.tell(r, func(ctx context.Context) { s.notify.ProposalCreated(ctx, m.TrajectoryID, id, m, s.now()) })
	}
	writeJSON(w, status, p)
	return nil
}

type acceptResponse struct {
	Proposal    proposalDTO `json:"proposal"`
	TrackerItem trackerItem `json:"tracker_item"`
}

// resolveProposal — POST /proposals/{id}/accept и /decline (F45): отвечает
// только ученик, закрытое предложение — 409.
func (s *Server) resolveProposal(accept bool) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx, m := r.Context(), me(r)
		if !permissionsOf(m).ResolveProposals {
			return forbidden("Отвечать на предложения может ученик.")
		}
		id := r.PathValue("id")
		if !uuidRe.MatchString(id) {
			return errNoProposal
		}
		itemID, err := s.store.ResolveProposal(ctx, m.TrajectoryID, id, m.MemberID, accept)
		switch {
		case errors.Is(err, store.ErrNotFound):
			return errNoProposal
		case errors.Is(err, store.ErrProposalClosed):
			return conflict("На это предложение уже ответили.")
		case err != nil:
			return err
		}
		s.tell(r, func(ctx context.Context) { s.notify.ProposalResolved(ctx, m.TrajectoryID, id, accept) })
		p, err := s.proposal(ctx, m.TrajectoryID, id)
		if err != nil {
			return err
		}
		if !accept {
			writeJSON(w, http.StatusOK, p)
			return nil
		}
		s.replan(r, m.TrajectoryID)
		item, err := s.trackerItem(ctx, m.TrajectoryID, itemID)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, acceptResponse{Proposal: p, TrackerItem: item})
		return nil
	}
}
