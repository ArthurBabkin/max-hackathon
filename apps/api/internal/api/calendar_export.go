package api

import (
	"cmp"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/auth"
	"github.com/ArthurBabkin/max-hackathon/packages/core/ics"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
)

// Выгрузка сроков в календарь телефона (F32).
//
// Файл .ics забирает не мини-приложение, а браузер телефона или сам MAX:
// заголовок Authorization им не передать. Поэтому мини-приложение сперва
// берёт ссылку (GET /calendar/link), а файл отдаётся по ней без сессии
// (GET /calendar.ics?token=…). Ссылка живёт десять минут и открывает только
// календарь своей семьи.

const (
	calendarLinkPurpose = "calendar"
	calendarLinkTTL     = 10 * time.Minute
)

type calendarLinkResponse struct {
	// URL — путь относительно базового адреса API.
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// calendarLink — GET /calendar/link.
func (s *Server) calendarLink(w http.ResponseWriter, r *http.Request) error {
	token, exp, err := auth.IssueLink(calendarLinkPurpose, me(r).MemberID, []byte(s.cfg.JWTSecret), calendarLinkTTL, s.now())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, calendarLinkResponse{
		URL:       "/calendar.ics?" + url.Values{"token": {token}}.Encode(),
		ExpiresAt: exp.UTC(),
	})
	return nil
}

var errCalendarLink = notFound("Ссылка на календарь устарела. Нажмите «Выгрузить в календарь» ещё раз.")

// calendarFile — GET /calendar.ics?token=…: все будущие сроки трекера семьи.
// Этапы, закрытые отметкой «зарегистрирован», не выгружаются — как и в
// календаре приложения.
func (s *Server) calendarFile(w http.ResponseWriter, r *http.Request) error {
	now := s.now()
	memberID, err := auth.ParseLink(calendarLinkPurpose, r.URL.Query().Get("token"), []byte(s.cfg.JWTSecret), now)
	if err != nil {
		return errCalendarLink
	}
	ctx := r.Context()
	m, err := s.store.ActiveMember(ctx, memberID)
	if err != nil {
		return errCalendarLink
	}
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

	v := voice.New(voice.Role(m.Role), "", m.FirstName)
	today := now.In(moscow).Format(time.DateOnly)
	var events []ics.Event
	for _, row := range rows {
		item := trackerItemOf(row, st[row.ProfileID], progress[row.ID], now)
		for i, x := range st[row.ProfileID] {
			if x.DeadlineAt == nil || stages.Settled(st[row.ProfileID], progress[row.ID], i) {
				continue
			}
			day := x.DeadlineAt.In(moscow)
			if day.Format(time.DateOnly) < today {
				continue
			}
			events = append(events, ics.Event{
				UID:         row.ID + "-" + x.ID + "@traektoria",
				Day:         day,
				Summary:     item.OlympiadName + ": " + x.Title,
				Description: v.T("calendar.icsDescription", voice.Vars{"subject": item.SubjectName, "date": stages.Day(day)}),
			})
		}
	}
	slices.SortStableFunc(events, func(a, b ics.Event) int {
		return cmp.Or(a.Day.Compare(b.Day), cmp.Compare(a.Summary, b.Summary))
	})

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	// inline: Safari на iPhone сам предлагает добавить события в «Календарь».
	w.Header().Set("Content-Disposition", `inline; filename="traektoria.ics"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(ics.Calendar(v.T("calendar.icsName", nil), events, now))
	return err
}
