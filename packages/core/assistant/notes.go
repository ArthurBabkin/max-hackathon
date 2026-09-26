package assistant

import (
	"regexp"
	"slices"
	"strings"

	"github.com/ArthurBabkin/max-hackathon/packages/core/pick"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
)

// Оговорки, которые модель должна сказать по правилам промпта, но
// пропускает через раз: какие даты — фактические или примерные — и что
// условия вуза уточняются. Недостающую оговорку дописываем в конец ответа
// по карточкам, на которые он ссылается.

// dates — олимпиады карточки с фактическими и с примерными датами.
type dates struct{ factual, approximate []string }

// add учитывает этапы олимпиады name: примерные, если хоть один этап —
// демо (сроки сезона не опубликованы), как в dated.
func (d *dates) add(name string, st []stages.Stage) {
	switch {
	case len(st) == 0:
	case approximate(st):
		d.approximate = appendNew(d.approximate, name)
	default:
		d.factual = appendNew(d.factual, name)
	}
}

func approximate(st []stages.Stage) bool {
	return slices.ContainsFunc(st, func(s stages.Stage) bool { return s.IsDemo })
}

// uniName — как вуз может называться в ответе: привычно и сокращением.
type uniName struct{ nick, short string }

var (
	// dateRe — в ответе есть дата: «11.10», «23 сентября», «в октябре».
	dateRe = regexp.MustCompile(`\d{1,2}\.\d{1,2}|(?i)(?:^|[^\p{L}])(?:январ|феврал|март|апрел|ма[еяй]|июн|июл|август|сентябр|октябр|ноябр|декабр)`)
	// datesSaid — модель сама сказала, какие даты.
	datesSaid = []string{"фактическ", "примерн", "ориентировочн", "предварительн", "прошлому году", "не опубликова"}
)

// withNotes — ответ с оговорками, которых в нём нет: о датах, если в ответе
// есть дата, и об условиях вузов, которые ответ называет.
func withNotes(text string, used []card) string {
	var d dates
	var unverified []string
	for _, cd := range used {
		for _, n := range cd.dates.factual {
			d.factual = appendNew(d.factual, n)
		}
		for _, n := range cd.dates.approximate {
			d.approximate = appendNew(d.approximate, n)
		}
		for _, u := range cd.unverified {
			if strings.Contains(text, u.nick) || strings.Contains(text, u.short) {
				unverified = appendNew(unverified, u.nick)
			}
		}
	}
	lower := strings.ToLower(text)
	var notes []string
	if dateRe.MatchString(text) && !slices.ContainsFunc(datesSaid, func(w string) bool { return strings.Contains(lower, w) }) {
		switch {
		case len(d.approximate) == 0 && len(d.factual) > 0:
			notes = append(notes, "Даты фактические — с сайта олимпиады.")
		case len(d.approximate) > 0 && len(d.factual) == 0:
			notes = append(notes, "Даты примерные: сроки этого сезона ещё не опубликованы — точные будут на сайте олимпиады.")
		case len(d.approximate) > 0:
			notes = append(notes, "Даты примерные, сроки ещё не опубликованы, у: "+strings.Join(d.approximate, ", ")+"; остальные — фактические.")
		}
	}
	switch {
	case len(unverified) == 0 || strings.Contains(lower, "уточня"):
	case len(unverified) == 1:
		notes = append(notes, unverified[0]+": условия льготы ещё уточняются — точные в правилах приёма вуза.")
	default:
		notes = append(notes, strings.Join(unverified, ", ")+": условия льгот ещё уточняются — точные в правилах приёма вузов.")
	}
	return strings.Join(append([]string{text}, notes...), " ")
}

// appendNewUni — вуз в оговорку «уточняется», без повторов.
func appendNewUni(xs []uniName, id, short string) []uniName {
	n := uniName{pick.Nick(id, short), short}
	if slices.Contains(xs, n) {
		return xs
	}
	return append(xs, n)
}

func appendNew(xs []string, x string) []string {
	if slices.Contains(xs, x) {
		return xs
	}
	return append(xs, x)
}
