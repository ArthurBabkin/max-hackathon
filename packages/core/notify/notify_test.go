package notify

import (
	"context"
	"strings"
	"testing"
	"time"

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
		Grade: 9, RegionCode: "16", TZ: "Europe/Moscow", GoalStatus: "known",
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
	item, _, _ := f.st.AddTrackerItem(ctx, f.kid.TrajectoryID, "p669-8-informatika", f.kid.MemberID)
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
	if len(p) != 2 || p[0] != "rem:done:"+item || p[1] != "rem:snooze:"+d.ID {
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
