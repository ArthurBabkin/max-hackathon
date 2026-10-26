package notify

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
)

// changeDetail — content_changes.detail (миграция 0030): каким этап или
// льгота были и какими стали. Объявление вуза приходит готовым текстом.
type changeDetail struct {
	Title string       `json:"title"`
	Kind  string       `json:"kind"`
	Old   *changeState `json:"old"`
	New   *changeState `json:"new"`
	Text  string       `json:"text"`
}

type changeState struct {
	StartsAt      *time.Time `json:"starts_at"`
	EndsAt        *time.Time `json:"ends_at"`
	DeadlineAt    *time.Time `json:"deadline_at"`
	Benefit       *string    `json:"benefit"`
	EgeMin        *int       `json:"ege_min"`
	DiplomaGrades []int32    `json:"diploma_grades"`
}

// Describe — изменение одной фразой: «Заключительный этап: 14 февраля (было
// 13 февраля)», «МФТИ: порог ЕГЭ 75 → 85». university — вуз льготы, у этапов
// пустой. Пустая строка — рассказать нечего: событие старше 0030 или правка,
// которую словами не передать; тогда остаётся общее «сроки этапов».
func Describe(v voice.Voice, raw []byte, university string) string {
	if len(raw) == 0 {
		return ""
	}
	var d changeDetail
	if json.Unmarshal(raw, &d) != nil {
		return ""
	}
	switch {
	case d.Text != "":
		return d.Text
	case university != "":
		return describeBenefit(v, d, university)
	default:
		return describeStage(v, d)
	}
}

// isAnnouncement — событие-объявление вуза с готовым текстом.
func isAnnouncement(raw []byte) bool {
	var d changeDetail
	return len(raw) > 0 && json.Unmarshal(raw, &d) == nil && d.Text != ""
}

func describeStage(v voice.Voice, d changeDetail) string {
	if d.Title == "" {
		return ""
	}
	when := func(s *changeState) string {
		sub := stages.Subtitle(stages.Stage{Kind: d.Kind, StartsAt: s.StartsAt, EndsAt: s.EndsAt, DeadlineAt: s.DeadlineAt}, location("Europe/Moscow"))
		if sub == "" {
			return v.T("change.dateLater", nil)
		}
		return sub
	}
	switch {
	case d.Old != nil && d.New != nil:
		was, now := when(d.Old), when(d.New)
		if was == now {
			return v.T("change.stageTime", voice.Vars{"title": d.Title, "now": now})
		}
		return v.T("change.stageMoved", voice.Vars{"title": d.Title, "now": now, "was": was})
	case d.New != nil:
		return v.T("change.stageAdded", voice.Vars{"title": d.Title, "now": when(d.New)})
	case d.Old != nil:
		return v.T("change.stageRemoved", voice.Vars{"title": d.Title})
	}
	return ""
}

func describeBenefit(v voice.Voice, d changeDetail, uni string) string {
	label := func(s *changeState) string {
		if s == nil || s.Benefit == nil {
			return ""
		}
		return v.T("change.grant."+*s.Benefit, nil)
	}
	switch {
	case d.Old == nil && d.New != nil:
		return v.T("change.benefitAdded", voice.Vars{"uni": uni, "now": label(d.New)})
	case d.Old != nil && d.New == nil:
		return v.T("change.benefitRemoved", voice.Vars{"uni": uni})
	case d.Old == nil:
		return ""
	}
	var parts []string
	if was, now := label(d.Old), label(d.New); was != now {
		parts = append(parts, v.T("change.benefitKind", voice.Vars{"uni": uni, "was": was, "now": now}))
	}
	if o, n := d.Old.EgeMin, d.New.EgeMin; n != nil && (o == nil || *o != *n) {
		if o == nil {
			parts = append(parts, v.T("change.benefitEgeNew", voice.Vars{"uni": uni, "now": *n}))
		} else {
			parts = append(parts, v.T("change.benefitEge", voice.Vars{"uni": uni, "was": *o, "now": *n}))
		}
	}
	if o, n := d.Old.DiplomaGrades, d.New.DiplomaGrades; len(n) > 0 && !slices.Equal(o, n) {
		parts = append(parts, v.T("change.benefitGrades", voice.Vars{"uni": uni, "now": GradesLabel(n)}))
	}
	return strings.Join(parts, "; ")
}

// GradesLabel — «10–11», «9, 11».
func GradesLabel(grades []int32) string {
	contiguous := true
	for i := 1; i < len(grades); i++ {
		contiguous = contiguous && grades[i] == grades[i-1]+1
	}
	switch {
	case len(grades) == 1:
		return fmt.Sprint(grades[0])
	case contiguous:
		return fmt.Sprintf("%d–%d", grades[0], grades[len(grades)-1])
	}
	parts := make([]string, len(grades))
	for i, g := range grades {
		parts[i] = fmt.Sprint(g)
	}
	return strings.Join(parts, ", ")
}
