package assistant

import (
	"context"
	"strings"
	"testing"
	"time"
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
