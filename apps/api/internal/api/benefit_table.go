package api

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ArthurBabkin/max-hackathon/packages/core/pick"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

// Таблица льгот в карточке олимпиады (F18, F19): вузы строками, в столбцах —
// что получат победитель и призёр и порог ЕГЭ. Вузов может быть сколько
// угодно — таблица растёт вниз; столбцов — закрытый набор, а всё редкое
// (класс диплома, другой предмет ЕГЭ) — условием в строке своего вуза.

// grants — что получат победитель и призёр: вид льготы плюс оговорки из
// примечания вуза (pick.Grants) с подписями. Призёру без льготы и вузу без
// записи — nil.
func grants(b store.BenefitRow) (winner, prizer *benefitGrant) {
	grant := func(kind string) *benefitGrant {
		switch kind {
		case "":
			return nil
		case "extra_points":
			return &benefitGrant{Kind: kind, Label: extraPointsLabel(b.ExtraPoints)}
		}
		return &benefitGrant{Kind: kind, Label: benefitLabel(kind, b.Note)}
	}
	w, p := pick.Grants(b)
	return grant(w), grant(p)
}

// Предмета в подписи нет, если сид его не знает (score100Unknown) или
// склонения нет в словаре: «100 баллов» без предмета лучше неверного падежа.
var score100Dative = map[string]string{
	"Информатика": "информатике", "Математика": "математике", "Физика": "физике", "Химия": "химии",
	"Биология": "биологии", "Обществознание": "обществознанию", "География": "географии",
}

var quotedRe = regexp.MustCompile(`«([^»]+)»`)

// benefitLabel — подпись льготы; у 100 баллов — с предметом из примечания
// сида: «100 баллов по информатике». Строк несколько (вузы в одной подписи) —
// предмет, только если он у всех один.
func benefitLabel(kind string, notes ...*string) string {
	label := benefitLabels[kind]
	if kind != "score100" {
		return label
	}
	subject := ""
	for i, note := range notes {
		s := score100Subject(note)
		if s == "" || (i > 0 && s != subject) {
			return label
		}
		subject = s
	}
	if subject == "" {
		return label
	}
	return label + " по " + subject
}

// score100Subject — «информатике или математике» из предложения «100 баллов
// засчитают по предмету «Информатика» или «Математика»».
func score100Subject(note *string) string {
	for _, s := range pick.NoteSentences(note) {
		if !strings.HasPrefix(s, score100Prefix) {
			continue
		}
		var out []string
		for _, m := range quotedRe.FindAllStringSubmatch(s, -1) {
			d, ok := score100Dative[m[1]]
			if !ok {
				return ""
			}
			out = append(out, d)
		}
		return strings.Join(out, " или ")
	}
	return ""
}

// extraPointsLabel — «+3 балла»; без числа — «доп. баллы».
func extraPointsLabel(points *int) string {
	if points == nil {
		return benefitLabels["extra_points"]
	}
	return fmt.Sprintf("+%d %s", *points, voice.Plural(*points, "балл", "балла", "баллов"))
}

var grantRank = map[string]int{"bvi": 0, "score100": 1, "extra_points": 2}

// sortBenefitRows — сначала самые выгодные: БВИ, 100 баллов, доп. баллы
// (больше — выше), при равных — по тому, что достанется призёру, дальше по
// алфавиту. Вузы, которые олимпиаду не учитывают, — в конце.
func sortBenefitRows(rows []benefitRow) {
	rank := func(g *benefitGrant) int {
		if g == nil {
			return len(grantRank)
		}
		return grantRank[g.Kind]
	}
	points := func(r benefitRow) int {
		if r.ExtraPoints == nil {
			return 0
		}
		return *r.ExtraPoints
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if x, y := rank(a.Winner), rank(b.Winner); x != y {
			return x < y
		}
		if x, y := points(a), points(b); x != y {
			return x > y
		}
		if x, y := rank(a.Prizer), rank(b.Prizer); x != y {
			return x < y
		}
		return strings.ToLower(a.UniversityNick) < strings.ToLower(b.UniversityNick)
	})
}

// benefitColumns — столбцы таблицы. Вне перечня — только доп. баллы. Порог
// ЕГЭ — если он в вузах ученика разный или зависит от программы; одинаковый
// уходит в общие условия.
func benefitColumns(p store.Profile, mine []store.BenefitRow) []string {
	if p.Kind == "other" {
		return []string{"extra_points"}
	}
	if egeInTable(mine) {
		return []string{"winner", "prizer", "ege"}
	}
	return []string{"winner", "prizer"}
}

func egeInTable(rows []store.BenefitRow) bool {
	seen := map[string]bool{}
	for _, b := range rows {
		if _, _, ok := egeRange(b.Note); ok {
			return true
		}
		key := "—"
		if b.EgeMin != nil {
			key = fmt.Sprint(*b.EgeMin)
		}
		seen[key] = true
	}
	return len(seen) > 1
}

// noteSubject — предмет ЕГЭ из примечания «Подтвердить ЕГЭ: …».
func noteSubject(note *string) string {
	for _, sentence := range pick.NoteSentences(note) {
		if subj, ok := strings.CutPrefix(sentence, "Подтвердить ЕГЭ: "); ok {
			return subj
		}
	}
	return ""
}

// subjects — варианты предмета через «или», только похожие на название
// предмета: с заглавной буквы, без перечислений через запятую. Остальное —
// хвосты разбора правил, в карточку их не выводим.
func subjects(s string) []string {
	var out []string
	for _, alt := range strings.Split(s, " или ") {
		alt = strings.TrimSpace(alt)
		first, _ := utf8.DecodeRuneInString(alt)
		if alt == "" || !unicode.IsUpper(first) || strings.Contains(alt, ",") || utf8.RuneCountInString(alt) > 40 {
			continue
		}
		out = append(out, alt)
	}
	return out
}

// sameSubject — есть ли общий вариант. «Информатика и ИКТ» — та же
// информатика, обрезанное название совпадает с полным по началу.
func sameSubject(a, b []string) bool {
	if len(b) == 0 {
		return true
	}
	norm := func(s string) string { return strings.TrimSuffix(s, " и ИКТ") }
	for _, x := range a {
		for _, y := range b {
			x, y := norm(x), norm(y)
			if strings.HasPrefix(x, y) || strings.HasPrefix(y, x) {
				return true
			}
		}
	}
	return false
}
