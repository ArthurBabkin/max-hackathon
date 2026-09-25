package store

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

var msk, _ = time.LoadLocation("Europe/Moscow")

func mskAt(month time.Month, day, hour int) time.Time {
	return time.Date(2026, month, day, hour, 0, 0, 0, msk)
}

type plannedRow struct {
	stage  string
	offset int
	fireAt time.Time
	status string
}

func planned(t *testing.T, s *Store, itemID string) map[string]plannedRow {
	t.Helper()
	rows, err := s.db.Query(context.Background(), `
		SELECT stage_id, offset_days, fire_at, status FROM reminders WHERE tracker_item_id = $1`, itemID)
	if err != nil {
		t.Fatal(err)
	}
	list, err := collect(rows, func(r rowScanner) (plannedRow, error) {
		var p plannedRow
		return p, r.Scan(&p.stage, &p.offset, &p.fireAt, &p.status)
	})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]plannedRow{}
	for _, p := range list {
		out[p.stage+"/"+strconv.Itoa(p.offset)] = p
	}
	return out
}

// deadlines — только напоминания о сроках, без вопросов об итоге.
func deadlines(rows map[string]plannedRow) map[string]plannedRow {
	out := map[string]plannedRow{}
	for k, r := range rows {
		if r.offset >= 0 {
			out[k] = r
		}
	}
	return out
}

func countStatus(rows map[string]plannedRow, status string) int {
	n := 0
	for _, r := range rows {
		if r.status == status {
			n++
		}
	}
	return n
}

// Этапы p669-8-informatika в сиде: регистрация до 22.09, отборочные до 11.10
// и 22.11, финал 05.02.2027 — все по Москве.
func TestSyncReminders_PlansFutureThresholdsOnly(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	item, _, err := s.AddTrackerItem(ctx, f.trajectoryID, "p669-8-informatika", f.creatorMember)
	if err != nil {
		t.Fatal(err)
	}
	now := mskAt(9, 1, 12)
	if err := s.SyncReminders(ctx, f.trajectoryID, 10, now); err != nil {
		t.Fatal(err)
	}
	got := planned(t, s, item)
	// Регистрация: «за 30 дней» (23.08) уже прошло — остаются 7, 3, 1.
	if _, ok := got["p669-8-informatika:registration:1/30"]; ok {
		t.Fatal("прошедший порог не планируется")
	}
	if len(got) != 3+4+4+4 {
		t.Fatalf("15 напоминаний, получили %d: %v", len(got), got)
	}
	if r := got["p669-8-informatika:registration:1/1"]; !r.fireAt.Equal(mskAt(9, 21, 10)) || r.status != "planned" {
		t.Fatalf("за день до регистрации в 10:00: %+v", r)
	}

	// Повторная синхронизация ничего не меняет.
	if err := s.SyncReminders(ctx, f.trajectoryID, 10, now); err != nil {
		t.Fatal(err)
	}
	if again := planned(t, s, item); len(again) != 15 || countStatus(again, "planned") != 15 {
		t.Fatalf("идемпотентность: %v", again)
	}
}

func TestSyncReminders_FollowsRegistrationAndTimeZone(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	item, _, _ := s.AddTrackerItem(ctx, f.trajectoryID, "p669-8-informatika", f.creatorMember)
	now := mskAt(9, 1, 12)
	_ = s.SyncReminders(ctx, f.trajectoryID, 10, now)

	if _, err := s.SetRegistered(ctx, f.trajectoryID, item, f.creatorMember, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SyncReminders(ctx, f.trajectoryID, 10, now); err != nil {
		t.Fatal(err)
	}
	got := planned(t, s, item)
	if r := got["p669-8-informatika:registration:1/1"]; r.status != "cancelled" {
		t.Fatalf("после отметки напоминания о регистрации отменены: %+v", r)
	}
	if countStatus(deadlines(got), "planned") != 12 {
		t.Fatalf("остальные этапы на месте: %v", got)
	}

	// Сняли отметку — напоминания о регистрации вернулись.
	if _, err := s.SetRegistered(ctx, f.trajectoryID, item, f.creatorMember, false); err != nil {
		t.Fatal(err)
	}
	_ = s.SyncReminders(ctx, f.trajectoryID, 10, now)
	if got := deadlines(planned(t, s, item)); countStatus(got, "planned") != 15 {
		t.Fatalf("после снятия отметки: %v", got)
	}

	// Сменили регион на Приморье — 10:00 по Владивостоку. Срок 11.10 23:59
	// по Москве там уже 12.10, поэтому «за день» — 11.10.
	if _, err := s.db.Exec(ctx, `UPDATE trajectories SET tz = 'Asia/Vladivostok' WHERE id = $1`, f.trajectoryID); err != nil {
		t.Fatal(err)
	}
	_ = s.SyncReminders(ctx, f.trajectoryID, 10, now)
	vl, _ := time.LoadLocation("Asia/Vladivostok")
	got = planned(t, s, item)
	if r := got["p669-8-informatika:qualifying:1/1"]; !r.fireAt.Equal(time.Date(2026, 10, 11, 10, 0, 0, 0, vl)) {
		t.Fatalf("пороги пересчитаны по новой зоне: %+v", r)
	}
}

func TestSyncReminders_LeavesDueReminderToWorker(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	item, _, _ := s.AddTrackerItem(ctx, f.trajectoryID, "p669-8-informatika", f.creatorMember)
	_ = s.SyncReminders(ctx, f.trajectoryID, 10, mskAt(9, 1, 12))

	// 21.09 в 10:05 порог «за день» до регистрации наступил, воркер ещё не
	// успел: синхронизация не должна отодвинуть его на 22.09.
	now := mskAt(9, 21, 10).Add(5 * time.Minute)
	if err := s.SyncReminders(ctx, f.trajectoryID, 10, now); err != nil {
		t.Fatal(err)
	}
	if r := planned(t, s, item)["p669-8-informatika:registration:1/1"]; !r.fireAt.Equal(mskAt(9, 21, 10)) || r.status != "planned" {
		t.Fatalf("наступившее напоминание остаётся воркеру: %+v", r)
	}
	due, err := s.DueReminders(ctx, now, 100)
	if err != nil {
		t.Fatal(err)
	}
	var found *DueReminder
	for i := range due {
		if due[i].StageID == "p669-8-informatika:registration:1" && due[i].Offset == 1 {
			found = &due[i]
		}
	}
	if found == nil || found.StudentName != "Артём" || found.TZ != "Europe/Moscow" || found.OlympiadName == "" {
		t.Fatalf("напоминание к отправке: %+v", due)
	}
}

func TestReminderDelivery_OncePerRecipient(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	uid, _ := s.UpsertUser(ctx, 900000002, "Ольга")
	olga := addMember(t, s, f.trajectoryID, uid, "parent", false)
	uid2, _ := s.UpsertUser(ctx, 900000003, "Игорь")
	igor := addMember(t, s, f.trajectoryID, uid2, "parent", false)
	// Игорь оставил только «за неделю».
	if _, err := s.db.Exec(ctx, `UPDATE members SET reminder_offsets = '{7}' WHERE id = $1`, igor); err != nil {
		t.Fatal(err)
	}
	_, _, _ = s.AddTrackerItem(ctx, f.trajectoryID, "p669-8-informatika", f.creatorMember)
	_ = s.SyncReminders(ctx, f.trajectoryID, 10, mskAt(9, 1, 12))

	now := mskAt(9, 21, 10).Add(time.Minute)
	due, _ := s.DueReminders(ctx, now, 100)
	var d DueReminder
	for _, x := range due {
		if x.Offset == 1 {
			d = x
		}
	}
	if d.ID == "" {
		t.Fatalf("нет напоминания «за день»: %+v", due)
	}
	rs, err := s.ReminderRecipients(ctx, d)
	if err != nil || len(rs) != 2 || rs[0].MemberID != f.creatorMember || rs[1].MemberID != olga || rs[1].MaxUserID != 900000002 {
		t.Fatalf("получатели «за день» — Артём и Ольга, без Игоря: %+v %v", rs, err)
	}

	claimed, done, err := s.ClaimDelivery(ctx, d.ID, olga)
	if err != nil || !claimed || done {
		t.Fatalf("первая отправка: claimed=%v done=%v %v", claimed, done, err)
	}
	// Пара занята, но отправка не завершена — так выглядит параллельный
	// запуск, ещё ждущий ответа MAX. Второй раз слать нельзя, но и
	// обслуженной пара не считается: иначе напоминание пометят разосланным
	// до того, как сообщение ушло.
	if claimed, done, _ := s.ClaimDelivery(ctx, d.ID, olga); claimed || done {
		t.Fatalf("занятая незавершённая пара: claimed=%v done=%v", claimed, done)
	}
	if err := s.ReleaseDelivery(ctx, d.ID, olga); err != nil {
		t.Fatal(err)
	}
	if claimed, _, _ := s.ClaimDelivery(ctx, d.ID, olga); !claimed {
		t.Fatal("после неудачной отправки — можно снова")
	}
	if err := s.SetDeliveryMessage(ctx, d.ID, olga, "mid-1"); err != nil {
		t.Fatal(err)
	}
	if claimed, done, _ := s.ClaimDelivery(ctx, d.ID, olga); claimed || !done {
		t.Fatalf("доставленная пара: claimed=%v done=%v", claimed, done)
	}
	// Остановивший бота получатель тоже обслужен: повторять нечего.
	if claimed, _, _ := s.ClaimDelivery(ctx, d.ID, f.creatorMember); !claimed {
		t.Fatal("пара второго получателя свободна")
	}
	if err := s.MarkDeliveryDone(ctx, d.ID, f.creatorMember); err != nil {
		t.Fatal(err)
	}
	if claimed, done, _ := s.ClaimDelivery(ctx, d.ID, f.creatorMember); claimed || !done {
		t.Fatalf("пара заблокировавшего бота: claimed=%v done=%v", claimed, done)
	}
	if err := s.MarkReminderSent(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	due, _ = s.DueReminders(ctx, now, 100)
	for _, x := range due {
		if x.ID == d.ID {
			t.Fatalf("отправленное больше не к отправке: %+v", x)
		}
	}
}

func TestRemindTomorrow_OnlyRequesterAndNoDuplicates(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	uid, _ := s.UpsertUser(ctx, 900000002, "Ольга")
	addMember(t, s, f.trajectoryID, uid, "parent", false)
	item, _, _ := s.AddTrackerItem(ctx, f.trajectoryID, "p669-8-informatika", f.creatorMember)

	stage := "p669-8-informatika:qualifying:1"
	fire := mskAt(10, 2, 10)
	created, err := s.RemindTomorrow(ctx, item, stage, f.creatorMember, fire)
	if err != nil || !created {
		t.Fatalf("первое нажатие: %v %v", created, err)
	}
	if created, _ := s.RemindTomorrow(ctx, item, stage, f.creatorMember, fire); created {
		t.Fatal("повторное нажатие до отправки ничего не добавляет")
	}
	due, _ := s.DueReminders(ctx, fire, 100)
	if len(due) != 1 || due[0].Offset != 0 || due[0].RequestedBy == nil || *due[0].RequestedBy != f.creatorMember {
		t.Fatalf("разовое напоминание: %+v", due)
	}
	rs, _ := s.ReminderRecipients(ctx, due[0])
	if len(rs) != 1 || rs[0].MemberID != f.creatorMember {
		t.Fatalf("получает только тот, кто просил: %+v", rs)
	}

	// Синхронизация плана разовые напоминания не отменяет.
	if err := s.SyncReminders(ctx, f.trajectoryID, 10, mskAt(10, 1, 12)); err != nil {
		t.Fatal(err)
	}
	if got := planned(t, s, item)[stage+"/0"]; got.status != "planned" {
		t.Fatalf("разовое напоминание пережило синхронизацию: %+v", got)
	}
}

func TestExpireReminders_AfterDeadline(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	item, _, _ := s.AddTrackerItem(ctx, f.trajectoryID, "p669-8-informatika", f.creatorMember)
	_ = s.SyncReminders(ctx, f.trajectoryID, 10, mskAt(9, 1, 12))

	// Воркер простаивал с 14.09: к 21.09 10:00 наступили «за неделю», «за
	// три дня» и «за день» до регистрации. Слать все три разом бессмысленно —
	// ближайший порог вытесняет прошлые.
	n, err := s.ExpireReminders(ctx, mskAt(9, 21, 10))
	if err != nil || n != 2 {
		t.Fatalf("вытеснены «за неделю» и «за три дня»: %d %v", n, err)
	}
	got := planned(t, s, item)
	if got["p669-8-informatika:registration:1/7"].status != "cancelled" || got["p669-8-informatika:registration:1/1"].status != "planned" {
		t.Fatalf("%+v", got)
	}

	n, err = s.ExpireReminders(ctx, mskAt(9, 23, 9))
	if err != nil || n != 1 {
		t.Fatalf("после срока отменяется и последнее: %d %v", n, err)
	}
	if r := planned(t, s, item)["p669-8-informatika:registration:1/1"]; r.status != "cancelled" {
		t.Fatalf("%+v", r)
	}
}
