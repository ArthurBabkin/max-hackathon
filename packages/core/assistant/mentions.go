// Package assistant — ИИ-помощник (ТЗ §6.4, F35–F37): RAG только по нашей
// базе. Находит в вопросе олимпиады и вузы, собирает их карточки из БД,
// спрашивает модель «только по контексту» и проверяет, что ответ опирается
// на эти карточки. Нет карточек или ссылок на них — шаблон «данных нет».
package assistant

import (
	"regexp"
	"slices"
	"strconv"
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
	// Grade — класс из вопроса («для 7 класса»); 0 — не назван.
	Grade int
	// Directions — направления подготовки из вопроса (FindDirections).
	Directions []string
}

func (m Mentions) Empty() bool {
	return len(m.Olympiads) == 0 && len(m.Universities) == 0 && !m.VSOSH && !m.Glossary
}

// olympiadNicknames — как олимпиады называют в разговоре. По этим названиям
// олимпиаду узнают и поиск по тексту вопроса, и Jev — один список, чтобы
// поиск не отставал. Кавычечное название («Высшая проба») и полное поиск
// берёт из базы сам. Только для распознавания: фактов о льготах здесь нет.
var olympiadNicknames = map[string][]string{
	"p669-2":  {"Третье тысячелетие", "Формула Единства"},
	"p669-4":  {"олимпиада Вернадского"},
	"p669-5":  {"НТО", "Национальная технологическая"},
	"p669-8":  {"олимпиада ВШЭ", "олимпиада Вышки", "проба"},
	"p669-11": {"Сеченовская олимпиада"},
	"p669-12": {"Толстовская олимпиада"},
	"p669-14": {"Всесиб", "Всесибирская"},
	"p669-22": {"Иннополис Опен", "Innopolis Open", "Иннополис Open"},
	"p669-37": {"МОШ", "Московская олимпиада"},
	"p669-41": {"ОММО", "ОМО", "Объединённая межвузовская"},
	"p669-43": {"Курчатов"},
	"p669-48": {"олимпиада РГГУ", "РГГУ"},
	"p669-50": {"Ломоносов", "Ломоносовская олимпиада", "олимпиада МГУ «Ломоносов»"},
	"p669-52": {"ПВГ", "Покори Воробьёвы горы"},
	"p669-54": {"олимпиада Физтеха", "олимпиада МФТИ"},
	"p669-55": {"олимпиада Бауманки", "Шаг в будущее"},
	"p669-57": {"Технокубок"},
	"p669-58": {"олимпиада РАНХиГС"},
	"p669-59": {"олимпиада СПбГУ"},
	"p669-61": {"ЮМШ"},
	"p669-67": {"ОРМО"},
	"p669-70": {"Росатом"},
	"p669-71": {"Пироговская олимпиада"},
	"p669-72": {"Плехановская олимпиада", "олимпиада Плехановки"},
	"p669-81": {"Турнир городов"},
	"p669-82": {"Турнир Ломоносова", "Турнир имени Ломоносова"},
}

// jevOnly — названия, которые по буквам не различить: «олимпиады ВШЭ» — и
// «Высшая проба», и олимпиады, которые принимает ВШЭ; «проба» — и «пробный
// тур». Поиск их пропускает, Jev понимает по смыслу вопроса.
var jevOnly = map[string]bool{
	"олимпиада ВШЭ": true, "олимпиада Вышки": true, "проба": true,
	"олимпиада Физтеха": true, "олимпиада МФТИ": true, "олимпиада СПбГУ": true,
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

// myWords — начала слов «мои вузы», «в моих вузах», «в выбранных вузах»;
// родитель — «в его вузах», «её вузы».
var myWords = []string{"мой", "мои", "моем", "моег", "выбранн", "его", "ее"}

// childWords — «в вузах ребёнка», «вузы сына», «вузы дочери».
var childWords = []string{"ребенк", "сын", "доч"}

// vsoshWords — разговорные названия ВсОШ, словом целиком: «всерос»
// префиксом поймал бы и «Всероссийскую олимпиаду «Высшая проба»».
// «Всош» склоняется («на всоше») и префиксом ничего лишнего не ловит.
var vsoshWords = []string{"всерос", "всероса", "всеросе", "всеросу", "всеросом", "всеросс"}

// gradeRe — «7 класса», «10-м классе», «11 класс»; перед «класс» — один
// номер или перечень: «7–11 классы», «8 и 9 класса».
var gradeRe = regexp.MustCompile(`(\d{1,2}(?:\s*(?:,|и|или|–|-)\s*\d{1,2})*)(?:\s*-?\s*(?:й|го|му|м|ом))?\s*класс`)

// grade — класс из вопроса. Перечень, два разных класса или класс вне 1–11 —
// не класс ученика: 0.
func grade(question string) int {
	found := 0
	for _, m := range gradeRe.FindAllStringSubmatch(question, -1) {
		g, err := strconv.Atoi(m[1])
		if err != nil || g < 1 || g > 11 || (found != 0 && found != g) {
			return 0
		}
		found = g
	}
	return found
}

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
		nicks := slices.DeleteFunc(slices.Clone(olympiadNicknames[o.ID]), func(n string) bool { return jevOnly[n] })
		add('o', o.ID, append(olympiadNames(o.Name), append(o.Aliases, nicks...)...))
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
	m.Grade = grade(question)
	for i := 0; i+1 < len(tokens); i++ {
		next := tokens[i+1]
		mine := slices.ContainsFunc(myWords, func(w string) bool { return strings.HasPrefix(tokens[i], w) }) && isUniversityWord(next)
		child := isUniversityWord(tokens[i]) && slices.ContainsFunc(childWords, func(w string) bool { return strings.HasPrefix(next, w) })
		if mine || child {
			m.MyUniversities = true
		}
	}
	return m
}

func isUniversityWord(w string) bool {
	return strings.HasPrefix(w, "вуз") || strings.HasPrefix(w, "универс")
}

// studentsUniversities — «в вузах Артёма»: слово «вуз» и следом имя ученика
// в любом падеже — основа имени без последней гласной.
func studentsUniversities(question, name string) bool {
	n := tokenize(name)
	if len(n) == 0 {
		return false
	}
	base := []rune(n[0])
	if len(base) > 3 && strings.ContainsRune("аеиоуыэюяйь", base[len(base)-1]) {
		base = base[:len(base)-1]
	}
	tokens := tokenize(question)
	for i := 0; i+1 < len(tokens); i++ {
		if isUniversityWord(tokens[i]) && strings.HasPrefix(tokens[i+1], string(base)) {
			return true
		}
	}
	return false
}

// olympiadNames — полное название, все названия в кавычках
// («Формула Единства»/«Третье тысячелетие» → оба) и первая фраза
// длинного названия.
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
		// «Миссия выполнима. Твое призвание - финансист!» зовут по первой фразе.
		if first, _, ok := strings.Cut(rest[:end], ". "); ok {
			out = append(out, first)
		}
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

// Direction — направление подготовки для поиска в вопросе.
type Direction struct{ ID, Code, Name string }

// directionShort — как сокращают направления. «ПИ» — и программная
// инженерия, и прикладная информатика.
var directionShort = []struct {
	code  string
	short []string
}{
	{"01.03.02", []string{"ПМИ", "ПМиИ"}}, {"09.03.04", []string{"ПИ"}}, {"09.03.03", []string{"ПИ"}},
	{"10.03.01", []string{"ИБ", "инфобез"}}, {"09.03.01", []string{"ИВТ"}}, {"02.03.02", []string{"ФИИТ"}},
	{"03.03.01", []string{"ПМФ", "ПМиФ"}}, {"02.03.03", []string{"МОАИС"}}, {"09.03.02", []string{"ИСиТ"}},
}

// shortOf — сокращения направления по коду.
func shortOf(code string) []string {
	for _, d := range directionShort {
		if d.code == code {
			return d.short
		}
	}
	return nil
}

var directionCodeRe = regexp.MustCompile(`\b\d{2}\.\d{2}\.\d{2}\b`)

// FindDirections — направления из вопроса по коду, сокращению или названию
// в любом падеже, в порядке упоминания. Название из одного слова,
// совпадающее с предметом («математика»), — предмет, не направление.
// Длинное название побеждает короткое на тех же словах, одноимённая
// укрупнённая группа (09.00.00) уступает направлению. Из неоднозначного
// сокращения берутся направления цели ученика goal, если они там есть.
func FindDirections(question string, dirs []Direction, goal []string) []string {
	byCode := map[string]Direction{}
	for _, d := range dirs {
		byCode[d.Code] = d
	}
	type hit struct {
		id         string
		start, end int
	}
	var hits []hit
	// Код — позиция в байтах, остальное — в словах: порядок нужен только
	// относительный, код ставим по числу слов перед ним.
	for _, loc := range directionCodeRe.FindAllStringIndex(question, -1) {
		if d, ok := byCode[question[loc[0]:loc[1]]]; ok {
			at := len(tokenize(question[:loc[0]]))
			hits = append(hits, hit{d.ID, at, at + 3})
		}
	}
	tokens := tokenize(question)
	for i, tok := range tokens {
		var ids []string
		for _, d := range directionShort {
			x, ok := byCode[d.code]
			if ok && slices.ContainsFunc(d.short, func(s string) bool { return strings.Join(tokenize(s), "") == tok }) {
				ids = append(ids, x.ID)
			}
		}
		if mine := slices.DeleteFunc(slices.Clone(ids), func(id string) bool { return !slices.Contains(goal, id) }); len(mine) > 0 {
			ids = mine
		}
		for _, id := range ids {
			hits = append(hits, hit{id, i, i + 1})
		}
	}
	for _, d := range dirs {
		words := tokenize(d.Name)
		if len(words) == 1 && isSubjectWord(words[0]) {
			continue
		}
		stems := stemAll(words)
		for _, at := range matchAt(tokens, stems) {
			hits = append(hits, hit{d.ID, at, at + len(stems)})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return a.start - b.start })
	var out []string
	for _, h := range hits {
		shadowed := slices.ContainsFunc(hits, func(o hit) bool {
			return o.end-o.start > h.end-h.start && o.start < h.end && h.start < o.end
		})
		if !shadowed && !slices.Contains(out, h.id) {
			out = append(out, h.id)
		}
	}
	// Укрупнённая группа с тем же названием, что у найденного направления.
	name := map[string]Direction{}
	for _, d := range dirs {
		name[d.ID] = d
	}
	return slices.DeleteFunc(out, func(id string) bool {
		d := name[id]
		return strings.HasSuffix(d.Code, ".00.00") && slices.ContainsFunc(out, func(o string) bool {
			return o != id && name[o].Name == d.Name
		})
	})
}

// isSubjectWord — слово — название предмета: «математика», «экономике».
func isSubjectWord(w string) bool {
	for _, stems := range subjectStems {
		for _, st := range stems {
			if len(matchAt([]string{w}, []string{st})) > 0 {
				return true
			}
		}
	}
	return false
}
