package api

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ArthurBabkin/max-hackathon/packages/core/names"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

// Даты этапов в датасетах опубликованы по Москве — подписи считаем так же.
var moscow = mustLoad("Europe/Moscow")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

type memberBrief struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func briefOf(m *store.MemberBrief) *memberBrief {
	if m == nil {
		return nil
	}
	return &memberBrief{ID: m.ID, Name: m.Name, Role: m.Role}
}

// badge — оформление плитки. Сервер его не заполняет: клиент строит
// аббревиатуру и цвет сам (apps/web/src/lib/badge.ts).
type badge struct {
	ShortName *string `json:"short_name"`
	Color     *string `json:"color"`
}

type trackerItem struct {
	badge
	ID                string       `json:"id"`
	OlympiadProfileID string       `json:"olympiad_profile_id"`
	OlympiadID        string       `json:"olympiad_id"`
	OlympiadName      string       `json:"olympiad_name"`
	SubjectName       string       `json:"subject_name"`
	Kind              string       `json:"kind"`
	Level             *string      `json:"level"`
	DeadlineAt        *time.Time   `json:"deadline_at"`
	NextStageTitle    *string      `json:"next_stage_title"`
	RegisteredAt      *time.Time   `json:"registered_at"`
	RegisteredBy      *memberBrief `json:"registered_by"`
	AddedBy           *memberBrief `json:"added_by"`
	// Status — open (нужно зарегистрироваться), active (участвует),
	// finished (участие закончено); Outcome — чем закончилось.
	Status  string         `json:"status"`
	Outcome *string        `json:"outcome"`
	Stages  []trackerStage `json:"stages"`
	// Action — одна строка действия в карточке: что отметить сейчас.
	Action *trackerAction `json:"action"`

	nextKind string // для «следующего шага» на главной
}

// trackerStage — этап в карточке трекера с отметками и тем, что можно
// отметить сейчас. Состояние locked — этап после закрывающего итога.
type trackerStage struct {
	ID             string     `json:"id"`
	Kind           string     `json:"kind"`
	Title          string     `json:"title"`
	Subtitle       *string    `json:"subtitle"`
	StartsAt       *time.Time `json:"starts_at"`
	EndsAt         *time.Time `json:"ends_at"`
	DeadlineAt     *time.Time `json:"deadline_at"`
	State          string     `json:"state"`
	Registered     bool       `json:"registered"`
	Result         *string    `json:"result"`
	CanRegister    bool       `json:"can_register"`
	Results        []string   `json:"results"`
	ResultsAllowed []string   `json:"results_allowed"`
	Asking         bool       `json:"asking"`
}

// trackerAction — register: отметить регистрацию (без этапа — «участвую»
// у олимпиады без этапа-регистрации, та же галочка registered), result —
// отметить итог этапа.
type trackerAction struct {
	Type    string  `json:"type"`
	StageID *string `json:"stage_id"`
}

// profileLabel — подпись профиля. У НТО пятнадцать профилей с предметом
// «Информатика», и различает их только название профиля, поэтому оно
// показывается, когда отличается от предмета.
func profileLabel(subjectName string, profileName *string) string {
	if profileName == nil || *profileName == "" || strings.EqualFold(*profileName, subjectName) {
		return subjectName
	}
	r, size := utf8.DecodeRuneInString(*profileName)
	return string(unicode.ToUpper(r)) + (*profileName)[size:]
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func trackerItemOf(t store.TrackerRow, st []stages.Stage, p stages.Progress, now time.Time) trackerItem {
	item := trackerItem{
		ID: t.ID, OlympiadProfileID: t.ProfileID, OlympiadID: t.OlympiadID, OlympiadName: names.Olympiad(t.OlympiadName),
		SubjectName: profileLabel(t.SubjectName, t.ProfileName), Kind: t.Kind, Level: t.Level,
		RegisteredAt: utc(t.RegisteredAt), RegisteredBy: briefOf(t.RegisteredBy), AddedBy: briefOf(t.AddedBy),
	}
	if cur := stages.Current(st, p, now); cur >= 0 {
		s := st[cur]
		item.DeadlineAt = utc(s.DeadlineAt)
		title := s.Title
		item.NextStageTitle = &title
		item.nextKind = s.Kind
	}
	status, outcome := stages.Status(st, p, now)
	item.Status = status
	if outcome != "" {
		item.Outcome = &outcome
	}
	item.Stages = trackerStagesOf(st, p, now)
	item.Action = actionOf(st, p, status, now)
	return item
}

func trackerStagesOf(st []stages.Stage, p stages.Progress, now time.Time) []trackerStage {
	states := stages.States(st, p, now)
	asking := stages.Asking(st, p, now)
	out := make([]trackerStage, len(st))
	for i, x := range st {
		registered := stages.RegisteredOn(st, p, i)
		result := p.Marks[x.ID].Result
		d := trackerStage{
			ID: x.ID, Kind: x.Kind, Title: x.Title, StartsAt: utc(x.StartsAt), EndsAt: utc(x.EndsAt),
			DeadlineAt: utc(x.DeadlineAt), State: states[i], Registered: registered,
			Results: stages.Results(st, i), ResultsAllowed: []string{}, Asking: i == asking,
		}
		if d.Results == nil {
			d.Results = []string{}
		}
		if stages.Locked(st, p, i) {
			d.State = "locked"
		}
		if sub := stages.Subtitle(x, moscow); sub != "" {
			d.Subtitle = &sub
		}
		if result != "" {
			d.Result = &result
		}
		if stages.RegistrationLike(x.Kind) {
			_, err := stages.Apply(st, p, x.ID, stages.Mark{Registered: !registered, Result: result}, now)
			d.CanRegister = err == nil
		}
		for _, r := range d.Results {
			if _, err := stages.Apply(st, p, x.ID, stages.Mark{Registered: registered, Result: r}, now); err == nil {
				d.ResultsAllowed = append(d.ResultsAllowed, r)
			}
		}
		out[i] = d
	}
	return out
}

// actionOf — что отметить сейчас. Не зарегистрирован — регистрацию, даже
// если она закрылась: вдруг отметить просто забыли. Этап закончился без
// итога — итог. Дальше регистрация на следующий этап — её.
func actionOf(st []stages.Stage, p stages.Progress, status string, now time.Time) *trackerAction {
	if !p.Registered && stages.ClosedAt(st, p) < 0 {
		a := &trackerAction{Type: "register"}
		if i := stages.FirstRegistration(st); i >= 0 {
			a.StageID = &st[i].ID
		}
		return a
	}
	if i := stages.Asking(st, p, now); i >= 0 {
		return &trackerAction{Type: "result", StageID: &st[i].ID}
	}
	if cur := stages.Current(st, p, now); status != stages.StatusFinished && cur >= 0 &&
		stages.RegistrationLike(st[cur].Kind) && !stages.RegisteredOn(st, p, cur) {
		return &trackerAction{Type: "register", StageID: &st[cur].ID}
	}
	return nil
}

// progressOf — отметки пунктов трекера по id пункта: первая регистрация —
// из самого пункта, остальное — отметки этапов.
func (s *Server) progressOf(ctx context.Context, rows []store.TrackerRow) (map[string]stages.Progress, error) {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	marks, err := s.store.StageMarks(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]stages.Progress, len(rows))
	for _, r := range rows {
		out[r.ID] = stages.Progress{Registered: r.RegisteredAt != nil, Marks: marks[r.ID]}
	}
	return out, nil
}

// sortByDeadline — ближайшие сроки сначала, пункты без срока в конце.
func sortByDeadline(items []trackerItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].DeadlineAt, items[j].DeadlineAt
		if (a == nil) != (b == nil) {
			return a != nil
		}
		return a != nil && a.Before(*b)
	})
}
