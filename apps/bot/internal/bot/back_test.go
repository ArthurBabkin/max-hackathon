package bot

import (
	"slices"
	"strings"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi/maxtest"
)

// tap нажимает кнопку под сообщением msg и возвращает то, чем бот заменил
// это сообщение: «Назад» правит вопрос на месте, нового сообщения нет.
func (h *harness) tap(u maxapi.User, msg maxapi.NewMessage, payload string) maxapi.NewMessage {
	h.t.Helper()
	if !slices.Contains(maxtest.Payloads(msg), payload) {
		h.t.Fatalf("под сообщением %q нет кнопки %s: %v", msg.Text, payload, maxtest.Payloads(msg))
	}
	a := h.answered(h.pressAny(u, payload, msg.Text))
	if a.Message == nil {
		h.t.Fatalf("%s: вопрос не заменён: %+v", payload, a)
	}
	return *a.Message
}

func hasBack(m maxapi.NewMessage) bool {
	return slices.ContainsFunc(maxtest.Payloads(m), func(p string) bool { return strings.HasPrefix(p, "back:") })
}

// «Назад» доходит до первого вопроса, ответы остаются отмеченными, и
// вперёд их можно пройти, ничего не выбирая заново.
func TestBack_KidToStartAndForward(t *testing.T) {
	h := newHarness(t)
	h.toRegion("9")
	h.text(artem, "Казань")
	h.press(artem, "subj:t:inf")
	h.press(artem, "subj:t:math")
	h.press(artem, "subj:done")
	h.press(artem, "dir:t:napr-09-03-04")

	q := h.tap(artem, h.fake.Last(artem.UserID), "back:direction")
	if b := maxtest.Buttons(q); !strings.HasPrefix(q.Text, "Шаг 5 из 9\n\nКакие предметы тебе нравятся?") ||
		!strings.Contains(b, "✓ Информатика") || !strings.Contains(b, "✓ Математика") || !strings.HasSuffix(b, "Готово | ← Назад") {
		t.Fatalf("назад к предметам: %q %s", q.Text, b)
	}
	q = h.tap(artem, q, "back:subjects")
	if !strings.HasPrefix(q.Text, "Шаг 4 из 9\n\nГде ты живёшь?") || !strings.HasPrefix(maxtest.Buttons(q), "✓ Казань · Татарстан | Москва") ||
		!slices.Contains(maxtest.Payloads(q), "region:keep") {
		t.Fatalf("назад к региону: %q %s %v", q.Text, maxtest.Buttons(q), maxtest.Payloads(q))
	}
	q = h.tap(artem, q, "back:region")
	if q.Text != "Шаг 3 из 9\n\nПриятно познакомиться, Артём! В каком ты классе?" || maxtest.Buttons(q) != "8 | ✓ 9 | 10 | 11 | ← Назад" {
		t.Fatalf("назад к классу: %q %s", q.Text, maxtest.Buttons(q))
	}
	q = h.tap(artem, q, "back:grade")
	if !strings.HasPrefix(q.Text, "Шаг 2 из 9\n\nТебя зовут Артём?") {
		t.Fatalf("назад к имени: %q", q.Text)
	}
	q = h.tap(artem, q, "back:name_confirm")
	if !strings.HasPrefix(q.Text, "Шаг 1 из 9\n\nПривет!") || maxtest.Buttons(q) != "✓ Я школьник | Я родитель" || hasBack(q) {
		t.Fatalf("первый вопрос, дальше назад некуда: %q %s", q.Text, maxtest.Buttons(q))
	}

	// Вперёд — всё на месте.
	h.pressAny(artem, "role:kid", q.Text)
	h.mustContain(artem, "Тебя зовут Артём?")
	h.press(artem, "name:ok")
	if b := maxtest.Buttons(h.fake.Last(artem.UserID)); !strings.Contains(b, "✓ 9") {
		t.Fatalf("класс отмечен: %s", b)
	}
	h.press(artem, "grade:9")
	h.press(artem, "region:keep")
	h.mustContain(artem, "Какие предметы тебе нравятся?")
	if b := maxtest.Buttons(h.fake.Last(artem.UserID)); !strings.Contains(b, "✓ Информатика") || !strings.Contains(b, "✓ Математика") {
		t.Fatalf("предметы отмечены: %s", b)
	}
	h.press(artem, "subj:done")
	if b := maxtest.Buttons(h.fake.Last(artem.UserID)); !strings.Contains(b, "✓ Программная инженерия") {
		t.Fatalf("направление отмечено: %s", b)
	}
	if d := h.dialog(artem); d.Step != stepDirection || d.Draft.Grade != 9 || d.Draft.RegionCode != "16" ||
		d.Draft.HomeCity != "Казань" || !slices.Equal(d.Draft.SubjectCodes, []string{"inf", "math"}) ||
		!slices.Equal(d.Draft.DirectionIDs, []string{"napr-09-03-04"}) {
		t.Fatalf("черновик: %+v", d)
	}
}

// «Назад» со старого вопроса ничего не меняет.
func TestBack_OldButtonIsStale(t *testing.T) {
	h := newHarness(t)
	h.toDirections("inf")
	q := h.fake.Last(artem.UserID)
	h.tap(artem, q, "back:direction")
	if a := h.answered(h.pressAny(artem, "back:direction", q.Text)); a.Notification != "Этот вопрос уже позади" {
		t.Fatalf("старая кнопка: %+v", a)
	}
	if d := h.dialog(artem); d.Step != stepSubjects {
		t.Fatalf("шаг: %s", d.Step)
	}
}

// Родитель вернулся к имени: введённое имя — кнопкой, можно оставить или
// написать другое. Смена роли имя сбрасывает: у родителя это имя ребёнка.
func TestBack_ParentName(t *testing.T) {
	h := newHarness(t)
	h.started(olga, "")
	h.press(olga, "role:parent")
	h.text(olga, "Артём")
	q := h.tap(olga, h.fake.Last(olga.UserID), "back:grade")
	if q.Text != "Шаг 2 из 9\n\nКак зовут вашего ребёнка?" || maxtest.Buttons(q) != "✓ Артём | ← Назад" {
		t.Fatalf("назад к имени: %q %s", q.Text, maxtest.Buttons(q))
	}
	h.pressAny(olga, "name:keep", q.Text)
	h.mustContain(olga, "В каком классе Артём?")
	q = h.tap(olga, h.fake.Last(olga.UserID), "back:grade")
	h.text(olga, "Тёма")
	h.mustContain(olga, "В каком классе Тёма?")

	q = h.tap(olga, h.fake.Last(olga.UserID), "back:grade")
	q = h.tap(olga, q, "back:name_input")
	h.pressAny(olga, "role:kid", q.Text)
	h.mustContain(olga, "Тебя зовут Ольга?")
}

// После «Помоги выбрать» «Назад» с опыта ведёт к предложенным направлениям
// и дальше по вопросам В2, В1 с отмеченными ответами; если направления
// выбраны вручную — к списку направлений.
func TestBack_DirectionHelp(t *testing.T) {
	h := newHarness(t)
	h.toDirections("inf", "math")
	h.press(artem, "dir:help")
	h.press(artem, "int:t:code")
	h.press(artem, "int:done")
	h.press(artem, "work:build")
	h.press(artem, "dir:done")
	q := h.tap(artem, h.fake.Last(artem.UserID), "back:experience")
	if !strings.HasPrefix(q.Text, "Шаг 6 из 9\n\nПохоже, тебе подойдут эти направления.") || !strings.Contains(maxtest.Buttons(q), "✓ Программная инженерия") {
		t.Fatalf("назад к предложенным: %q %s", q.Text, maxtest.Buttons(q))
	}
	q = h.tap(artem, q, "back:suggest")
	if q.Text != "Шаг 6 из 9\n\nКакая работа тебе ближе?" || !strings.Contains(maxtest.Buttons(q), "✓ Разрабатывать продукты и технологии") {
		t.Fatalf("назад к В2: %q %s", q.Text, maxtest.Buttons(q))
	}
	q = h.tap(artem, q, "back:work")
	if !strings.Contains(maxtest.Buttons(q), "✓ Писать код, делать игры и программы") {
		t.Fatalf("назад к В1: %s", maxtest.Buttons(q))
	}
	q = h.tap(artem, q, "back:interest")
	if !strings.HasPrefix(q.Text, "Шаг 6 из 9\n\nКуда думаешь поступать?") {
		t.Fatalf("назад к направлениям: %q", q.Text)
	}
	h.pressAny(artem, "dir:done", q.Text)
	q = h.tap(artem, h.fake.Last(artem.UserID), "back:experience")
	if !strings.HasPrefix(q.Text, "Шаг 6 из 9\n\nКуда думаешь поступать?") {
		t.Fatalf("выбрано вручную — назад к направлениям: %q", q.Text)
	}
}

// «Где учиться»: места, добавленные текстом, и «Не важно» переживают
// возврат к опыту и обратно.
func TestBack_PlacesKept(t *testing.T) {
	h := newHarness(t)
	h.toDirections("inf")
	h.press(artem, "dir:later")
	h.press(artem, "exp:school")
	h.text(artem, "Самара")
	h.press(artem, "place:done")
	q := h.tap(artem, h.fake.Last(artem.UserID), "back:universities")
	if !strings.HasPrefix(q.Text, "Шаг 8 из 9\n\nГде хочешь учиться?") || !strings.Contains(maxtest.Buttons(q), "✓ Самара") {
		t.Fatalf("назад к местам: %q %s", q.Text, maxtest.Buttons(q))
	}
	q = h.tap(artem, q, "back:target")
	if !strings.Contains(maxtest.Buttons(q), "✓ Школьный или муниципальный этап") {
		t.Fatalf("назад к опыту: %s", maxtest.Buttons(q))
	}
	h.pressAny(artem, "exp:school", q.Text)
	if b := maxtest.Buttons(h.fake.Last(artem.UserID)); !strings.Contains(b, "✓ Самара") {
		t.Fatalf("Самара на месте: %s", b)
	}
	h.press(artem, "place:any")
	q = h.tap(artem, h.fake.Last(artem.UserID), "back:universities")
	if !strings.Contains(maxtest.Buttons(q), "✓ Не важно") {
		t.Fatalf("«Не важно» отмечено: %s", maxtest.Buttons(q))
	}
}

// «Не важно» в регионе после «Назад» отмечено; выбранный потом регион его
// снимает.
func TestBack_RegionSkipMarked(t *testing.T) {
	h := newHarness(t)
	h.toRegion("9")
	h.press(artem, "region:skip")
	q := h.tap(artem, h.fake.Last(artem.UserID), "back:subjects")
	if b := maxtest.Buttons(q); !strings.Contains(b, "✓ Не важно") || slices.Contains(maxtest.Payloads(q), "region:keep") {
		t.Fatalf("«Не важно» отмечено: %s", b)
	}
	h.pressAny(artem, "region:77", q.Text)
	q = h.tap(artem, h.fake.Last(artem.UserID), "back:subjects")
	if b := maxtest.Buttons(q); !strings.HasPrefix(b, "✓ Москва | ") || strings.Contains(b, "✓ Не важно") {
		t.Fatalf("выбрана Москва: %s", b)
	}
}

// Правка из карточки профиля: «← Назад» под вопросом поля возвращает меню
// «Что поменять?» на той же карточке; внутри «Помоги выбрать» — прошлый
// вопрос того же поля.
func TestBack_EditReturnsToSummaryMenu(t *testing.T) {
	h := newHarness(t)
	h.toSummary()
	h.edit("grade")
	q := h.fake.Last(artem.UserID)
	if !hasBack(q) {
		t.Fatalf("под вопросом правки — «Назад»: %s", maxtest.Buttons(q))
	}
	q = h.tap(artem, q, "back:grade")
	if !strings.HasSuffix(q.Text, "\n\nЧто поменять?") || !slices.Contains(maxtest.Payloads(q), "sum:f:grade") {
		t.Fatalf("«Назад» при правке — меню правки: %q %v", q.Text, maxtest.Payloads(q))
	}
	if d := h.dialog(artem); d.Step != stepSummary || d.Draft.EditStage != 0 || !d.Draft.EditMenu || d.Draft.Grade != 9 {
		t.Fatalf("диалог после «Назад»: %+v", d)
	}

	h.pressAny(artem, "sum:f:goal", "профиль")
	h.press(artem, "dir:help")
	q = h.fake.Last(artem.UserID)
	q = h.tap(artem, q, "back:interest")
	if !strings.HasPrefix(q.Text, "Куда думаешь поступать?") {
		t.Fatalf("из «Помоги выбрать» — к направлениям: %q", q.Text)
	}
	q = h.tap(artem, q, "back:direction")
	if !strings.HasSuffix(q.Text, "\n\nЧто поменять?") {
		t.Fatalf("с направлений при правке — меню: %q", q.Text)
	}
}
