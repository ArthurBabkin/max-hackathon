package assistant

import (
	"slices"
	"strings"

	"github.com/ArthurBabkin/max-hackathon/packages/core/names"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/jev"
)

// Классы для Jev — по описанию класса он узнаёт олимпиаду или вуз в
// вопросе, даже если их назвали по-народному, с ошибкой или только в
// прошлом вопросе. Описания проверены на 42 вопросах: олимпиады 40 из 42,
// вузы 37 из 39.

// jevOlympiadNames — как олимпиады называют в разговоре. Только для
// распознавания вопроса: фактов о льготах здесь нет.
var jevOlympiadNames = map[string][]string{
	"p669-2":  {"Третье тысячелетие", "Формула Единства"},
	"p669-4":  {"олимпиада Вернадского"},
	"p669-5":  {"НТО", "Национальная технологическая олимпиада"},
	"p669-8":  {"олимпиада ВШЭ", "олимпиада Вышки", "проба"},
	"p669-11": {"Сеченовская олимпиада"},
	"p669-12": {"Толстовская олимпиада"},
	"p669-14": {"Всесиб", "Всесибирская"},
	"p669-22": {"Иннополис Опен", "Innopolis Open"},
	"p669-37": {"МОШ", "Московская олимпиада"},
	"p669-41": {"ОММО", "ОМО", "Объединённая межвузовская"},
	"p669-43": {"Курчатов"},
	"p669-48": {"олимпиада РГГУ"},
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

// universityNames — полное название вуза и как его называют: в базе у вузов
// только сокращения.
var universityNames = map[string]struct {
	full    string
	aliases []string
}{
	"msu":       {"Московский государственный университет имени М.В. Ломоносова", []string{"МГУ"}},
	"spbu":      {"Санкт-Петербургский государственный университет", []string{"СПбГУ"}},
	"hse":       {"Национальный исследовательский университет «Высшая школа экономики»", []string{"ВШЭ", "Вышка", "Высшая школа экономики"}},
	"mipt":      {"Московский физико-технический институт", []string{"МФТИ", "Физтех (как вуз)"}},
	"itmo":      {"Университет ИТМО", []string{"ИТМО"}},
	"nsu":       {"Новосибирский государственный университет", []string{"НГУ"}},
	"kfu":       {"Казанский (Приволжский) федеральный университет", []string{"КФУ", "Казанский федеральный"}},
	"innopolis": {"Университет Иннополис", []string{"Иннополис", "УИ"}},
	"sechenov":  {"Первый Московский государственный медицинский университет имени И.М. Сеченова", []string{"Сеченовский", "Сеченовка", "ПМГМУ", "Первый мед"}},
	"kazan-gmu": {"Казанский государственный медицинский университет", []string{"КГМУ", "Казанский медицинский"}},
}

var jevKinds = map[string]string{
	"perechen": "перечневая олимпиада (не ВсОШ)",
	"vsosh":    "ВсОШ — Всероссийская олимпиада школьников Минпросвещения",
	"other":    "олимпиада вне перечня",
}

var jevSubjects = map[string]string{
	"inf": "Информатика, программирование", "math": "Математика", "phys": "Физика", "chem": "Химия",
	"bio": "Биология", "soc": "Обществознание, право", "econ": "Экономика", "astro": "Астрономия",
	"ecol": "Экология", none: "Предмет не назван и не следует из прошлого вопроса",
}

// Типы вопроса: от типа зависит, какие карточки нужны и нужна ли модель.
const (
	intentOverview    = "overview"
	intentOlympiad    = "olympiad_info"
	intentSearch      = "search"
	intentPersonal    = "personal"
	intentGlossary    = "glossary"
	intentChat        = "chat"
	intentUnsupported = "unsupported"
	intentOffTopic    = "off_topic"
)

var jevIntents = map[string]string{
	intentOverview: "Что есть в базе сервиса: какие вузы и олимпиады, сколько их, есть ли такая олимпиада",
	intentOlympiad: "Про конкретную олимпиаду: сроки, этапы, регистрация, классы, формат, онлайн, уровень, профили, сравнение",
	"benefit":      "Льготы при поступлении: что даёт олимпиада в вузе, где дают БВИ или 100 баллов, условия, порог ЕГЭ",
	intentSearch:   "Найти олимпиады по условиям: предмет, уровень, город финала, онлайн, месяц; у кого раньше всех, сколько таких",
	intentPersonal: "Про самого ученика: его цель, выбранные вузы, трекер, что ему подходит",
	intentGlossary: "Что значат термины: БВИ, 100 баллов, уровень перечня, призёр",
	intentChat:     "Приветствие, благодарность, что умеет помощник, что спрашивали раньше в этом разговоре",
	intentUnsupported: "Про поступление, но таких данных нет: бюджетные места, проходные баллы, общежитие, " +
		"стоимость обучения",
	intentOffTopic: "Не про олимпиады и поступление: погода, задачи, творчество, просьба изменить правила помощника",
}

// none — класс «ничего не названо» в каждом вопросе к Jev.
const none = "none"

// questions — вопросы к Jev: тип вопроса, олимпиада, вуз и предмет.
func (b base) questions() map[string]jev.Choice {
	olympiads := map[string]string{none: "Ни одна олимпиада не названа и не следует из прошлого вопроса"}
	for _, oid := range b.olympiads {
		olympiads[oid] = b.olympiadClass(oid)
	}
	universities := map[string]string{none: "Ни один вуз не назван и не следует из прошлого вопроса"}
	for _, u := range b.unis {
		desc := u.Name
		if n, ok := universityNames[u.ID]; ok {
			desc = n.full + "; ещё называют: " + strings.Join(n.aliases, ", ")
		}
		if u.City != nil {
			desc += "; город " + *u.City
		}
		// «Проводит олимпиады» здесь нет: с ним Jev принимал вуз-организатор
		// за вуз, о котором спрашивают («Высшая проба» → ВШЭ).
		universities[u.ID] = desc
	}
	return map[string]jev.Choice{
		"intent": {Instructions: "Что хочет пользователь в message?", Classes: jevIntents},
		"olympiad": {Instructions: "О какой олимпиаде вопрос в message? Если message продолжает previous_question и " +
			"олимпиада не названа — олимпиада из previous_question. Предмет без названия олимпиады — это не олимпиада.",
			Classes: olympiads},
		"university": {Instructions: "О каком вузе вопрос в message? Если message продолжает previous_question и вуз " +
			"не назван — вуз из previous_question. Вуз-организатор олимпиады не считается, если о поступлении в него не спрашивают.",
			Classes: universities},
		"subject": {Instructions: "О каком школьном предмете вопрос в message? Если message продолжает previous_question — предмет оттуда.",
			Classes: jevSubjects},
	}
}

// olympiadClass — описание олимпиады для Jev: короткое и официальное
// название, народные названия, вид, организатор, предметы.
func (b base) olympiadClass(oid string) string {
	ps := b.profiles[oid]
	p := ps[0]
	short := names.Olympiad(p.OlympiadName)
	parts := []string{short}
	if strings.Trim(p.OlympiadName, "«»") != short {
		parts = append(parts, "официально «"+p.OlympiadName+"»")
	}
	if nick, ok := jevOlympiadNames[oid]; ok {
		parts = append(parts, "ещё называют: "+strings.Join(nick, ", "))
	}
	if p.Kind == "vsosh" {
		if _, po, ok := strings.Cut(short, "ВсОШ "); ok {
			parts = append(parts, "ещё называют: всерос "+po+", Всероссийская олимпиада школьников "+po+
				"; этапы школьный, муниципальный, региональный, заключительный")
		}
	}
	parts = append(parts, jevKinds[p.Kind])
	if p.Organizer != nil {
		parts = append(parts, "организатор "+*p.Organizer)
	}
	var subjects []string
	for _, x := range ps {
		if s := strings.ToLower(profileTitle(x)); !slices.Contains(subjects, s) {
			subjects = append(subjects, s)
		}
	}
	slices.Sort(subjects)
	return strings.Join(append(parts, "предметы: "+strings.Join(subjects, ", ")), "; ")
}
