package pick

import (
	"regexp"
	"slices"
	"strconv"
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

// score100Grades — оговорка сида «За диплом 9 класса — 100 баллов»: классы
// строки — у лучшей льготы, за остальные классы вуз даёт 100 баллов (#78).
var score100Grades = regexp.MustCompile(`^За диплом ([0-9]+(?:(?:–|, )[0-9]+)*) класса — 100 баллов$`)

// Score100Grades — эта оговорка из примечания и её классы.
func Score100Grades(note *string) (string, []int32) {
	for _, s := range NoteSentences(note) {
		m := score100Grades.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		var grades []int32
		for _, part := range strings.Split(m[1], ", ") {
			from, to, ok := strings.Cut(part, "–")
			if !ok {
				to = from
			}
			a, _ := strconv.Atoi(from)
			b, _ := strconv.Atoi(to)
			for g := a; g <= b; g++ {
				grades = append(grades, int32(g))
			}
		}
		return s, grades
	}
	return "", nil
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
