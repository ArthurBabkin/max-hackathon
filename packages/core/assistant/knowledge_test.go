package assistant

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
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
	c := cardIn(t, contextFor(t, "Что даёт Плехановская олимпиада в СПбГУ?"), "olympiad:p669-72")
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

// В карточке вуза, и полной, и шапкой, — льготы по уровням перечня. Без
// сводки на «какой минимальный уровень даёт БВИ в Иннополис?» модель
// обобщала по одной олимпиаде из контекста: «за II и III уровень — ничего».
func TestKnowledge_UniversityBenefitsByLevel(t *testing.T) {
	for _, q := range []string{"Какой минимальный уровень олимпиады даёт БВИ в Иннополис?", "Можно ли поступить в Иннополис по ОРМО?"} {
		c := cardIn(t, contextFor(t, q), "university:innopolis")
		for _, want := range []string{"Льготы по уровню олимпиады", "  I уровень: победителю и призёру — БВИ",
			"  II уровень: победителю и призёру — БВИ", "  III уровень: победителю и призёру — БВИ"} {
			if !strings.Contains(c, want) {
				t.Errorf("%s: в карточке Иннополиса нет %q:\n%s", q, want, c)
			}
		}
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
		"что получит победитель и что призёр",
		"не обобщай по отдельным олимпиадам",
		"«Регистрация закрыта» — в этом сезоне в олимпиаду уже не вступить",
		"олимпиады из строки «Только на другие направления вуза» на направления ученика льготы не дают",
		"Если в вопросе названо направление — отвечай про льготы на него",
		"что ещё добавить или что ученику подходит — предлагай олимпиады из карточки «Подбор»",
		"«Участие завершено» — не предлагай ученику следующие этапы этой олимпиады",
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

// Итоги этапов — в карточке ученика: после «не прошёл» помощник не
// должен советовать готовиться к заключительному.
func TestKnowledge_StudentCardHasStageResults(t *testing.T) {
	st, tr := setup(t)
	ctx := context.Background()
	m, _ := st.CurrentMember(ctx, 900000001)
	item, _, _ := st.AddTrackerItem(ctx, tr.ID, "p669-14-biologiya", m.MemberID)
	if _, err := st.SetStageMark(ctx, tr.ID, item, "p669-14-biologiya:qualifying:1", m.MemberID,
		stages.Mark{Result: stages.Failed}, time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	a := &Assistant{Store: st}
	b, err := a.loadBase(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c := collected{clock: a.clock(tr)}
	if err := a.studentCard(ctx, b, tr, &c); err != nil {
		t.Fatal(err)
	}
	card := c.cards[0].text
	if !strings.Contains(card, "отборочный этап — не прошёл") || !strings.Contains(card, "участие завершено") {
		t.Fatalf("итог этапа в карточке ученика:\n%s", card)
	}
}

// Льгота в вузах ученика — на его направления (F65): Innopolis Open по
// информационной безопасности на Программной инженерии ВШЭ даёт БВИ не на
// всех программах (только в Нижнем Новгороде); Иннополис даёт БВИ на
// укрупнённую группу 09.00.00.
func TestKnowledge_TargetBenefits(t *testing.T) {
	st, tr := setup(t)
	ctx := context.Background()
	a := &Assistant{Store: st}
	b, err := a.loadBase(ctx)
	if err != nil {
		t.Fatal(err)
	}
	lines, err := a.targetText(ctx, b, tr.ID, []string{"hse", "innopolis"}, b.profiles["p669-22"])
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(lines, "\n")
	for _, want := range []string{
		"Информационная безопасность:\n    ВШЭ: победителю и призёру — БВИ (направления ученика: Программная инженерия; зависит от программы)",
		"Иннополис: победителю и призёру — БВИ (направления ученика: Информатика и вычислительная техника)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("нет %q:\n%s", want, text)
		}
	}

	// В карточке олимпиады это видно рядом со льготами вузов целиком.
	c := cardIn(t, contextFor(t, "Какие льготы даёт «Высшая проба» в моих вузах?"), "olympiad:p669-8")
	if !strings.Contains(c, "ВШЭ: победителю и призёру — БВИ (направления ученика: Программная инженерия; зависит от программы)") {
		t.Fatalf("льгота на направления ученика в карточке олимпиады:\n%s", c)
	}
}

// В карточке ученика у вуза — направления, на которые он смотрит льготы.
func TestKnowledge_StudentCardHasUniversityDirections(t *testing.T) {
	st, tr := setup(t)
	ctx := context.Background()
	m, _ := st.CurrentMember(ctx, 900000001)
	if err := st.SetUniversityDirections(ctx, tr.ID, "hse", m.MemberID, []string{"napr-01-03-02"}); err != nil {
		t.Fatal(err)
	}
	a := &Assistant{Store: st}
	b, _ := a.loadBase(ctx)
	c := collected{clock: a.clock(tr)}
	tr, _ = st.Trajectory(ctx, tr.ID)
	if err := a.studentCard(ctx, b, tr, &c); err != nil {
		t.Fatal(err)
	}
	card := c.cards[0].text
	// Вузы — теми же короткими названиями, что в строках льгот.
	if !strings.Contains(card, "Вузы: ВШЭ, Иннополис") {
		t.Errorf("названия вузов ученика:\n%s", card)
	}
	if !strings.Contains(card, "ВШЭ: Прикладная математика и информатика (выбрано в вузе)") ||
		!strings.Contains(card, "Иннополис: Информатика и вычислительная техника (по цели)") {
		t.Fatalf("направления в вузах ученика:\n%s", card)
	}
}

// Срок первого этапа прошёл — в карточке прямо сказано, что регистрация
// закрыта, как плашка в приложении: по одним датам модель советовала
// олимпиаду, в которую уже не вступить.
func TestKnowledge_OlympiadCardSaysRegistrationClosed(t *testing.T) {
	ask := func(now time.Time) string {
		st, tr := setup(t)
		f := &fakeLLM{reply: `{"answer": "", "card_ids": [], "no_data": true}`}
		a := &Assistant{Store: st, LLM: f, Now: func() time.Time { return now }}
		if _, err := a.Ask(context.Background(), kid, tr, nil, "Что даёт «Высшая проба»?"); err != nil {
			t.Fatal(err)
		}
		return cardIn(t, f.calls[0][0].Content, "olympiad:p669-8")
	}
	if c := ask(time.Date(2027, 3, 1, 9, 0, 0, 0, time.UTC)); !strings.Contains(c, "Регистрация закрыта") {
		t.Errorf("срок прошёл — «Регистрация закрыта»:\n%s", c)
	}
	if c := ask(time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)); strings.Contains(c, "Регистрация закрыта") {
		t.Errorf("регистрация идёт — не закрыта:\n%s", c)
	}
}

// Трекер в карточке ученика — как в приложении: регистрация закрылась без
// отметки — так и написано (ближайший этап остаётся: вдруг ученик записался
// и не отметил); вторая регистрация видна.
func TestKnowledge_StudentCardMissedAndSecondRegistration(t *testing.T) {
	st, tr := setup(t)
	ctx := context.Background()
	m, _ := st.CurrentMember(ctx, 900000001)
	bio, _, _ := st.AddTrackerItem(ctx, tr.ID, "p669-14-biologiya", m.MemberID)
	_, _, _ = st.AddTrackerItem(ctx, tr.ID, "p669-8-informatika", m.MemberID)
	card := func(now time.Time) string {
		a := &Assistant{Store: st, Now: func() time.Time { return now }}
		b, _ := a.loadBase(ctx)
		c := collected{clock: a.clock(tr)}
		if err := a.studentCard(ctx, b, tr, &c); err != nil {
			t.Fatal(err)
		}
		return c.cards[0].text
	}
	nov := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	text := card(nov)
	line := text[strings.Index(text, "Всесибирская"):]
	line = line[:strings.Index(line, "\n")]
	if !strings.Contains(line, "регистрация закрылась без отметки") {
		t.Fatalf("пропущенная регистрация: %s", line)
	}

	if _, err := st.SetStageMark(ctx, tr.ID, bio, "p669-14-biologiya:qualifying:1", m.MemberID, stages.Mark{Result: stages.Passed}, nov); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetStageMark(ctx, tr.ID, bio, "p669-14-biologiya:registration:2", m.MemberID, stages.Mark{Registered: true}, nov); err != nil {
		t.Fatal(err)
	}
	if text := card(nov); !strings.Contains(text, "регистрация на заключительный этап — отмечена") {
		t.Fatalf("вторая регистрация:\n%s", text)
	}
}

// contextAfter — системное сообщение модели на вопрос после правки
// траектории (выбор направлений в вузе и т. п.).
func contextAfter(t *testing.T, question string, prep func(ctx context.Context, st *store.Store, tr store.Trajectory, member string)) string {
	t.Helper()
	st, tr := setup(t)
	ctx := context.Background()
	m, err := st.CurrentMember(ctx, 900000001)
	if err != nil {
		t.Fatal(err)
	}
	prep(ctx, st, tr, m.MemberID)
	tr, _ = st.Trajectory(ctx, tr.ID)
	f := &fakeLLM{reply: `{"answer": "", "card_ids": [], "no_data": true}`}
	if _, err := (&Assistant{Store: st, LLM: f}).Ask(ctx, kid, tr, nil, question); err != nil {
		t.Fatal(err)
	}
	return f.calls[0][0].Content
}

// Направления вуза — как в его карточке в приложении: код, число программ и
// олимпиад с льготой, «льготы уточняются»; направления ученика помечены, и
// отдельной строкой — на что ему здесь считаются льготы.
func TestKnowledge_UniversityCardDirections(t *testing.T) {
	c := cardIn(t, contextFor(t, "Можно ли поступить в ВШЭ по Высшей пробе?"), "university:hse")
	for _, want := range []string{
		"Направления вуза (19) — программ, олимпиад с льготой:",
		`\n  09.03.04 Программная инженерия — 7 программ, 55 олимпиад с льготой; направление ученика (по цели)\n`,
		`\n    программы: Дизайн и разработка информационны х продуктов; Компьютерные науки и технологии; `,
		`\n  38.03.01 Экономика — 11 программ, `,
		"Льготы ученику здесь считаются на направления: Программная инженерия (по цели)",
	} {
		if !strings.Contains(c, want) {
			t.Errorf("в карточке ВШЭ нет %q:\n%s", want, c)
		}
	}
	if strings.Contains(c, `\nНаправления: `) {
		t.Errorf("старая строка направлений без кодов и льгот:\n%s", c)
	}

	if !strings.Contains(c, `\n  40.03.01 Юриспруденция — 1 программа, льготы уточняются`) {
		t.Errorf("непроверенные льготы на направлении:\n%s", c)
	}
	nsu := cardIn(t, contextFor(t, "Можно ли поступить в НГУ по Высшей пробе?"), "university:nsu")
	if !strings.Contains(nsu, "Ни выбранных в вузе, ни из цели ученика направлений здесь нет — льготы указаны по вузу целиком") {
		t.Errorf("цели ученика в вузе нет:\n%s", nsu)
	}
}

// Полная карточка вуза — олимпиады с льготой на направления ученика, как
// «На мои направления» в приложении; остальные — отдельно, с тем, на скольких
// направлениях вуза они дают льготу: льгота «где-то в вузе» — не льгота на
// направление ученика.
func TestKnowledge_UniversityCardFullOnStudentDirections(t *testing.T) {
	c := cardIn(t, contextFor(t, "Какие олимпиады принимает ВШЭ?"), "university:hse")
	mine := strings.Index(c, "Олимпиады с льготой на направления ученика (Программная инженерия)")
	others := strings.Index(c, "Только на другие направления вуза")
	if mine < 0 || others < mine {
		t.Fatalf("олимпиады на направления ученика и остальные:\n%s", c)
	}
	if i := strings.Index(c, `\n  Высшая проба — `); i < mine || i > others {
		t.Errorf("«Высшая проба» даёт льготу на ПИ:\n%s", c)
	}
	if !strings.Contains(c[others:], "ВсОШ по экономике (на 5 из 19 направлений)") || strings.Contains(c[:others], "ВсОШ по экономике") {
		t.Errorf("ВсОШ по экономике — только на другие направления:\n%s", c)
	}
	if !strings.Contains(c, "Льготы по уровню олимпиады на направления ученика") {
		t.Errorf("сводка по уровням — на направления ученика:\n%s", c)
	}
	// Примечание вуза уже говорит, от чего зависит льгота, — без повтора.
	for _, line := range strings.Split(c, `\n`) {
		if strings.Contains(line, "Зависит от программы:") && strings.Contains(line, "; зависит от программы") {
			t.Errorf("«зависит от программы» дважды:\n%s", line)
			break
		}
	}

	chosen := cardIn(t, contextAfter(t, "Какие олимпиады принимает ВШЭ?", func(ctx context.Context, st *store.Store, tr store.Trajectory, m string) {
		if err := st.SetUniversityDirections(ctx, tr.ID, "hse", m, []string{"napr-01-03-02"}); err != nil {
			t.Fatal(err)
		}
	}), "university:hse")
	for _, want := range []string{
		"Льготы ученику здесь считаются на направления: Прикладная математика и информатика (выбрано в вузе)",
		"; направление ученика (выбрано в вузе)",
		"Олимпиады с льготой на направления ученика (Прикладная математика и информатика)",
	} {
		if !strings.Contains(chosen, want) {
			t.Errorf("выбранное в вузе направление: нет %q:\n%s", want, chosen)
		}
	}
	if strings.Contains(chosen, "(по цели)") {
		t.Errorf("при выборе в вузе цель не помечается:\n%s", chosen)
	}
}

// Льготы на выбранные направления ещё проверяются — так и сказано, а ниже
// льготы вуза целиком с тем, на скольких направлениях они есть.
func TestKnowledge_UniversityCardUnverifiedDirections(t *testing.T) {
	c := cardIn(t, contextAfter(t, "Какие олимпиады принимает ВШЭ?", func(ctx context.Context, st *store.Store, tr store.Trajectory, m string) {
		if err := st.SetUniversityDirections(ctx, tr.ID, "hse", m, []string{"napr-40-03-01"}); err != nil {
			t.Fatal(err)
		}
	}), "university:hse")
	for _, want := range []string{
		"Льготы ученику здесь считаются на направления: Юриспруденция (выбрано в вузе) — льготы на них ещё уточняются",
		"Олимпиады с льготами или баллами в вузе целиком",
		" из 19 направлений)",
	} {
		if !strings.Contains(c, want) {
			t.Errorf("нет %q:\n%s", want, c)
		}
	}
}

// Направление в вопросе — его карточка: в каких вузах базы оно есть, сколько
// олимпиад дают на нём льготу, где льготы уточняются; вузы ученика помечены,
// вузы без направления перечислены — на «есть ли ПМИ в МФТИ?» есть ответ.
func TestKnowledge_DirectionCard(t *testing.T) {
	c := cardIn(t, contextFor(t, "В каких вузах есть ПМИ?"), "direction:napr-01-03-02")
	for _, want := range []string{
		// Сокращение из вопроса — рядом с названием: иначе модель сама
		// расшифровывала «ПМИ» как программную инженерию.
		"Направление 01.03.02 Прикладная математика и информатика (сокращённо: ПМИ, ПМиИ)",
		"Вузы базы с этим направлением (7), олимпиад с льготой на нём:",
		`\n  ВШЭ, Москва — 59 олимпиад с льготой; вуз ученика`,
		`\n  НГУ, Новосибирск — 41 олимпиада с льготой`,
		"Этого направления нет: ",
	} {
		if !strings.Contains(c, want) {
			t.Errorf("в карточке ПМИ нет %q:\n%s", want, c)
		}
	}
	if strings.Contains(c, "в цели ученика") {
		t.Errorf("ПМИ не в цели ученика:\n%s", c)
	}

	pi := cardIn(t, contextFor(t, "Где учат на программную инженерию?"), "direction:napr-09-03-04")
	for _, want := range []string{
		"Направление 09.03.04 Программная инженерия (сокращённо: ПИ) — в цели ученика",
		`Иннополис — как 09.00.00 Информатика и вычислительная техника (укрупнённая группа), `,
	} {
		if !strings.Contains(pi, want) {
			t.Errorf("в карточке ПИ нет %q:\n%s", want, pi)
		}
	}
}

// Направление в вопросе — льготы олимпиады на него, а не на цель ученика
// (ПИ): Innopolis Open по информационной безопасности в ВШЭ на ИБ даёт БВИ.
// Вуз из вопроса без направления — так и сказано.
func TestKnowledge_OlympiadCardOnQuestionDirection(t *testing.T) {
	c := cardIn(t, contextFor(t, "Что даёт Innopolis Open на ИБ в ВШЭ?"), "olympiad:p669-22")
	i := strings.Index(c, "Льгота на направление из вопроса (Информационная безопасность) — главнее льготы вуза целиком:")
	if i < 0 {
		t.Fatalf("льгота на направление из вопроса:\n%s", c)
	}
	sec := c[i:]
	for _, want := range []string{`\n  Информационная безопасность:\n    ВШЭ: победителю и призёру — БВИ, ЕГЭ от 75, диплом за 11 класс`, `\n  Робототехника:\n    ВШЭ: победителю и призёру — 100 баллов`} {
		if !strings.Contains(sec, want) {
			t.Errorf("нет %q:\n%s", want, sec)
		}
	}
	if strings.Contains(c, "направления ученика") {
		t.Errorf("вопрос про ИБ — цель ученика ни при чём:\n%s", c)
	}

	all := cardIn(t, contextFor(t, "Что даёт Innopolis Open на ИБ?"), "olympiad:p669-22")
	sec = all[strings.Index(all, "Льгота на направление из вопроса"):]
	for _, want := range []string{"КФУ: победителю и призёру — БВИ", "ИТМО: льготы нет"} {
		if !strings.Contains(sec, want) {
			t.Errorf("все вузы с ИБ: нет %q:\n%s", want, sec)
		}
	}
	if strings.Contains(sec, "Иннополис:") {
		t.Errorf("в Иннополисе ИБ нет:\n%s", sec)
	}

	// На ПИ в ВШЭ «Высшая проба» по информатике — БВИ на всех программах
	// (в том числе Пермь и Петербург), по анализу данных — не на всех. БВИ —
	// за диплом 10–11 класса, за 9 класс — 100 баллов (Нижний Новгород, #78).
	pi := cardIn(t, contextFor(t, "Что даёт Высшая проба по информатике на ПИ в ВШЭ?"), "olympiad:p669-8")
	varies := regexp.MustCompile(`Анализ данных:\\n    ВШЭ: победителю и призёру — БВИ[^\\]*\(зависит от программы\)`)
	everywhere := regexp.MustCompile(`Информатика:\\n    ВШЭ: победителю и призёру — БВИ, ЕГЭ от 75, диплом за 10–11 класс; за диплом 9 класса — 100 баллов\\n`)
	if sec := pi[strings.Index(pi, "Льгота на направление из вопроса"):]; !varies.MatchString(sec) || !everywhere.MatchString(sec) {
		t.Errorf("зависит от программы:\n%s", sec)
	}

	// Предмет из вопроса важнее предметов ученика (информатика, математика).
	econ := cardIn(t, contextFor(t, "Что даёт Высшая проба по экономике на ПМИ в ВШЭ?"), "olympiad:p669-8")
	if sec := econ[strings.Index(econ, "Льгота на направление из вопроса"):]; strings.Contains(sec, "Информатика") || !strings.Contains(sec, "ВШЭ: ") {
		t.Errorf("профили по экономике:\n%s", sec)
	}

	mipt := cardIn(t, contextFor(t, "Что даёт Innopolis Open на ИБ в МФТИ?"), "olympiad:p669-22")
	if !strings.Contains(mipt, "Нет этого направления: МФТИ") {
		t.Errorf("вуз из вопроса без направления:\n%s", mipt)
	}
}

// Направление в вопросе — карточка вуза про него: направление помечено,
// олимпиады — с льготой на него. Нет направления в вузе — только шапка.
func TestKnowledge_UniversityCardOnQuestionDirection(t *testing.T) {
	c := cardIn(t, contextFor(t, "Какие олимпиады дают льготу на ИБ в ВШЭ?"), "university:hse")
	for _, want := range []string{
		"Льготы здесь считаются на направление из вопроса: Информационная безопасность.",
		`\n  10.03.01 Информационная безопасность — 1 программа, 50 олимпиад с льготой; направление из вопроса`,
		"Льготы по уровню олимпиады на направление из вопроса",
		"Олимпиады с льготой на направление из вопроса (Информационная безопасность)",
		"Только на другие направления вуза, не на направление из вопроса",
	} {
		if !strings.Contains(c, want) {
			t.Errorf("нет %q:\n%s", want, c)
		}
	}
	if strings.Contains(c, "Льготы ученику здесь считаются") || strings.Contains(c, "направление ученика") {
		t.Errorf("вопрос про ИБ — цель ученика ни при чём:\n%s", c)
	}

	itmo := cardIn(t, contextFor(t, "Есть ли лечебное дело в ИТМО?"), "university:itmo")
	if !strings.Contains(itmo, "Направления из вопроса (Лечебное дело) в этом вузе нет.") {
		t.Errorf("направления нет:\n%s", itmo)
	}
	if strings.Contains(itmo, "Олимпиады с льгот") || strings.Contains(itmo, "Льготы по уровню") {
		t.Errorf("без направления — только шапка:\n%s", itmo)
	}
}

// Олимпиада в трекере — её карточка говорит, что там отмечено, как лист
// олимпиады в приложении: на «что у ребёнка с Высшей пробой?» Jev видит
// вопрос о льготах, карточки ученика нет, и модель советовала отбор после
// «не прошёл».
func TestKnowledge_OlympiadCardHasTrackerStatus(t *testing.T) {
	c := cardIn(t, contextAfter(t, "Что у меня с Высшей пробой?", func(ctx context.Context, st *store.Store, tr store.Trajectory, m string) {
		item, _, err := st.AddTrackerItem(ctx, tr.ID, "p669-8-informatika", m)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.SetStageMark(ctx, tr.ID, item, "p669-8-informatika:qualifying:1", m,
			stages.Mark{Result: stages.Failed}, time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}), "olympiad:p669-8")
	// «Участие завершено» — и следующие этапы уже не для ученика: иначе
	// модель после «не прошёл» предлагала 2 тур.
	if want := "В трекере ученика: Информатика (регистрация отмечена); отборочный этап, 1 тур — не прошёл; участие завершено: следующие этапы уже не для ученика"; !strings.Contains(c, want) {
		t.Fatalf("нет %q:\n%s", want, c)
	}
	if c := cardIn(t, contextFor(t, "Что даёт «Высшая проба»?"), "olympiad:p669-8"); strings.Contains(c, "В трекере ученика") {
		t.Fatalf("олимпиады нет в трекере:\n%s", c)
	}
}
