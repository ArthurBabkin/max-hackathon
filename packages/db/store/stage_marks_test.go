package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

// Биология Всесибирской олимпиады в сиде: регистрация до 30.10, отборочный 01.11,
// регистрация на заключительный до 05.03, финал 07.03.2027 — по Москве.
const (
	bioProfile = "p669-14-biologiya"
	bioReg1    = "p669-14-biologiya:registration:1"
	bioQual    = "p669-14-biologiya:qualifying:1"
	bioReg2    = "p669-14-biologiya:registration:2"
	bioFinal   = "p669-14-biologiya:final:1"
)

func bioItem(t *testing.T, s *Store, f fixture) string {
	t.Helper()
	id, _, err := s.AddTrackerItem(context.Background(), f.trajectoryID, bioProfile, f.creatorMember)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func stageStatuses(t *testing.T, s *Store, itemID string) map[string]string {
	t.Helper()
	out := map[string]string{}
	// Этап «planned», если планом стоит хоть один его порог.
	for _, r := range planned(t, s, itemID) {
		if r.offset > 0 && (out[r.stage] == "" || r.status == "planned") {
			out[r.stage] = r.status
		}
	}
	return out
}

func TestSetStageMark_ResultImpliesRegistrationAndCancelsReminders(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	item := bioItem(t, s, f)
	if err := s.SyncReminders(ctx, f.trajectoryID, 10, mskAt(9, 25, 12)); err != nil {
		t.Fatal(err)
	}
	now := mskAt(11, 5, 12)

	changed, err := s.SetStageMark(ctx, f.trajectoryID, item, bioQual, f.creatorMember, stages.Mark{Result: stages.Passed}, now)
	if err != nil || !changed {
		t.Fatalf("итог отборочного: %v %v", changed, err)
	}
	row, _ := s.TrackerItem(ctx, f.trajectoryID, item)
	if row.RegisteredAt == nil {
		t.Fatal("итог подразумевает регистрацию — registered_at проставлен")
	}
	if got := stageStatuses(t, s, item); got[bioReg1] != "cancelled" || got[bioQual] != "cancelled" ||
		got[bioReg2] != "planned" || got[bioFinal] != "planned" {
		t.Fatalf("отменены напоминания отмеченных этапов, остальные на месте: %v", got)
	}
	if changed, _ := s.SetStageMark(ctx, f.trajectoryID, item, bioQual, f.creatorMember, stages.Mark{Result: stages.Passed}, now); changed {
		t.Fatal("та же отметка ничего не меняет")
	}

	if _, err := s.SetStageMark(ctx, f.trajectoryID, item, bioQual, f.creatorMember, stages.Mark{Result: stages.Failed}, now); err != nil {
		t.Fatal(err)
	}
	if got := stageStatuses(t, s, item); got[bioReg2] != "cancelled" || got[bioFinal] != "cancelled" {
		t.Fatalf("«не прошёл» закрывает олимпиаду — напоминаний нет: %v", got)
	}
	if _, err := s.SetStageMark(ctx, f.trajectoryID, item, bioReg2, f.creatorMember, stages.Mark{Registered: true}, now); !errors.Is(err, stages.ErrConflict) {
		t.Fatalf("отметка после закрывающей — ErrConflict: %v", err)
	}

	// Сняли итог — напоминания вернулись.
	if _, err := s.SetStageMark(ctx, f.trajectoryID, item, bioQual, f.creatorMember, stages.Mark{}, now); err != nil {
		t.Fatal(err)
	}
	if err := s.SyncReminders(ctx, f.trajectoryID, 10, now); err != nil {
		t.Fatal(err)
	}
	if got := stageStatuses(t, s, item); got[bioReg2] != "planned" || got[bioFinal] != "planned" {
		t.Fatalf("после снятия итога напоминания вернулись: %v", got)
	}
}

func TestSetStageMark_Errors(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	item := bioItem(t, s, f)
	now := mskAt(11, 5, 12)

	if _, err := s.SetStageMark(ctx, f.trajectoryID, item, "p669-8-informatika:final:1", f.creatorMember, stages.Mark{Registered: true}, now); !errors.Is(err, stages.ErrUnknownStage) {
		t.Fatalf("этап другой олимпиады — ErrUnknownStage: %v", err)
	}
	if _, err := s.SetStageMark(ctx, f.trajectoryID, item, bioFinal, f.creatorMember, stages.Mark{Result: stages.Winner}, now); !errors.Is(err, stages.ErrNotAllowed) {
		t.Fatalf("финал ещё не начался — ErrNotAllowed: %v", err)
	}
	other := seedTrajectory(t, s, 900000003, "kid")
	if _, err := s.SetStageMark(ctx, other.trajectoryID, item, bioQual, other.creatorMember, stages.Mark{Result: stages.Passed}, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("чужой пункт — ErrNotFound: %v", err)
	}
}

func TestSetRegistered_ConflictWithStageMarks(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	item := bioItem(t, s, f)
	if _, err := s.SetStageMark(ctx, f.trajectoryID, item, bioQual, f.creatorMember, stages.Mark{Result: stages.Passed}, mskAt(11, 5, 12)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetRegistered(ctx, f.trajectoryID, item, f.creatorMember, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("снять регистрацию при итогах — ErrConflict: %v", err)
	}
	row, _ := s.TrackerItem(ctx, f.trajectoryID, item)
	if row.RegisteredAt == nil {
		t.Fatal("регистрация осталась")
	}
}

func TestStageMarksAndTrackerStateProgress(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	item := bioItem(t, s, f)
	now := mskAt(11, 5, 12)
	_, _ = s.SetStageMark(ctx, f.trajectoryID, item, bioQual, f.creatorMember, stages.Mark{Result: stages.Passed}, now)
	_, _ = s.SetStageMark(ctx, f.trajectoryID, item, bioReg2, f.creatorMember, stages.Mark{Registered: true}, mskAt(11, 6, 12))

	marks, err := s.StageMarks(ctx, []string{item})
	if err != nil {
		t.Fatal(err)
	}
	if m := marks[item]; m[bioQual].Result != stages.Passed || !m[bioReg2].Registered || len(m) != 2 {
		t.Fatalf("отметки пункта: %+v", m)
	}
	state, err := s.TrackerState(ctx, f.trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if p := state.Progress[bioProfile]; !p.Registered || p.Marks[bioQual].Result != stages.Passed {
		t.Fatalf("прогресс в состоянии трекера: %+v", p)
	}
}

// Вопрос об итоге этапа: на следующий день после окончания и через неделю,
// только зарегистрированным и только пока итог нужен.
func TestSyncReminders_PlansResultAsks(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	item := bioItem(t, s, f)
	now := mskAt(10, 1, 12)

	_ = s.SyncReminders(ctx, f.trajectoryID, 10, now)
	if _, ok := planned(t, s, item)[bioQual+"/-1"]; ok {
		t.Fatal("без регистрации итог не спрашиваем")
	}

	if _, err := s.SetRegistered(ctx, f.trajectoryID, item, f.creatorMember, true); err != nil {
		t.Fatal(err)
	}
	_ = s.SyncReminders(ctx, f.trajectoryID, 10, now)
	got := planned(t, s, item)
	if r := got[bioQual+"/-1"]; !r.fireAt.Equal(mskAt(11, 2, 10)) || r.status != "planned" {
		t.Fatalf("на следующий день после отборочного в 10:00: %+v", r)
	}
	if r := got[bioQual+"/-8"]; !r.fireAt.Equal(mskAt(11, 9, 10)) || r.status != "planned" {
		t.Fatalf("повтор через неделю: %+v", r)
	}
	if r := got[bioFinal+"/-1"]; r.status != "planned" {
		t.Fatalf("итог финала тоже спросим: %+v", r)
	}
	if _, ok := got[bioReg2+"/-1"]; ok {
		t.Fatal("у регистрации итога нет")
	}

	later := mskAt(11, 5, 12)
	if _, err := s.SetStageMark(ctx, f.trajectoryID, item, bioQual, f.creatorMember, stages.Mark{Result: stages.Passed}, later); err != nil {
		t.Fatal(err)
	}
	_ = s.SyncReminders(ctx, f.trajectoryID, 10, later)
	got = planned(t, s, item)
	if got[bioQual+"/-8"].status != "cancelled" || got[bioFinal+"/-1"].status != "planned" {
		t.Fatalf("итог отмечен — вопрос о нём отменён, о финале остался: %v", got)
	}
}

// После выкладки у давно зарегистрированных не должна прийти лавина
// вопросов о прошедших этапах.
func TestSyncReminders_NoPastResultAsks(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	item := bioItem(t, s, f)
	_, _ = s.SetRegistered(ctx, f.trajectoryID, item, f.creatorMember, true)
	_ = s.SyncReminders(ctx, f.trajectoryID, 10, mskAt(11, 20, 12))
	got := planned(t, s, item)
	if _, ok := got[bioQual+"/-1"]; ok {
		t.Fatalf("прошедший вопрос не планируется: %v", got)
	}
	if _, ok := got[bioQual+"/-8"]; ok {
		t.Fatalf("прошедший повтор не планируется: %v", got)
	}
}

func TestResultAsks_DueExpireAndRecipients(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	uid, _ := s.UpsertUser(ctx, 900000002, "Ольга")
	addMember(t, s, f.trajectoryID, uid, "parent", false)
	item := bioItem(t, s, f)
	_, _ = s.SetRegistered(ctx, f.trajectoryID, item, f.creatorMember, true)
	_ = s.SyncReminders(ctx, f.trajectoryID, 10, mskAt(10, 1, 12))

	now := mskAt(11, 2, 10).Add(5 * time.Minute)
	if n, err := s.ExpireReminders(ctx, now); err != nil || planned(t, s, item)[bioQual+"/-1"].status != "planned" {
		t.Fatalf("вопрос об итоге не истекает оттого, что срок этапа прошёл (%d, %v)", n, err)
	}
	due, err := s.DueReminders(ctx, now, 100)
	if err != nil {
		t.Fatal(err)
	}
	var ask *DueReminder
	for i := range due {
		if due[i].StageID == bioQual && due[i].Offset == -1 {
			ask = &due[i]
		}
	}
	if ask == nil {
		t.Fatalf("вопрос пора отправить: %+v", due)
	}
	rs, err := s.ReminderRecipients(ctx, *ask)
	if err != nil || len(rs) != 1 || rs[0].Role != "kid" {
		t.Fatalf("итог спрашиваем у ученика, а не у всей семьи: %+v %v", rs, err)
	}
	if _, err := s.db.Exec(ctx, `UPDATE members SET reminder_offsets = '{}' WHERE id = $1`, f.creatorMember); err != nil {
		t.Fatal(err)
	}
	if rs, _ := s.ReminderRecipients(ctx, *ask); len(rs) != 0 {
		t.Fatalf("ученик выключил напоминания — не спрашиваем: %+v", rs)
	}

	if _, err := s.ExpireReminders(ctx, mskAt(11, 5, 11)); err != nil {
		t.Fatal(err)
	}
	if r := planned(t, s, item)[bioQual+"/-1"]; r.status != "cancelled" {
		t.Fatalf("через три дня вопрос устарел: %+v", r)
	}
}

// Без ученика в семье итог спрашиваем у родителей.
func TestResultAsks_ParentWhenNoKid(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "parent")
	item := bioItem(t, s, f)
	_, _ = s.SetRegistered(ctx, f.trajectoryID, item, f.creatorMember, true)
	_ = s.SyncReminders(ctx, f.trajectoryID, 10, mskAt(10, 1, 12))
	due, _ := s.DueReminders(ctx, mskAt(11, 2, 10).Add(time.Minute), 100)
	for _, d := range due {
		if d.Offset == -1 && d.StageID == bioQual {
			rs, err := s.ReminderRecipients(ctx, d)
			if err != nil || len(rs) != 1 || rs[0].Role != "parent" {
				t.Fatalf("родитель без ученика: %+v %v", rs, err)
			}
			return
		}
	}
	t.Fatal("вопрос не найден")
}

func TestRemindTomorrow_NotForClosedOlympiad(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	item := bioItem(t, s, f)
	if _, err := s.SetStageMark(ctx, f.trajectoryID, item, bioQual, f.creatorMember, stages.Mark{Result: stages.Failed}, mskAt(11, 5, 12)); err != nil {
		t.Fatal(err)
	}
	created, err := s.RemindTomorrow(ctx, item, bioReg2, f.creatorMember, mskAt(11, 6, 10))
	if err != nil || created {
		t.Fatalf("олимпиада закрыта — «напомнить завтра» ничего не ставит: %v %v", created, err)
	}
}
