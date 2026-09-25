package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// calendarLink — GET /calendar/link: путь к файлу относительно /api/v1.
func (e *env) calendarLink(token string) string {
	e.t.Helper()
	r := e.do("GET", "/api/v1/calendar/link", token, nil)
	if r.code != http.StatusOK {
		e.t.Fatalf("ссылка на календарь: %d %s", r.code, r.raw)
	}
	link, _ := r.body["url"].(string)
	if !strings.HasPrefix(link, "/calendar.ics?token=") {
		e.t.Fatalf("ссылка: %q", link)
	}
	return link
}

// icsEvents — события файла: «ГГГГММДД Заголовок» в порядке файла.
func icsEvents(t *testing.T, raw []byte) []string {
	t.Helper()
	var out []string
	var day string
	for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n ", ""), "\r\n") {
		switch {
		case strings.HasPrefix(line, "DTSTART;VALUE=DATE:"):
			day = strings.TrimPrefix(line, "DTSTART;VALUE=DATE:")
		case strings.HasPrefix(line, "SUMMARY:"):
			out = append(out, day+" "+strings.TrimPrefix(line, "SUMMARY:"))
		}
	}
	return out
}

func TestCalendarExport(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	nto := e.track(f, "p669-5-iskusstvennyy-intellekt", false)
	e.track(f, "vsosh-informatika", false)
	token := e.login(900000002, "Ольга")

	r := e.do("GET", "/api/v1/calendar/link", token, nil)
	if r.code != 200 || r.body["expires_at"] != e.now.Add(10*time.Minute).Format(time.RFC3339) {
		t.Fatalf("ссылка живёт десять минут: %d %s", r.code, r.raw)
	}

	// Файл забирает браузер телефона — без заголовка Authorization.
	link := e.calendarLink(token)
	file := e.do("GET", "/api/v1"+link, "", nil)
	if file.code != 200 {
		t.Fatalf("файл: %d %s", file.code, file.raw)
	}
	if ct := file.hdr.Get("Content-Type"); ct != "text/calendar; charset=utf-8" {
		t.Fatalf("Content-Type: %q", ct)
	}
	if cc := file.hdr.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("файл со сроками не кешируется: %q", cc)
	}
	if !strings.Contains(string(file.raw), "X-WR-CALNAME:Траектория: сроки олимпиад\r\n") {
		t.Fatalf("название календаря:\n%s", file.raw)
	}

	// Все будущие сроки обоих пунктов, по дате, день — по Москве.
	got := icsEvents(t, file.raw)
	want := []string{
		"20261022 : Регистрация",
		"20261023 : Первый (индивидуальный) этап",
		"20261028 ВсОШ по информатике: Школьный этап",
		"20261128 ВсОШ по информатике: Муниципальный этап",
		"20261211 : Второй (командный) этап",
		"20270123 ВсОШ по информатике: Региональный этап",
		"20270409 ВсОШ по информатике: Заключительный этап",
	}
	if len(got) != len(want) {
		t.Fatalf("события:\n%s", strings.Join(got, "\n"))
	}
	for i := range want {
		date, suffix, _ := strings.Cut(want[i], " ")
		if !strings.HasPrefix(got[i], date+" ") || !strings.HasSuffix(got[i], suffix) {
			t.Fatalf("событие %d: %q, ждали %q", i, got[i], want[i])
		}
	}
	if !strings.Contains(strings.ReplaceAll(string(file.raw), "\r\n ", ""), "DESCRIPTION:Информатика. Срок — 28 октября.\r\n") {
		t.Fatalf("в описании — предмет и день срока:\n%s", file.raw)
	}

	// Отметка «зарегистрирован» убирает срок регистрации, как в календаре.
	if _, err := e.pool.Exec(context.Background(), `UPDATE tracker_items SET registered_at = now() WHERE id = $1`, nto); err != nil {
		t.Fatal(err)
	}
	if got := icsEvents(t, e.do("GET", "/api/v1"+e.calendarLink(token), "", nil).raw); len(got) != 6 || strings.HasSuffix(got[0], "Регистрация") {
		t.Fatalf("после отметки:\n%s", strings.Join(got, "\n"))
	}

	// Итог «не прошёл» у ВсОШ закрывает олимпиаду — её дальнейших сроков
	// в файле нет.
	if _, err := e.pool.Exec(context.Background(), `INSERT INTO tracker_stage_results (tracker_item_id, stage_id, result)
		SELECT id, 'vsosh-informatika:school:1', 'failed' FROM tracker_items WHERE olympiad_profile_id = 'vsosh-informatika'`); err != nil {
		t.Fatal(err)
	}
	if got := icsEvents(t, e.do("GET", "/api/v1"+e.calendarLink(token), "", nil).raw); len(got) != 2 {
		t.Fatalf("после «не прошёл» у ВсОШ:\n%s", strings.Join(got, "\n"))
	}
	if _, err := e.pool.Exec(context.Background(), `DELETE FROM tracker_stage_results`); err != nil {
		t.Fatal(err)
	}

	// Прошедшие сроки не выгружаются.
	e.now = time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	token = e.login(900000002, "Ольга") // прежняя сессия за месяц истекла
	if got := icsEvents(t, e.do("GET", "/api/v1"+e.calendarLink(token), "", nil).raw); len(got) != 5 || !strings.HasPrefix(got[0], "20261028 ") {
		t.Fatalf("после 23 октября:\n%s", strings.Join(got, "\n"))
	}
}

func TestCalendarExport_LinkIsAPass(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	e.track(f, "vsosh-informatika", false)
	token := e.login(900000002, "Ольга")

	if r := e.do("GET", "/api/v1/calendar/link", "", nil); r.code != 401 {
		t.Fatalf("ссылку выдают только своим: %d", r.code)
	}

	link := e.calendarLink(token)
	for name, path := range map[string]string{
		"без токена":        "/calendar.ics",
		"испорченный токен": link + "x",
		"токен сессии":      "/calendar.ics?token=" + token,
	} {
		if r := e.do("GET", "/api/v1"+path, "", nil); r.code != 404 || r.errCode() != "NOT_FOUND" {
			t.Fatalf("%s — 404: %d %s", name, r.code, r.raw)
		}
	}

	// Ссылка живёт десять минут.
	e.now = e.now.Add(11 * time.Minute)
	if r := e.do("GET", "/api/v1"+link, "", nil); r.code != 404 {
		t.Fatalf("просроченная ссылка — 404: %d", r.code)
	}

	// Участника удалили из семьи — его ссылка больше не работает.
	e.now = testNow
	if _, err := e.pool.Exec(context.Background(), `DELETE FROM members WHERE id = $1`, f.parent.MemberID); err != nil {
		t.Fatal(err)
	}
	if r := e.do("GET", "/api/v1"+link, "", nil); r.code != 404 {
		t.Fatalf("ссылка удалённого участника — 404: %d", r.code)
	}
}
