package voice

import "strings"

// Склонение имён в родительный и дательный падеж — порт
// apps/web/src/lib/declension.ts, правила и исключения те же.
//
// MAX отдаёт имя только в именительном падеже, а родительскому голосу нужно
// «Цель Артёма» и «Предложить Артёму». Пол для имён почти не нужен: образец
// склонения задаёт окончание. Никита и Ольга склоняются одинаково.

type grammaticalCase int

const (
	genitiveCase grammaticalCase = iota
	dativeCase
)

// exceptions — имена, которые не подчиняются правилам.
var exceptions = map[string][2]string{
	// Беглая гласная в основе.
	"павел": {"Павла", "Павлу"},
	"лев":   {"Льва", "Льву"},
	"пётр":  {"Петра", "Петру"},
	"петр":  {"Петра", "Петру"},
	// Женское на -ь склоняется по третьему склонению, а не как Игорь.
	"любовь": {"Любови", "Любови"},
}

// hushing — после этих согласных вместо «ы» пишется «и».
const hushing = "гкхжчшщ"

// indeclinableEndings — окончания, после которых имя не склоняется: Отто, Мэри, Нино.
const indeclinableEndings = "оеэуюиы"

func isCyrillic(r rune) bool {
	return (r >= 'а' && r <= 'я') || (r >= 'А' && r <= 'Я') || r == 'ё' || r == 'Ё'
}

func decline(name string, c grammaticalCase) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return trimmed
	}
	// Латиница и всё нерусское не склоняем — правила к ним неприменимы.
	if !strings.ContainsFunc(trimmed, isCyrillic) {
		return trimmed
	}
	if forms, ok := exceptions[strings.ToLower(trimmed)]; ok {
		return forms[c]
	}

	runes := []rune(trimmed)
	n := len(runes)
	last := strings.ToLower(string(runes[n-1]))
	stem := string(runes[:n-1])
	beforeLast := ""
	if n >= 2 {
		beforeLast = strings.ToLower(string(runes[n-2]))
	}

	// Мария, Ксения — в обоих падежах «-ии».
	if n > 2 && beforeLast+last == "ия" {
		return stem + "и"
	}

	switch {
	case last == "а":
		if c == dativeCase {
			return stem + "е"
		}
		if beforeLast != "" && strings.Contains(hushing, beforeLast) {
			return stem + "и"
		}
		return stem + "ы"
	case last == "я":
		return stem + pick(c, "и", "е")
	case last == "й" || last == "ь":
		return stem + pick(c, "я", "ю")
	case strings.Contains(indeclinableEndings, last):
		return trimmed
	}
	// Осталась согласная: Артём, Иван, Тимур.
	return trimmed + pick(c, "а", "у")
}

func pick(c grammaticalCase, gen, dat string) string {
	if c == genitiveCase {
		return gen
	}
	return dat
}

// Genitive — кого? чего? «Цель Артёма», «В вузах Ольги».
func Genitive(name string) string { return decline(name, genitiveCase) }

// Dative — кому? чему? «Предложить Артёму».
func Dative(name string) string { return decline(name, dativeCase) }

// Accusative — кого? «Пригласить Артёма», «Пригласить Машу». У имён на
// согласную и -й/-ь совпадает с родительным, у имён на -а/-я — своё
// окончание; Любовь не меняется.
func Accusative(name string) string {
	trimmed := strings.TrimSpace(name)
	runes := []rune(trimmed)
	if len(runes) < 2 || !strings.ContainsFunc(trimmed, isCyrillic) {
		return trimmed
	}
	if strings.ToLower(trimmed) == "любовь" {
		return trimmed
	}
	stem := string(runes[:len(runes)-1])
	switch strings.ToLower(string(runes[len(runes)-1])) {
	case "а":
		return stem + "у"
	case "я":
		return stem + "ю"
	}
	return Genitive(trimmed)
}
