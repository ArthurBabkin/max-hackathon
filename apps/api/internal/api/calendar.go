package api

import (
	"net/http"
	"regexp"
	"sort"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
)

var monthRe = regexp.MustCompile(`^\d{4}-\d{2}$`)

type calendarDay struct {
	Date  string        `json:"date"`
	Items []trackerItem `json:"items"`
}

type calendarMonth struct {
	Month string        `json:"month"`
	Days  []calendarDay `json:"days"`
}

// calendar — GET /calendar?month=YYYY-MM (F32): все сроки этапов пунктов
// трекера за месяц, а не только ближайший. Запись — пункт трекера, у
// которого срок и название этапа — того этапа, что приходится на день.
// Даты — по Москве, как опубликованы сроки. Отмеченные этапы и всё после
// закрывающего итога в календарь не попадают.
func (s *Server) calendar(w http.ResponseWriter, r *http.Request) error {
	month := r.URL.Query().Get("month")
	start, err := time.ParseInLocation("2006-01", month, moscow)
	if !monthRe.MatchString(month) || err != nil {
		return badRequest("Месяц — в формате ГГГГ-ММ.")
	}
	end := start.AddDate(0, 1, 0)

	ctx, m := r.Context(), me(r)
	rows, err := s.store.TrackerItems(ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ProfileID
	}
	st, err := s.store.StagesFor(ctx, ids)
	if err != nil {
		return err
	}
	progress, err := s.progressOf(ctx, rows)
	if err != nil {
		return err
	}
	now := s.now()
	byDate := map[string][]trackerItem{}
	for _, row := range rows {
		base := trackerItemOf(row, st[row.ProfileID], progress[row.ID], now)
		for i, x := range st[row.ProfileID] {
			if x.DeadlineAt == nil || x.DeadlineAt.Before(start) || !x.DeadlineAt.Before(end) {
				continue
			}
			if stages.Settled(st[row.ProfileID], progress[row.ID], i) {
				continue
			}
			it := base
			it.DeadlineAt = utc(x.DeadlineAt)
			title := x.Title
			it.NextStageTitle = &title
			date := x.DeadlineAt.In(moscow).Format(time.DateOnly)
			byDate[date] = append(byDate[date], it)
		}
	}

	out := calendarMonth{Month: month, Days: []calendarDay{}}
	for date, items := range byDate {
		sort.SliceStable(items, func(i, j int) bool {
			if !items[i].DeadlineAt.Equal(*items[j].DeadlineAt) {
				return items[i].DeadlineAt.Before(*items[j].DeadlineAt)
			}
			return items[i].OlympiadName < items[j].OlympiadName
		})
		out.Days = append(out.Days, calendarDay{Date: date, Items: items})
	}
	sort.Slice(out.Days, func(i, j int) bool { return out.Days[i].Date < out.Days[j].Date })
	writeJSON(w, http.StatusOK, out)
	return nil
}
