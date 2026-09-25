package notify

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi/maxtest"
)

var msk, _ = time.LoadLocation("Europe/Moscow")

type family struct {
	st        *store.Store
	fake      *maxtest.Fake
	n         *Notifier
	kid, olga store.Member
}

func newFamily(t *testing.T) family {
	t.Helper()
	st := store.New(dbtest.Open(t))
	ctx := context.Background()
	kidUser, _ := st.UpsertUser(ctx, 900000001, "Артём")
	kid, err := st.CreateTrajectory(ctx, store.NewTrajectory{CreatorUserID: kidUser, Role: "kid", StudentName: "Артём",
		Grade: 9, RegionCode: "16", TZ: "Europe/Moscow",
		SubjectCodes: []string{"inf", "math"}, UniversityIDs: []string{"hse"}})
	if err != nil {
		t.Fatal(err)
	}
	olgaUser, _ := st.UpsertUser(ctx, 900000002, "Ольга")
	olga, err := st.AddMember(ctx, kid.TrajectoryID, olgaUser, "parent")
	if err != nil {
		t.Fatal(err)
	}
	fake := &maxtest.Fake{}
	return family{st: st, fake: fake, kid: kid, olga: olga,
		n: &Notifier{Store: st, Max: fake, BotName: "test_bot", BotID: 42, ReminderHour: 10}}
}

func TestReminder_VoiceAndButtons(t *testing.T) {
	f := newFamily(t)
	ctx := context.Background()
	_, _, _ = f.st.AddTrackerItem(ctx, f.kid.TrajectoryID, "p669-8-informatika", f.kid.MemberID)
	_ = f.st.SyncReminders(ctx, f.kid.TrajectoryID, 10, time.Date(2026, 9, 1, 12, 0, 0, 0, msk))
	now := time.Date(2026, 9, 19, 10, 1, 0, 0, msk)
	due, _ := f.st.DueReminders(ctx, now, 10)
	var d store.DueReminder
	for _, x := range due {
		if x.StageKind == "registration" && x.Offset == 3 {
			d = x
		}
	}
	if d.ID == "" {
		t.Fatalf("нет напоминания «за 3 дня»: %+v", due)
	}
	tr, _ := f.st.Trajectory(ctx, f.kid.TrajectoryID)
	rs, _ := f.st.ReminderRecipients(ctx, d)

	kidMsg := f.n.Reminder(d, rs[0], tr, now)
	if kidMsg.Text != "⏰ Через 3 дня — регистрация на олимпиаду «Высшая проба»." {
		t.Fatalf("ученику: %q", kidMsg.Text)
	}
	parentMsg := f.n.Reminder(d, rs[1], tr, now)
	if !strings.HasPrefix(parentMsg.Text, "⏰ У Артёма через 3 дня — регистрация") ||
		!strings.Contains(parentMsg.Text, "согласие родителя") {
		t.Fatalf("родителю: %q", parentMsg.Text)
	}
	p := maxtest.Payloads(kidMsg)
	// Регистрация отмечается по напоминанию, а не по пункту: у олимпиады
	// бывает и вторая регистрация — на заключительный этап.
	if len(p) != 2 || p[0] != "rem:reg:"+d.ID || p[1] != "rem:snooze:"+d.ID {
		t.Fatalf("кнопки: %v (%s)", p, maxtest.Buttons(kidMsg))
	}
	if kb := kidMsg.Keyboard(); kb[0][0].Type != "link" || !strings.HasPrefix(kb[0][0].URL, "https://") {
		t.Fatalf("первая кнопка — ссылка на регистрацию: %+v", kb[0][0])
	}

	// В последний день «напомнить завтра» уже нельзя.
	last := time.Date(2026, 9, 22, 10, 0, 0, 0, msk)
	if m := f.n.Reminder(d, rs[0], tr, last); !strings.HasPrefix(m.Text, "⏰ Сегодня последний день") ||
		strings.Contains(strings.Join(maxtest.Payloads(m), ","), "snooze") {
		t.Fatalf("последний день: %q %v", m.Text, maxtest.Payloads(m))
	}
}

func TestRegistered_NotifiesOthersOnly(t *testing.T) {
	f := newFamily(t)
	ctx := context.Background()
	item, _, _ := f.st.AddTrackerItem(ctx, f.kid.TrajectoryID, "p669-8-informatika", f.kid.MemberID)
	f.n.Registered(ctx, f.kid.TrajectoryID, item, f.olga)
	if len(f.fake.To(900000002)) != 0 {
		t.Fatal("автору отметки уведомление не нужно")
	}
	got := f.fake.Last(900000001)
	if got.Text != "✅ Ольга отмечает: регистрация на «Высшая проба» пройдена." {
		t.Fatalf("ученику: %q", got.Text)
	}
	if kb := got.Keyboard(); kb[0][0].Type != "open_app" || kb[0][0].Payload != "tracker" || kb[0][0].ContactID != 42 {
		t.Fatalf("кнопка трекера: %+v", kb)
	}
}

func TestProposal_KidGetsButtonsParentGetsAnswer(t *testing.T) {
	f := newFamily(t)
	ctx := context.Background()
	id, _, err := f.st.CreateProposal(ctx, f.kid.TrajectoryID, "p669-8-informatika", f.olga.MemberID)
	if err != nil {
		t.Fatal(err)
	}
	f.n.ProposalCreated(ctx, f.kid.TrajectoryID, id, f.olga, time.Date(2026, 9, 1, 12, 0, 0, 0, msk))
	msg := f.fake.Last(900000001)
	if !strings.HasPrefix(msg.Text, "📩 Ольга предлагает добавить в трекер") || !strings.Contains(msg.Text, "Ближайший срок — 22 сентября.") {
		t.Fatalf("предложение: %q", msg.Text)
	}
	if p := maxtest.Payloads(msg); len(p) != 3 || p[0] != "prop:acc:"+id || p[2] != "prop:dec:"+id {
		t.Fatalf("кнопки: %v", p)
	}
	if len(f.fake.To(900000002)) != 0 {
		t.Fatal("родителю своё же предложение не шлём")
	}

	if _, err := f.st.ResolveProposal(ctx, f.kid.TrajectoryID, id, f.kid.MemberID, true); err != nil {
		t.Fatal(err)
	}
	f.n.ProposalResolved(ctx, f.kid.TrajectoryID, id, true)
	if got := f.fake.Last(900000002).Text; !strings.HasPrefix(got, "✅ Артём добавляет «") {
		t.Fatalf("родителю: %q", got)
	}
}

func TestNilSenderIsSilent(t *testing.T) {
	f := newFamily(t)
	f.n.Max = nil
	f.n.Joined(context.Background(), f.olga) // не паникует и ничего не шлёт
	if len(f.fake.Sent) != 0 {
		t.Fatal("без отправителя сообщений нет")
	}
}

func TestShort(t *testing.T) {
	for in, want := range map[string]string{
		"Всероссийская олимпиада школьников «Высшая проба»": "Высшая проба",
		"«Формула Единства»/«Третье тысячелетие»":           "Формула Единства",
		"ВсОШ по информатике":                               "ВсОШ по информатике",
		"Турнир юных программистов Казани":                  "Турнир юных программистов Казани",
	} {
		if got := Short(in); got != want {
			t.Errorf("Short(%q) = %q", in, got)
		}
	}
}

func TestChanges_ListAndButtons(t *testing.T) {
	n := &Notifier{BotName: "test_bot", BotID: 42}
	ptr := func(s string) *string { return &s }
	var items []store.ChangeItem
	items = append(items,
		store.ChangeItem{ProfileID: "vsosh-informatika", ProfileName: ptr("информатика"),
			OlympiadName: "ВсОШ по информатике", OfficialURL: ptr("http://insecure.example"), Kind: "stages"},
		store.ChangeItem{ProfileID: "p669-8-informatika", ProfileName: ptr("информатика"),
			OlympiadName: "Всероссийская олимпиада школьников «Высшая проба»", Kind: "benefits",
			UniversityID: ptr("innopolis"), UniversityShort: ptr("УИ"), RulesURL: ptr("https://innopolis.university/rules.pdf")})
	for i := range 9 {
		id := string(rune('a' + i))
		items = append(items, store.ChangeItem{ProfileID: id, OlympiadName: "Олимпиада " + id, Kind: "stages"})
	}
	m := n.Changes(items, store.Recipient{Role: "kid", Name: "Артём"}, store.Trajectory{StudentName: "Артём"})

	if !strings.Contains(m.Text, "• «ВсОШ по информатике» — сроки этапов\n") {
		t.Fatalf("профиль не повторяет название: %q", m.Text)
	}
	if !strings.Contains(m.Text, "• «Высшая проба», информатика — льготы (Иннополис)") {
		t.Fatalf("вуз узнаваемо: %q", m.Text)
	}
	if !strings.Contains(m.Text, "…и ещё 3\n") || strings.Contains(m.Text, "Олимпиада g") {
		t.Fatalf("список обрезан после восьми: %q", m.Text)
	}
	want := "Карточка «ВсОШ по информатике» | Карточка «Высшая проба» | Правила приёма Иннополис | Карточка «Олимпиада a»"
	if got := maxtest.Buttons(m); got != want {
		t.Fatalf("кнопки — три карточки, только https-ссылки:\n got %s\nwant %s", got, want)
	}
}

// Отметку этапа видит остальная семья: регистрацию, проход дальше и
// диплом. Первая регистрация — тот же текст, что у галочки (F46).
func TestStageMarked_TextsForFamily(t *testing.T) {
	f := newFamily(t)
	ctx := context.Background()
	item, _, _ := f.st.AddTrackerItem(ctx, f.kid.TrajectoryID, "p669-14-biologiya", f.kid.MemberID)
	cases := []struct {
		stage string
		mark  stages.Mark
		want  string
	}{
		{"p669-14-biologiya:registration:1", stages.Mark{Registered: true}, "✅ Артём отмечает: регистрация на «Всесибирская открытая олимпиада школьников» пройдена."},
		{"p669-14-biologiya:qualifying:1", stages.Mark{Result: stages.Passed}, "🎉 Артём отмечает в трекере «Всесибирская открытая олимпиада школьников»: отборочный этап — пройден."},
		{"p669-14-biologiya:qualifying:1", stages.Mark{Result: stages.Failed}, "Артём отмечает в трекере «Всесибирская открытая олимпиада школьников»: отборочный этап — не пройден."},
		{"p669-14-biologiya:registration:2", stages.Mark{Registered: true}, "✅ Артём отмечает в трекере «Всесибирская открытая олимпиада школьников»: регистрация на заключительный этап — пройдена."},
		{"p669-14-biologiya:final:1", stages.Mark{Result: stages.Winner}, "🏆 Артём отмечает в трекере «Всесибирская открытая олимпиада школьников»: заключительный этап — диплом победителя!"},
		{"p669-14-biologiya:final:1", stages.Mark{Result: stages.Prizer}, "🏅 Артём отмечает в трекере «Всесибирская открытая олимпиада школьников»: заключительный этап — диплом призёра!"},
		{"p669-14-biologiya:final:1", stages.Mark{Result: stages.Participant}, "Артём отмечает в трекере «Всесибирская открытая олимпиада школьников»: заключительный этап — без диплома."},
	}
	for _, c := range cases {
		f.fake.Reset()
		f.n.StageMarked(ctx, f.kid.TrajectoryID, item, c.stage, c.mark, f.kid)
		got := f.fake.To(900000002)
		if len(got) != 1 || !strings.HasPrefix(got[0].Msg.Text, c.want) {
			t.Errorf("%s %+v: %v, ожидали %q", c.stage, c.mark, got, c.want)
		}
		if n := len(f.fake.To(900000001)); n != 0 {
			t.Errorf("автору отметки не пишем: %d", n)
		}
	}
}

// Вопрос об итоге этапа: кнопки — итоги этого этапа и «Итогов ещё нет».
func TestResultAsk_ButtonsByStage(t *testing.T) {
	f := newFamily(t)
	ctx := context.Background()
	item, _, _ := f.st.AddTrackerItem(ctx, f.kid.TrajectoryID, "p669-14-biologiya", f.kid.MemberID)
	_, _ = f.st.SetRegistered(ctx, f.kid.TrajectoryID, item, f.kid.MemberID, true)
	_ = f.st.SyncReminders(ctx, f.kid.TrajectoryID, 10, time.Date(2026, 10, 1, 12, 0, 0, 0, msk))
	due, _ := f.st.DueReminders(ctx, time.Date(2026, 11, 2, 10, 5, 0, 0, msk), 10)
	var d store.DueReminder
	for _, x := range due {
		if x.StageKind == "qualifying" && x.Offset == -1 {
			d = x
		}
	}
	if d.ID == "" {
		t.Fatalf("нет вопроса об итоге отборочного: %+v", due)
	}
	tr, _ := f.st.Trajectory(ctx, f.kid.TrajectoryID)
	kid := store.Recipient{MemberID: f.kid.MemberID, Name: "Артём", Role: "kid"}
	msg := f.n.ResultAsk(d, kid, tr, []string{stages.Passed, stages.Failed})
	if msg.Text != "📝 Как прошёл отборочный этап олимпиады «Всесибирская открытая олимпиада школьников»? Отметь итог — трекер покажет, что дальше." {
		t.Fatalf("ученику: %q", msg.Text)
	}
	if b := maxtest.Buttons(msg); b != "✅ Прохожу дальше | Дальше не прохожу | Итогов ещё нет" {
		t.Fatalf("кнопки: %s", b)
	}
	if p := maxtest.Payloads(msg); strings.Join(p, " ") != "res:"+d.ID+":p res:"+d.ID+":f res:"+d.ID+":n" {
		t.Fatalf("payload: %v", p)
	}

	parent := store.Recipient{MemberID: f.olga.MemberID, Name: "Ольга", Role: "parent"}
	final := f.n.ResultAsk(d, parent, tr, []string{stages.Winner, stages.Prizer, stages.Participant})
	if !strings.HasPrefix(final.Text, "📝 Как у Артёма прошёл") || !strings.HasSuffix(final.Text, "Отметьте итог — трекер покажет, что дальше.") {
		t.Fatalf("родителю: %q", final.Text)
	}
	if b := maxtest.Buttons(final); b != "🏆 Диплом победителя | 🏅 Диплом призёра | Без диплома | Итогов ещё нет" {
		t.Fatalf("кнопки финала: %s", b)
	}
	if p := maxtest.Payloads(final); strings.Join(p, " ") != "res:"+d.ID+":w res:"+d.ID+":z res:"+d.ID+":u res:"+d.ID+":n" {
		t.Fatalf("payload финала: %v", p)
	}
	if r, ok := ResultByCode("z"); !ok || r != stages.Prizer {
		t.Fatal("код z — призёр")
	}
	if _, ok := ResultByCode("x"); ok {
		t.Fatal("неизвестный код")
	}
}
