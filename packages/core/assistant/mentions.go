// Package assistant — ИИ-помощник (ТЗ §6.4, F35–F37): RAG только по нашей
// базе. Находит в вопросе олимпиады и вузы, собирает их карточки из БД,
// спрашивает модель «только по контексту» и проверяет, что ответ опирается
// на эти карточки. Нет карточек или ссылок на них — шаблон «данных нет».
package assistant

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Named — олимпиада или вуз с названием.
type Named struct {
	ID      string
	Name    string
	Aliases []string // дополнительные написания: аббревиатуры, народные названия
}

// Mentions — что упомянуто в вопросе.
type Mentions struct {
	Olympiads    []string
	Universities []string
	Subjects     []string // коды предметов
	// VSOSH — «ВсОШ» без предмета: профиль выбирается по предметам ученика.
	VSOSH bool
	// Glossary — вопрос о терминах: БВИ, 100 баллов, уровни перечня.
	Glossary bool
	// MyUniversities — вопрос про вузы ученика: «в моих вузах».
	MyUniversities bool
}

func (m Mentions) Empty() bool {
	return len(m.Olympiads) == 0 && len(m.Universities) == 0 && !m.VSOSH && !m.Glossary
}

// olympiadAliases — как олимпиады называют в разговоре. Кавычечное
// название («Высшая проба») и полное берутся из базы автоматически.
var olympiadAliases = map[string][]string{
	"p669-2":  {"третье тысячелетие"},
	"p669-5":  {"нто", "национальная технологическая"},
	"p669-22": {"иннополис опен", "иннополис open"},
	"p669-37": {"мош", "московская олимпиада"},
	"p669-41": {"омо", "объединенная межвузовская"},
	"p669-43": {"курчатов"},
	"p669-48": {"рггу"},
	"p669-57": {"технокубок"},
	"p669-70": {"росатом"},
	"p669-81": {"турнир городов"},
}

// UniversityAliases — как называют вузы из базы.
var UniversityAliases = map[string][]string{
	"msu":       {"мгу", "московский государственный университет"},
	"spbu":      {"спбгу", "санкт петербургский государственный университет"},
	"hse":       {"вшэ", "вышка", "высшая школа экономики"},
	"mipt":      {"мфти", "физтех"},
	"itmo":      {"итмо"},
	"nsu":       {"нгу", "новосибирский государственный университет"},
	"kfu":       {"кфу", "казанский федеральный", "казанский приволжский федеральный"},
	"innopolis": {"иннополис", "университет иннополис"},
	"sechenov":  {"сеченовский", "сеченовка", "пмгму", "первый мед"},
	"kazan-gmu": {"кгму", "казанский медицинский", "казанский гму"},
}

var subjectStems = map[string][]string{
	"inf":   {"информатик", "программир"},
	"math":  {"математик", "матем"},
	"phys":  {"физик"},
	"chem":  {"хими"},
	"bio":   {"биолог"},
	"soc":   {"обществозн"},
	"econ":  {"экономик"},
	"astro": {"астроном"},
	"ecol":  {"эколог"},
}

var vsoshSubjects = map[string]string{
	"inf": "vsosh-informatika", "math": "vsosh-matematika", "phys": "vsosh-fizika", "chem": "vsosh-himiya",
	"bio": "vsosh-biologiya", "soc": "vsosh-obschestvoznanie", "econ": "vsosh-ekonomika",
	"astro": "vsosh-astronomiya", "ecol": "vsosh-ekologiya",
}

// VSOSHProfile — профиль ВсОШ по коду предмета.
func VSOSHProfile(subject string) (string, bool) {
	id, ok := vsoshSubjects[subject]
	return id, ok
}

var glossaryStems = [][]string{
	{"бви"}, {"без", "вступительн"}, {"100", "балл"}, {"сто", "балл"}, {"стобалльн"},
	{"перечн"}, {"уровн"}, {"льгот"}, {"особ", "прав"}, {"призер"}, {"победител"}, {"подтвержд"}, {"егэ"},
}

// myWords — начала слов «мои вузы», «в моих вузах», «в выбранных вузах».
var myWords = []string{"мой", "мои", "моем", "моег", "выбранн"}

// vsoshWords — разговорные названия ВсОШ, словом целиком: «всерос»
// префиксом поймал бы и «Всероссийскую олимпиаду «Высшая проба»».
// «Всош» склоняется («на всоше») и префиксом ничего лишнего не ловит.
var vsoshWords = []string{"всерос", "всероса", "всеросе", "всеросу", "всеросом", "всеросс"}

// Find ищет упоминания. Длинное совпадение побеждает короткое на тех же
// словах: «высшая школа экономики» — вуз, а не «экономика» как предмет.
func Find(question string, olympiads, universities []Named) Mentions {
	tokens := tokenize(question)
	type hit struct {
		kind       byte // o — олимпиада, u — вуз
		id         string
		start, end int
	}
	var hits []hit
	add := func(kind byte, id string, names []string) {
		for _, name := range names {
			stems := stemAll(tokenize(name))
			for _, at := range matchAt(tokens, stems) {
				hits = append(hits, hit{kind, id, at, at + len(stems)})
			}
		}
	}
	for _, o := range olympiads {
		add('o', o.ID, append(olympiadNames(o.Name), append(o.Aliases, olympiadAliases[o.ID]...)...))
	}
	for _, u := range universities {
		add('u', u.ID, append([]string{u.Name}, append(u.Aliases, UniversityAliases[u.ID]...)...))
	}

	used := make([]bool, len(tokens))
	var m Mentions
	for _, h := range hits {
		shadowed := false
		for _, o := range hits {
			if o.end-o.start > h.end-h.start && o.start < h.end && h.start < o.end {
				shadowed = true
				break
			}
		}
		if shadowed {
			continue
		}
		for i := h.start; i < h.end; i++ {
			used[i] = true
		}
		if h.kind == 'o' && !slices.Contains(m.Olympiads, h.id) {
			m.Olympiads = append(m.Olympiads, h.id)
		}
		if h.kind == 'u' && !slices.Contains(m.Universities, h.id) {
			m.Universities = append(m.Universities, h.id)
		}
	}

	free := make([]string, 0, len(tokens))
	for i, tok := range tokens {
		if !used[i] {
			free = append(free, tok)
		}
	}
	for code, stems := range subjectStems {
		for _, st := range stems {
			if len(matchAt(free, []string{st})) > 0 && !slices.Contains(m.Subjects, code) {
				m.Subjects = append(m.Subjects, code)
			}
		}
	}
	slices.Sort(m.Subjects)
	for _, tok := range free {
		if strings.HasPrefix(tok, "всош") || slices.Contains(vsoshWords, tok) {
			m.VSOSH = true
		}
	}
	// «Всероссийская олимпиада школьников» без названия в кавычках — ВсОШ;
	// с названием — это полное имя олимпиады перечня, оно уже найдено выше.
	if len(m.Olympiads) == 0 && len(matchAt(free, stemAll([]string{"всероссийская", "олимпиада", "школьников"}))) > 0 {
		m.VSOSH = true
	}
	if m.VSOSH {
		for _, code := range m.Subjects {
			if id, ok := vsoshSubjects[code]; ok && !slices.Contains(m.Olympiads, id) {
				m.Olympiads = append(m.Olympiads, id)
			}
		}
	}
	for _, st := range glossaryStems {
		if len(matchAt(free, st)) > 0 {
			m.Glossary = true
		}
	}
	for i := 0; i+1 < len(tokens); i++ {
		next := tokens[i+1]
		if slices.ContainsFunc(myWords, func(w string) bool { return strings.HasPrefix(tokens[i], w) }) &&
			(strings.HasPrefix(next, "вуз") || strings.HasPrefix(next, "универс")) {
			m.MyUniversities = true
		}
	}
	return m
}

// olympiadNames — полное название и все названия в кавычках:
// «Формула Единства»/«Третье тысячелетие» → оба.
func olympiadNames(name string) []string {
	out := []string{name}
	rest := name
	for {
		open := strings.Index(rest, "«")
		if open < 0 {
			return out
		}
		rest = rest[open+len("«"):]
		end := strings.Index(rest, "»")
		if end < 0 {
			return out
		}
		out = append(out, rest[:end])
		rest = rest[end:]
	}
}

// tokenize — слова в нижнем регистре, ё → е, без знаков препинания.
func tokenize(s string) []string {
	s = strings.ReplaceAll(strings.ToLower(s), "ё", "е")
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// stem — грубая основа слова для падежей: «высшей пробы» ищется как
// «высш… проб…». Короткие слова (аббревиатуры) сравниваются целиком.
func stem(w string) string {
	n := utf8.RuneCountInString(w)
	r := []rune(w)
	switch {
	case n >= 6:
		return string(r[:n-2])
	case n == 5:
		return string(r[:n-1])
	}
	return w
}

func stemAll(words []string) []string {
	out := make([]string, len(words))
	for i, w := range words {
		out[i] = stem(w)
	}
	return out
}

// matchAt — позиции, с которых подряд идущие слова начинаются с основ.
// Основа короче четырёх букв — аббревиатура, её слово совпадает целиком.
func matchAt(tokens, stems []string) []int {
	var out []int
	if len(stems) == 0 {
		return nil
	}
	for i := 0; i+len(stems) <= len(tokens); i++ {
		ok := true
		for j, st := range stems {
			tok := tokens[i+j]
			if utf8.RuneCountInString(st) <= 3 {
				ok = tok == st
			} else {
				ok = strings.HasPrefix(tok, st)
			}
			if !ok {
				break
			}
		}
		if ok {
			out = append(out, i)
		}
	}
	return out
}
