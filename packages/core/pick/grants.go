package pick

import (
	"slices"
	"strings"

	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

// Grants — что получат победитель и призёр по строке льготы: bvi, score100,
// extra_points или "" — ничего. Вид льготы в базе один на строку, а
// «победителю БВИ, призёру 100 баллов» и «только победителю» записаны в
// примечании. Один расчёт для таблицы льгот и помощника.
func Grants(b store.BenefitRow) (winner, prizer string) {
	note := NoteSentences(b.Note)
	switch b.Benefit {
	case "bvi", "score100", "extra_points":
		winner = b.Benefit
	case "bvi_winners":
		if slices.Contains(note, "Победителю — БВИ, призёру — 100 баллов") {
			return "bvi", "score100"
		}
		return "bvi", ""
	default:
		return "", ""
	}
	if slices.Contains(note, "БВИ только победителю") || slices.Contains(note, "100 баллов только победителю") {
		return winner, ""
	}
	return winner, winner
}

// NoteSentences — предложения примечания к льготе без точек на концах.
func NoteSentences(note *string) []string {
	if note == nil {
		return nil
	}
	var out []string
	// Точка в скобках или кавычках — сокращение («в г. Севастополе»),
	// предложение там не кончается.
	depth, start := 0, 0
	text := *note
	for i, r := range text {
		switch r {
		case '(', '«':
			depth++
		case ')', '»':
			depth = max(depth-1, 0)
		case '.':
			if depth == 0 && strings.HasPrefix(text[i:], ". ") {
				out = append(out, strings.TrimSpace(text[start:i]))
				start = i + len(". ")
			}
		}
	}
	return append(out, strings.TrimSuffix(strings.TrimSpace(text[start:]), "."))
}

// universityNicks — как вуз называют в строке льгот, если аббревиатура
// из справочника ничего не скажет школьнику.
var universityNicks = map[string]string{"innopolis": "Иннополис", "sechenov": "Сеченовский"}

// Nick — название вуза в строке льгот.
func Nick(universityID, shortName string) string {
	if n, ok := universityNicks[universityID]; ok {
		return n
	}
	return shortName
}
