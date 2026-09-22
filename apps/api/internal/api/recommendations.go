package api

import (
	"context"
	"net/http"

	"github.com/ArthurBabkin/max-hackathon/packages/core/match"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

type recommendationsResponse struct {
	Items   []olympiadCard `json:"items"`
	Outside []olympiadCard `json:"outside"`
	Note    string         `json:"note"`
}

// recommendations — GET /recommendations (F13–F16): кандидаты — профили по
// предметам ученика, в которых участвует его класс; скоринг и отбор — в
// core/match.
func (s *Server) recommendations(w http.ResponseWriter, r *http.Request) error {
	filter := r.URL.Query().Get("filter")
	switch filter {
	case "":
		filter = "all"
	case "all", "level1", "soon", "online":
	default:
		return badRequest("Неизвестный фильтр подбора.")
	}
	ctx, m := r.Context(), me(r)
	t, err := s.store.Trajectory(ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	codes, err := s.subjectCodes(ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	profiles, err := s.store.Profiles(ctx, store.ProfileQuery{SubjectCodes: codes, Grade: t.Grade})
	if err != nil {
		return err
	}
	cs, err := s.cardSet(ctx, m, t, codes, profiles)
	if err != nil {
		return err
	}
	byID := make(map[string]store.Profile, len(profiles))
	cands := make([]match.Candidate, len(profiles))
	for i, p := range profiles {
		byID[p.ID] = p
		cands[i] = cs.candidate(p)
	}
	items, outside := match.Recommend(cands, cs.student(), match.DefaultWeights, cs.now, filter)
	toCards := func(rs []match.Result) []olympiadCard {
		out := make([]olympiadCard, len(rs))
		for i, res := range rs {
			out[i] = cs.card(byID[res.ProfileID], res)
		}
		return out
	}
	writeJSON(w, http.StatusOK, recommendationsResponse{
		Items: toCards(items), Outside: toCards(outside), Note: cs.voice.T("match.note", nil),
	})
	return nil
}

// subjectCodes — коды предметов ученика; пустой срез, а не nil: nil в
// ProfileQuery означает «без фильтра».
func (s *Server) subjectCodes(ctx context.Context, trajectoryID string) ([]string, error) {
	subjects, err := s.store.TrajectorySubjects(ctx, trajectoryID)
	if err != nil {
		return nil, err
	}
	codes := make([]string, len(subjects))
	for i, sub := range subjects {
		codes[i] = sub.Code
	}
	return codes, nil
}
