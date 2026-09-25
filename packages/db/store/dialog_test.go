package store

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

func TestMarkUpdateSeen_SecondDeliveryIsDuplicate(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	first, err := s.MarkUpdateSeen(ctx, "cb:abc")
	if err != nil || !first {
		t.Fatalf("первая доставка: %v %v", first, err)
	}
	if again, _ := s.MarkUpdateSeen(ctx, "cb:abc"); again {
		t.Fatal("повторная доставка — не обрабатывать")
	}
	if n, err := s.PurgeSeenUpdates(ctx, time.Now().Add(time.Minute)); err != nil || n != 1 {
		t.Fatalf("очистка: %d %v", n, err)
	}
}

func TestDialog_StaleStepChangesNothing(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	uid, _ := s.UpsertUser(ctx, 900000001, "Артём")
	payload := "src_class_9a"
	d, err := s.StartDialog(ctx, Dialog{UserID: uid, Step: "role", SourcePayload: &payload})
	if err != nil || d.Step != "role" || d.Role != "" || *d.SourcePayload != payload {
		t.Fatalf("старт: %+v %v", d, err)
	}
	advance := func(expect, next string, patch func(*Dialog)) (Dialog, error) {
		return s.UpdateDialog(ctx, uid, func(_ *Store, d *Dialog) error {
			if d.Step != expect {
				return ErrStale
			}
			d.Step = next
			patch(d)
			return nil
		})
	}
	d, err = advance("role", "name_confirm", func(d *Dialog) { d.Role = "kid"; d.Draft.Name = "Артём" })
	if err != nil || d.Step != "name_confirm" || d.Role != "kid" || d.Draft.Name != "Артём" {
		t.Fatalf("переход: %+v %v", d, err)
	}
	if _, err := advance("role", "name_input", func(d *Dialog) { d.Role = "parent" }); !errors.Is(err, ErrStale) {
		t.Fatalf("старая кнопка роли — ErrStale: %v", err)
	}
	if d, _ := s.Dialog(ctx, uid); d.Role != "kid" || d.Step != "name_confirm" {
		t.Fatalf("старая кнопка ничего не меняет: %+v", d)
	}

	// Два одновременных нажатия одного шага: проходит одно.
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = advance("name_confirm", "grade", func(*Dialog) {})
		}(i)
	}
	wg.Wait()
	if (results[0] == nil) == (results[1] == nil) {
		t.Fatalf("ровно одно нажатие проходит: %v", results)
	}

	// /start заново сбрасывает черновик.
	d, _ = s.StartDialog(ctx, Dialog{UserID: uid, Step: "role"})
	if d.Draft.Name != "" || d.Role != "" || d.SourcePayload != nil {
		t.Fatalf("сброс: %+v", d)
	}
}

func TestJoinByInvite_OneTimeAndKeepsLinkOnConflict(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000002, "parent")
	inv, err := s.CreateInvite(ctx, f.trajectoryID, f.creatorMember, "kid", "kidInviteTokenA1")
	if err != nil {
		t.Fatal(err)
	}
	if name, _ := s.InviterName(ctx, inv.Token); name != "Артём" { // seedTrajectory называет создателя Артёмом
		t.Fatalf("имя пригласившего: %q", name)
	}

	kid, _ := s.UpsertUser(ctx, 900000001, "Артём")
	other, _ := s.UpsertUser(ctx, 900000003, "Игорь")
	// Двое по одной ссылке одновременно: входит один.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, uid := range []string{kid, other} {
		wg.Add(1)
		go func(i int, uid string) {
			defer wg.Done()
			_, errs[i] = s.JoinByInvite(ctx, inv.Token, uid)
		}(i, uid)
	}
	wg.Wait()
	ok, invalid := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrInviteInvalid):
			invalid++
		default:
			t.Fatalf("неожиданная ошибка: %v", err)
		}
	}
	if ok != 1 || invalid != 1 {
		t.Fatalf("ссылка одноразовая: %v", errs)
	}

	// Вторая ссылка на ученика, когда он уже есть: ErrKidExists, ссылка цела.
	inv2, _ := s.db.Exec(ctx, `INSERT INTO invites (trajectory_id, token, role, created_by_member_id)
		VALUES ($1, 'kidInviteTokenA2', 'kid', $2)`, f.trajectoryID, f.creatorMember)
	_ = inv2
	third, _ := s.UpsertUser(ctx, 900000004, "Пётр")
	if _, err := s.JoinByInvite(ctx, "kidInviteTokenA2", third); !errors.Is(err, ErrKidExists) {
		t.Fatalf("ученик уже есть: %v", err)
	}
	var used *time.Time
	_ = s.db.QueryRow(ctx, `SELECT used_at FROM invites WHERE token = 'kidInviteTokenA2'`).Scan(&used)
	if used != nil {
		t.Fatal("при отказе ссылка не расходуется")
	}

	// Создатель по своей же ссылке — уже участник.
	parentInv, _ := s.CreateInvite(ctx, f.trajectoryID, f.creatorMember, "parent", "parentInviteTokA")
	if _, err := s.JoinByInvite(ctx, parentInv.Token, f.creatorUser); !errors.Is(err, ErrAlreadyMember) {
		t.Fatalf("уже участник: %v", err)
	}

	// Удалённая траектория: ссылка недействительна.
	if err := s.DeleteTrajectory(ctx, f.trajectoryID, f.creatorMember); err != nil {
		t.Fatal(err)
	}
	if _, err := s.JoinByInvite(ctx, parentInv.Token, third); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("траектория удалена: %v", err)
	}
}

func TestDeleteTrajectory_OnlyCreatorAndPurge(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	uid, _ := s.UpsertUser(ctx, 900000002, "Ольга")
	olga := addMember(t, s, f.trajectoryID, uid, "parent", false)
	_, _, _ = s.AddTrackerItem(ctx, f.trajectoryID, "p669-8-informatika", f.creatorMember)
	_ = s.SyncReminders(ctx, f.trajectoryID, 10, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))

	others, _ := s.ActiveRecipients(ctx, f.trajectoryID, f.creatorMember)
	if len(others) != 1 || others[0].MemberID != olga || others[0].MaxUserID != 900000002 {
		t.Fatalf("остальные участники: %+v", others)
	}
	if err := s.DeleteTrajectory(ctx, f.trajectoryID, olga); !errors.Is(err, ErrNotFound) {
		t.Fatalf("не создатель удалить не может: %v", err)
	}
	if err := s.DeleteTrajectory(ctx, f.trajectoryID, f.creatorMember); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CurrentMember(ctx, 900000002); !errors.Is(err, ErrNotFound) {
		t.Fatalf("доступ закрыт сразу: %v", err)
	}
	var planned int
	_ = s.db.QueryRow(ctx, `SELECT count(*) FROM reminders WHERE status = 'planned'`).Scan(&planned)
	if planned != 0 {
		t.Fatalf("напоминания отменены: %d", planned)
	}
	if n, _ := s.PurgeDeletedTrajectories(ctx, time.Now().Add(-time.Hour)); n != 0 {
		t.Fatal("свежую пометку не трогаем")
	}
	if n, err := s.PurgeDeletedTrajectories(ctx, time.Now().Add(time.Minute)); err != nil || n != 1 {
		t.Fatalf("физическое удаление: %d %v", n, err)
	}
	var left int
	_ = s.db.QueryRow(ctx, `SELECT count(*) FROM members WHERE trajectory_id = $1`, f.trajectoryID).Scan(&left)
	if left != 0 {
		t.Fatal("участия удалены каскадом")
	}
}

func TestToggleReminderOffset(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	got, err := s.ToggleReminderOffset(ctx, f.creatorMember, 7)
	if err != nil || len(got) != 3 || got[0] != 30 || got[1] != 3 {
		t.Fatalf("выключили «за неделю»: %v %v", got, err)
	}
	got, _ = s.ToggleReminderOffset(ctx, f.creatorMember, 7)
	if len(got) != 4 || got[1] != 7 {
		t.Fatalf("включили обратно, порядок от дальнего: %v", got)
	}
	if _, err := s.ToggleReminderOffset(ctx, f.creatorMember, 5); err == nil {
		t.Fatal("порог вне 30/7/3/1 отбивает CHECK")
	}
}

func TestReferenceLists(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	subs, _ := s.AllSubjects(ctx)
	dirs, _ := s.Directions(ctx)
	unis, _ := s.FindUniversities(ctx, "")
	found, _ := s.FindUniversities(ctx, "иннопол")
	if len(subs) != 9 || len(dirs) != 16 || len(unis) != 10 || len(found) != 1 || found[0].ID != "innopolis" {
		t.Fatalf("справочники: %d предметов, %d направлений, %d вузов, поиск %+v", len(subs), len(dirs), len(unis), found)
	}
}

// Все направления — для вузов и цели в мини-приложении: 72, у основных
// шестнадцати — пометка onboarding, у каждого код и группы.
func TestAllDirections(t *testing.T) {
	s := New(dbtest.Open(t))
	all, err := s.AllDirections(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	popular := 0
	var se *Direction
	for i, d := range all {
		if d.Onboarding {
			popular++
		}
		if d.ID == "napr-09-03-04" {
			se = &all[i]
		}
	}
	if len(all) != 72 || popular != 16 || se == nil || se.Code != "09.03.04" || !se.Onboarding || len(se.Groups) == 0 {
		t.Fatalf("направлений %d, основных %d, ПИ %+v", len(all), popular, se)
	}
}
