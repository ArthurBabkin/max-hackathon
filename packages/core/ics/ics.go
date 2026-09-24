// Package ics собирает файл календаря (RFC 5545) со сроками олимпиад: его
// открывают «Календарь» iPhone, Google Календарь и другие.
//
// Срок — событие на весь день: у публикуемых сроков есть час (23:59 по
// Москве), но в календаре важен день, а событие «23:59–23:59» читалось бы
// хуже. Напоминание — накануне в 9 утра: бот и так пишет за месяц, неделю и
// три дня, календарь добавляет одно, самое срочное.
package ics

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Event — срок одного этапа олимпиады.
type Event struct {
	// UID — постоянный id: при повторной выгрузке календарь обновит
	// событие, а не заведёт второе.
	UID string
	// Day — момент срока в поясе, по которому считается день (Москва).
	Day         time.Time
	Summary     string
	Description string
}

// maxLine — предел строки в байтах без CRLF (RFC 5545, 3.1).
const maxLine = 75

// Calendar собирает календарь name из событий. stamp — момент выгрузки
// (DTSTAMP).
func Calendar(name string, events []Event, stamp time.Time) []byte {
	var b strings.Builder
	line := func(s string) { fold(&b, s) }

	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//Traektoria//Olympiad deadlines//RU")
	line("CALSCALE:GREGORIAN")
	line("METHOD:PUBLISH")
	line("X-WR-CALNAME:" + escape(name))
	line("X-WR-TIMEZONE:Europe/Moscow")
	for _, e := range events {
		day := e.Day.Format("20060102")
		next := e.Day.AddDate(0, 0, 1).Format("20060102")
		line("BEGIN:VEVENT")
		line("UID:" + escape(e.UID))
		line("DTSTAMP:" + stamp.UTC().Format("20060102T150405Z"))
		line("DTSTART;VALUE=DATE:" + day)
		line("DTEND;VALUE=DATE:" + next)
		line("SUMMARY:" + escape(e.Summary))
		if e.Description != "" {
			line("DESCRIPTION:" + escape(e.Description))
		}
		line("TRANSP:TRANSPARENT")
		line("BEGIN:VALARM")
		line("ACTION:DISPLAY")
		line("TRIGGER:-PT15H")
		line("DESCRIPTION:" + escape(e.Summary))
		line("END:VALARM")
		line("END:VEVENT")
	}
	line("END:VCALENDAR")
	return []byte(b.String())
}

// escape экранирует текстовое значение (RFC 5545, 3.3.11).
func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`).Replace(s)
}

// fold пишет строку, перенося её по 75 байт: продолжение начинается с
// пробела, а буква не разрезается посередине — иначе кириллица сломалась бы.
func fold(b *strings.Builder, s string) {
	limit := maxLine
	for len(s) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		b.WriteString(s[:cut])
		b.WriteString("\r\n ")
		s = s[cut:]
		// Пробел в начале продолжения — тоже байт строки.
		limit = maxLine - 1
	}
	b.WriteString(s)
	b.WriteString("\r\n")
}
