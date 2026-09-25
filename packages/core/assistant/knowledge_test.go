package assistant

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
)

// contextFor — системное сообщение модели на вопрос.
func contextFor(t *testing.T, question string) string {
	t.Helper()
	st, tr := setup(t)
	f := &fakeLLM{reply: `{"answer": "", "card_ids": [], "no_data": true}`}
	if _, err := (&Assistant{Store: st, LLM: f}).Ask(context.Background(), kid, tr, nil, question); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("модель не вызвана для %q", question)
	}
	return f.calls[0][0].Content
}

// Карточка олимпиады — вся олимпиада, как её лист в приложении: все профили
// с уровнями и классами, этапы, льготы во всех вузах базы, а не только в
// вузах ученика.
func TestKnowledge_OlympiadCardIsWholeOlympiad(t *testing.T) {
	sys := contextFor(t, "Что даёт «Высшая проба»?")
	for _, want := range []string{
		`"id":"olympiad:p669-8"`, "Высшая проба", "организатор НИУ ВШЭ", "перечень", "Информатика — I уровень, 9–11 классы",
		"Экономика", "Регистрация", "ИТМО", "МФТИ", "БВИ",
	} {
		if !strings.Contains(sys, want) {
			t.Errorf("в карточке нет %q", want)
		}
	}
	if strings.Contains(sys, `"id":"olympiad:p669-8-`) {
		t.Error("карточка одна на олимпиаду, не на профиль")
	}
}

// cardIn — текст карточки id из системного сообщения, до следующей карточки.
func cardIn(t *testing.T, sys, id string) string {
	t.Helper()
	i := strings.Index(sys, `"id":"`+id+`"`)
	if i < 0 {
		t.Fatalf("нет карточки %s:\n%s", id, sys)
	}
	c := sys[i:]
	if end := strings.Index(c, `},{`); end > 0 {
		c = c[:end]
	}
	return c
}

// Вуз в вопросе — в карточке олимпиады только его условия: строки соседних
// вузов модель путает (МГУ «победителю и призёру — БВИ» приписала МФТИ).
// Остальные вузы с льготами — списком, без условий.
func TestKnowledge_OlympiadCardOnlyUniversitiesInQuestion(t *testing.T) {
	c := cardIn(t, contextFor(t, "Что даёт «Высшая проба» в ИТМО?"), "olympiad:p669-8")
	if !strings.Contains(c, `\n  ИТМО — `) || strings.Contains(c, `\n  МФТИ — `) {
		t.Errorf("условия — только ИТМО:\n%s", c)
	}
	if !strings.Contains(c, "в других вузах базы") || !strings.Contains(c, "МФТИ") {
		t.Errorf("другие вузы с льготами — списком:\n%s", c)
	}
}

// Вуз из вопроса без льгот по олимпиаде — строка «ничего не даёт», а не
// молчание: иначе модель ищет его условия в строках других вузов.
func TestKnowledge_UniversityInQuestionWithoutBenefit(t *testing.T) {
	c := cardIn(t, contextFor(t, "Что даёт «Высшая проба» в СПбГУ?"), "olympiad:p669-8")
	if !strings.Contains(c, "СПбГУ — по этой олимпиаде ничего не даёт") {
		t.Errorf("вуз без льготы:\n%s", c)
	}
}

// Вуз без олимпиады в вопросе — полная карточка: все олимпиады с льготами.
// С олимпиадой — только шапка: условия вуза уже в карточке олимпиады.
func TestKnowledge_UniversityCardFullOrHeader(t *testing.T) {
	full := contextFor(t, "Какие олимпиады принимает ИТМО?")
	for _, want := range []string{`"id":"university:itmo"`, "Санкт-Петербург", "Innopolis Open", "Высшая проба", "ВсОШ по информатике"} {
		if !strings.Contains(full, want) {
			t.Errorf("в полной карточке вуза нет %q", want)
		}
	}
	both := contextFor(t, "Можно ли поступить в ИТМО по Innopolis Open?")
	i := strings.Index(both, `"id":"university:itmo"`)
	if i < 0 {
		t.Fatal("нет карточки ИТМО")
	}
	uniCard := both[i:]
	if end := strings.Index(uniCard, `},{`); end > 0 {
		uniCard = uniCard[:end]
	}
	if strings.Contains(uniCard, "Высшая проба") {
		t.Errorf("при олимпиаде в вопросе у вуза только шапка:\n%s", uniCard)
	}
}

// Модель знает сегодняшнюю дату и видит, какие этапы уже прошли: иначе на
// «успею ли зарегистрироваться?» не ответить.
func TestKnowledge_TodayAndStageStates(t *testing.T) {
	st, tr := setup(t)
	f := &fakeLLM{reply: `{"answer": "", "card_ids": [], "no_data": true}`}
	now := func() time.Time { return time.Date(2027, 3, 1, 9, 0, 0, 0, time.UTC) }
	if _, err := (&Assistant{Store: st, LLM: f, Now: now}).Ask(context.Background(), kid, tr, nil, "Что даёт «Высшая проба»?"); err != nil {
		t.Fatal(err)
	}
	sys := f.calls[0][0].Content
	if !strings.Contains(sys, "Сегодня: 01.03.2027") || !strings.Contains(sys, "прошёл") {
		t.Fatalf("дата и прошедшие этапы:\n%s", sys)
	}
}

// «Идёт сейчас» — только этап, который уже начался. Ближайший этап, до
// которого ещё месяц, не идёт: иначе модель скажет «регистрация открыта».
func TestKnowledge_StageNotStartedIsNotCurrent(t *testing.T) {
	st, tr := setup(t)
	for day, want := range map[time.Time]bool{
		time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC): false, // до регистрации «Высшей пробы» (с 20 августа)
		time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC): true,  // регистрация идёт
	} {
		f := &fakeLLM{reply: `{"answer": "", "card_ids": [], "no_data": true}`}
		now := func() time.Time { return day }
		if _, err := (&Assistant{Store: st, LLM: f, Now: now}).Ask(context.Background(), kid, tr, nil, "Что даёт «Высшая проба»?"); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(f.calls[0][0].Content, "идёт сейчас"); got != want {
			t.Errorf("%s: «идёт сейчас» %v, ждали %v", day.Format(time.DateOnly), got, want)
		}
	}
}

// У каждого расписания — пометка, фактические даты или примерные (по
// прошлому году): модель должна сказать это в ответе.
func TestKnowledge_DatesFactualOrApproximate(t *testing.T) {
	if sys := contextFor(t, "Когда регистрация на «Высшую пробу»?"); !strings.Contains(sys, "даты фактические: Регистрация") {
		t.Errorf("у «Высшей пробы» даты с сайта олимпиады:\n%s", sys)
	}
	if sys := contextFor(t, "Когда школьный этап ВсОШ по информатике?"); !strings.Contains(sys, "даты примерные, по прошлому году: Школьный этап") {
		t.Errorf("у ВсОШ даты по прошлому году:\n%s", sys)
	}
}

// Оговорка — рядом с условием и словами, которые можно повторить в ответе.
func TestKnowledge_UnverifiedBenefitSaysDataIsBeingChecked(t *testing.T) {
	sys := contextFor(t, "Что даёт «Высшая проба»?")
	if strings.Contains(sys, "[источник не указан]") || !strings.Contains(sys, "(данные уточняются)") {
		t.Fatalf("пометка условия без источника:\n%s", sys)
	}
}

// Системный промпт: кто помощник и где он, кому помогает, запрет выдумывать,
// даты фактические или примерные, класс диплома.
func TestPrompt_RoleAndRules(t *testing.T) {
	sys := contextFor(t, "Что даёт «Высшая проба»?")
	for _, want := range []string{
		"помощник мини-приложения «Траектория» в мессенджере MAX", "школьникам 7–11 классов и их родителям",
		"Не выдумывай", "не пиши «или 100 баллов»", "даты фактические", "даты примерные", "только за определённые классы",
		"«сейчас не идёт» — этап ещё не начался",
	} {
		if !strings.Contains(sys, want) {
			t.Errorf("в промпте нет %q", want)
		}
	}
}

// Этап только с крайним сроком — «до 14.10.2026», как в приложении, а не
// просто дата: иначе «приём заявок 14 октября» читается как день приёма.
func TestSchedule_DeadlineOnlyStage(t *testing.T) {
	deadline := time.Date(2026, 10, 14, 20, 59, 0, 0, time.UTC)
	c := clock{now: time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC), loc: time.UTC}
	got := c.schedule([]stages.Stage{{Kind: "registration", Title: "Приём заявок", DeadlineAt: &deadline}})
	if !strings.HasPrefix(got, "Приём заявок до 14.10.2026") {
		t.Fatalf("этап с крайним сроком: %q", got)
	}
}

// Ближайший этап, который ещё не начался, помечен «сейчас не идёт, начнётся
// …»: без пометки модель видит «Регистрация 05.10–25.10» и пишет «регистрация
// открыта», а «ещё не идёт» читает как «ещё идёт».
func TestSchedule_NearestStageNotStarted(t *testing.T) {
	day := func(m time.Month, d int) *time.Time { x := time.Date(2026, m, d, 9, 0, 0, 0, time.UTC); return &x }
	c := clock{now: *day(9, 25), loc: time.UTC}
	got := c.schedule([]stages.Stage{
		{Kind: "registration", Title: "Регистрация", StartsAt: day(10, 5), EndsAt: day(10, 25)},
		{Kind: "final", Title: "Заключительный этап", StartsAt: day(12, 14), EndsAt: day(12, 16)},
	})
	if want := "Регистрация 05.10.2026–25.10.2026 (сейчас не идёт, начнётся 05.10.2026) → Заключительный этап 14.12.2026–16.12.2026"; got != want {
		t.Fatalf("расписание:\n%q\nждали\n%q", got, want)
	}
}

// «В моих вузах» без предмета — условия по предметам ученика: пересказать
// все профили олимпиады в шести вузах модель не может без ошибок.
func TestKnowledge_MyUniversitiesStudentSubjects(t *testing.T) {
	c := cardIn(t, contextFor(t, "Какие льготы даёт «Высшая проба» в моих вузах?"), "olympiad:p669-8")
	i := strings.Index(c, `\n  ВШЭ — `)
	if i < 0 || !strings.Contains(c, "по предметам ученика: информатика, математика") {
		t.Fatalf("условия по предметам ученика:\n%s", c)
	}
	hse := c[i+3:]
	if end := strings.Index(hse, `\n`); end > 0 {
		hse = hse[:end]
	}
	if !strings.Contains(hse, "Информатика") || strings.Contains(hse, "Экономика") {
		t.Fatalf("строка ВШЭ — только информатика и математика: %s", hse)
	}
}
