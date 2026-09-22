package api

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ArthurBabkin/max-hackathon/packages/core/names"
	"github.com/ArthurBabkin/max-hackathon/packages/core/pick"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

// maxQueryLen — предел поиска по контракту (q, maxLength 100).
const maxQueryLen = 100

func searchQuery(r *http.Request) (string, error) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(q) > maxQueryLen {
		return "", badRequest("Запрос длиннее 100 символов.")
	}
	return q, nil
}

type primaryProfile struct {
	OlympiadProfileID string  `json:"olympiad_profile_id"`
	SubjectCode       string  `json:"subject_code"`
	SubjectName       string  `json:"subject_name"`
	Level             *string `json:"level"`
}

type olympiadListItem struct {
	badge
	OlympiadID     string         `json:"olympiad_id"`
	Name           string         `json:"name"`
	Organizer      *string        `json:"organizer"`
	Kind           string         `json:"kind"`
	FinalCity      *string        `json:"final_city"`
	PrimaryProfile primaryProfile `json:"primary_profile"`
	ProfilesCount  int            `json:"profiles_count"`
}

type listResponse[T any] struct {
	Items []T `json:"items"`
}

// olympiads — GET /olympiads (F24, F27): строка на олимпиаду; по тапу
// открывается профиль по предмету ученика, а с фильтром — по этому предмету.
func (s *Server) olympiads(w http.ResponseWriter, r *http.Request) error {
	q, err := searchQuery(r)
	if err != nil {
		return err
	}
	subject, city := r.URL.Query().Get("subject"), r.URL.Query().Get("city")
	ctx, m := r.Context(), me(r)
	codes, err := pick.SubjectCodes(ctx, s.store, m.TrajectoryID)
	if err != nil {
		return err
	}
	mine := map[string]bool{}
	for _, c := range codes {
		mine[c] = true
	}
	profiles, err := s.store.Profiles(ctx, store.ProfileQuery{Search: q, City: city})
	if err != nil {
		return err
	}

	// Профили приходят по названию олимпиады — группы сохраняют этот порядок.
	var order []string
	groups := map[string][]store.Profile{}
	for _, p := range profiles {
		if _, ok := groups[p.OlympiadID]; !ok {
			order = append(order, p.OlympiadID)
		}
		groups[p.OlympiadID] = append(groups[p.OlympiadID], p)
	}
	out := listResponse[olympiadListItem]{Items: []olympiadListItem{}}
	for _, id := range order {
		ps := groups[id]
		candidates := ps
		if subject != "" {
			candidates = nil
			for _, p := range ps {
				if p.SubjectCode == subject {
					candidates = append(candidates, p)
				}
			}
			if len(candidates) == 0 {
				continue
			}
		}
		p := primaryOf(candidates, mine)
		out.Items = append(out.Items, olympiadListItem{
			OlympiadID: p.OlympiadID, Name: names.Olympiad(p.OlympiadName), Organizer: p.Organizer, Kind: p.Kind, FinalCity: p.FinalCity,
			PrimaryProfile: primaryProfile{OlympiadProfileID: p.ID, SubjectCode: p.SubjectCode,
				SubjectName: profileLabel(p.SubjectName, p.ProfileName), Level: p.Level},
			ProfilesCount: len(ps),
		})
	}
	// По алфавиту того названия, что видно в списке, а не официального:
	// иначе «Высшая проба» стояла бы среди «Всероссийских…».
	sort.SliceStable(out.Items, func(i, j int) bool { return names.Key(out.Items[i].Name) < names.Key(out.Items[j].Name) })
	writeJSON(w, http.StatusOK, out)
	return nil
}

// primaryOf — профиль по предмету ученика, среди них — сильнейший по уровню.
func primaryOf(ps []store.Profile, mine map[string]bool) store.Profile {
	best := ps[0]
	better := func(a, b store.Profile) bool {
		if mine[a.SubjectCode] != mine[b.SubjectCode] {
			return mine[a.SubjectCode]
		}
		if ra, rb := levelRank(a.Level), levelRank(b.Level); ra != rb {
			return ra < rb
		}
		return profileLabel(a.SubjectName, a.ProfileName) < profileLabel(b.SubjectName, b.ProfileName)
	}
	for _, p := range ps[1:] {
		if better(p, best) {
			best = p
		}
	}
	return best
}

// universities — GET /universities (F25, F27).
func (s *Server) universities(w http.ResponseWriter, r *http.Request) error {
	q, err := searchQuery(r)
	if err != nil {
		return err
	}
	us, err := s.store.Universities(r.Context(), me(r).TrajectoryID, q, r.URL.Query().Get("city"))
	if err != nil {
		return err
	}
	out := listResponse[universityItem]{Items: make([]universityItem, len(us))}
	for i, u := range us {
		out.Items[i] = universityItemOf(u)
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

type universityOlympiad struct {
	badge
	OlympiadProfileID string  `json:"olympiad_profile_id"`
	OlympiadID        string  `json:"olympiad_id"`
	Name              string  `json:"name"`
	SubjectCode       string  `json:"subject_code"`
	SubjectName       string  `json:"subject_name"`
	Level             *string `json:"level"`
	Benefit           string  `json:"benefit"`
	BenefitLabel      string  `json:"benefit_label"`
}

type universityDetail struct {
	universityItem
	Directions      []string             `json:"directions"`
	EgeNote         *string              `json:"ege_note"`
	RulesURL        *string              `json:"rules_url"`
	RulesVerifiedAt *string              `json:"rules_verified_at"`
	Olympiads       []universityOlympiad `json:"olympiads"`
}

// university — GET /universities/{id} (F25, F26). Олимпиады по предметам
// ученика — первыми: у Иннополиса их больше шестидесяти.
func (s *Server) university(w http.ResponseWriter, r *http.Request) error {
	ctx, m := r.Context(), me(r)
	d, err := s.store.University(ctx, m.TrajectoryID, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return notFound("Вуз не найден.")
		}
		return err
	}
	rows, err := s.store.UniversityOlympiads(ctx, d.ID)
	if err != nil {
		return err
	}
	codes, err := pick.SubjectCodes(ctx, s.store, m.TrajectoryID)
	if err != nil {
		return err
	}
	mine := map[string]bool{}
	for _, c := range codes {
		mine[c] = true
	}
	sort.SliceStable(rows, func(i, j int) bool { return mine[rows[i].SubjectCode] && !mine[rows[j].SubjectCode] })

	out := universityDetail{
		universityItem: universityItemOf(d.University), Directions: d.Directions, EgeNote: d.EgeNote,
		RulesURL: d.RulesURL, RulesVerifiedAt: dateOf(d.RulesVerifiedAt), Olympiads: make([]universityOlympiad, len(rows)),
	}
	if out.Directions == nil {
		out.Directions = []string{}
	}
	for i, x := range rows {
		out.Olympiads[i] = universityOlympiad{
			OlympiadProfileID: x.ProfileID, OlympiadID: x.OlympiadID, Name: names.Olympiad(x.OlympiadName), SubjectCode: x.SubjectCode,
			SubjectName: profileLabel(x.SubjectName, x.ProfileName), Level: x.Level, Benefit: x.Benefit,
			BenefitLabel: benefitLabels[x.Benefit],
		}
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
