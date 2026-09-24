package ics

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

var (
	msk   = time.FixedZone("MSK", 3*60*60)
	stamp = time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC)
)

func event() Event {
	return Event{
		UID:         "tr-1-st-1@traektoria",
		Day:         time.Date(2026, 9, 28, 23, 59, 0, 0, msk),
		Summary:     "Высшая проба (информатика): Регистрация",
		Description: "Последний день — 28 сентября, 23:59 по Москве.",
	}
}

// Строки файла без переносов-продолжений (RFC 5545, 3.1).
func unfold(t *testing.T, b []byte) []string {
	t.Helper()
	s := string(b)
	if !strings.HasSuffix(s, "\r\n") {
		t.Fatalf("файл кончается не CRLF: %q", s[max(0, len(s)-20):])
	}
	if strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\n") {
		t.Fatal("перенос строки без CR")
	}
	return strings.Split(strings.TrimSuffix(strings.ReplaceAll(s, "\r\n ", ""), "\r\n"), "\r\n")
}

func index(lines []string, want string) int {
	for i, l := range lines {
		if l == want {
			return i
		}
	}
	return -1
}

func TestCalendar_Structure(t *testing.T) {
	lines := unfold(t, Calendar("Траектория: сроки олимпиад", []Event{event()}, stamp))

	if lines[0] != "BEGIN:VCALENDAR" || lines[len(lines)-1] != "END:VCALENDAR" {
		t.Fatalf("обёртка: %q … %q", lines[0], lines[len(lines)-1])
	}
	for _, want := range []string{
		"VERSION:2.0",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"X-WR-CALNAME:Траектория: сроки олимпиад",
		"BEGIN:VEVENT",
		"UID:tr-1-st-1@traektoria",
		"DTSTAMP:20260924T090000Z",
		// Срок — событие на весь день: дата по поясу срока, конец — следующий день.
		"DTSTART;VALUE=DATE:20260928",
		"DTEND;VALUE=DATE:20260929",
		"SUMMARY:Высшая проба (информатика): Регистрация",
		"END:VEVENT",
	} {
		if index(lines, want) < 0 {
			t.Errorf("нет строки %q в\n%s", want, strings.Join(lines, "\n"))
		}
	}
	if !strings.HasPrefix(lines[index(lines, "VERSION:2.0")+1], "PRODID:") {
		t.Error("PRODID — сразу после VERSION")
	}
}

func TestCalendar_DateInEventZone(t *testing.T) {
	// 21:30 UTC 31 декабря — это уже 1 января по Москве.
	e := event()
	e.Day = time.Date(2026, 12, 31, 21, 30, 0, 0, time.UTC).In(msk)
	lines := unfold(t, Calendar("x", []Event{e}, stamp))
	if index(lines, "DTSTART;VALUE=DATE:20270101") < 0 || index(lines, "DTEND;VALUE=DATE:20270102") < 0 {
		t.Fatalf("дата не по поясу срока:\n%s", strings.Join(lines, "\n"))
	}
}

func TestCalendar_Alarm(t *testing.T) {
	lines := unfold(t, Calendar("x", []Event{event()}, stamp))
	begin, end := index(lines, "BEGIN:VALARM"), index(lines, "END:VALARM")
	if begin < 0 || end < begin || end > index(lines, "END:VEVENT") {
		t.Fatalf("напоминание — внутри события:\n%s", strings.Join(lines, "\n"))
	}
	alarm := strings.Join(lines[begin:end], "\n")
	// Весь день начинается в полночь: минус 15 часов — 9 утра накануне.
	for _, want := range []string{"ACTION:DISPLAY", "TRIGGER:-PT15H", "DESCRIPTION:Высшая проба (информатика): Регистрация"} {
		if !strings.Contains(alarm, want) {
			t.Errorf("в напоминании нет %q:\n%s", want, alarm)
		}
	}
}

func TestCalendar_Escaping(t *testing.T) {
	e := event()
	e.Summary = `ОММО; тур 1, заочный \ онлайн`
	e.Description = "Строка один\nстрока два"
	lines := unfold(t, Calendar("a, b; c", []Event{e}, stamp))
	for _, want := range []string{
		`X-WR-CALNAME:a\, b\; c`,
		`SUMMARY:ОММО\; тур 1\, заочный \\ онлайн`,
		`DESCRIPTION:Строка один\nстрока два`,
	} {
		if index(lines, want) < 0 {
			t.Errorf("нет строки %q в\n%s", want, strings.Join(lines, "\n"))
		}
	}
}

func TestCalendar_Folding(t *testing.T) {
	e := event()
	e.Description = strings.Repeat("Всероссийская олимпиада школьников по информатике, ", 6)
	raw := Calendar("x", []Event{e}, stamp)

	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\r\n"), "\r\n") {
		if len(line) > 75 {
			t.Fatalf("строка длиннее 75 байт (%d): %q", len(line), line)
		}
		if !utf8.ValidString(line) {
			t.Fatalf("перенос разрезал букву: %q", line)
		}
	}
	lines := unfold(t, raw)
	if index(lines, "DESCRIPTION:"+strings.ReplaceAll(e.Description, ",", `\,`)) < 0 {
		t.Fatalf("после склейки описание не то:\n%s", strings.Join(lines, "\n"))
	}
}

func TestCalendar_Empty(t *testing.T) {
	lines := unfold(t, Calendar("x", nil, stamp))
	if index(lines, "BEGIN:VEVENT") >= 0 || lines[len(lines)-1] != "END:VCALENDAR" {
		t.Fatalf("пустой календарь:\n%s", strings.Join(lines, "\n"))
	}
}
