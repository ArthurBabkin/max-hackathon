package bot

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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

// register — «Начать» и «Принимаю»: согласие с политикой спрашивается до
// анкеты и до входа по приглашению.
func (h *harness) register(u maxapi.User, payload string) {
	h.t.Helper()
	h.started(u, payload)
	h.press(u, "consent:ok")
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

// step — последний вопрос начинается со счётчика «Шаг n из 5».
func (h *harness) step(u maxapi.User, n int) {
	h.t.Helper()
	if got := h.lastText(u); !strings.HasPrefix(got, fmt.Sprintf("Шаг %d из 5\n\n", n)) {
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

// oldDialog — ученик с согласием и анкетой, брошенной на шаге step.
func (h *harness) oldDialog(u maxapi.User, step string, dr store.Draft) {
	h.t.Helper()
	ctx := context.Background()
	uid, _ := h.st.UpsertUser(ctx, u.UserID, u.FirstName)
	_ = h.st.AcceptPrivacy(ctx, uid)
	if _, err := h.st.StartDialog(ctx, store.Dialog{UserID: uid, Step: step, Role: "kid", Draft: dr}); err != nil {
		h.t.Fatal(err)
	}
}

// toSubjects — ученик Артём до вопроса о предметах.
func (h *harness) toSubjects(grade string) {
	h.t.Helper()
	h.register(artem, "")
	h.press(artem, "role:kid")
	h.press(artem, "grade:"+grade)
	h.mustContain(artem, "Шаг 3 из 5\n\nКакие предметы тебе нравятся?")
}

// toUniversities — ученик с предметами до вопроса о вузах.
func (h *harness) toUniversities(subjects ...string) {
	h.t.Helper()
	h.toSubjects("9")
	for _, s := range subjects {
		h.press(artem, "subj:t:"+s)
	}
	h.press(artem, "subj:done")
}

// toDirections — ученик с предметами, без вузов, до вопроса о направлениях.
func (h *harness) toDirections(subjects ...string) {
	h.t.Helper()
	h.toUniversities(subjects...)
	h.press(artem, "vuz:done")
}

// kidOnboarding проходит анкету ученика целиком (ТЗ §5.1, SPEC 3–8):
// роль, класс, предметы, вузы, направления.
func (h *harness) kidOnboarding() store.Member {
	h.t.Helper()
	h.register(artem, "src_class_9a")
	h.step(artem, 1)
	h.mustContain(artem, "Кто вы?")
	h.press(artem, "role:kid")
	// Имя — из профиля MAX, без вопроса.
	h.step(artem, 2)
	h.mustContain(artem, "Приятно познакомиться, Артём! В каком ты классе?")
	h.press(artem, "grade:9")
	h.step(artem, 3)
	h.mustContain(artem, "Какие предметы тебе нравятся?")
	h.press(artem, "subj:t:inf")
	h.press(artem, "subj:t:math")
	h.press(artem, "subj:done")
	h.step(artem, 4)
	h.mustContain(artem, "Куда хочешь поступать? Сверху — вузы, где больше направлений под твои предметы.")
	h.text(artem, "вышка")
	if got := h.sentBack(artem, 1).Text; got != "Добавил ВШЭ." {
		h.t.Fatalf("поиск по алиасу: %q", got)
	}
	// Иннополис с одним ИТ-направлением — на второй странице.
	more := h.answered(h.press(artem, "vuz:more"))
	h.pressAny(artem, "vuz:t:innopolis", more.Message.Text)
	h.press(artem, "vuz:done")
	h.step(artem, 5)
	h.mustContain(artem, "Какое направление интересно? В выбранных вузах под твои предметы подходят эти")
	h.press(artem, "dir:t:napr-09-03-04")
	h.press(artem, "dir:t:napr-09-03-01")
	// «Готово» на направлениях создаёт траекторию: карточки профиля нет.
	h.press(artem, "dir:done")
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
	// Регион, опыт и места анкета не спрашивает: московское время, опыта
	// нет, учиться где угодно.
	tr, _ := h.st.Trajectory(context.Background(), m.TrajectoryID)
	if tr.StudentName != "Артём" || tr.Grade != 9 || tr.RegionCode != "" || tr.TZ != "Europe/Moscow" ||
		len(tr.Directions) != 2 || tr.Directions[0].ID != "napr-09-03-04" || tr.Directions[1].ID != "napr-09-03-01" ||
		tr.GoalStatus != "known" || tr.Experience != "" || tr.HomeCity != nil || len(tr.Places) != 0 || tr.GoalByKid {
		t.Fatalf("траектория: %+v", tr)
	}
	unis, _ := h.st.TrajectoryUniversities(context.Background(), tr.ID)
	ids := []string{}
	for _, u := range unis {
		ids = append(ids, u.ID)
	}
	if slices.Sort(ids); !slices.Equal(ids, []string{"hse", "innopolis"}) {
		t.Fatalf("вузы: %v", ids)
	}
	// Итог — сразу после вопроса о направлениях.
	if s := h.sentBack(artem, 1).Text; !strings.HasPrefix(s, "Шаг 5 из 5\n\nКакое направление интересно?") {
		t.Fatalf("перед итогом — вопрос о направлениях: %q", s)
	}
	result := h.fake.Last(artem.UserID)
	// Опыта нет — стартовая для первых олимпиад, она же ближайшая.
	if !strings.HasPrefix(result.Text, "Под твою цель подходят 5 олимпиад и ВсОШ.\n\nДля старта советую эту — она же ближайшая:\nФизтех · Информатика\nIII уровень") ||
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
// Опыт анкета не спрашивает, он есть у диалогов, начатых до #127: с ним
// стартовая — по опыту.
func TestResult_NearestAfterStart(t *testing.T) {
	h := newHarness(t)
	h.oldDialog(artem, stepSummary, store.Draft{Name: "Артём", Grade: 9, RegionCode: "16", HomeCity: "Казань",
		SubjectCodes: []string{"inf", "math"}, DirectionIDs: []string{"napr-09-03-04", "napr-09-03-01"},
		Experience: "school", Places: []store.Place{{RegionCode: "16"}}, UniversityIDs: []string{"innopolis", "hse"}})
	h.pressAny(artem, "sum:ok", "профиль")
	h.press(artem, "vuz:done")
	h.press(artem, "dir:done")
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
	blocks := strings.Split(h.lastText(artem), "\n\n")
	if len(blocks) != 3 || !strings.HasPrefix(blocks[1], "Для старта советую эту — она же ближайшая:\nФизтех · Информатика\n") ||
		!strings.Contains(blocks[1], "до 7 сентября, 6 дней") {
		t.Fatalf("итог: %q", blocks)
	}
}

func TestParentOnboarding_Undecided(t *testing.T) {
	h := newHarness(t)
	h.register(olga, "")
	h.press(olga, "role:parent")
	// Имя ребёнка — часть первого шага: номер тот же.
	h.step(olga, 1)
	h.mustContain(olga, "Как зовут вашего ребёнка?")
	h.text(olga, strings.Repeat("я", 41))
	h.mustContain(olga, "от 1 до 40 символов")
	h.text(olga, "  Артём  ")
	h.step(olga, 2)
	h.mustContain(olga, "В каком классе Артём?")
	h.press(olga, "grade:10")
	h.step(olga, 3)
	h.mustContain(olga, "Какие предметы нравятся Артёму?")
	id := h.press(olga, "subj:done")
	if a := h.answered(id); a.Notification != "Выберите хотя бы один предмет" {
		t.Fatalf("«Готово» без предметов: %+v", a)
	}
	h.press(olga, "subj:t:phys")
	h.press(olga, "subj:done")

	// Вузы — по предметам; выбрать можно позже.
	h.step(olga, 4)
	h.mustContain(olga, "Куда хочет поступать Артём? Сверху — вузы, где больше направлений под выбранные предметы.")
	if b := maxtest.Buttons(h.fake.Last(olga.UserID)); !strings.HasSuffix(b, "Показать ещё | Выберем позже | ← Назад") {
		t.Fatalf("кнопки вузов: %s", b)
	}
	if a := h.answered(h.press(olga, "vuz:done")); a.Message == nil || maxtest.Buttons(*a.Message) != "✓ Выберем позже" {
		t.Fatalf("«Выберем позже» на месте вопроса: %+v", a.Message)
	}

	// Вузов нет — под физику только физические направления, остальные по кнопке.
	h.step(olga, 5)
	h.mustContain(olga, "Какое направление интересно Артёму? Под выбранные предметы подходят")
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
	// «Пока не знаем» — конец анкеты.
	h.pressAny(olga, "dir:later", "")
	tr := h.trajectory(olga)
	m, _ := h.st.CurrentMember(context.Background(), olga.UserID)
	if m.Role != "parent" || !m.IsCreator || m.HasKid {
		t.Fatalf("родитель — создатель без ученика: %+v", m)
	}
	if tr.GoalStatus != "exploring" || len(tr.Directions) != 0 || len(tr.Places) != 0 || tr.Grade != 10 ||
		tr.RegionCode != "" || tr.Experience != "" || tr.HomeCity != nil || tr.GoalByKid {
		t.Fatalf("цель не выбрана: %+v", tr)
	}
	if unis, _ := h.st.TrajectoryUniversities(context.Background(), tr.ID); len(unis) != 0 {
		t.Fatalf("вузы не выбраны: %+v", unis)
	}
	result := h.fake.Last(olga.UserID)
	if !strings.HasPrefix(result.Text, "По предметам Артёма подходят ") {
		t.Fatalf("итог родителю: %q", result.Text)
	}
	if p := maxtest.Payloads(result); !slices.Equal(p, []string{"home"}) {
		t.Fatalf("у родителя одна кнопка: %s", maxtest.Buttons(result))
	}
}

// «Не знаю — помоги выбрать»: В1, В2, предложенные направления (SPEC 5).
func TestDirectionHelp_KidFlow(t *testing.T) {
	h := newHarness(t)
	h.toDirections("inf", "math")
	h.press(artem, "dir:help")
	h.mustContain(artem, "Два вопроса про интересы.")
	h.step(artem, 5)
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
	h.step(artem, 5)
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
	tr := h.trajectory(artem)
	if tr.GoalStatus != "suggested" || len(tr.Directions) != 2 || tr.Directions[0].ID != "napr-09-03-04" ||
		tr.Directions[1].ID != "napr-01-03-02" {
		t.Fatalf("предложенная цель: %+v", tr)
	}
}

// Родитель: «Помогите выбрать», «Пока не знаем», «Всё равно не знаем».
func TestDirectionHelp_ParentStillUndecided(t *testing.T) {
	h := newHarness(t)
	h.register(olga, "")
	h.press(olga, "role:parent")
	h.text(olga, "Артём")
	h.press(olga, "grade:11")
	h.press(olga, "subj:t:bio")
	h.press(olga, "subj:done")
	h.press(olga, "vuz:done")
	h.press(olga, "dir:help")
	h.mustContain(olga, "Два вопроса про интересы. Что из этого больше всего нравится Артёму?")
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
	tr := h.trajectory(olga)
	if tr.GoalStatus != "exploring" || len(tr.Directions) != 0 || tr.Experience != "" || len(tr.Places) != 0 {
		t.Fatalf("траектория: %+v", tr)
	}
}

// Вузы: по пять, сначала подходящие под предметы, «Показать ещё» и
// «Выберу позже», после отметки — «Готово» (SPEC 8.1).
func TestUniversities_MoreAndLater(t *testing.T) {
	h := newHarness(t)
	h.toUniversities("inf", "math")
	last := h.fake.Last(artem.UserID)
	p := maxtest.Payloads(last)
	if !slices.Contains(p, "vuz:more") || strings.Count(strings.Join(p, " "), "vuz:t:") != 5 {
		t.Fatalf("первые 5 вузов и «Показать ещё»: %v", p)
	}
	// Под информатику и математику медицинских вузов на первой странице нет.
	if !slices.Contains(p, "vuz:t:hse") || slices.Contains(p, "vuz:t:sechenov") || slices.Contains(p, "vuz:t:kazan-gmu") {
		t.Fatalf("порядок вузов по предметам: %v", p)
	}
	if b := maxtest.Buttons(last); !strings.Contains(b, "ВШЭ · Москва") || !strings.HasSuffix(b, "Показать ещё | Выберу позже | ← Назад") {
		t.Fatalf("кнопки вузов: %s", b)
	}
	if strings.Contains(last.Text, "в этих местах") {
		t.Fatalf("мест анкета не спрашивает: %q", last.Text)
	}
	a := h.answered(h.press(artem, "vuz:more"))
	if a.Message == nil || strings.Count(strings.Join(maxtest.Payloads(*a.Message), " "), "vuz:t:") != 10 ||
		slices.Contains(maxtest.Payloads(*a.Message), "vuz:more") {
		t.Fatalf("все 10 вузов: %+v", a.Message)
	}
	a = h.answered(h.pressAny(artem, "vuz:t:hse", a.Message.Text))
	if a.Message == nil || !strings.HasSuffix(maxtest.Buttons(*a.Message), "Готово | ← Назад") {
		t.Fatalf("вуз отмечен — «Готово»: %+v", a.Message)
	}
	h.pressAny(artem, "vuz:done", a.Message.Text)
	h.step(artem, 5)
}

// Направления выбранных вузов под предметы — по пять (#129): сначала те,
// где ученик сдаёт все ключевые предметы, дальше «Показать ещё», потом
// «Показать все направления».
func TestDirections_TopFiveAndMore(t *testing.T) {
	h := newHarness(t)
	h.toUniversities("inf", "math")
	h.press(artem, "vuz:t:hse")
	h.press(artem, "vuz:done")
	h.step(artem, 5)
	h.mustContain(artem, "Какое направление интересно? В выбранных вузах под твои предметы подходят эти")
	q := h.fake.Last(artem.UserID)
	if b := maxtest.Buttons(q); !strings.HasPrefix(b, "Информатика и вычислительная техника | Информационная безопасность | "+
		"Прикладная математика и информатика | Программная инженерия | Физика | Показать ещё | Готово | Решу позже") {
		t.Fatalf("первые пять направлений ВШЭ: %s", b)
	}
	a := h.answered(h.press(artem, "dir:more"))
	p := maxtest.Payloads(*a.Message)
	if strings.Count(strings.Join(p, " "), "dir:t:") != 8 || slices.Contains(p, "dir:more") || !slices.Contains(p, "dir:all") ||
		slices.Contains(p, "dir:t:napr-06-03-01") {
		t.Fatalf("все направления ВШЭ под предметы, без биологии: %v", p)
	}
	a = h.answered(h.pressAny(artem, "dir:all", a.Message.Text))
	p = maxtest.Payloads(*a.Message)
	if !strings.HasPrefix(a.Message.Text, "Шаг 5 из 5\n\nКакое направление интересно? Отметь") ||
		strings.Count(strings.Join(p, " "), "dir:t:") != 15 || !slices.Contains(p, "dir:more") {
		t.Fatalf("остальные направления — следующей страницей: %q %v", a.Message.Text, p)
	}
	// Отмеченное со второй страницы остаётся на виду и после «Назад».
	h.pressAny(artem, "dir:t:napr-38-03-01", a.Message.Text)
	q = h.tap(artem, q, "back:direction")
	h.pressAny(artem, "vuz:done", q.Text)
	if b := maxtest.Buttons(h.fake.Last(artem.UserID)); !strings.Contains(b, "✓ Экономика") || !strings.Contains(b, "Показать ещё") {
		t.Fatalf("отмеченное на виду, список снова с первой страницы: %s", b)
	}
}

// Родитель передаёт вопросы об интересах ребёнку (SPEC 10).
func TestParentAsksKid_KidAnswersAfterJoin(t *testing.T) {
	h := newHarness(t)
	h.register(olga, "")
	h.press(olga, "role:parent")
	h.text(olga, "Артём")
	h.press(olga, "grade:10")
	h.press(olga, "subj:t:bio")
	h.press(olga, "subj:done")
	h.press(olga, "vuz:done")
	// «Пусть ответит» — конец анкеты: траектория создаётся сразу.
	h.press(olga, "dir:kid")
	if got := h.sentBack(olga, 1).Text; !strings.HasPrefix(got, "Хорошо. Сейчас дам ссылку-приглашение: Артём ответит") {
		t.Fatalf("ответ на «Пусть ответит»: %q", got)
	}
	tr := h.trajectory(olga)
	if !tr.GoalByKid || len(tr.Directions) != 0 {
		t.Fatalf("цель ждёт ребёнка: %+v", tr)
	}
	result := h.fake.Last(olga.UserID)
	if !strings.HasSuffix(result.Text, "Отправьте ссылку Артёму: ответы про интересы уточнят подборку.") ||
		result.Keyboard()[0][0].Payload != "inv:new" || result.Keyboard()[0][0].Text != "👋 Пригласить Артёма" {
		t.Fatalf("приглашение первым: %q %s", result.Text, maxtest.Buttons(result))
	}
	h.press(olga, "inv:new")
	link := h.lastText(olga)
	token := link[strings.Index(link, "start=inv_")+len("start=inv_"):]

	h.register(artem, "inv_"+token)
	check := h.fake.Last(artem.UserID)
	if check.Text != "**Проверь, всё ли верно:**\nКласс: 10\nПредметы: биология\nЦель: ждёт твоего ответа" || check.Format != "markdown" {
		t.Fatalf("проверка профиля: %q", check.Text)
	}
	h.press(artem, "join:ok")
	h.mustContain(artem, "Два вопроса про интересы")
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
	h.register(artem, "")
	h.press(artem, "role:kid")
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

// Анкета, брошенная на шаге, которого больше нет (регион, «Тебя зовут…?»,
// опыт, «Где учиться», карточка профиля), продолжается со следующего
// вопроса новой анкеты; ответы, данные раньше, попадают в траекторию.
func TestRetiredSteps_ResumeQuestionnaire(t *testing.T) {
	h := newHarness(t)
	pupil := func(n int, step string, dr store.Draft) maxapi.User {
		u := maxapi.User{UserID: int64(900000100 + n), FirstName: "Артём"}
		h.oldDialog(u, step, dr)
		return u
	}
	stale := func(id string) {
		t.Helper()
		if a := h.answered(id); a.Notification != "Этот вопрос уже позади" {
			t.Fatalf("старая кнопка: %+v", a)
		}
	}
	full := store.Draft{Name: "Артём", Grade: 9, RegionCode: "16", HomeCity: "Казань", SubjectCodes: []string{"inf"},
		DirectionIDs: []string{"napr-09-03-04"}, Experience: "school", Places: []store.Place{{RegionCode: "16", City: "Казань"}},
		UniversityIDs: []string{"kfu"}}

	// «Да, это я» — к классу.
	u := pupil(1, "name_confirm", store.Draft{Name: "Артём"})
	stale(h.pressAny(u, "name:ok", "Тебя зовут Артём?"))
	h.step(u, 2)
	h.mustContain(u, "Приятно познакомиться, Артём! В каком ты классе?")

	// Регион текстом или геопозицией — к предметам.
	u = pupil(2, "region", store.Draft{Name: "Артём", Grade: 9})
	h.text(u, "Казань")
	h.step(u, 3)
	h.mustContain(u, "Какие предметы тебе нравятся?")
	if d := h.dialog(u); d.Step != stepSubjects || d.Draft.RegionCode != "" {
		t.Fatalf("после региона: %+v", d)
	}
	u = pupil(3, "region", store.Draft{Name: "Артём", Grade: 9})
	h.geo(u, 55.79, 49.11)
	h.step(u, 3)

	// Опыт и «Где учиться» — к вузам.
	u = pupil(4, "experience", full)
	stale(h.pressAny(u, "exp:school", "Какой у тебя опыт в олимпиадах?"))
	h.step(u, 4)
	u = pupil(5, "target", full)
	h.text(u, "Москва")
	h.step(u, 4)
	stale(h.pressAny(u, "vuz:places", "Вот вузы"))
	if d := h.dialog(u); d.Step != stepUniversities {
		t.Fatalf("«Изменить места» со старого вопроса: %+v", d)
	}

	// Карточка профиля — тоже к вузам, дальше направления с отмеченным;
	// «Готово» создаёт траекторию с прежними ответами.
	u = pupil(6, "summary", full)
	stale(h.pressAny(u, "sum:ok", "**Артём, собрали твой профиль**"))
	h.step(u, 4)
	if b := maxtest.Buttons(h.fake.Last(u.UserID)); !strings.Contains(b, "✓ КФУ · Казань") {
		t.Fatalf("вуз отмечен: %s", b)
	}
	h.press(u, "vuz:done")
	h.step(u, 5)
	if b := maxtest.Buttons(h.fake.Last(u.UserID)); !strings.Contains(b, "✓ Программная инженерия") {
		t.Fatalf("направление отмечено: %s", b)
	}
	h.press(u, "dir:done")
	tr := h.trajectory(u)
	if tr.RegionCode != "16" || tr.HomeCity == nil || *tr.HomeCity != "Казань" || tr.Experience != "school" ||
		!slices.Equal(tr.Places, []store.Place{{RegionCode: "16", City: "Казань"}}) || len(tr.Directions) != 1 {
		t.Fatalf("прежние ответы: %+v", tr)
	}
	h.mustContain(u, "Под твою цель подходят")
}

func TestDuplicateDeliveryIsIgnored(t *testing.T) {
	h := newHarness(t)
	h.register(artem, "")
	cb := maxapi.Update{UpdateType: maxapi.UpdateMessageCallback, Timestamp: 1,
		Callback: &maxapi.Callback{CallbackID: "same-id", Payload: "role:kid", User: artem}}
	for i := 0; i < 2; i++ {
		if err := h.bot.Handle(context.Background(), cb); err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	for _, a := range h.fake.Answered {
		if a.CallbackID == "same-id" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("повторная доставка — без второго ответа: %d", n)
	}
}

func TestUnknownStartPayloadIsPlainWelcome(t *testing.T) {
	h := newHarness(t)
	h.register(artem, "inv_") // битое приглашение
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

	// Сначала согласие: до «Принимаю» Артём не в траектории и Ольге ничего не приходит.
	h.started(artem, "inv_"+token)
	if q := h.lastText(artem); !strings.HasPrefix(q, "Прежде чем подключиться к траектории") {
		t.Fatalf("согласие приглашённого: %q", q)
	}
	if _, err := h.st.CurrentMember(context.Background(), artem.UserID); err == nil {
		t.Fatal("в траектории до согласия")
	}
	if got := h.lastText(olga); strings.Contains(got, "теперь в траектории") {
		t.Fatalf("Ольге — до согласия: %q", got)
	}
	h.press(artem, "consent:ok")
	// Траектория с регионом (создана до #127): место в профиле видно, пустых мест — нет.
	h.mustContain(artem, "**Проверь, всё ли верно:**\nКласс: 9 · Татарстан\nПредметы: информатика\nЦель: программная инженерия\nВузы: ВШЭ")
	if hello := h.fake.To(artem.UserID)[1].Msg.Text; hello != "Привет, Артём! 👋 Ольга приглашает тебя в «Траекторию» — подборка олимпиад уже готова." {
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
	h.register(igor, "inv_"+token)
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
	h.register(artem, "inv_"+token)
	h.press(artem, "join:goal")
	h.mustContain(artem, "Какое направление интересно?")
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
	// /delete отзывает согласие: вернувшегося бот спросит снова.
	h.text(artem, "/start")
	h.mustContain(artem, "политике конфиденциальности")
}

func TestLeave_InvitedMember(t *testing.T) {
	h := newHarness(t)
	m := h.kidOnboarding()
	uid, _ := h.st.UpsertUser(context.Background(), olga.UserID, olga.FirstName)
	_, _ = h.st.AddMember(context.Background(), m.TrajectoryID, uid, "parent")
	_ = h.st.AcceptPrivacy(context.Background(), uid)
	h.text(olga, "/delete")
	h.press(olga, "leave:yes")
	h.mustContain(olga, "Готово: вы больше не в траектории Артёма.")
	h.mustContain(artem, "Ольга больше не в траектории.")
	// Выход отзывает согласие: вернувшуюся Ольгу бот спросит снова.
	h.text(olga, "/start")
	h.mustContain(olga, "политике конфиденциальности")
}

// Согласие с политикой конфиденциальности (152-ФЗ) — до первого вопроса
// анкеты: ответы начинают сохраняться только после «Принимаю».
func TestConsent_BeforeQuestionnaire(t *testing.T) {
	h := newHarness(t)
	h.started(artem, "src_class_9a")
	q := h.fake.Last(artem.UserID)
	if !strings.HasPrefix(q.Text, "Привет! Помогу подобрать олимпиады для поступления и не пропустить сроки.") ||
		!strings.Contains(q.Text, "политике конфиденциальности") || strings.HasPrefix(q.Text, "Шаг ") ||
		!strings.Contains(q.Text, "имя, класс, предметы, цель и вузы") {
		t.Fatalf("вопрос о согласии: %q", q.Text)
	}
	kb := q.Keyboard()
	if len(kb) != 2 || kb[0][0].Type != "link" || kb[0][0].URL != privacyURL || kb[0][0].Text != "Политика конфиденциальности" ||
		kb[1][0].Payload != "consent:ok" || kb[1][0].Text != "Принимаю" {
		t.Fatalf("кнопки согласия: %s", maxtest.Buttons(q))
	}

	// Без согласия анкета не идёт: кнопка роли ничего не меняет.
	if a := h.answered(h.pressAny(artem, "role:kid", "Кто вы?")); a.Notification != "Этот вопрос уже позади" {
		t.Fatalf("роль до согласия: %+v", a)
	}
	if d := h.dialog(artem); d.Step != stepConsent || d.Role != "" {
		t.Fatalf("диалог до согласия: %+v", d)
	}
	uid, _ := h.st.UpsertUser(context.Background(), artem.UserID, artem.FirstName)
	if ok, _ := h.st.PrivacyAccepted(context.Background(), uid); ok {
		t.Fatal("согласие записано до «Принимаю»")
	}

	// У вопроса остаются ссылка на политику и отметка выбора.
	a := h.answered(h.press(artem, "consent:ok"))
	if a.Message == nil || maxtest.Buttons(*a.Message) != "Политика конфиденциальности | ✓ Принимаю" {
		t.Fatalf("ответ на «Принимаю»: %+v", a)
	}
	if ok, _ := h.st.PrivacyAccepted(context.Background(), uid); !ok {
		t.Fatal("согласие не записано")
	}
	h.step(artem, 1)
	h.mustContain(artem, "Задам несколько коротких вопросов. Кто вы?")
	if d := h.dialog(artem); d.Step != stepRole || d.SourcePayload == nil || *d.SourcePayload != "src_class_9a" {
		t.Fatalf("анкета с источником ссылки: %+v", d)
	}

	// Согласие уже есть — /start посреди анкеты сразу к первому вопросу.
	h.press(artem, "role:kid")
	h.text(artem, "/start")
	h.step(artem, 1)
}

// Согласие дано, анкета брошена: /delete стирает черновик и отзывает согласие.
// Без черновика и согласия удалять нечего — прежний ответ.
func TestDelete_UnfinishedQuestionnaire(t *testing.T) {
	h := newHarness(t)
	h.text(igor, "/delete")
	h.mustContain(igor, "Сначала нужно ответить на несколько коротких вопросов")

	h.register(artem, "")
	h.press(artem, "role:kid")
	h.text(artem, "/delete")
	h.mustContain(artem, "Удалил ответы анкеты и согласие на обработку данных.")
	uid, _ := h.st.UpsertUser(context.Background(), artem.UserID, artem.FirstName)
	if _, err := h.st.Dialog(context.Background(), uid); err == nil {
		t.Fatal("черновик анкеты остался")
	}
	h.text(artem, "/start")
	h.mustContain(artem, "политике конфиденциальности")
}

// Политика — по ссылке из /help (ТЗ §12, 152-ФЗ).
func TestHelp_LinksPrivacyPolicy(t *testing.T) {
	h := newHarness(t)
	h.text(artem, "/help")
	msg := h.fake.Last(artem.UserID)
	kb := msg.Keyboard()
	if !strings.Contains(msg.Text, "/delete") || len(kb) != 1 || kb[0][0].Type != "link" || kb[0][0].URL != privacyURL {
		t.Fatalf("/help: %q %s", msg.Text, maxtest.Buttons(msg))
	}
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
	h.toSubjects("9")
	h.press(artem, "subj:t:inf")
	h.press(artem, "subj:t:math")
	if a := h.answered(h.press(artem, "subj:done")); a.Message == nil || maxtest.Buttons(*a.Message) != "✓ Информатика | ✓ Математика" {
		t.Fatalf("предметы после «Готово»: %+v", a.Message)
	}
	h.press(artem, "vuz:t:itmo")
	h.press(artem, "vuz:t:kfu")
	a := h.answered(h.press(artem, "vuz:done"))
	if a.Message == nil || !strings.Contains(maxtest.Buttons(*a.Message), "✓ ИТМО") ||
		!strings.Contains(maxtest.Buttons(*a.Message), "✓ КФУ · Казань") || strings.Contains(maxtest.Buttons(*a.Message), "Готово") {
		t.Fatalf("вузы после «Готово»: %+v", a.Message)
	}
	h.press(artem, "dir:t:napr-09-03-04")
	h.press(artem, "dir:t:napr-09-03-01")
	if a := h.answered(h.press(artem, "dir:done")); a.Message == nil ||
		maxtest.Buttons(*a.Message) != "✓ Программная инженерия | ✓ Информатика и вычислительная техника" {
		t.Fatalf("направления после «Готово»: %+v", a.Message)
	}
}

// bioAsk — Всесибирская по биологии в трекере Артёма, регистрация
// отмечена, 02.11 в 10:05 — время вопроса об итоге отборочного.
func (h *harness) bioAsk(m store.Member) (item, reminderID string) {
	h.t.Helper()
	ctx := context.Background()
	item, _, _ = h.st.AddTrackerItem(ctx, m.TrajectoryID, "p669-14-biologiya", m.MemberID)
	if _, err := h.st.SetRegistered(ctx, m.TrajectoryID, item, m.MemberID, true); err != nil {
		h.t.Fatal(err)
	}
	_ = h.st.SyncReminders(ctx, m.TrajectoryID, 10, time.Date(2026, 10, 1, 12, 0, 0, 0, msk))
	now := time.Date(2026, 11, 2, 10, 5, 0, 0, msk)
	due, _ := h.st.DueReminders(ctx, now, 50)
	for _, d := range due {
		if d.Ask() && d.StageID == "p669-14-biologiya:qualifying:1" && d.Offset == -1 {
			reminderID = d.ID
		}
	}
	if reminderID == "" {
		h.t.Fatalf("нет вопроса об итоге: %+v", due)
	}
	h.bot.now = func() time.Time { return now }
	return item, reminderID
}

// Ответ на вопрос об итоге — отметка этапа и сообщение семье. Кнопка из
// старого сообщения итог не меняет.
func TestResultButtons_MarkStage(t *testing.T) {
	h := newHarness(t)
	m := h.kidOnboarding()
	uid, _ := h.st.UpsertUser(context.Background(), olga.UserID, olga.FirstName)
	_, _ = h.st.AddMember(context.Background(), m.TrajectoryID, uid, "parent")
	item, rid := h.bioAsk(m)

	if a := h.answered(h.pressAny(artem, "res:"+rid+":n", "📝")); a.Notification != "Хорошо, спрошу ещё раз через неделю." {
		t.Fatalf("итогов ещё нет: %+v", a)
	}
	if a := h.answered(h.pressAny(artem, "res:"+rid+":p", "📝")); a.Notification != "✅ Записал в трекер." {
		t.Fatalf("итог записан: %+v", a)
	}
	marks, _ := h.st.StageMarks(context.Background(), []string{item})
	if marks[item]["p669-14-biologiya:qualifying:1"].Result != "passed" {
		t.Fatalf("отметка этапа: %+v", marks[item])
	}
	h.mustContain(olga, "🎉 Артём отмечает в трекере «Всесибирская открытая олимпиада школьников»: отборочный этап — пройден.")

	if a := h.answered(h.pressAny(artem, "res:"+rid+":f", "📝")); a.Notification != "Уже отмечено в трекере — изменить можно там." {
		t.Fatalf("старая кнопка: %+v", a)
	}
	marks, _ = h.st.StageMarks(context.Background(), []string{item})
	if marks[item]["p669-14-biologiya:qualifying:1"].Result != "passed" {
		t.Fatalf("старая кнопка ничего не меняет: %+v", marks[item])
	}
	if a := h.answered(h.pressAny(igor, "res:"+rid+":p", "📝")); a.Notification != "Этой олимпиады уже нет в трекере." {
		t.Fatalf("чужая кнопка: %+v", a)
	}
	if a := h.answered(h.pressAny(artem, "res:"+rid+":x", "📝")); a.Notification == "✅ Записал в трекер." {
		t.Fatalf("неизвестный код: %+v", a)
	}
}

// Кнопка в напоминании о регистрации на заключительный этап отмечает
// именно эту регистрацию — первая уже отмечена.
func TestReminderButtons_SecondRegistration(t *testing.T) {
	h := newHarness(t)
	m := h.kidOnboarding()
	uid, _ := h.st.UpsertUser(context.Background(), olga.UserID, olga.FirstName)
	_, _ = h.st.AddMember(context.Background(), m.TrajectoryID, uid, "parent")
	item, _ := h.bioAsk(m)
	now := time.Date(2027, 2, 26, 10, 5, 0, 0, msk)
	due, _ := h.st.DueReminders(context.Background(), now, 50)
	var rid string
	for _, d := range due {
		if d.StageID == "p669-14-biologiya:registration:2" && d.Offset == 7 {
			rid = d.ID
		}
	}
	if rid == "" {
		t.Fatalf("нет напоминания о регистрации на финал: %+v", due)
	}
	h.bot.now = func() time.Time { return now }
	if a := h.answered(h.pressAny(artem, "rem:reg:"+rid, "📅")); !strings.HasPrefix(a.Notification, "✅ Отмечено: регистрация на «Всесибирская") {
		t.Fatalf("отметка: %+v", a)
	}
	marks, _ := h.st.StageMarks(context.Background(), []string{item})
	if !marks[item]["p669-14-biologiya:registration:2"].Registered {
		t.Fatalf("регистрация на финал отмечена: %+v", marks[item])
	}
	h.mustContain(olga, "регистрация на заключительный этап — пройдена.")
}
