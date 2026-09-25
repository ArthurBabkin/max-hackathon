package assistant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/jev"
)

// fakeJev — классификатор с заготовленными вероятностями; запоминает, что
// ему прислали.
type fakeJev struct {
	probs     map[string]map[string]float64
	err       error
	states    []map[string]string
	questions []map[string]jev.Choice
}

func (f *fakeJev) Classify(_ context.Context, state map[string]string, q map[string]jev.Choice) (map[string]map[string]float64, error) {
	f.states = append(f.states, state)
	f.questions = append(f.questions, q)
	return f.probs, f.err
}

// olympiadsIn — сколько олимпиад с профилями в базе.
func olympiadsIn(t *testing.T, st *store.Store) int {
	t.Helper()
	ps, err := st.Profiles(context.Background(), store.ProfileQuery{})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, p := range ps {
		ids[p.OlympiadID] = true
	}
	return len(ids)
}

// said — ответ Jev: тип вопроса и вероятности олимпиад и вузов.
func said(intent string, olympiads, universities map[string]float64) *fakeJev {
	return &fakeJev{probs: map[string]map[string]float64{
		"intent": {intent: 1}, "olympiad": olympiads, "university": universities, "subject": {"none": 1},
	}}
}

// Олимпиада берётся от Jev с вероятности 0,15, вуз — с 0,25; ВсОШ — только
// если её назвали словами (поиск по названиям), Jev её путает с предметом.
func TestAsk_JevThresholds(t *testing.T) {
	st, tr := setup(t)
	cases := []struct {
		name       string
		olympiads  map[string]float64
		unis       map[string]float64
		wantCard   string // "" — карточек нет, отказ без модели
		wantCalled bool
	}{
		{"олимпиада ниже порога", map[string]float64{"p669-8": 0.14, none: 0.86}, map[string]float64{none: 1}, "", false},
		{"олимпиада на пороге", map[string]float64{"p669-8": 0.15, none: 0.85}, map[string]float64{none: 1}, `"id":"olympiad:p669-8"`, true},
		{"вуз ниже порога", map[string]float64{none: 1}, map[string]float64{"hse": 0.24, none: 0.76}, "", false},
		{"вуз на пороге", map[string]float64{none: 1}, map[string]float64{"hse": 0.25, none: 0.75}, `"id":"university:hse"`, true},
		{"ВсОШ от Jev не берём", map[string]float64{"vsosh-informatika": 0.9, none: 0.1}, map[string]float64{none: 1}, "", false},
		// ВсОШ отброшена — её вероятность делится между остальными: 0,12 / (1 − 0,3) ≈ 0,17.
		{"без ВсОШ — перенормировка", map[string]float64{"p669-8": 0.12, "vsosh-informatika": 0.3, none: 0.58}, map[string]float64{none: 1}, `"id":"olympiad:p669-8"`, true},
		// Вопрос про ВсОШ: соседи по 0,01 после деления стали бы по 0,5 — но
		// случайная доля остаётся случайной.
		{"почти наверняка ВсОШ — соседи не раздуваются", map[string]float64{"vsosh-matematika": 0.98, "p669-50": 0.01, "p669-52": 0.01}, map[string]float64{none: 1}, "", false},
	}
	for _, c := range cases {
		f := &fakeLLM{reply: `{"answer": "", "card_ids": [], "no_data": true}`}
		a := &Assistant{Store: st, LLM: f, Classifier: said("olympiad_info", c.olympiads, c.unis)}
		if _, err := a.Ask(context.Background(), kid, tr, nil, "когда регистрация на эту олимпиаду?"); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if (len(f.calls) == 1) != c.wantCalled {
			t.Errorf("%s: вызовов модели %d", c.name, len(f.calls))
			continue
		}
		if c.wantCard != "" && !strings.Contains(f.calls[0][0].Content, c.wantCard) {
			t.Errorf("%s: в контексте нет %s", c.name, c.wantCard)
		}
	}
}

// Вопросы, на которые в базе ответа нет, и вопросы не по теме — сразу отказ,
// без модели; первоисточник — правила названного вуза.
func TestAsk_JevNoDataIntentsRefuseWithoutModel(t *testing.T) {
	st, tr := setup(t)
	for intent, q := range map[string]string{"unsupported": "Сколько бюджетных мест в ВШЭ?", "off_topic": "Какая завтра погода в ВШЭ?"} {
		f := &fakeLLM{reply: `{"answer": "Много", "card_ids": ["university:hse"], "no_data": false}`}
		a := &Assistant{Store: st, LLM: f, Classifier: said(intent, map[string]float64{none: 1}, map[string]float64{"hse": 0.9, none: 0.1})}
		ans, err := a.Ask(context.Background(), kid, tr, nil, q)
		if err != nil {
			t.Fatal(err)
		}
		if !ans.Refused || len(f.calls) != 0 || len(ans.Sources) == 0 || ans.Sources[0].ID != "rules-hse" {
			t.Errorf("%s: ждали отказ без модели с правилами ВШЭ: %+v, вызовов %d", intent, ans, len(f.calls))
		}
	}
}

// Приветствие, благодарность, «что ты умеешь», «что я спрашивал» — модель
// отвечает без карточек.
func TestAsk_JevChatAnswersWithoutCards(t *testing.T) {
	st, tr := setup(t)
	f := &fakeLLM{reply: `{"answer": "Привет! Я отвечаю про олимпиады и льготы при поступлении.", "card_ids": [], "no_data": false}`}
	a := &Assistant{Store: st, LLM: f, Classifier: said("chat", map[string]float64{none: 1}, map[string]float64{none: 1})}
	ans, err := a.Ask(context.Background(), kid, tr, nil, "Привет! Что ты умеешь?")
	if err != nil {
		t.Fatal(err)
	}
	if ans.Refused || len(ans.CardRefs) != 0 || !strings.HasPrefix(ans.Text, "Привет") {
		t.Fatalf("болтовня без карточек: %+v", ans)
	}
	// Без Jev тот же ответ без ссылок не принимается: тип вопроса неизвестен.
	ans, _ = (&Assistant{Store: st, LLM: f}).Ask(context.Background(), kid, tr, nil, "Привет! Что ты умеешь?")
	if !ans.Refused {
		t.Fatalf("без Jev — отказ: %+v", ans)
	}
}

// «Какие вузы у тебя в базе?», «Сколько олимпиад?» — каталог базы.
func TestAsk_JevOverviewGetsCatalog(t *testing.T) {
	st, tr := setup(t)
	f := &fakeLLM{reply: `{"answer": "В базе 10 вузов.", "card_ids": ["catalog"], "no_data": false}`}
	a := &Assistant{Store: st, LLM: f, Classifier: said("overview", map[string]float64{none: 1}, map[string]float64{none: 1})}
	ans, err := a.Ask(context.Background(), kid, tr, nil, "Какие вузы у тебя в базе?")
	if err != nil {
		t.Fatal(err)
	}
	sys := f.calls[0][0].Content
	for _, want := range []string{`"id":"catalog"`, "НИУ ВШЭ", "Высшая проба", fmt.Sprintf("Олимпиад в базе: %d", olympiadsIn(t, st))} {
		if !strings.Contains(sys, want) {
			t.Fatalf("в каталоге нет %s:\n%s", want, sys)
		}
	}
	if strings.Contains(sys, "ИТМО (ИТМО)") {
		t.Fatal("сокращение в скобках — только если оно отличается от названия")
	}
	if ans.Refused || len(ans.Sources) == 0 || ans.Sources[0].Kind != "order" {
		t.Fatalf("ответ по каталогу со ссылкой на перечень: %+v", ans)
	}
}

// «Какая у меня цель?» — карточка ученика: класс, предметы, цель, вузы, трекер.
func TestAsk_JevPersonalGetsStudentCard(t *testing.T) {
	st, tr := setup(t)
	f := &fakeLLM{reply: `{"answer": "Твоя цель — программная инженерия.", "card_ids": ["student"], "no_data": false}`}
	a := &Assistant{Store: st, LLM: f, Classifier: said("personal", map[string]float64{none: 1}, map[string]float64{none: 1})}
	ans, err := a.Ask(context.Background(), kid, tr, nil, "Какая у меня цель?")
	if err != nil {
		t.Fatal(err)
	}
	sys := f.calls[0][0].Content
	if !strings.Contains(sys, `"id":"student"`) || !strings.Contains(sys, "Программная инженерия") {
		t.Fatalf("карточка ученика:\n%s", sys)
	}
	if ans.Refused || ans.Text != "Твоя цель — программная инженерия." {
		t.Fatalf("ответ по карточке ученика: %+v", ans)
	}
}

// Jev видит прошлый вопрос: «а когда у неё регистрация?» без него не понять.
func TestAsk_JevGetsPreviousQuestion(t *testing.T) {
	st, tr := setup(t)
	j := said("olympiad_info", map[string]float64{"p669-70": 0.9, none: 0.1}, map[string]float64{none: 1})
	f := &fakeLLM{reply: `{"answer": "", "card_ids": [], "no_data": true}`}
	h := history("Что даёт олимпиада «Росатом»?", "Данных нет.")
	if _, err := (&Assistant{Store: st, LLM: f, Classifier: j}).Ask(context.Background(), kid, tr, h, "А когда у неё регистрация?"); err != nil {
		t.Fatal(err)
	}
	if len(j.states) != 1 || j.states[0]["previous_question"] != "Что даёт олимпиада «Росатом»?" ||
		j.states[0]["message"] != "А когда у неё регистрация?" {
		t.Fatalf("состояние для Jev: %+v", j.states)
	}
	if len(f.calls) != 1 || !strings.Contains(f.calls[0][0].Content, `"id":"olympiad:p669-70"`) {
		t.Fatal("карточка «Росатома» по пониманию Jev")
	}
	// В классах — все олимпиады базы и «ничего».
	if n, want := len(j.questions[0]["olympiad"].Classes), olympiadsIn(t, st)+1; n != want {
		t.Fatalf("классов олимпиад %d, ждали %d", n, want)
	}
}

// Jev недоступен — отвечаем по поиску названий, как без него.
func TestAsk_JevErrorFallsBackToNames(t *testing.T) {
	st, tr := setup(t)
	f := &fakeLLM{reply: `{"answer": "", "card_ids": [], "no_data": true}`}
	a := &Assistant{Store: st, LLM: f, Classifier: &fakeJev{err: errors.New("timeout")}}
	if _, err := a.Ask(context.Background(), kid, tr, nil, "Что даёт «Высшая проба»?"); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || !strings.Contains(f.calls[0][0].Content, `"id":"olympiad:p669-8"`) {
		t.Fatal("карточка по названию из вопроса")
	}
}

// «В моих вузах» — карточки вузов ученика.
func TestAsk_MyUniversitiesCards(t *testing.T) {
	st, tr := setup(t)
	f := &fakeLLM{reply: `{"answer": "", "card_ids": [], "no_data": true}`}
	if _, err := (&Assistant{Store: st, LLM: f}).Ask(context.Background(), kid, tr, nil, "Где в моих вузах дают БВИ по информатике?"); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 {
		t.Fatal("модель должна быть вызвана")
	}
	for _, id := range []string{"hse", "innopolis"} {
		if !strings.Contains(f.calls[0][0].Content, `"id":"university:`+id+`"`) {
			t.Errorf("нет карточки вуза ученика %s", id)
		}
	}
}

// Порядок карточек: названное в вопросе, затем то, на что опирался прошлый
// ответ, и только потом догадки Jev. «А в ИТМО она что даёт?» — «она» из
// прошлого ответа, а олимпиада ИТМО — догадка по названию вуза.
func TestAsk_CarriedCardsBeforeJevGuesses(t *testing.T) {
	st, tr := setup(t)
	j := said("benefit", map[string]float64{"p669-22": 0.6, "p669-8": 0.3, none: 0.1}, map[string]float64{"itmo": 0.9, none: 0.1})
	f := &fakeLLM{reply: `{"answer": "", "card_ids": [], "no_data": true}`}
	h := history("когда регистрация на вышку по инфе", "Регистрация на «Высшую пробу» прошла.",
		store.AiCardRef{Type: "olympiad", ID: "p669-8-informatika", Title: "Высшая проба, информатика"})
	if _, err := (&Assistant{Store: st, LLM: f, Classifier: j}).Ask(context.Background(), kid, tr, h, "а в ИТМО она что даёт?"); err != nil {
		t.Fatal(err)
	}
	sys := f.calls[0][0].Content
	carried, guessed := strings.Index(sys, `"id":"olympiad:p669-8"`), strings.Index(sys, `"id":"olympiad:p669-22"`)
	if carried < 0 || guessed < 0 || carried > guessed {
		t.Fatalf("карточка прошлого ответа — раньше догадки Jev: %d, %d", carried, guessed)
	}
}

// Вуз из прошлого ответа, названный и в новом вопросе, — полной карточкой,
// а не перенесённой шапкой.
func TestAsk_UniversityInQuestionBeatsCarriedHeader(t *testing.T) {
	st, tr := setup(t)
	f := &fakeLLM{reply: `{"answer": "", "card_ids": [], "no_data": true}`}
	h := history("Расскажи про ИТМО", "ИТМО — вуз в Санкт-Петербурге.", uni("itmo"))
	if _, err := (&Assistant{Store: st, LLM: f}).Ask(context.Background(), kid, tr, h, "Какие олимпиады принимает ИТМО?"); err != nil {
		t.Fatal(err)
	}
	if sys := f.calls[0][0].Content; !strings.Contains(sys, "Innopolis Open") {
		t.Fatalf("полная карточка ИТМО:\n%s", sys)
	}
}

// Каталог сгруппирован по предмету и уровню: «олимпиады по физике I уровня»
// — одна строка, а не поиск по 75 олимпиадам.
func TestAsk_CatalogBySubjectAndLevel(t *testing.T) {
	st, tr := setup(t)
	f := &fakeLLM{reply: `{"answer": "", "card_ids": [], "no_data": true}`}
	a := &Assistant{Store: st, LLM: f, Classifier: said("search", map[string]float64{none: 1}, map[string]float64{none: 1})}
	if _, err := a.Ask(context.Background(), kid, tr, nil, "Какие олимпиады по физике I уровня есть в базе?"); err != nil {
		t.Fatal(err)
	}
	var line string
	for _, l := range strings.Split(f.calls[0][0].Content, `\n`) {
		if strings.HasPrefix(strings.TrimSpace(l), "Физика, I уровень:") {
			line = l
		}
	}
	if !strings.Contains(line, "Физтех") || !strings.Contains(line, "Росатом") || strings.Contains(line, "Турнир городов") {
		t.Fatalf("строка «Физика, I уровень»: %q", line)
	}
}

// Под ответом — правила вузов из вопроса, а не первых попавшихся.
func TestAsk_SourcesOfUniversitiesInQuestion(t *testing.T) {
	st, tr := setup(t)
	f := &fakeLLM{reply: `{"answer": "Победителю — БВИ, призёру — 100 баллов.", "card_ids": ["olympiad:p669-57", "university:mipt"], "no_data": false}`}
	ans, err := (&Assistant{Store: st, LLM: f}).Ask(context.Background(), kid, tr, nil, "Даёт ли МФТИ БВИ за Технокубок?")
	if err != nil || ans.Refused {
		t.Fatalf("%+v %v", ans, err)
	}
	mipt := false
	for _, s := range ans.Sources {
		if s.Kind == "site" || s.Kind == "order" {
			continue
		}
		if !strings.HasPrefix(s.Title, "МФТИ") {
			t.Errorf("источник не про МФТИ: %s", s.Title)
		}
		mipt = true
	}
	if !mipt {
		t.Fatalf("нет правил МФТИ: %+v", ans.Sources)
	}
}

// Кнопка вуза — привычное название, а не аббревиатура из справочника.
func TestAsk_UniversityButtonUsesNick(t *testing.T) {
	st, tr := setup(t)
	f := &fakeLLM{reply: `{"answer": "Иннополис — вуз в Татарстане.", "card_ids": ["university:innopolis"], "no_data": false}`}
	ans, err := (&Assistant{Store: st, LLM: f}).Ask(context.Background(), kid, tr, nil, "Расскажи про Иннополис")
	if err != nil || len(ans.CardRefs) != 1 || ans.CardRefs[0].Title != "Иннополис" {
		t.Fatalf("кнопка: %+v %v", ans.CardRefs, err)
	}
}

// Поиск по предмету — каталог только этого предмета, и у каждой олимпиады её
// профили по нему с уровнем и классами. Иначе классы приходится сводить из
// двух списков по всей базе, и модель ошибается: «Бельчонок» (8–11) назвала
// олимпиадой для 7 класса, а ВсОШ (с 5 класса) пропустила.
func TestAsk_CatalogOfAskedSubject(t *testing.T) {
	st, tr := setup(t)
	f := &fakeLLM{reply: `{"answer": "", "card_ids": [], "no_data": true}`}
	a := &Assistant{Store: st, LLM: f, Classifier: said("search", map[string]float64{none: 1}, map[string]float64{none: 1})}
	if _, err := a.Ask(context.Background(), kid, tr, nil, "Какие олимпиады по информатике есть для 7 класса?"); err != nil {
		t.Fatal(err)
	}
	c := cardIn(t, f.calls[0][0].Content, "catalog")
	line := func(prefix string) string {
		for _, l := range strings.Split(c, `\n`) {
			if strings.HasPrefix(strings.TrimSpace(l), prefix) {
				return l
			}
		}
		return ""
	}
	if l := line("Бельчонок —"); !strings.Contains(l, "информатика (II уровень, 8–11 классы)") || strings.Contains(l, "химия") {
		t.Errorf("строка «Бельчонок» — только информатика с классами: %q", l)
	}
	if l := line("ВсОШ по информатике —"); !strings.Contains(l, "5–11 классы") {
		t.Errorf("строка ВсОШ по информатике: %q", l)
	}
	if !strings.Contains(c, "Информатика, I уровень:") || strings.Contains(c, "Потомки Менделеева") || strings.Contains(c, "Химия, ") {
		t.Errorf("каталог — только информатика:\n%s", c)
	}
}
