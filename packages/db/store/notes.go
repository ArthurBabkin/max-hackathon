package store

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

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

// Оговорки сида (datasets/parser/build_seed.py, aggregate_key).
var (
	// Классы строки — у лучшей льготы, за остальные классы вуз даёт 100
	// баллов (#78).
	score100Re = regexp.MustCompile(`^За диплом ([0-9]+(?:(?:–|, )[0-9]+)*) класса — 100 баллов$`)
	egeRangeRe = regexp.MustCompile(`^Порог ЕГЭ зависит от программы: (\d+)–(\d+) баллов$`)
)

// grantNotes — оговорки сида о том, что получат победитель и призёр.
var grantNotes = []string{"Победителю — БВИ, призёру — 100 баллов", "БВИ только победителю", "100 баллов только победителю"}

// grantNote — какая из них в примечании; "" — никакой.
func grantNote(note *string) string {
	for _, s := range NoteSentences(note) {
		if slices.Contains(grantNotes, s) {
			return s
		}
	}
	return ""
}

// Score100Grades — оговорка «За диплом 9 класса — 100 баллов» из примечания
// и её классы.
func Score100Grades(note *string) (string, []int32) {
	for _, s := range NoteSentences(note) {
		if grades := score100Grades(s); grades != nil {
			return s, grades
		}
	}
	return "", nil
}

func score100Grades(sentence string) []int32 {
	m := score100Re.FindStringSubmatch(sentence)
	if m == nil {
		return nil
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
	return grades
}

// gradesText — 9, 9–10, 9, 11: как в сиде.
func gradesText(grades []int32) string {
	if n := len(grades); n > 1 && grades[n-1]-grades[0] == int32(n-1) {
		return fmt.Sprintf("%d–%d", grades[0], grades[n-1])
	}
	parts := make([]string, len(grades))
	for i, g := range grades {
		parts[i] = strconv.Itoa(int(g))
	}
	return strings.Join(parts, ", ")
}

// mergeDirections — одна строка на направления с одинаковой лучшей льготой
// (rows и их названия names — в порядке цели): порог — наименьший, классы —
// всех направлений; «зависит от программы» — с названием своего
// направления; разброс порога, предмет ЕГЭ и классы со 100 баллами — одним
// предложением на все, прочие оговорки — без повторов.
func mergeDirections(rows []BenefitRow, names []string) BenefitRow {
	out := rows[0]
	out.Varies = false
	var scores []int
	var grades, weaker []int32
	var subjects, varies, rest []string
	open := false
	for i, r := range rows {
		out.Varies = out.Varies || r.Varies
		if r.EgeMin != nil {
			scores = append(scores, *r.EgeMin)
		}
		open = open || r.DiplomaGrades == nil
		grades = append(grades, r.DiplomaGrades...)
		for _, s := range NoteSentences(r.Note) {
			if m := egeRangeRe.FindStringSubmatch(s); m != nil {
				lo, _ := strconv.Atoi(m[1])
				hi, _ := strconv.Atoi(m[2])
				scores = append(scores, lo, hi)
			} else if subj, ok := strings.CutPrefix(s, "Подтвердить ЕГЭ: "); ok {
				subjects = append(subjects, strings.Split(subj, " или ")...)
			} else if g := score100Grades(s); g != nil {
				weaker = append(weaker, g...)
			} else if v, ok := strings.CutPrefix(s, "Зависит от программы: "); ok {
				varies = append(varies, "Зависит от программы ("+names[i]+"): "+v)
			} else if s != "" && !slices.Contains(rest, s) {
				rest = append(rest, s)
			}
		}
	}
	notes := append(varies, rest...)
	out.DiplomaGrades = nil
	if !open {
		slices.Sort(grades)
		out.DiplomaGrades = slices.Compact(grades)
		weaker = slices.DeleteFunc(weaker, func(g int32) bool { return slices.Contains(out.DiplomaGrades, g) })
		slices.Sort(weaker)
		if weaker = slices.Compact(weaker); len(weaker) > 0 {
			notes = append(notes, "За диплом "+gradesText(weaker)+" класса — 100 баллов")
		}
	}
	out.EgeMin = nil
	if len(scores) > 0 {
		lo, hi := slices.Min(scores), slices.Max(scores)
		out.EgeMin = &lo
		if lo != hi {
			notes = append(notes, fmt.Sprintf("Порог ЕГЭ зависит от программы: %d–%d баллов", lo, hi))
		}
	}
	if len(subjects) > 0 {
		slices.Sort(subjects)
		notes = append(notes, "Подтвердить ЕГЭ: "+strings.Join(slices.Compact(subjects), " или "))
	}
	out.Note = nil
	if len(notes) > 0 {
		note := strings.Join(notes, ". ")
		out.Note = &note
	}
	return out
}
