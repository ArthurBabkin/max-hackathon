package api

import (
	"net/http"

	"github.com/ArthurBabkin/max-hackathon/packages/core/match"
	"github.com/ArthurBabkin/max-hackathon/packages/core/pick"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
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
	res, err := pick.Recommend(ctx, s.store, t, s.now(), filter)
	if err != nil {
		return err
	}
	cs := cardSet{Set: res.Set, voice: voice.New(voice.Role(m.Role), t.StudentName, m.FirstName)}
	toCards := func(rs []match.Result) []olympiadCard {
		out := make([]olympiadCard, len(rs))
		for i, r := range rs {
			out[i] = cs.card(res.Profiles[r.ProfileID], r)
		}
		return out
	}
	writeJSON(w, http.StatusOK, recommendationsResponse{
		Items: toCards(res.Items), Outside: toCards(res.Outside), Note: cs.voice.T("match.note", nil),
	})
	return nil
}
