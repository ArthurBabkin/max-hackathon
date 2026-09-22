package api

import (
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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

	nextKind string // для «следующего шага» на главной
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

func trackerItemOf(t store.TrackerRow, st []stages.Stage, now time.Time) trackerItem {
	item := trackerItem{
		ID: t.ID, OlympiadProfileID: t.ProfileID, OlympiadID: t.OlympiadID, OlympiadName: t.OlympiadName,
		SubjectName: profileLabel(t.SubjectName, t.ProfileName), Kind: t.Kind, Level: t.Level,
		RegisteredAt: utc(t.RegisteredAt), RegisteredBy: briefOf(t.RegisteredBy), AddedBy: briefOf(t.AddedBy),
	}
	if cur := stages.Current(st, t.RegisteredAt != nil, now); cur >= 0 {
		s := st[cur]
		item.DeadlineAt = utc(s.DeadlineAt)
		title := s.Title
		item.NextStageTitle = &title
		item.nextKind = s.Kind
	}
	return item
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
