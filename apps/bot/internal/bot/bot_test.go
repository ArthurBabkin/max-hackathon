package bot

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArthurBabkin/max-hackathon/packages/core/refdata"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi/maxtest"
)

var (
	artem = maxapi.User{UserID: 900000001, FirstName: "Артём"}
	olga  = maxapi.User{UserID: 900000002, FirstName: "Ольга"}
	igor  = maxapi.User{UserID: 900000003, FirstName: "Игорь"}
	msk   = mustLoc("Europe/Moscow")
)

func mustLoc(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return l
}

type harness struct {
	t    *testing.T
	db   *pgxpool.Pool
	st   *store.Store
	fake *maxtest.Fake
	bot  *Bot
	seq  int
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db := dbtest.Open(t)
	st := store.New(db)
	fake := &maxtest.Fake{}
	b := New(st, fake, Config{BotName: "test_bot", BotID: 42, ReminderHour: 10})
	b.now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, msk) }
	return &harness{t: t, db: db, st: st, fake: fake, bot: b}
}

func (h *harness) handle(u maxapi.Update) {
	h.t.Helper()
	h.seq++
	u.Timestamp = int64(h.seq)
	if err := h.bot.Handle(context.Background(), u); err != nil {
		h.t.Fatalf("обработка %s: %v", u.UpdateType, err)
	}
}

func (h *harness) started(u maxapi.User, payload string) {
	h.t.Helper()
	var p *string
	if payload != "" {
		p = &payload
	}
	h.handle(maxapi.Update{UpdateType: maxapi.UpdateBotStarted, User: &u, Payload: p})
}

func (h *harness) text(u maxapi.User, text string) {
	h.t.Helper()
	h.seq++
	h.handle(maxapi.Update{UpdateType: maxapi.UpdateMessageCreated, Message: &maxapi.Message{
		Sender: &u, Recipient: maxapi.Recipient{ChatType: "dialog"},
		Body: maxapi.MessageBody{MID: fmt.Sprintf("mid.in.%d", h.seq), Text: text}}})
}

func (h *harness) geo(u maxapi.User, lat, lon float64) {
	h.t.Helper()
	h.seq++
	h.handle(maxapi.Update{UpdateType: maxapi.UpdateMessageCreated, Message: &maxapi.Message{
		Sender: &u, Recipient: maxapi.Recipient{ChatType: "dialog"},
		Body: maxapi.MessageBody{MID: fmt.Sprintf("mid.geo.%d", h.seq),
			Attachments: []maxapi.Attachment{{Type: "location", Latitude: lat, Longitude: lon}}}}})
}

// press нажимает кнопку под последним сообщением пользователю. Кнопки
// с таким payload там нет — тест падает: жать можно только то, что видно.
func (h *harness) press(u maxapi.User, payload string) string {
	h.t.Helper()
	last := h.fake.Last(u.UserID)
	if !slices.Contains(maxtest.Payloads(last), payload) {
		h.t.Fatalf("под сообщением %q нет кнопки %s: %v", last.Text, payload, maxtest.Payloads(last))
	}
	return h.pressAny(u, payload, last.Text)
}

// pressAny — нажатие без проверки: кнопка из старого сообщения.
func (h *harness) pressAny(u maxapi.User, payload, question string) string {
	h.t.Helper()
	h.seq++
	id := fmt.Sprintf("cb-%d", h.seq)
	h.handle(maxapi.Update{UpdateType: maxapi.UpdateMessageCallback,
		Callback: &maxapi.Callback{CallbackID: id, Payload: payload, User: u},
		Message: &maxapi.Message{Recipient: maxapi.Recipient{ChatType: "dialog"},
			Body: maxapi.MessageBody{MID: "mid.q", Text: question}}})
	return id
}

func (h *harness) lastText(u maxapi.User) string { return h.fake.Last(u.UserID).Text }

func (h *harness) answered(id string) maxapi.CallbackAnswer {
	for _, a := range h.fake.Answered {
		if a.CallbackID == id {
			return a.Answer
		}
	}
	h.t.Fatalf("на нажатие %s не ответили", id)
	return maxapi.CallbackAnswer{}
}

// step — последний вопрос начинается со счётчика «Шаг n из 9».
func (h *harness) step(u maxapi.User, n int) {
	h.t.Helper()
	if got := h.lastText(u); !strings.HasPrefix(got, fmt.Sprintf("Шаг %d из 9\n\n", n)) {
		h.t.Fatalf("ждали шаг %d, последнее сообщение: %q", n, got)
	}
}

func (h *harness) mustContain(u maxapi.User, want string) {
	h.t.Helper()
	if got := h.lastText(u); !strings.Contains(got, want) {
		h.t.Fatalf("ждали «%s», последнее сообщение: %q", want, got)
	}
}

// dialog — черновик и шаг онбординга пользователя.
func (h *harness) dialog(u maxapi.User) store.Dialog {
	h.t.Helper()
	uid, _ := h.st.UpsertUser(context.Background(), u.UserID, u.FirstName)
	d, err := h.st.Dialog(context.Background(), uid)
	if err != nil {
		h.t.Fatalf("диалог: %v", err)
	}
	return d
}

func (h *harness) trajectory(u maxapi.User) store.Trajectory {
	h.t.Helper()
	m, err := h.st.CurrentMember(context.Background(), u.UserID)
	if err != nil {
		h.t.Fatalf("траектория создана: %v", err)
	}
	tr, err := h.st.Trajectory(context.Background(), m.TrajectoryID)
	if err != nil {
		h.t.Fatal(err)
	}
	return tr
}

// sentBack — сообщение пользователю, n-е с конца (0 — последнее).
func (h *harness) sentBack(u maxapi.User, n int) maxapi.NewMessage {
	sent := h.fake.To(u.UserID)
	return sent[len(sent)-1-n].Msg
}

// toRegion — ученик Артём до вопроса о регионе.
func (h *harness) toRegion(grade string) {
	h.t.Helper()
	h.started(artem, "")
	h.press(artem, "role:kid")
	h.press(artem, "name:ok")
	h.press(artem, "grade:"+grade)
	h.mustContain(artem, "Шаг 4 из 9\n\nГде ты живёшь?")
}

// toDirections — ученик из Казани с предметами до вопроса о направлениях.
func (h *harness) toDirections(subjects ...string) {
	h.t.Helper()
	h.toRegion("9")
	h.geo(artem, 55.79, 49.11) // Казань
	for _, s := range subjects {
		h.press(artem, "subj:t:"+s)
	}
	h.press(artem, "subj:done")
}

// toSummary — ученик из Казани (информатика, программная инженерия,
// школьный этап, Татарстан, без вузов) до карточки профиля.
func (h *harness) toSummary() {
	h.t.Helper()
	h.toDirections("inf")
	h.press(artem, "dir:t:napr-09-03-04")
	h.press(artem, "dir:done")
	h.press(artem, "exp:school")
	h.press(artem, "place:t:1")
	h.press(artem, "place:done")
	h.press(artem, "vuz:done")
	h.mustContain(artem, "**Артём, сформировали твой профиль**")
}

// edit открывает меню правки и выбирает поле.
func (h *harness) edit(field string) {
	h.t.Helper()
	h.press(artem, "sum:edit")
	// Меню — на той же карточке, в последнем сообщении его нет.
	h.pressAny(artem, "sum:f:"+field, "профиль")
}

// kidOnboarding проходит онбординг ученика целиком (ТЗ §5.1, SPEC 3–9).
func (h *harness) kidOnboarding() store.Member {
	h.t.Helper()
	h.started(artem, "src_class_9a")
	h.step(artem, 1)
	h.mustContain(artem, "Кто вы?")
	h.press(artem, "role:kid")
	h.step(artem, 2)
	h.mustContain(artem, "Тебя зовут Артём?")
	h.press(artem, "name:ok")
	h.step(artem, 3)
	h.mustContain(artem, "Приятно познакомиться, Артём! В каком ты классе?")
	h.press(artem, "grade:9")
	h.step(artem, 4)
	h.mustContain(artem, "Где ты живёшь? Напиши город или регион")
	h.text(artem, "Казань")
	if got := h.sentBack(artem, 1); got.Text != "Казань → Татарстан ✓" || !slices.Contains(maxtest.Payloads(got), "region:change") {
		h.t.Fatalf("найден город: %q %v", got.Text, maxtest.Payloads(got))
	}
	h.step(artem, 5)
	h.press(artem, "subj:t:inf")
	h.press(artem, "subj:t:math")
	h.press(artem, "subj:done")
	h.step(artem, 6)
	h.mustContain(artem, "Куда думаешь поступать? Под твои предметы подходят")
	h.press(artem, "dir:t:napr-09-03-04")
	h.press(artem, "dir:t:napr-09-03-01")
	h.press(artem, "dir:done")
	h.step(artem, 7)
	h.mustContain(artem, "Какой у тебя опыт в олимпиадах?")
	h.press(artem, "exp:school")
	h.step(artem, 8)
	h.mustContain(artem, "Где хочешь учиться? Можно выбрать несколько мест или написать город.")
	if b := maxtest.Buttons(h.fake.Last(artem.UserID)); !strings.HasPrefix(b, "Только Казань | Весь регион · Татарстан | Москва | Санкт-Петербург") {
		h.t.Fatalf("варианты мест: %s", b)
	}
	h.press(artem, "place:t:1")
	h.press(artem, "place:done")
	h.step(artem, 9)
	h.mustContain(artem, "Вот вузы с направлениями программная инженерия, информатика и вычислительная техника в этих местах.")
	h.press(artem, "vuz:t:innopolis")
	h.text(artem, "вышка")
	if got := h.sentBack(artem, 1).Text; got != "Добавил ВШЭ." {
		h.t.Fatalf("поиск по алиасу: %q", got)
	}
	h.press(artem, "vuz:done")
	// Профиль на подтверждение: траектории до «Готово» ещё нет (SPEC 9).
	if _, err := h.st.CurrentMember(context.Background(), artem.UserID); err == nil {
		h.t.Fatal("траектория создана до «Готово»")
	}
	summary := h.fake.Last(artem.UserID)
	if summary.Text != "**Артём, сформировали твой профиль**\nКласс: 9 · Казань\nПредметы: информатика, математика\n"+
		"Цель: программная инженерия, информатика и вычислительная техника\nОпыт: школьный или муниципальный этап\n"+
		"Где учиться: Татарстан\nВузы: Иннополис, ВШЭ\n\nВсё верно? Если нужно поправить — нажми «Изменить»." ||
		summary.Format != "markdown" || !slices.Equal(maxtest.Payloads(summary), []string{"sum:edit", "sum:ok"}) {
		h.t.Fatalf("профиль: %q (%s) %v", summary.Text, summary.Format, maxtest.Payloads(summary))
	}
	h.press(artem, "sum:ok")
	m, err := h.st.CurrentMember(context.Background(), artem.UserID)
	if err != nil {
		h.t.Fatalf("траектория создана: %v", err)
	}
	return m
}

func TestKidOnboarding_CreatesTrajectoryAndShowsResult(t *testing.T) {
	h := newHarness(t)
	m := h.kidOnboarding()
	if m.Role != "kid" || !m.IsCreator {
		t.Fatalf("ученик — создатель: %+v", m)
	}
	tr, _ := h.st.Trajectory(context.Background(), m.TrajectoryID)
	if tr.StudentName != "Артём" || tr.Grade != 9 || tr.RegionCode != "16" || tr.TZ != "Europe/Moscow" ||
		len(tr.Directions) != 2 || tr.Directions[0].ID != "napr-09-03-04" || tr.Directions[1].ID != "napr-09-03-01" ||
		tr.GoalStatus != "known" || tr.Experience != "school" || tr.HomeCity == nil || *tr.HomeCity != "Казань" ||
		!slices.Equal(tr.Places, []store.Place{{RegionCode: "16"}}) || tr.GoalByKid {
		t.Fatalf("траектория: %+v", tr)
	}
	// После «Готово» — только итог: профиль уже был на подтверждении.
	if s := h.sentBack(artem, 1).Text; !strings.HasPrefix(s, "**Артём, сформировали твой профиль**") {
		t.Fatalf("перед итогом — карточка профиля: %q", s)
	}
	result := h.fake.Last(artem.UserID)
	if !strings.HasPrefix(result.Text, "Под твою цель подходят 5 олимпиад и ВсОШ.\n\nДля старта советую эту:") ||
		!strings.HasSuffix(result.Text, "\n\nВ мини-приложении — льготы в твоих вузах, источники и трекер сроков.") {
		t.Fatalf("итог: %q", result.Text)
	}
	// Одна кнопка: приложение на главной, где новичку покажут обучение.
	kb := result.Keyboard()
	if len(kb) != 1 || len(kb[0]) != 1 || kb[0][0].Type != "open_app" || kb[0][0].ContactID != 42 ||
		kb[0][0].Payload != "home" || kb[0][0].Text != "Открыть «Траекторию»" {
		t.Fatalf("кнопки итога: %s", maxtest.Buttons(result))
	}
	d := h.dialog(artem)
	if d.Step != stepDone || d.SourcePayload == nil || *d.SourcePayload != "src_class_9a" {
		t.Fatalf("диалог закрыт, источник сохранён: %+v", d)
	}
}

// Итог: кроме стартовой олимпиады — ближайшая по сроку, если это другая.
func TestResult_NearestAfterStart(t *testing.T) {
	h := newHarness(t)
	h.kidOnboarding()
	blocks := strings.Split(h.lastText(artem), "\n\n")
	if len(blocks) != 4 || !strings.HasPrefix(blocks[1], "Для старта советую эту:\nInnopolis Open · Информатика\n") ||
		blocks[2] != "Ближайшая олимпиада:\nФизтех · Математика\nII уровень · до 7 сентября, 6 дней · онлайн" {
		t.Fatalf("итог: %q", blocks)
	}
}

// Стартовая и есть ближайшая — один блок.
func TestResult_StartIsNearest(t *testing.T) {
	h := newHarness(t)
	h.toDirections("inf")
	h.press(artem, "dir:later")
	h.press(artem, "exp:none")
	h.press(artem, "place:any")
	h.press(artem, "vuz:done")
	h.press(artem, "sum:ok")
	blocks := strings.Split(h.lastText(artem), "\n\n")
	if len(blocks) != 3 || !strings.HasPrefix(blocks[1], "Для старта советую эту — она же ближайшая:\nФизтех · Информатика\n") ||
		!strings.Contains(blocks[1], "до 7 сентября, 6 дней") {
		t.Fatalf("итог: %q", blocks)
	}
}

func TestParentOnboarding_UndecidedAnywhere(t *testing.T) {
	h := newHarness(t)
	h.started(olga, "")
	h.press(olga, "role:parent")
	h.mustContain(olga, "Как зовут вашего ребёнка?")
	h.text(olga, strings.Repeat("я", 41))
	h.mustContain(olga, "от 1 до 40 символов")
	h.text(olga, "  Артём  ")
	h.mustContain(olga, "В каком классе Артём?")
	h.press(olga, "grade:10")
	h.mustContain(olga, "Где живёт Артём? Напишите город или регион")
	// Геолокации кнопкой нет: в MAX она работает только в мобильном
	// приложении, в вебе и на компьютере не нажимается.
	if last := h.fake.Last(olga.UserID); maxtest.Buttons(last) != "Москва | Санкт-Петербург | Московская обл. | По алфавиту А–Я | Не важно | ← Назад" ||
		!slices.Equal(maxtest.Payloads(last), []string{"region:77", "region:78", "region:50", "region:abc", "region:skip", "back:region"}) {
		t.Fatalf("кнопки региона: %s %v", maxtest.Buttons(last), maxtest.Payloads(last))
	}
	// Алфавит правит тот же вопрос, нового сообщения нет.
	id := h.press(olga, "region:abc")
	if a := h.answered(id); a.Message == nil || !strings.HasPrefix(a.Message.Text, "Шаг 4 из 9\n\nНа какую букву ваш регион?") ||
		!slices.Contains(maxtest.Payloads(*a.Message), "region:l:Т") {
		t.Fatalf("буквы на месте вопроса: %+v", a)
	}
	id = h.pressAny(olga, "region:l:Т", "")
	if a := h.answered(id); a.Message == nil || !strings.Contains(maxtest.Buttons(*a.Message), "Татарстан") ||
		!slices.Contains(maxtest.Payloads(*a.Message), "region:16") || !slices.Contains(maxtest.Payloads(*a.Message), "region:abc") {
		t.Fatalf("регионы на «Т»: %+v", a)
	}
	h.pressAny(olga, "region:16", "")
	h.mustContain(olga, "Какие предметы нравятся Артёму?")
	id = h.pressAny(olga, "subj:done", "")
	if a := h.answered(id); a.Notification != "Выберите хотя бы один предмет" {
		t.Fatalf("«Готово» без предметов: %+v", a)
	}
	h.press(olga, "subj:t:phys")
	h.press(olga, "subj:done")

	// Под физику — только физические направления, остальные по кнопке.
	h.mustContain(olga, "Куда думает поступать Артём? Под выбранные предметы подходят")
	payloads := maxtest.Payloads(h.fake.Last(olga.UserID))
	if !slices.Contains(payloads, "dir:t:napr-03-03-01") || slices.Contains(payloads, "dir:t:napr-31-05-01") ||
		!slices.Contains(payloads, "dir:all") || !slices.Contains(payloads, "dir:help") || !slices.Contains(payloads, "dir:kid") {
		t.Fatalf("направления под физику: %v", payloads)
	}
	id = h.press(olga, "dir:done")
	if a := h.answered(id); a.Notification != "Отметьте направление или нажмите «Пока не знаем»" {
		t.Fatalf("«Готово» без направлений: %+v", a)
	}
	id = h.press(olga, "dir:all")
	if a := h.answered(id); a.Message == nil || !slices.Contains(maxtest.Payloads(*a.Message), "dir:t:napr-31-05-01") ||
		slices.Contains(maxtest.Payloads(*a.Message), "dir:all") {
		t.Fatalf("все направления на месте вопроса: %+v", a)
	}
	h.pressAny(olga, "dir:later", "")

	h.mustContain(olga, "Какой у Артёма опыт в олимпиадах?")
	h.press(olga, "exp:none:idk")

	// Город не известен — подсказка его написать; Москва и Петербург — не свой регион.
	h.mustContain(olga, "Где Артём хочет учиться? Можно выбрать несколько мест или написать город. Если важно учиться именно в своём городе")
	last := h.fake.Last(olga.UserID)
	if b := maxtest.Buttons(last); b != "Весь регион · Татарстан | Москва | Санкт-Петербург | Другой город | Не важно | Готово | ← Назад" {
		t.Fatalf("варианты мест: %s", b)
	}
	id = h.press(olga, "place:done")
	if a := h.answered(id); a.Notification != "Выберите хотя бы одно место или «Не важно»" {
		t.Fatalf("«Готово» без мест: %+v", a)
	}
	h.press(olga, "place:any")
	h.mustContain(olga, "Вот вузы в этих местах. Отметьте, куда хочет поступать Артём")
	h.press(olga, "vuz:done")
	h.press(olga, "sum:ok")
	tr := h.trajectory(olga)
	m, _ := h.st.CurrentMember(context.Background(), olga.UserID)
	if m.Role != "parent" || !m.IsCreator || m.HasKid {
		t.Fatalf("родитель — создатель без ученика: %+v", m)
	}
	if tr.GoalStatus != "exploring" || len(tr.Directions) != 0 || len(tr.Places) != 0 || tr.Grade != 10 ||
		tr.Experience != "none" || tr.HomeCity != nil || tr.GoalByKid {
		t.Fatalf("цель не выбрана, место не важно: %+v", tr)
	}
	summary := h.sentBack(olga, 1).Text
	if summary != "**Ольга, сформировали профиль Артёма**\nКласс: 10 · Татарстан\nПредметы: физика\nЦель: пока не выбрана\n"+
		"Опыт: первые олимпиады\nГде учиться: не важно\nВузы: не выбраны\n\nВсё верно? Если нужно поправить — нажмите «Изменить»." {
		t.Fatalf("профиль родителю: %q", summary)
	}
	result := h.fake.Last(olga.UserID)
	if !strings.HasPrefix(result.Text, "По предметам Артёма подходят ") {
		t.Fatalf("итог родителю: %q", result.Text)
	}
	if p := maxtest.Payloads(result); !slices.Equal(p, []string{"home"}) {
		t.Fatalf("у родителя одна кнопка: %s", maxtest.Buttons(result))
	}
}

// Регион текстом: одно совпадение сохраняется сразу, «Не то» возвращает к
// вопросу, пока предметы не отмечены (SPEC 3.2, R2).
func TestRegion_TextSingleAndChange(t *testing.T) {
	h := newHarness(t)
	h.toRegion("9")
	h.text(artem, "набережные челны")
	found := h.sentBack(artem, 1)
	if found.Text != "Набережные Челны → Татарстан ✓" {
		t.Fatalf("найдено: %q", found.Text)
	}
	h.mustContain(artem, "Какие предметы тебе нравятся?")
	if d := h.dialog(artem); d.Step != stepSubjects || d.Draft.RegionCode != "16" || d.Draft.HomeCity != "Набережные Челны" {
		t.Fatalf("регион и город: %+v", d)
	}
	h.pressAny(artem, "region:change", found.Text)
	h.mustContain(artem, "Шаг 4 из 9\n\nГде ты живёшь?")
	if d := h.dialog(artem); d.Step != stepRegion || d.Draft.RegionCode != "" || d.Draft.HomeCity != "" {
		t.Fatalf("снова регион: %+v", d)
	}
	h.text(artem, "г. Москва")
	if got := h.sentBack(artem, 1).Text; got != "Москва ✓" {
		t.Fatalf("регион без города: %q", got)
	}
	// Предмет отмечен — «Не то» уже не работает.
	h.press(artem, "subj:t:inf")
	id := h.pressAny(artem, "region:change", "Москва ✓")
	if a := h.answered(id); a.Notification != "Этот вопрос уже позади" {
		t.Fatalf("«Не то» после предметов: %+v", a)
	}
	if d := h.dialog(artem); d.Step != stepSubjects || d.Draft.RegionCode != "77" {
		t.Fatalf("регион не изменился: %+v", d)
	}
}

// Несколько совпадений — кнопки выбора (R3).
func TestRegion_TextMany(t *testing.T) {
	h := newHarness(t)
	h.toRegion("9")
	h.text(artem, "советск")
	last := h.fake.Last(artem.UserID)
	p := maxtest.Payloads(last)
	if last.Text != "Шаг 4 из 9\n\nНашёл несколько. Какой твой?" || len(p) != 4 || p[0] != "region:o:0" || p[3] != "region:abc" ||
		!strings.HasPrefix(maxtest.Buttons(last), "Советск · Калининградская обл.") {
		t.Fatalf("варианты: %q %s %v", last.Text, maxtest.Buttons(last), p)
	}
	h.press(artem, "region:o:0")
	if got := h.sentBack(artem, 1).Text; got != "Советск → Калининградская обл. ✓" {
		t.Fatalf("выбран: %q", got)
	}
	if d := h.dialog(artem); d.Step != stepSubjects || d.Draft.RegionCode != "39" || d.Draft.HomeCity != "Советск" {
		t.Fatalf("регион: %+v", d)
	}
}

// Опечатка — переспрос (R4); «Написать заново» возвращает вопрос.
func TestRegion_TextFuzzy(t *testing.T) {
	h := newHarness(t)
	h.toRegion("9")
	h.text(artem, "Казнь")
	last := h.fake.Last(artem.UserID)
	if last.Text != "Шаг 4 из 9\n\nНе нашёл «Казнь». Может, это Казань — Татарстан?" ||
		!slices.Equal(maxtest.Payloads(last), []string{"region:o:0", "region:retry", "region:abc"}) {
		t.Fatalf("переспрос: %q %v", last.Text, maxtest.Payloads(last))
	}
	id := h.press(artem, "region:retry")
	if a := h.answered(id); a.Message == nil || !strings.HasPrefix(a.Message.Text, "Шаг 4 из 9\n\nГде ты живёшь?") {
		t.Fatalf("написать заново: %+v", a)
	}
	h.text(artem, "Казнь")
	h.press(artem, "region:o:0")
	if d := h.dialog(artem); d.Step != stepSubjects || d.Draft.RegionCode != "16" || d.Draft.HomeCity != "Казань" {
		t.Fatalf("регион по опечатке: %+v", d)
	}
}

func TestRegion_TextNotFound(t *testing.T) {
	h := newHarness(t)
	h.toRegion("9")
	h.text(artem, "asdf")
	last := h.fake.Last(artem.UserID)
	if last.Text != "Шаг 4 из 9\n\nНе нашёл «asdf». Попробуй написать иначе или выбери по алфавиту." ||
		!slices.Equal(maxtest.Payloads(last), []string{"region:abc"}) {
		t.Fatalf("не найдено: %q %v", last.Text, maxtest.Payloads(last))
	}
	id := h.press(artem, "region:abc")
	if a := h.answered(id); a.Message == nil || !slices.Contains(maxtest.Payloads(*a.Message), "region:l:А") {
		t.Fatalf("алфавит: %+v", a)
	}
	if d := h.dialog(artem); d.Step != stepRegion {
		t.Fatalf("шаг региона: %+v", d)
	}
}

// «Не важно»: регион не указан, напоминания — по Москве.
func TestRegion_Skip(t *testing.T) {
	h := newHarness(t)
	h.toRegion("9")
	id := h.press(artem, "region:skip")
	if a := h.answered(id); a.Message == nil || maxtest.Buttons(*a.Message) != "✓ Не важно" {
		t.Fatalf("выбор на месте вопроса: %+v", a)
	}
	if got := h.sentBack(artem, 1).Text; got != "Хорошо. Напоминания будут приходить по московскому времени — поменять можно в профиле." {
		t.Fatalf("после «Не важно»: %q", got)
	}
	h.mustContain(artem, "Какие предметы тебе нравятся?")
	if d := h.dialog(artem); d.Step != stepSubjects || d.Draft.RegionCode != "" || d.Draft.HomeCity != "" {
		t.Fatalf("без региона: %+v", d)
	}
}

// Без региона траектория создаётся с московским временем; в местах —
// только Москва и Петербург, в профиле — класс без места.
func TestRegion_SkipCreatesTrajectory(t *testing.T) {
	h := newHarness(t)
	h.toRegion("9")
	h.press(artem, "region:skip")
	h.press(artem, "subj:t:inf")
	h.press(artem, "subj:done")
	h.press(artem, "dir:later")
	h.press(artem, "exp:none")
	if b := maxtest.Buttons(h.fake.Last(artem.UserID)); !strings.HasPrefix(b, "Москва | Санкт-Петербург | Другой город") {
		t.Fatalf("варианты мест: %s", b)
	}
	h.press(artem, "place:any")
	h.press(artem, "vuz:done")
	h.press(artem, "sum:ok")
	tr := h.trajectory(artem)
	if tr.RegionCode != "" || tr.TZ != "Europe/Moscow" || tr.HomeCity != nil {
		t.Fatalf("траектория без региона: %+v", tr)
	}
	if s := h.sentBack(artem, 1).Text; !strings.HasPrefix(s, "**Артём, сформировали твой профиль**\nКласс: 9\nПредметы: информатика\n") {
		t.Fatalf("профиль: %q", s)
	}
}

// Геолокация: город ближе 30 км — с городом, иначе регион; и там и там «Не то».
func TestRegion_Geolocation(t *testing.T) {
	h := newHarness(t)
	h.toRegion("9")
	h.geo(artem, 55.79, 49.11)
	found := h.sentBack(artem, 1)
	if found.Text != "Казань → Татарстан ✓" || !slices.Contains(maxtest.Payloads(found), "region:change") {
		t.Fatalf("по геолокации: %q", found.Text)
	}
	h.pressAny(artem, "region:change", found.Text)
	h.geo(artem, 63.0, 100.0) // тайга — ближайший город далеко
	found = h.sentBack(artem, 1)
	if !strings.HasSuffix(found.Text, " ✓") || strings.Contains(found.Text, "→") ||
		!slices.Contains(maxtest.Payloads(found), "region:change") {
		t.Fatalf("только регион: %q", found.Text)
	}
	if d := h.dialog(artem); d.Draft.HomeCity != "" || d.Step != stepSubjects {
		t.Fatalf("без города: %+v", d)
	}
	// Кнопка «Москва» из первого вопроса уже не действует.
	id := h.pressAny(artem, "region:77", "Шаг 4 из 9\n\nГде ты живёшь?")
	if a := h.answered(id); a.Notification != "Этот вопрос уже позади" {
		t.Fatalf("старая кнопка региона: %+v", a)
	}
}

// «Не знаю — помоги выбрать»: В1, В2, предложенные направления (SPEC 5).
func TestDirectionHelp_KidFlow(t *testing.T) {
	h := newHarness(t)
	h.toDirections("inf", "math")
	h.press(artem, "dir:help")
	h.mustContain(artem, "Давай разберёмся вместе — два вопроса.")
	h.step(artem, 6)
	id := h.press(artem, "int:done")
	if a := h.answered(id); a.Notification != "Выбери хотя бы один вариант" {
		t.Fatalf("«Готово» без интересов: %+v", a)
	}
	h.press(artem, "int:t:code")
	h.press(artem, "int:t:tasks")
	id = h.press(artem, "int:t:world")
	if a := h.answered(id); a.Notification != "Можно выбрать два варианта" {
		t.Fatalf("третий интерес: %+v", a)
	}
	if a := h.answered(h.press(artem, "int:done")); a.Message == nil ||
		maxtest.Buttons(*a.Message) != "✓ Писать код, делать игры и программы | ✓ Решать сложные задачи и головоломки" {
		t.Fatalf("интересы после «Готово»: %+v", a.Message)
	}
	h.mustContain(artem, "Какая работа тебе ближе?")
	h.step(artem, 6)
	id = h.press(artem, "work:text")
	if a := h.answered(id); a.Notification != "Скоро здесь можно будет написать своими словами. Пока выбери вариант выше." {
		t.Fatalf("своими словами: %+v", a)
	}
	h.press(artem, "work:build")
	h.mustContain(artem, "Похоже, тебе подойдут эти направления.")
	last := h.fake.Last(artem.UserID)
	if b := maxtest.Buttons(last); !strings.HasPrefix(b, "✓ Программная инженерия | ✓ Прикладная математика и информатика | ✓ Информатика и вычислительная техника") ||
		!slices.Contains(maxtest.Payloads(last), "dir:later") || !slices.Contains(maxtest.Payloads(last), "dir:all") {
		t.Fatalf("предложенные направления: %s", b)
	}
	h.press(artem, "dir:t:napr-09-03-01") // лишнее снято
	h.press(artem, "dir:done")
	h.mustContain(artem, "Какой у тебя опыт в олимпиадах?")
	if p := maxtest.Payloads(h.fake.Last(artem.UserID)); slices.Contains(p, "exp:none:idk") {
		t.Fatalf("«Не знаю» только у родителя: %v", p)
	}
	h.press(artem, "exp:none")
	h.press(artem, "place:any")
	h.press(artem, "vuz:done")
	h.press(artem, "sum:ok")
	tr := h.trajectory(artem)
	if tr.GoalStatus != "suggested" || len(tr.Directions) != 2 || tr.Directions[0].ID != "napr-09-03-04" ||
		tr.Directions[1].ID != "napr-01-03-02" {
		t.Fatalf("предложенная цель: %+v", tr)
	}
	if s := h.sentBack(artem, 1).Text; !strings.Contains(s, "\nЦель: программная инженерия, прикладная математика и информатика · подобрали вместе\n") {
		t.Fatalf("приписка в профиле: %q", s)
	}
}

// Родитель: «Помогите выбрать», «Пока не знаем», «Всё равно не знаем».
func TestDirectionHelp_ParentStillUndecided(t *testing.T) {
	h := newHarness(t)
	h.started(olga, "")
	h.press(olga, "role:parent")
	h.text(olga, "Артём")
	h.press(olga, "grade:11")
	h.press(olga, "region:77")
	h.press(olga, "subj:t:bio")
	h.press(olga, "subj:done")
	h.press(olga, "dir:help")
	h.mustContain(olga, "Давайте разберёмся — два вопроса. Что из этого больше всего нравится Артёму?")
	h.press(olga, "int:t:bio")
	h.press(olga, "int:done")
	h.mustContain(olga, "Какая работа ближе Артёму?")
	if b := maxtest.Buttons(h.fake.Last(olga.UserID)); !strings.Contains(b, "Пока не знаем") {
		t.Fatalf("вариант родителя: %s", b)
	}
	h.press(olga, "work:unknown")
	h.mustContain(olga, "Похоже, Артёму подойдут эти направления.")
	if a := h.answered(h.press(olga, "dir:later")); a.Message == nil || !strings.Contains(maxtest.Buttons(*a.Message), "Всё равно не знаем") {
		t.Fatalf("«Всё равно не знаем»: %+v", a.Message)
	}
	h.mustContain(olga, "Какой у Артёма опыт в олимпиадах?")
	h.press(olga, "exp:region")
	// Москва — свой регион: только она и Петербург.
	if b := maxtest.Buttons(h.fake.Last(olga.UserID)); !strings.HasPrefix(b, "Москва | Санкт-Петербург | Другой город") {
		t.Fatalf("места москвича: %s", b)
	}
	if strings.Contains(h.lastText(olga), "в своём городе") {
		t.Fatalf("у Москвы нет «своего города»: %q", h.lastText(olga))
	}
	h.press(olga, "place:t:0")
	h.press(olga, "place:done")
	h.press(olga, "vuz:done")
	h.press(olga, "sum:ok")
	tr := h.trajectory(olga)
	if tr.GoalStatus != "exploring" || len(tr.Directions) != 0 || tr.Experience != "region" ||
		!slices.Equal(tr.Places, []store.Place{{RegionCode: "77"}}) {
		t.Fatalf("траектория: %+v", tr)
	}
}

// «Где учиться»: места добавляются текстом, несколько совпадений — выбор (W2).
func TestTarget_AddPlacesByText(t *testing.T) {
	h := newHarness(t)
	h.toDirections("inf")
	h.press(artem, "dir:later")
	h.press(artem, "exp:none")
	h.text(artem, "Иннополис")
	last := h.fake.Last(artem.UserID)
	if last.Text != "Шаг 8 из 9\n\nДобавил Иннополис. Что-то ещё?" || !strings.Contains(maxtest.Buttons(last), "✓ Иннополис · Татарстан") {
		t.Fatalf("добавлено: %q %s", last.Text, maxtest.Buttons(last))
	}
	h.text(artem, "советск")
	last = h.fake.Last(artem.UserID)
	if last.Text != "Нашёл несколько. Какое место добавить?" || !slices.Contains(maxtest.Payloads(last), "place:f:1") {
		t.Fatalf("несколько: %q %v", last.Text, maxtest.Payloads(last))
	}
	if a := h.answered(h.press(artem, "place:f:0")); a.Message == nil || !strings.Contains(maxtest.Buttons(*a.Message), "✓ Советск · Калининградская обл.") {
		t.Fatalf("выбранное место на клавиатуре: %+v", a)
	}
	h.text(artem, "qwerty")
	h.mustContain(artem, "Не нашёл «qwerty». Попробуй написать иначе.")
	h.pressAny(artem, "place:other", "")
	h.mustContain(artem, "Напиши город или регион")
	h.pressAny(artem, "place:done", "")
	h.mustContain(artem, "Вот вузы в этих местах.")
	h.press(artem, "vuz:done")
	h.press(artem, "sum:ok")
	if tr := h.trajectory(artem); !slices.Equal(tr.Places, []store.Place{{RegionCode: "16", City: "Иннополис"}, {RegionCode: "39", City: "Советск"}}) {
		t.Fatalf("места: %+v", tr.Places)
	}
}

// Вузы: «Показать ещё», «Изменить места» и пустая подборка (SPEC 8.1, 8.3).
func TestUniversities_MoreAndChangePlaces(t *testing.T) {
	h := newHarness(t)
	h.toDirections("inf")
	h.press(artem, "dir:later")
	h.press(artem, "exp:none")
	h.press(artem, "place:any")
	last := h.fake.Last(artem.UserID)
	p := maxtest.Payloads(last)
	if !slices.Contains(p, "vuz:more") || strings.Count(strings.Join(p, " "), "vuz:t:") != 6 {
		t.Fatalf("первые 6 вузов и «Показать ещё»: %v", p)
	}
	if !strings.Contains(maxtest.Buttons(last), "ВШЭ · Москва") {
		t.Fatalf("подпись с городом: %s", maxtest.Buttons(last))
	}
	a := h.answered(h.press(artem, "vuz:more"))
	if a.Message == nil || strings.Count(strings.Join(maxtest.Payloads(*a.Message), " "), "vuz:t:") != 10 ||
		slices.Contains(maxtest.Payloads(*a.Message), "vuz:more") {
		t.Fatalf("все 10 вузов: %+v", a.Message)
	}
	h.pressAny(artem, "vuz:places", "")
	h.mustContain(artem, "Где хочешь учиться?")
	h.press(artem, "place:t:3") // Санкт-Петербург
	h.press(artem, "place:done")
	p = maxtest.Payloads(h.fake.Last(artem.UserID))
	if !slices.Contains(p, "vuz:t:itmo") || !slices.Contains(p, "vuz:t:spbu") || slices.Contains(p, "vuz:t:hse") {
		t.Fatalf("вузы Петербурга: %v", p)
	}
}

// В выбранных местах нет направления — ближайшие вузы, все вузы с
// направлением и похожие направления здесь (V3).
func TestUniversities_NoDirectionHere(t *testing.T) {
	h := newHarness(t)
	h.toDirections("bio", "chem")
	h.press(artem, "dir:all")
	h.press(artem, "dir:t:napr-19-03-01") // биотехнология: в Татарстане её нет
	h.press(artem, "dir:done")
	h.press(artem, "exp:none")
	h.press(artem, "place:t:1") // весь Татарстан
	h.press(artem, "place:done")
	last := h.fake.Last(artem.UserID)
	p := maxtest.Payloads(last)
	if last.Text != "Шаг 9 из 9\n\nВ выбранных местах нет программ по направлению «Биотехнология». Ближайшие вузы, где оно есть:" ||
		!slices.Contains(p, "vuz:add:sechenov") || !slices.Contains(p, "vuz:add:mipt") || !slices.Contains(p, "vuz:add:itmo") ||
		!slices.Contains(p, "vuz:alldir") || !slices.Contains(p, "vuz:similar") {
		t.Fatalf("пустая подборка: %q %v", last.Text, p)
	}
	if b := maxtest.Buttons(last); !strings.Contains(b, "Похожие здесь: биология, лечебное дело") || !strings.Contains(b, "Пропустить вузы") ||
		!strings.Contains(b, "+ ИТМО · Санкт-Петербург") {
		t.Fatalf("кнопки V3: %s", b)
	}
	a := h.answered(h.press(artem, "vuz:similar"))
	if a.Message == nil || !slices.Contains(maxtest.Payloads(*a.Message), "vuz:t:kfu") {
		t.Fatalf("похожие направления здесь: %+v", a.Message)
	}
	if d := h.dialog(artem); len(d.Draft.DirectionIDs) != 3 {
		t.Fatalf("направления добавлены: %v", d.Draft.DirectionIDs)
	}
}

// Ближайшие вузы считаются от своего города или региона, а без региона
// («Не важно») — от первого выбранного места.
func TestNearbyOrigin(t *testing.T) {
	kazanLat, kazanLon, _ := refdata.CityCoords("Казань", "16")
	spb, _ := refdata.ByCode("78")
	for _, c := range []struct {
		name     string
		dr       store.Draft
		places   []store.Place
		lat, lon float64
	}{
		{"свой город", store.Draft{RegionCode: "16", HomeCity: "Казань"}, []store.Place{{RegionCode: "78"}}, kazanLat, kazanLon},
		{"без региона — город места", store.Draft{}, []store.Place{{RegionCode: "16", City: "Казань"}}, kazanLat, kazanLon},
		{"без региона — регион места", store.Draft{}, []store.Place{{RegionCode: "78"}}, spb.Lat, spb.Lon},
	} {
		if lat, lon := nearbyOrigin(c.dr, c.places); lat != c.lat || lon != c.lon {
			t.Errorf("%s: %v, %v; ждали %v, %v", c.name, lat, lon, c.lat, c.lon)
		}
	}
}

func TestUniversities_AddNearbyAndAllDirections(t *testing.T) {
	h := newHarness(t)
	h.toDirections("bio", "chem")
	h.press(artem, "dir:all")
	h.press(artem, "dir:t:napr-19-03-01")
	h.press(artem, "dir:done")
	h.press(artem, "exp:none")
	h.press(artem, "place:t:0") // только Казань
	h.press(artem, "place:done")
	a := h.answered(h.press(artem, "vuz:alldir"))
	if a.Message == nil || !slices.Contains(maxtest.Payloads(*a.Message), "vuz:t:itmo") {
		t.Fatalf("все вузы с направлением: %+v", a.Message)
	}
	h.pressAny(artem, "vuz:places", "")
	h.press(artem, "place:done")
	a = h.answered(h.press(artem, "vuz:add:itmo"))
	if a.Message == nil || !strings.Contains(maxtest.Buttons(*a.Message), "✓ ИТМО · Санкт-Петербург") {
		t.Fatalf("добавлен ближайший вуз: %+v", a.Message)
	}
	h.press(artem, "vuz:done")
	h.press(artem, "sum:ok")
	tr := h.trajectory(artem)
	unis, _ := h.st.TrajectoryUniversities(context.Background(), tr.ID)
	if len(unis) != 1 || unis[0].ID != "itmo" ||
		!slices.Equal(tr.Places, []store.Place{{RegionCode: "16", City: "Казань"}, {RegionCode: "78"}}) {
		t.Fatalf("вуз и его место: %v %+v", unis, tr.Places)
	}
}

// Родитель передаёт вопросы об интересах ребёнку (SPEC 10).
func TestParentAsksKid_KidAnswersAfterJoin(t *testing.T) {
	h := newHarness(t)
	h.started(olga, "")
	h.press(olga, "role:parent")
	h.step(olga, 2) // у родителя имя вводится, номер тот же
	h.text(olga, "Артём")
	h.press(olga, "grade:10")
	h.text(olga, "Казань")
	h.press(olga, "subj:t:bio")
	h.press(olga, "subj:done")
	h.press(olga, "dir:kid")
	h.step(olga, 7)
	if got := h.sentBack(olga, 1).Text; !strings.HasPrefix(got, "Хорошо. Когда закончим, дам ссылку-приглашение: Артём ответит") {
		t.Fatalf("ответ на «Пусть ответит»: %q", got)
	}
	h.mustContain(olga, "Какой у Артёма опыт в олимпиадах?")
	h.press(olga, "exp:none")
	h.press(olga, "place:any")
	h.press(olga, "vuz:done")
	h.press(olga, "sum:ok")
	tr := h.trajectory(olga)
	if !tr.GoalByKid || len(tr.Directions) != 0 {
		t.Fatalf("цель ждёт ребёнка: %+v", tr)
	}
	if s := h.sentBack(olga, 1).Text; !strings.Contains(s, "\nЦель: ждёт ответа Артёма\n") {
		t.Fatalf("профиль: %q", s)
	}
	result := h.fake.Last(olga.UserID)
	if !strings.HasSuffix(result.Text, "Отправьте ссылку Артёму: ответы про интересы уточнят подборку.") ||
		result.Keyboard()[0][0].Payload != "inv:new" || result.Keyboard()[0][0].Text != "👋 Пригласить Артёма" {
		t.Fatalf("приглашение первым: %q %s", result.Text, maxtest.Buttons(result))
	}
	h.press(olga, "inv:new")
	link := h.lastText(olga)
	token := link[strings.Index(link, "start=inv_")+len("start=inv_"):]

	h.started(artem, "inv_"+token)
	check := h.fake.Last(artem.UserID)
	if check.Text != "**Проверь, всё ли верно:**\nКласс: 10 · Казань\nПредметы: биология\nЦель: ждёт твоего ответа\n"+
		"Опыт: первые олимпиады\nГде учиться: не важно" || check.Format != "markdown" {
		t.Fatalf("проверка профиля: %q", check.Text)
	}
	h.press(artem, "join:ok")
	h.mustContain(artem, "Давай разберёмся вместе")
	if got := h.lastText(artem); strings.HasPrefix(got, "Шаг ") {
		t.Fatalf("у приглашённого ученика счётчика нет: %q", got)
	}
	h.press(artem, "int:t:bio")
	h.press(artem, "int:done")
	h.press(artem, "work:health")
	h.mustContain(artem, "Похоже, тебе подойдут эти направления.")
	h.press(artem, "dir:done")
	tr = h.trajectory(olga)
	if tr.GoalByKid || tr.GoalStatus != "suggested" || len(tr.Directions) == 0 || tr.Directions[0].ID != "napr-31-05-01" {
		t.Fatalf("цель выбрана ребёнком: %+v", tr)
	}
	h.mustContain(olga, "Артём: цель выбрана — лечебное дело")
	h.mustContain(artem, "Отлично! Под твою цель подходят ")
}

func TestSuggestDirections(t *testing.T) {
	var dirs []store.Direction
	for id, subjects := range map[string][]string{
		"napr-01-03-02": {"inf", "math"}, "napr-09-03-01": {"inf", "math"}, "napr-09-03-04": {"inf", "math"},
		"napr-10-03-01": {"inf", "math"}, "napr-03-03-01": {"phys", "math"}, "napr-03-03-02": {"phys", "math"},
		"napr-31-05-01": {"bio", "chem"}, "napr-33-05-01": {"bio", "chem"}, "napr-06-03-01": {"bio", "chem", "ecol"},
		"napr-19-03-01": {"bio", "chem"}, "napr-99-99-99": {"inf"},
	} {
		dirs = append(dirs, store.Direction{ID: id, SubjectCodes: subjects})
	}
	cases := []struct {
		interests []string
		work      string
		subjects  []string
		want      []string
	}{
		{[]string{"code", "tasks"}, "build", nil, []string{"napr-09-03-04", "napr-01-03-02", "napr-09-03-01"}},
		{[]string{"code", "tasks"}, "build", []string{"inf", "math"}, []string{"napr-09-03-04", "napr-01-03-02", "napr-09-03-01"}},
		{[]string{"bio"}, "health", nil, []string{"napr-31-05-01", "napr-33-05-01"}},
		// «Пока не знаю» не ломает подсчёт: два лучших по интересу.
		{[]string{"world"}, "unknown", nil, []string{"napr-03-03-01", "napr-03-03-02"}},
		// Направление без веса получает только баллы за предметы.
		{[]string{"society"}, "unknown", []string{"inf"}, []string{"napr-01-03-02", "napr-09-03-01"}},
	}
	for _, c := range cases {
		if got := suggestDirections(c.interests, c.work, c.subjects, dirs); !slices.Equal(got, c.want) {
			t.Errorf("%v %s %v: %v, ждали %v", c.interests, c.work, c.subjects, got, c.want)
		}
	}
}

func TestAliasUniversity(t *testing.T) {
	for q, want := range map[string]string{"Вышка": "hse", "вшэ": "hse", "Казанский федеральный университет": "kfu",
		"первый мед": "sechenov", "физтех!": "mipt", "мгушка": ""} {
		if got := aliasUniversity(q); got != want {
			t.Errorf("%q: %q, ждали %q", q, got, want)
		}
	}
}

func TestStaleButtonChangesNothing(t *testing.T) {
	h := newHarness(t)
	h.started(artem, "")
	h.press(artem, "role:kid")
	h.press(artem, "name:ok")
	// Нажали «Я родитель» в первом сообщении — диалог уже на классе.
	id := h.pressAny(artem, "role:parent", "Кто вы?")
	if a := h.answered(id); a.Notification != "Этот вопрос уже позади" {
		t.Fatalf("старое нажатие: %+v", a)
	}
	h.mustContain(artem, "В каком ты классе?") // текущий вопрос переспрошен
	uid, _ := h.st.UpsertUser(context.Background(), artem.UserID, "Артём")
	if d, _ := h.st.Dialog(context.Background(), uid); d.Role != "kid" || d.Step != stepGrade {
		t.Fatalf("диалог не изменился: %+v", d)
	}
}

func TestDuplicateDeliveryIsIgnored(t *testing.T) {
	h := newHarness(t)
	h.started(artem, "")
	cb := maxapi.Update{UpdateType: maxapi.UpdateMessageCallback, Timestamp: 1,
		Callback: &maxapi.Callback{CallbackID: "same-id", Payload: "role:kid", User: artem}}
	for i := 0; i < 2; i++ {
		if err := h.bot.Handle(context.Background(), cb); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(h.fake.Answered); n != 1 {
		t.Fatalf("повторная доставка — без второго ответа: %d", n)
	}
}

func TestUnknownStartPayloadIsPlainWelcome(t *testing.T) {
	h := newHarness(t)
	h.started(artem, "inv_") // битое приглашение
	h.mustContain(artem, "Кто вы?")
	h.text(artem, "/start <script>")
	h.mustContain(artem, "Кто вы?")
}

func TestInvite_KidJoinsParentTrajectoryOnce(t *testing.T) {
	h := newHarness(t)
	// Ольга создаёт траекторию сама.
	uid, _ := h.st.UpsertUser(context.Background(), olga.UserID, olga.FirstName)
	parent, err := h.st.CreateTrajectory(context.Background(), store.NewTrajectory{CreatorUserID: uid, Role: "parent",
		StudentName: "Артём", Grade: 9, RegionCode: "16", TZ: "Europe/Moscow",
		DirectionIDs: []string{"napr-09-03-04"}, SubjectCodes: []string{"inf"}, UniversityIDs: []string{"hse"}})
	if err != nil {
		t.Fatal(err)
	}
	h.text(olga, "/menu")
	h.fake.Sent = append(h.fake.Sent, maxtest.Sent{UserID: olga.UserID, Msg: maxapi.WithKeyboard("итог",
		maxapi.Keyboard{maxapi.Row(maxapi.CallbackButton("👋", "inv:new"))})})
	h.press(olga, "inv:new")
	link := h.lastText(olga)
	if !strings.HasPrefix(link, "Перешлите Артёму эту ссылку в MAX.") || !strings.Contains(link, "https://max.ru/test_bot?start=inv_") {
		t.Fatalf("ссылка: %q", link)
	}
	token := link[strings.Index(link, "start=inv_")+len("start=inv_"):]

	h.started(artem, "inv_"+token)
	h.mustContain(artem, "**Проверь, всё ли верно:**\nКласс: 9 · Татарстан\nПредметы: информатика\nЦель: программная инженерия\nГде учиться: не важно\nВузы: ВШЭ")
	if hello := h.fake.To(artem.UserID)[0].Msg.Text; hello != "Привет, Артём! 👋 Ольга приглашает тебя в «Траекторию» — подборка олимпиад уже готова." {
		t.Fatalf("приветствие приглашённого: %q", hello)
	}
	if got := h.lastText(olga); got != "✓ Артём теперь в траектории." {
		t.Fatalf("пригласившему: %q", got)
	}
	h.press(artem, "join:ok")
	h.mustContain(artem, "Отлично! Под твою цель подходят ")
	m, _ := h.st.CurrentMember(context.Background(), artem.UserID)
	if m.TrajectoryID != parent.TrajectoryID || m.Role != "kid" || m.IsCreator {
		t.Fatalf("ученик в траектории Ольги: %+v", m)
	}

	// Та же ссылка второй раз — недействительна.
	h.started(igor, "inv_"+token)
	h.mustContain(igor, "Приглашение уже недействительно")
}

// «Изменить цель» приглашённого (F42): те же направления с «✓», можно
// добавить ещё одно.
func TestInvite_KidChangesGoal(t *testing.T) {
	h := newHarness(t)
	uid, _ := h.st.UpsertUser(context.Background(), olga.UserID, olga.FirstName)
	parent, err := h.st.CreateTrajectory(context.Background(), store.NewTrajectory{CreatorUserID: uid, Role: "parent",
		StudentName: "Артём", Grade: 9, RegionCode: "16", TZ: "Europe/Moscow",
		DirectionIDs: []string{"napr-09-03-04"}, SubjectCodes: []string{"inf", "math"}, UniversityIDs: []string{"hse"}})
	if err != nil {
		t.Fatal(err)
	}
	const token = "kidInviteToken01"
	if _, err := h.st.CreateInvite(context.Background(), parent.TrajectoryID, parent.MemberID, "kid", token); err != nil {
		t.Fatal(err)
	}
	h.started(artem, "inv_"+token)
	h.press(artem, "join:goal")
	h.mustContain(artem, "Куда думаешь поступать?")
	if b := maxtest.Buttons(h.fake.Last(artem.UserID)); !strings.Contains(b, "✓ Программная инженерия") {
		t.Fatalf("текущая цель отмечена: %s", b)
	}
	h.press(artem, "dir:t:napr-01-03-02")
	h.press(artem, "dir:done")
	tr, _ := h.st.Trajectory(context.Background(), parent.TrajectoryID)
	if len(tr.Directions) != 2 || tr.Directions[1].ID != "napr-01-03-02" || tr.GoalStatus != "known" {
		t.Fatalf("цель после правки: %+v", tr.Directions)
	}
	h.mustContain(artem, "Под твою цель подходят ")
}

func ptr(s string) *string { return &s }

func mustVoice(h *harness, u maxapi.User, m store.Member) voice.Voice {
	v, _, err := h.bot.voiceOf(&turn{ctx: context.Background(), user: u}, m)
	if err != nil {
		h.t.Fatal(err)
	}
	return v
}

func TestDelete_CreatorAndInvited(t *testing.T) {
	h := newHarness(t)
	m := h.kidOnboarding()
	uid, _ := h.st.UpsertUser(context.Background(), olga.UserID, olga.FirstName)
	if _, err := h.st.AddMember(context.Background(), m.TrajectoryID, uid, "parent"); err != nil {
		t.Fatal(err)
	}
	h.text(olga, "/delete")
	h.mustContain(olga, "Удалить траекторию может только её создатель — Артём.")
	h.press(olga, "del:no")
	h.mustContain(olga, "Хорошо, ничего не удаляю.")

	h.text(artem, "/delete")
	h.mustContain(artem, "Удалить свою траекторию и все данные? Доступ потеряют: Ольга.")
	h.press(artem, "del:yes")
	h.mustContain(artem, "Траектория и все данные удалены.")
	h.mustContain(olga, "удалена её создателем")
	if _, err := h.st.CurrentMember(context.Background(), olga.UserID); err == nil {
		t.Fatal("доступ закрыт")
	}
}

func TestLeave_InvitedMember(t *testing.T) {
	h := newHarness(t)
	m := h.kidOnboarding()
	uid, _ := h.st.UpsertUser(context.Background(), olga.UserID, olga.FirstName)
	_, _ = h.st.AddMember(context.Background(), m.TrajectoryID, uid, "parent")
	h.text(olga, "/delete")
	h.press(olga, "leave:yes")
	h.mustContain(olga, "Готово: вы больше не в траектории Артёма.")
	h.mustContain(artem, "Ольга больше не в траектории.")
}

func TestSettings_ToggleOffsets(t *testing.T) {
	h := newHarness(t)
	h.kidOnboarding()
	h.text(artem, "/settings")
	msg := h.fake.Last(artem.UserID)
	if msg.Text != "Когда напоминать о сроках?" || maxtest.Buttons(msg) != "✓ За месяц | ✓ За неделю | ✓ За 3 дня | ✓ За 1 день" {
		t.Fatalf("настройки: %q %s", msg.Text, maxtest.Buttons(msg))
	}
	id := h.pressAny(artem, "set:rem:30", msg.Text)
	a := h.answered(id)
	if a.Message == nil || maxtest.Buttons(*a.Message) != "За месяц | ✓ За неделю | ✓ За 3 дня | ✓ За 1 день" {
		t.Fatalf("клавиатура правится на месте: %+v", a)
	}
	m, _ := h.st.CurrentMember(context.Background(), artem.UserID)
	if slices.Contains(m.ReminderOffsets, 30) {
		t.Fatalf("порог выключен: %v", m.ReminderOffsets)
	}
}

func TestRemindOn_FillsEmptyTrackerAndPlans(t *testing.T) {
	h := newHarness(t)
	m := h.kidOnboarding()
	// Кнопка из итога прежних версий: в новом итоге её нет.
	h.pressAny(artem, "rem:on", "итог")
	h.mustContain(artem, "Буду напоминать за месяц, неделю, 3 дня и 1 день до каждого срока.")
	h.mustContain(artem, "Добавил в трекер олимпиады из подборки: 7 олимпиад.")
	items, _ := h.st.TrackerItems(context.Background(), m.TrajectoryID)
	var planned int
	_ = h.db.QueryRow(context.Background(), `SELECT count(*) FROM reminders WHERE status = 'planned'`).Scan(&planned)
	if len(items) != 7 || planned == 0 {
		t.Fatalf("трекер заполнен (%d), напоминания запланированы (%d)", len(items), planned)
	}
}

func TestReminderButtons_MarkAndSnooze(t *testing.T) {
	h := newHarness(t)
	m := h.kidOnboarding()
	uid, _ := h.st.UpsertUser(context.Background(), olga.UserID, olga.FirstName)
	_, _ = h.st.AddMember(context.Background(), m.TrajectoryID, uid, "parent")
	item, _, _ := h.st.AddTrackerItem(context.Background(), m.TrajectoryID, "p669-8-informatika", m.MemberID)
	_ = h.st.SyncReminders(context.Background(), m.TrajectoryID, 10, h.bot.now())
	due, _ := h.st.DueReminders(context.Background(), time.Date(2026, 9, 19, 10, 5, 0, 0, msk), 10)
	var reminderID string
	for _, d := range due {
		if d.StageKind == "registration" {
			reminderID = d.ID
		}
	}
	h.bot.now = func() time.Time { return time.Date(2026, 9, 19, 10, 5, 0, 0, msk) }

	id := h.pressAny(olga, "rem:snooze:"+reminderID, "⏰")
	if a := h.answered(id); a.Notification != "Хорошо, напомню завтра в 10:00." {
		t.Fatalf("напомнить завтра: %+v", a)
	}
	id = h.pressAny(olga, "rem:done:"+item, "⏰")
	if a := h.answered(id); !strings.HasPrefix(a.Notification, "✅ Отмечено: регистрация на «Высшая проба»") {
		t.Fatalf("отметка: %+v", a)
	}
	h.mustContain(artem, "✅ Ольга отмечает: регистрация на «Высшая проба» пройдена.")
	row, _ := h.st.TrackerItem(context.Background(), m.TrajectoryID, item)
	if row.RegisteredBy == nil || row.RegisteredBy.Name != "Ольга" {
		t.Fatalf("отметила Ольга: %+v", row.RegisteredBy)
	}
	// Чужой пункт: пользователь не в той траектории.
	id = h.pressAny(igor, "rem:done:"+item, "⏰")
	if a := h.answered(id); a.Notification != "Этой олимпиады уже нет в трекере." {
		t.Fatalf("чужая кнопка: %+v", a)
	}
}

func TestProposal_KidAcceptsFromChat(t *testing.T) {
	h := newHarness(t)
	m := h.kidOnboarding()
	uid, _ := h.st.UpsertUser(context.Background(), olga.UserID, olga.FirstName)
	parent, _ := h.st.AddMember(context.Background(), m.TrajectoryID, uid, "parent")
	pid, _, err := h.st.CreateProposal(context.Background(), m.TrajectoryID, "p669-8-informatika", parent.MemberID)
	if err != nil {
		t.Fatal(err)
	}
	h.bot.notify.ProposalCreated(context.Background(), m.TrajectoryID, pid, parent, h.bot.now())
	h.mustContain(artem, "📩 Ольга предлагает добавить в трекер «Высшая проба».")
	h.press(artem, "prop:acc:"+pid)
	h.mustContain(artem, "Добавил. Ольга увидит это в общей траектории")
	h.mustContain(olga, "✅ Артём добавляет «Высшая проба» в трекер.")
	id := h.pressAny(artem, "prop:dec:"+pid, "📩")
	if a := h.answered(id); a.Notification != "На это предложение уже ответили." {
		t.Fatalf("повторный ответ: %+v", a)
	}
}

// После «Готово» у вопроса остаётся выбор, а не «✓ Готово» (прототип, экраны A4 и A6).
func TestMultiselectDoneKeepsPicked(t *testing.T) {
	h := newHarness(t)
	h.toRegion("9")
	h.geo(artem, 55.79, 49.11)
	h.press(artem, "subj:t:inf")
	h.press(artem, "subj:t:math")
	if a := h.answered(h.press(artem, "subj:done")); a.Message == nil || maxtest.Buttons(*a.Message) != "✓ Информатика | ✓ Математика" {
		t.Fatalf("предметы после «Готово»: %+v", a.Message)
	}
	h.press(artem, "dir:t:napr-09-03-04")
	h.press(artem, "dir:t:napr-09-03-01")
	if a := h.answered(h.press(artem, "dir:done")); a.Message == nil ||
		maxtest.Buttons(*a.Message) != "✓ Информатика и вычислительная техника | ✓ Программная инженерия" {
		t.Fatalf("направления после «Готово»: %+v", a.Message)
	}
	if a := h.answered(h.press(artem, "exp:region")); a.Message == nil || maxtest.Buttons(*a.Message) != "✓ Региональный этап и выше" {
		t.Fatalf("опыт: %+v", a.Message)
	}
	h.press(artem, "place:t:0")
	h.press(artem, "place:t:1")
	if a := h.answered(h.press(artem, "place:done")); a.Message == nil ||
		maxtest.Buttons(*a.Message) != "✓ Только Казань | ✓ Весь регион · Татарстан" {
		t.Fatalf("места после «Готово»: %+v", a.Message)
	}
	h.press(artem, "vuz:t:innopolis")
	h.press(artem, "vuz:t:kfu")
	a := h.answered(h.press(artem, "vuz:done"))
	if a.Message == nil || !strings.Contains(maxtest.Buttons(*a.Message), "✓ Иннополис") ||
		!strings.Contains(maxtest.Buttons(*a.Message), "✓ КФУ · Казань") || strings.Contains(maxtest.Buttons(*a.Message), "Готово") {
		t.Fatalf("вузы после «Готово»: %+v", a.Message)
	}
}

// «Изменить» открывает меню на той же карточке, «Назад» закрывает (SPEC 9).
func TestSummary_EditMenuAndBack(t *testing.T) {
	h := newHarness(t)
	h.toSummary()
	n := len(h.fake.To(artem.UserID))
	a := h.answered(h.press(artem, "sum:edit"))
	if a.Message == nil || !strings.HasSuffix(a.Message.Text, "\n\nЧто поменять?") || a.Message.Format != "markdown" ||
		!slices.Equal(maxtest.Payloads(*a.Message), []string{"sum:f:name", "sum:f:grade", "sum:f:region", "sum:f:subjects",
			"sum:f:goal", "sum:f:exp", "sum:f:places", "sum:f:vuz", "sum:back"}) {
		t.Fatalf("меню правки: %+v", a.Message)
	}
	a = h.answered(h.pressAny(artem, "sum:back", "профиль"))
	if a.Message == nil || !slices.Equal(maxtest.Payloads(*a.Message), []string{"sum:edit", "sum:ok"}) {
		t.Fatalf("«Назад»: %+v", a.Message)
	}
	if len(h.fake.To(artem.UserID)) != n {
		t.Fatal("меню правится на месте, новых сообщений нет")
	}
}

// Правка класса: вопрос без счётчика и приветствия, затем снова профиль;
// траектория создаётся с новым классом только по «Готово».
func TestSummary_EditGrade(t *testing.T) {
	h := newHarness(t)
	h.toSummary()
	h.edit("grade")
	if got := h.lastText(artem); got != "В каком ты классе?" {
		t.Fatalf("вопрос при правке: %q", got)
	}
	h.press(artem, "grade:10")
	h.mustContain(artem, "**Артём, сформировали твой профиль**\nКласс: 10 · Казань\n")
	if d := h.dialog(artem); d.Step != stepSummary || d.Draft.EditStage != 0 || d.Draft.RegionCode != "16" {
		t.Fatalf("после правки — профиль, регион прежний: %+v", d)
	}
	h.press(artem, "sum:ok")
	if tr := h.trajectory(artem); tr.Grade != 10 || len(tr.Directions) != 1 {
		t.Fatalf("траектория: %+v", tr)
	}
}

// «Помоги выбрать» при правке направлений проходит целиком — это тот же шаг.
func TestSummary_EditDirectionsWithHelp(t *testing.T) {
	h := newHarness(t)
	h.toSummary()
	h.edit("goal")
	if got := h.lastText(artem); !strings.HasPrefix(got, "Куда думаешь поступать?") {
		t.Fatalf("вопрос без счётчика: %q", got)
	}
	h.press(artem, "dir:help")
	h.press(artem, "int:t:code")
	h.press(artem, "int:done")
	h.press(artem, "work:build")
	h.mustContain(artem, "Похоже, тебе подойдут эти направления.")
	h.press(artem, "dir:done")
	h.mustContain(artem, " · подобрали вместе\nОпыт: школьный или муниципальный этап\n")
	if d := h.dialog(artem); d.Step != stepSummary || d.Draft.Experience != "school" {
		t.Fatalf("опыт не спрашивали заново: %+v", d)
	}
}

// Регион при правке: без кнопки «Не то», сразу профиль.
func TestSummary_EditRegion(t *testing.T) {
	h := newHarness(t)
	h.toSummary()
	h.edit("region")
	h.text(artem, "Москва")
	if found := h.sentBack(artem, 1); found.Text != "Москва ✓" || len(found.Keyboard()) != 0 {
		t.Fatalf("найдено: %q %v", found.Text, maxtest.Payloads(found))
	}
	h.mustContain(artem, "Класс: 9 · Москва\n")
	if d := h.dialog(artem); d.Step != stepSummary || d.Draft.RegionCode != "77" || d.Draft.HomeCity != "" {
		t.Fatalf("регион: %+v", d)
	}
}

// Вузы: «← Назад» при правке — к меню «Что поменять?», места правятся своим
// полем. «Изменить места» со старого сообщения — шаг назад, правка
// продолжается до «Готово» вузов.
func TestSummary_EditUniversitiesViaPlaces(t *testing.T) {
	h := newHarness(t)
	h.toSummary()
	h.edit("vuz")
	h.press(artem, "back:universities")
	if d := h.dialog(artem); d.Step != stepSummary || !d.Draft.EditMenu || d.Draft.EditStage != 0 {
		t.Fatalf("назад с вузов при правке: %+v", d)
	}
	h.pressAny(artem, "sum:f:vuz", "профиль")
	h.pressAny(artem, "vuz:places", "")
	if d := h.dialog(artem); d.Step != stepTarget || d.Draft.EditStage != 9 {
		t.Fatalf("места при правке вузов: %+v", d)
	}
	h.press(artem, "place:any")
	h.press(artem, "vuz:t:mipt")
	h.press(artem, "vuz:done")
	h.mustContain(artem, "Где учиться: не важно\nВузы: МФТИ\n")
}

// «Готово» из старой карточки после создания траектории ничего не меняет.
func TestSummary_StaleDone(t *testing.T) {
	h := newHarness(t)
	h.toSummary()
	h.press(artem, "sum:ok")
	id := h.pressAny(artem, "sum:ok", "профиль")
	if a := h.answered(id); a.Notification == "" {
		t.Fatalf("старая кнопка: %+v", a)
	}
	var n int
	_ = h.db.QueryRow(context.Background(), `SELECT count(*) FROM trajectories`).Scan(&n)
	if n != 1 {
		t.Fatalf("траекторий: %d", n)
	}
}
