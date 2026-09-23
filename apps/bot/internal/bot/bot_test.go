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

func (h *harness) mustContain(u maxapi.User, want string) {
	h.t.Helper()
	if got := h.lastText(u); !strings.Contains(got, want) {
		h.t.Fatalf("ждали «%s», последнее сообщение: %q", want, got)
	}
}

// kidOnboarding проходит онбординг ученика целиком (ТЗ §5.1).
func (h *harness) kidOnboarding() store.Member {
	h.t.Helper()
	h.started(artem, "src_class_9a")
	h.mustContain(artem, "Кто вы?")
	h.press(artem, "role:kid")
	h.mustContain(artem, "Тебя зовут Артём?")
	h.press(artem, "name:ok")
	h.mustContain(artem, "Приятно познакомиться, Артём! В каком ты классе?")
	h.press(artem, "grade:9")
	h.mustContain(artem, "Где ты учишься?")
	h.geo(artem, 55.79, 49.11) // Казань
	h.press(artem, "subj:t:inf")
	h.press(artem, "subj:t:math")
	h.press(artem, "subj:done")
	h.mustContain(artem, "Куда думаешь поступать? Под твои предметы подходят")
	h.press(artem, "dir:t:napr-09-03-04")
	h.press(artem, "dir:t:napr-09-03-01")
	h.press(artem, "dir:done")
	h.mustContain(artem, "Где хочешь учиться?")
	h.press(artem, "target:16")
	h.mustContain(artem, "Вот вузы, которые подходят.")
	h.press(artem, "vuz:t:innopolis")
	h.press(artem, "vuz:other")
	h.text(artem, "вшэ")
	h.press(artem, "vuz:done")
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
		tr.GoalStatus != "known" || tr.TargetRegionCode == nil || *tr.TargetRegionCode != "16" {
		t.Fatalf("траектория: %+v", tr)
	}
	sent := h.fake.To(artem.UserID)
	summary := sent[len(sent)-2].Msg.Text
	if summary != "Готово: Артём, 9 класс, Республика Татарстан. Предметы: информатика, математика. "+
		"Цель: программная инженерия, информатика и вычислительная техника. Вузы: ВШЭ, Иннополис. "+
		"Всё это можно поменять в профиле навигатора." {
		t.Fatalf("сводка: %q", summary)
	}
	result := h.fake.Last(artem.UserID)
	if !strings.HasPrefix(result.Text, "Под твою цель подходят 5 олимпиад и ВсОШ.") || !strings.Contains(result.Text, "Ближайший срок — ") {
		t.Fatalf("итог: %q", result.Text)
	}
	kb := result.Keyboard()
	if kb[0][0].Type != "open_app" || kb[0][0].ContactID != 42 || kb[1][0].Payload != "rem:on" || kb[2][0].Payload != "inv:new" {
		t.Fatalf("кнопки итога: %s", maxtest.Buttons(result))
	}
	d, _ := h.st.Dialog(context.Background(), m.UserID)
	if d.Step != stepDone || d.SourcePayload == nil || *d.SourcePayload != "src_class_9a" {
		t.Fatalf("диалог закрыт, источник сохранён: %+v", d)
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
	h.mustContain(olga, "Где учится Артём?")
	// Список округов и субъектов правит тот же вопрос, нового сообщения нет.
	id := h.press(olga, "region:list")
	if a := h.answered(id); a.Message == nil || !slices.Contains(maxtest.Payloads(*a.Message), "region:d:5") {
		t.Fatalf("округа на месте вопроса: %+v", a)
	}
	id = h.pressAny(olga, "region:d:5", "")
	if a := h.answered(id); a.Message == nil || !slices.Contains(maxtest.Payloads(*a.Message), "region:16") {
		t.Fatalf("субъекты округа: %+v", a)
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
		!slices.Contains(payloads, "dir:all") {
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

	// Москва и Петербург — отдельными кнопками: Артём не из них.
	h.mustContain(olga, "Где Артём хочет учиться?")
	payloads = maxtest.Payloads(h.fake.Last(olga.UserID))
	if !slices.Equal(payloads, []string{"target:16", "target:77", "target:78", "target:any"}) {
		t.Fatalf("варианты города: %v", payloads)
	}
	h.press(olga, "target:any")
	h.mustContain(olga, "Вот вузы, которые подходят.")
	h.press(olga, "vuz:other")
	h.text(olga, "мфти")
	h.press(olga, "vuz:done")
	m, err := h.st.CurrentMember(context.Background(), olga.UserID)
	if err != nil || m.Role != "parent" || !m.IsCreator || m.HasKid {
		t.Fatalf("родитель — создатель без ученика: %+v %v", m, err)
	}
	tr, _ := h.st.Trajectory(context.Background(), m.TrajectoryID)
	if tr.GoalStatus != "exploring" || len(tr.Directions) != 0 || tr.TargetRegionCode != nil || tr.Grade != 10 {
		t.Fatalf("цель не выбрана, город не важен: %+v", tr)
	}
	result := h.fake.Last(olga.UserID)
	if !strings.HasPrefix(result.Text, "По предметам Артёма подход") {
		t.Fatalf("итог родителю: %q", result.Text)
	}
	if kb := result.Keyboard(); kb[2][0].Text != "👋 Пригласить Артёма" {
		t.Fatalf("третья кнопка родителя: %s", maxtest.Buttons(result))
	}
}

// Вузы выбирать не обязательно, а если в выбранном городе подходящих нет,
// бот честно говорит об этом и даёт выбрать другой город.
func TestOnboarding_UniversitiesOptionalAndCityChange(t *testing.T) {
	h := newHarness(t)
	h.started(artem, "")
	h.press(artem, "role:kid")
	h.press(artem, "name:ok")
	h.press(artem, "grade:11")
	h.geo(artem, 55.03, 82.92) // Новосибирск
	h.press(artem, "subj:t:bio")
	h.press(artem, "subj:done")
	h.press(artem, "dir:t:napr-31-05-01") // лечебное дело
	h.press(artem, "dir:done")
	h.press(artem, "target:54")
	// В НГУ лечебное дело есть — предложен он один.
	if p := maxtest.Payloads(h.fake.Last(artem.UserID)); !slices.Contains(p, "vuz:t:nsu") || slices.Contains(p, "vuz:t:kfu") {
		t.Fatalf("вузы Новосибирска: %v", p)
	}
	h.press(artem, "vuz:city")
	h.mustContain(artem, "Где хочешь учиться?")
	h.press(artem, "target:77")
	if p := maxtest.Payloads(h.fake.Last(artem.UserID)); !slices.Contains(p, "vuz:t:sechenov") || slices.Contains(p, "vuz:t:nsu") {
		t.Fatalf("вузы Москвы: %v", p)
	}
	last := h.fake.Last(artem.UserID)
	if kb := last.Keyboard(); kb[len(kb)-1][0].Text != "Пропустить" {
		t.Fatalf("без вузов — «Пропустить»: %s", maxtest.Buttons(last))
	}
	h.press(artem, "vuz:done")
	m, err := h.st.CurrentMember(context.Background(), artem.UserID)
	if err != nil {
		t.Fatalf("траектория без вузов создана: %v", err)
	}
	unis, _ := h.st.TrajectoryUniversities(context.Background(), m.TrajectoryID)
	tr, _ := h.st.Trajectory(context.Background(), m.TrajectoryID)
	if len(unis) != 0 || tr.TargetRegionCode == nil || *tr.TargetRegionCode != "77" {
		t.Fatalf("вузы %v, город %v", unis, tr.TargetRegionCode)
	}
	sent := h.fake.To(artem.UserID)
	if summary := sent[len(sent)-2].Msg.Text; !strings.Contains(summary, "Вузы: пока не выбраны.") {
		t.Fatalf("сводка без вузов: %q", summary)
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
		h.bot.resultKeyboard(mustVoice(h, olga, parent), parent, false))})
	h.press(olga, "inv:new")
	link := h.lastText(olga)
	if !strings.HasPrefix(link, "Перешлите Артёму эту ссылку в MAX.") || !strings.Contains(link, "https://max.ru/test_bot?start=inv_") {
		t.Fatalf("ссылка: %q", link)
	}
	token := link[strings.Index(link, "start=inv_")+len("start=inv_"):]

	h.started(artem, "inv_"+token)
	h.mustContain(artem, "Проверь, всё ли верно:\nИмя: Артём\nКласс: 9\nЦель: программная инженерия")
	if hello := h.fake.To(artem.UserID)[0].Msg.Text; hello != "Привет, Артём! 👋 Ольга приглашает тебя в «Траекторию» — подборка олимпиад уже готова." {
		t.Fatalf("приветствие приглашённого: %q", hello)
	}
	if got := h.lastText(olga); got != "✓ Артём теперь в траектории." {
		t.Fatalf("пригласившему: %q", got)
	}
	h.press(artem, "join:ok")
	h.mustContain(artem, "Отлично! Под твою цель подход")
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
	h.mustContain(artem, "Под твою цель подход")
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
	h.press(artem, "rem:on")
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
	h.started(artem, "")
	h.press(artem, "role:kid")
	h.press(artem, "name:ok")
	h.press(artem, "grade:9")
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
	h.press(artem, "target:16")
	h.press(artem, "vuz:t:innopolis")
	h.press(artem, "vuz:t:kfu")
	a := h.answered(h.press(artem, "vuz:done"))
	if a.Message == nil || !strings.Contains(maxtest.Buttons(*a.Message), "✓ Иннополис") ||
		!strings.Contains(maxtest.Buttons(*a.Message), "✓ КФУ") || strings.Contains(maxtest.Buttons(*a.Message), "Готово") {
		t.Fatalf("вузы после «Готово»: %+v", a.Message)
	}
}
