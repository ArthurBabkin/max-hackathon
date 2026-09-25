package api

import (
	"context"
	"errors"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"net/http"
	"sort"
	"strconv"
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
	// RegistrationClosed — по основному профилю: вступить в этом сезоне нельзя.
	RegistrationClosed bool `json:"registration_closed"`
	// MyBenefits — с mine=true: сильная льгота основного профиля в моих вузах
	// на мои направления, от сильной к слабой. Без фильтра пусто.
	MyBenefits []myBenefit `json:"my_benefits"`
}

type myBenefit struct {
	Benefit      string   `json:"benefit"`
	BenefitLabel string   `json:"benefit_label"`
	Universities []string `json:"universities"`
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
	mineOnly := false
	if v := r.URL.Query().Get("mine"); v != "" {
		if mineOnly, err = strconv.ParseBool(v); err != nil {
			return badRequest("mine — true или false.")
		}
	}
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
	var leads map[string][]myBenefit
	if mineOnly {
		if leads, err = s.leadsToMine(ctx, m.TrajectoryID, profiles); err != nil {
			return err
		}
	}
	out := listResponse[olympiadListItem]{Items: []olympiadListItem{}}
	var primaries []store.Profile
	for _, id := range order {
		ps := groups[id]
		var candidates []store.Profile
		for _, p := range ps {
			if (subject == "" || p.SubjectCode == subject) && (!mineOnly || leads[p.ID] != nil) {
				candidates = append(candidates, p)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		p := primaryOf(candidates, mine)
		primaries = append(primaries, p)
		out.Items = append(out.Items, olympiadListItem{
			OlympiadID: p.OlympiadID, Name: names.Olympiad(p.OlympiadName), Organizer: p.Organizer, Kind: p.Kind, FinalCity: p.FinalCity,
			PrimaryProfile: primaryProfile{OlympiadProfileID: p.ID, SubjectCode: p.SubjectCode,
				SubjectName: profileLabel(p.SubjectName, p.ProfileName), Level: p.Level},
			ProfilesCount: len(ps), MyBenefits: orEmpty(leads[p.ID]),
		})
	}
	ids := make([]string, len(primaries))
	for i, p := range primaries {
		ids[i] = p.ID
	}
	st, err := s.store.StagesFor(ctx, ids)
	if err != nil {
		return err
	}
	now := s.now()
	for i, p := range primaries {
		out.Items[i].RegistrationClosed = !stages.Joinable(st[p.ID], now)
	}
	// По алфавиту того названия, что видно в списке, а не официального:
	// иначе «Высшая проба» стояла бы среди «Всероссийских…».
	sort.SliceStable(out.Items, func(i, j int) bool { return names.Key(out.Items[i].Name) < names.Key(out.Items[j].Name) })
	writeJSON(w, http.StatusOK, out)
	return nil
}

// leadsToMine — по профилю: сильные льготы (БВИ, БВИ победителям, 100 баллов)
// в моих вузах на мои направления по правилу целей, от сильной к слабой;
// вузы — в порядке моих. Профиля без такой льготы в ответе нет.
func (s *Server) leadsToMine(ctx context.Context, trajectoryID string, profiles []store.Profile) (map[string][]myBenefit, error) {
	unis, err := s.store.TrajectoryUniversities(ctx, trajectoryID)
	if err != nil || len(unis) == 0 {
		return nil, err
	}
	ids, uniIDs := make([]string, len(profiles)), make([]string, len(unis))
	for i, p := range profiles {
		ids[i] = p.ID
	}
	for i, u := range unis {
		uniIDs[i] = u.ID
	}
	rows, err := s.store.TargetBenefits(ctx, trajectoryID, ids, uniIDs)
	if err != nil {
		return nil, err
	}
	byPair := map[string]string{}
	for _, b := range rows {
		byPair[b.ProfileID+"/"+b.UniversityID] = b.Benefit
	}
	out := map[string][]myBenefit{}
	for _, p := range profiles {
		for _, kind := range []string{"bvi", "bvi_winners", "score100"} {
			var names []string
			for _, u := range unis {
				if byPair[p.ID+"/"+u.ID] == kind {
					names = append(names, nick(u.ID, u.ShortName))
				}
			}
			if names != nil {
				out[p.ID] = append(out[p.ID], myBenefit{Benefit: kind, BenefitLabel: benefitLabels[kind], Universities: names})
			}
		}
	}
	return out, nil
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
	// Льгота на мои направления в этом вузе (F65); null — на них льготы нет
	// или она уточняется. DirectionsCount из DirectionsTotal — на скольких
	// направлениях вуза олимпиада даёт льготу.
	MyBenefit       *string  `json:"my_benefit"`
	MyBenefitLabel  *string  `json:"my_benefit_label"`
	MyDirections    []string `json:"my_directions"`
	DirectionsCount int      `json:"directions_count"`
	DirectionsTotal int      `json:"directions_total"`
}

// offeredDirection — направление вуза в карточке (D3): is_mine — выбрано
// учеником в этом вузе, is_goal — покрывает цель.
type offeredDirection struct {
	ID                    string `json:"id"`
	Code                  string `json:"code"`
	Name                  string `json:"name"`
	Status                string `json:"status"`
	Programs              int    `json:"programs"`
	BudgetPlaces          *int   `json:"budget_places"`
	BenefitOlympiadsCount int    `json:"benefit_olympiads_count"`
	IsMine                bool   `json:"is_mine"`
	IsGoal                bool   `json:"is_goal"`
}

type universityDetail struct {
	universityItem
	Directions      []string             `json:"directions"`
	EgeNote         *string              `json:"ege_note"`
	RulesURL        *string              `json:"rules_url"`
	RulesVerifiedAt *string              `json:"rules_verified_at"`
	Description     *string              `json:"description"`
	SiteURL         *string              `json:"site_url"`
	Olympiads       []universityOlympiad `json:"olympiads"`
	// На какие направления вуза ученик смотрит льготы (core/targets).
	OfferedDirections []offeredDirection `json:"offered_directions"`
	TargetBasis       string             `json:"target_basis"`
	TargetDirections  []directionItem    `json:"target_directions"`
	TargetUnverified  bool               `json:"target_unverified"`
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
		RulesURL: d.RulesURL, RulesVerifiedAt: dateOf(d.RulesVerifiedAt), Description: d.Description, SiteURL: d.SiteURL, Olympiads: make([]universityOlympiad, len(rows)),
	}
	if out.Directions == nil {
		out.Directions = []string{}
	}
	if err := s.universityTargets(r, &out, rows); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// universityTargets — направления вуза в карточке и льготы олимпиад на мои
// направления (F65).
func (s *Server) universityTargets(r *http.Request, out *universityDetail, rows []store.UniversityOlympiad) error {
	ctx, m, uni := r.Context(), me(r), []string{out.ID}
	ds, err := s.store.UniversityDirections(ctx, m.TrajectoryID, out.ID)
	if err != nil {
		return err
	}
	out.OfferedDirections = make([]offeredDirection, len(ds))
	for i, d := range ds {
		out.OfferedDirections[i] = offeredDirection{ID: d.ID, Code: d.Code, Name: d.Name, Status: d.Status,
			Programs: d.Programs, BudgetPlaces: d.BudgetPlaces, BenefitOlympiadsCount: d.BenefitOlympiads,
			IsMine: d.IsMine, IsGoal: d.IsGoal}
	}
	tg, err := s.store.TargetsOf(ctx, m.TrajectoryID, uni)
	if err != nil {
		return err
	}
	t := tg[out.ID]
	out.TargetBasis, out.TargetUnverified = t.Basis, t.Unverified
	out.TargetDirections = make([]directionItem, len(t.DirectionIDs))
	for i, id := range t.DirectionIDs {
		out.TargetDirections[i] = directionItem{ID: id, Name: t.DirectionNames[i]}
	}

	ids := make([]string, len(rows))
	for i, x := range rows {
		ids[i] = x.ProfileID
	}
	cov, err := s.store.DirectionCoverage(ctx, ids, uni)
	if err != nil {
		return err
	}
	benefits, err := s.store.TargetBenefits(ctx, m.TrajectoryID, ids, uni)
	if err != nil {
		return err
	}
	my := map[string]store.BenefitRow{}
	for _, b := range benefits {
		if !b.Unverified && b.Benefit != "extra_points" {
			my[b.ProfileID] = b
		}
	}
	for i, x := range rows {
		c := cov[x.ProfileID+"/"+out.ID]
		o := universityOlympiad{
			OlympiadProfileID: x.ProfileID, OlympiadID: x.OlympiadID, Name: names.Olympiad(x.OlympiadName), SubjectCode: x.SubjectCode,
			SubjectName: profileLabel(x.SubjectName, x.ProfileName), Level: x.Level, Benefit: x.Benefit,
			BenefitLabel: benefitLabels[x.Benefit], MyDirections: []string{},
			DirectionsCount: c.Count, DirectionsTotal: c.Total,
		}
		if b, ok := my[x.ProfileID]; ok {
			label := benefitLabels[b.Benefit]
			o.MyBenefit, o.MyBenefitLabel, o.MyDirections = &b.Benefit, &label, orEmpty(b.DirectionNames)
		}
		out.Olympiads[i] = o
	}
	return nil
}

type directionItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// directionOption — направление в справочнике: код, группы для выбора с
// поиском (D4) и popular — одно из основных, что показываются чипами.
type directionOption struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Code    string   `json:"code"`
	Groups  []string `json:"groups"`
	Popular bool     `json:"popular"`
}

// directions — справочник целей для правки профиля (F49): все направления
// вузов, основные — те же, что в онбординге бота.
func (s *Server) directions(w http.ResponseWriter, r *http.Request) error {
	ds, err := s.store.AllDirections(r.Context())
	if err != nil {
		return err
	}
	out := listResponse[directionOption]{Items: make([]directionOption, len(ds))}
	for i, d := range ds {
		out.Items[i] = directionOption{ID: d.ID, Name: d.Name, Code: d.Code, Groups: orEmpty(d.Groups), Popular: d.Onboarding}
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
