package reminders

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/config"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi/maxtest"
)

const (
	kidMax    = 900000001
	parentMax = 900000002
	// «Высшая проба», информатика: регистрация до 22.09.2026, первый
	// отборочный до 11.10.2026. С 01.09 первым наступает «за месяц» до
	// отборочного (11.09), затем «за неделю» до конца регистрации (15.09).
	profile = "p669-8-informatika"
)

var (
	msk, _    = time.LoadLocation("Europe/Moscow")
	direction = "napr-09-03-04"
)

type env struct {
	db     *pgxpool.Pool
	st     *store.Store
	fake   *maxtest.Fake
	w      *Worker
	kid    store.Member
	parent store.Member
	item   string
}

func setup(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	db := dbtest.Open(t)
	st := store.New(db)
	kidUser, _ := st.UpsertUser(ctx, kidMax, "Артём")
	kid, err := st.CreateTrajectory(ctx, store.NewTrajectory{CreatorUserID: kidUser, Role: "kid",
		StudentName: "Артём", Grade: 9, RegionCode: "16", TZ: "Europe/Moscow", GoalStatus: "known", DirectionID: &direction,
		SubjectCodes: []string{"inf"}})
	if err != nil {
		t.Fatal(err)
	}
	parentUser, _ := st.UpsertUser(ctx, parentMax, "Ольга")
	parent, err := st.AddMember(ctx, kid.TrajectoryID, parentUser, "parent")
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := st.AddTrackerItem(ctx, kid.TrajectoryID, profile, kid.MemberID)
	if err != nil {
		t.Fatal(err)
	}
	fake := &maxtest.Fake{}
	w := New(st, fake, config.Bot{Name: "test_bot", ID: 42}, 10)
	w.now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, msk) }
	if err := st.SyncReminders(ctx, kid.TrajectoryID, 10, w.now()); err != nil {
		t.Fatal(err)
	}
	return &env{db: db, st: st, fake: fake, w: w, kid: kid, parent: parent, item: item}
}

// at переводит часы воркера на первое наступившее напоминание.
func (e *env) atFirstDue(t *testing.T) time.Time {
	t.Helper()
	var first time.Time
	if err := e.db.QueryRow(context.Background(),
		`SELECT min(fire_at) FROM reminders WHERE status = 'planned'`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	now := first.Add(5 * time.Minute)
	e.w.now = func() time.Time { return now }
	return now
}

func (e *env) run(t *testing.T) Result {
	t.Helper()
	res, err := e.w.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestRun_SendsToEveryoneOnceAndSecondRunIsQuiet(t *testing.T) {
	e := setup(t)
	now := e.atFirstDue(t)
	if now.In(msk).Hour() != 10 {
		t.Fatalf("напоминания в 10:00 по зоне ученика: %s", now.In(msk))
	}
	res := e.run(t)
	if res.Sent != 2 || res.Failed != 0 || res.Processed != 1 {
		t.Fatalf("первый запуск: %+v", res)
	}
	kid := e.fake.Last(kidMax)
	if kid.Text != "📅 Через месяц — отборочный этап олимпиады «Высшая проба»." {
		t.Fatalf("ученику: %q", kid.Text)
	}
	if b := maxtest.Buttons(kid); strings.Contains(b, "Отметить регистрацию") || !strings.Contains(b, "Напомнить завтра") {
		t.Fatalf("кнопки напоминания об отборочном: %s", b)
	}
	var mids int
	_ = e.db.QueryRow(context.Background(), `SELECT count(max_message_id) FROM reminder_deliveries`).Scan(&mids)
	if mids != 2 {
		t.Fatalf("id сообщений сохранены: %d", mids)
	}

	if res := e.run(t); res.Sent != 0 || res.Processed != 0 {
		t.Fatalf("второй запуск в ту же минуту не шлёт ничего: %+v", res)
	}
	if n := len(e.fake.Sent); n != 2 {
		t.Fatalf("всего сообщений: %d", n)
	}
}

func TestRun_RegistrationReminder(t *testing.T) {
	e := setup(t)
	e.atFirstDue(t)
	e.run(t)
	week := time.Date(2026, 9, 15, 10, 5, 0, 0, msk)
	e.w.now = func() time.Time { return week }
	if res := e.run(t); res.Sent != 2 || res.Processed != 1 {
		t.Fatalf("за неделю до конца регистрации: %+v", res)
	}
	kid := e.fake.Last(kidMax)
	if !strings.Contains(kid.Text, "регистрация на олимпиаду «Высшая проба»") || strings.Contains(kid.Text, "согласие") {
		t.Fatalf("ученику: %q", kid.Text)
	}
	if b := maxtest.Buttons(kid); !strings.Contains(b, "Отметить регистрацию") {
		t.Fatalf("кнопки напоминания о регистрации: %s", b)
	}
	if parent := e.fake.Last(parentMax); !strings.Contains(parent.Text, "согласие") {
		t.Fatalf("родителю — про согласие на обработку ПД: %q", parent.Text)
	}

	// Отметили регистрацию — напоминания о ней больше не приходят.
	if _, err := e.st.SetRegistered(context.Background(), e.kid.TrajectoryID, e.item, e.kid.MemberID, true); err != nil {
		t.Fatal(err)
	}
	e.w.now = func() time.Time { return time.Date(2026, 9, 21, 10, 5, 0, 0, msk) }
	if res := e.run(t); res.Sent != 0 {
		t.Fatalf("после отметки: %+v", res)
	}
}

func TestRun_MemberWithoutOffsetsGetsNothing(t *testing.T) {
	e := setup(t)
	for _, off := range []int{30, 7, 3, 1} {
		if _, err := e.st.ToggleReminderOffset(context.Background(), e.parent.MemberID, off); err != nil {
			t.Fatal(err)
		}
	}
	e.atFirstDue(t)
	if res := e.run(t); res.Sent != 1 {
		t.Fatalf("только ученику: %+v", res)
	}
	if got := e.fake.To(parentMax); len(got) != 0 {
		t.Fatalf("родитель отключил все пороги: %v", got)
	}
}

func TestRun_TransientFailureRetriesOnlyThatRecipient(t *testing.T) {
	e := setup(t)
	e.atFirstDue(t)
	e.fake.Fail = func(userID int64) error {
		if userID == parentMax {
			return &maxapi.Error{Status: 503, Message: "unavailable"}
		}
		return nil
	}
	if res := e.run(t); res.Sent != 1 || res.Failed != 1 {
		t.Fatalf("сбой у одного: %+v", res)
	}
	e.fake.Fail = nil
	if res := e.run(t); res.Sent != 1 || res.Failed != 0 {
		t.Fatalf("повтор только недоставленному: %+v", res)
	}
	if kid, parent := len(e.fake.To(kidMax)), len(e.fake.To(parentMax)); kid != 1 || parent != 1 {
		t.Fatalf("по одному сообщению: ученик %d, родитель %d", kid, parent)
	}
	if res := e.run(t); res.Processed != 0 {
		t.Fatalf("после доставки всем напоминание закрыто: %+v", res)
	}
}

func TestRun_BlockedRecipientIsNotRetried(t *testing.T) {
	e := setup(t)
	e.atFirstDue(t)
	e.fake.Fail = func(userID int64) error {
		if userID == parentMax {
			return &maxapi.Error{Status: 403, Message: "chat.denied"}
		}
		return nil
	}
	if res := e.run(t); res.Sent != 1 || res.Skipped != 1 {
		t.Fatalf("остановивший бота пропущен: %+v", res)
	}
	e.fake.Fail = nil
	if res := e.run(t); res.Processed != 0 {
		t.Fatalf("напоминание закрыто: %+v", res)
	}
}

func TestRun_SnoozeGoesOnlyToRequester(t *testing.T) {
	e := setup(t)
	now := e.atFirstDue(t)
	e.run(t)
	due, err := e.st.DueReminders(context.Background(), now.Add(-time.Hour), 10)
	if err != nil || len(due) != 0 {
		t.Fatalf("очередь пуста: %v %v", due, err)
	}
	var reminderID, stageID string
	_ = e.db.QueryRow(context.Background(),
		`SELECT id::text, stage_id FROM reminders WHERE status = 'sent'`).Scan(&reminderID, &stageID)
	tomorrow := time.Date(now.In(msk).Year(), now.In(msk).Month(), now.In(msk).Day()+1, 10, 0, 0, 0, msk)
	if _, err := e.st.RemindTomorrow(context.Background(), e.item, stageID, e.parent.MemberID, tomorrow); err != nil {
		t.Fatal(err)
	}
	e.w.now = func() time.Time { return tomorrow.Add(time.Minute) }
	res := e.run(t)
	if res.Sent != 1 || len(e.fake.To(parentMax)) != 2 {
		t.Fatalf("разовое — родителю: %+v", res)
	}
	if n := len(e.fake.To(kidMax)); n != 1 {
		t.Fatalf("ученику чужое разовое напоминание не уходит: %d", n)
	}
}

func TestRun_DeletedTrajectoryIsSilentAndPurged(t *testing.T) {
	e := setup(t)
	e.atFirstDue(t)
	if err := e.st.DeleteTrajectory(context.Background(), e.kid.TrajectoryID, e.kid.MemberID); err != nil {
		t.Fatal(err)
	}
	if res := e.run(t); res.Sent != 0 {
		t.Fatalf("удалённой траектории не напоминаем: %+v", res)
	}
	// Часы теста отстают от now() базы: сдвигаем метку удаления к ним.
	if _, err := e.db.Exec(context.Background(), `UPDATE trajectories SET deleted_at = $1`, e.w.now()); err != nil {
		t.Fatal(err)
	}
	later := e.w.now().Add(2 * time.Hour)
	e.w.now = func() time.Time { return later }
	e.run(t)
	var n int
	_ = e.db.QueryRow(context.Background(), `SELECT count(*) FROM trajectories`).Scan(&n)
	if n != 0 {
		t.Fatalf("данные стёрты физически: %d траекторий", n)
	}
}

func TestRun_WithoutTokenOnlyMaintainsPlan(t *testing.T) {
	e := setup(t)
	e.atFirstDue(t)
	e.w.max = nil
	res, err := e.w.Run(context.Background())
	if err != nil || res.Sent != 0 || res.Processed != 0 {
		t.Fatalf("без токена: %+v %v", res, err)
	}
	var planned int
	_ = e.db.QueryRow(context.Background(), `SELECT count(*) FROM reminders WHERE status = 'planned'`).Scan(&planned)
	if planned == 0 {
		t.Fatal("план сохранён до появления токена")
	}
}
