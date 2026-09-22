package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

func TestAddTrackerItem_IdempotentAndAcceptsPendingProposal(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	if _, err := s.db.Exec(ctx, `INSERT INTO proposals (trajectory_id, olympiad_profile_id) VALUES ($1, 'p669-8-informatika')`,
		f.trajectoryID); err != nil {
		t.Fatal(err)
	}

	id, created, err := s.AddTrackerItem(ctx, f.trajectoryID, "p669-8-informatika", f.creatorMember)
	if err != nil || !created || id == "" {
		t.Fatalf("первое добавление: %q %v %v", id, created, err)
	}
	again, created, err := s.AddTrackerItem(ctx, f.trajectoryID, "p669-8-informatika", f.creatorMember)
	if err != nil || created || again != id {
		t.Fatalf("повторное добавление — тот же пункт: %q %v %v", again, created, err)
	}
	var status string
	_ = s.db.QueryRow(ctx, `SELECT status FROM proposals WHERE trajectory_id = $1`, f.trajectoryID).Scan(&status)
	if status != "accepted" {
		t.Fatalf("ученик добавил предложенное сам — предложение принято, а не %q", status)
	}
	if _, _, err := s.AddTrackerItem(ctx, f.trajectoryID, "nope", f.creatorMember); !errors.Is(err, ErrNotFound) {
		t.Fatalf("неизвестный профиль — ErrNotFound, получили %v", err)
	}

	row, err := s.TrackerItem(ctx, f.trajectoryID, id)
	if err != nil || row.AddedBy == nil || row.AddedBy.Name != "Артём" || row.RegisteredAt != nil {
		t.Fatalf("пункт: %+v %v", row, err)
	}
	other := seedTrajectory(t, s, 900000003, "kid")
	if _, err := s.TrackerItem(ctx, other.trajectoryID, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("чужой пункт не виден: %v", err)
	}
	if err := s.DeleteTrackerItem(ctx, other.trajectoryID, id, other.creatorMember); !errors.Is(err, ErrNotFound) {
		t.Fatalf("чужой пункт не удаляется: %v", err)
	}
	if err := s.DeleteTrackerItem(ctx, f.trajectoryID, id, f.creatorMember); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTrackerItem(ctx, f.trajectoryID, id, f.creatorMember); !errors.Is(err, ErrNotFound) {
		t.Fatalf("второе удаление — ErrNotFound: %v", err)
	}
}

func TestSetRegistered_KeepsFirstMarkerAndCancelsRegistrationReminders(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	uid, _ := s.UpsertUser(ctx, 900000002, "Ольга")
	parent := addMember(t, s, f.trajectoryID, uid, "parent", false)
	id, _, _ := s.AddTrackerItem(ctx, f.trajectoryID, "p669-8-informatika", f.creatorMember)
	if _, err := s.db.Exec(ctx, `
		INSERT INTO reminders (tracker_item_id, stage_id, offset_days, fire_at)
		SELECT $1, st.id, 1, now() + interval '1 day' FROM stages st
		WHERE st.olympiad_profile_id = 'p669-8-informatika' AND st.kind IN ('registration', 'final')`, id); err != nil {
		t.Fatal(err)
	}

	changed, err := s.SetRegistered(ctx, f.trajectoryID, id, parent, true)
	if err != nil || !changed {
		t.Fatalf("отметка: %v %v", changed, err)
	}
	if changed, _ := s.SetRegistered(ctx, f.trajectoryID, id, f.creatorMember, true); changed {
		t.Fatal("повторная отметка ничего не меняет")
	}
	row, _ := s.TrackerItem(ctx, f.trajectoryID, id)
	if row.RegisteredAt == nil || row.RegisteredBy == nil || row.RegisteredBy.Name != "Ольга" {
		t.Fatalf("отметила Ольга: %+v", row.RegisteredBy)
	}
	statuses := map[string]string{}
	rows, _ := s.db.Query(ctx, `
		SELECT st.kind, r.status FROM reminders r JOIN stages st ON st.id = r.stage_id WHERE r.tracker_item_id = $1`, id)
	for rows.Next() {
		var kind, status string
		_ = rows.Scan(&kind, &status)
		statuses[kind] = status
	}
	if statuses["registration"] != "cancelled" || statuses["final"] != "planned" {
		t.Fatalf("отменяются только напоминания о регистрации: %v", statuses)
	}

	if changed, err := s.SetRegistered(ctx, f.trajectoryID, id, f.creatorMember, false); err != nil || !changed {
		t.Fatalf("снятие отметки: %v %v", changed, err)
	}
	row, _ = s.TrackerItem(ctx, f.trajectoryID, id)
	if row.RegisteredAt != nil || row.RegisteredBy != nil {
		t.Fatalf("отметка снята: %+v", row)
	}
	if _, err := s.SetRegistered(ctx, f.trajectoryID, "00000000-0000-4000-8000-000000000999", parent, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("нет пункта — ErrNotFound: %v", err)
	}
}

func TestPendingProposals(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	uid, _ := s.UpsertUser(ctx, 900000002, "Ольга")
	parent := addMember(t, s, f.trajectoryID, uid, "parent", false)
	_, err := s.db.Exec(ctx, `
		INSERT INTO proposals (trajectory_id, olympiad_profile_id, proposed_by_member_id, status, resolved_at) VALUES
		  ($1, 'p669-8-informatika', $2, 'pending', NULL),
		  ($1, 'vsosh-informatika', $2, 'declined', now())`, f.trajectoryID, parent)
	if err != nil {
		t.Fatal(err)
	}
	all, err := s.PendingProposals(ctx, f.trajectoryID, "")
	if err != nil || len(all) != 1 || all[0].ProposedBy == nil || all[0].ProposedBy.Name != "Ольга" ||
		all[0].OlympiadName == "" {
		t.Fatalf("ждущие ответа: %+v %v", all, err)
	}
	if mine, _ := s.PendingProposals(ctx, f.trajectoryID, parent); len(mine) != 1 {
		t.Fatalf("свои предложения родителя: %+v", mine)
	}
	if none, _ := s.PendingProposals(ctx, f.trajectoryID, f.creatorMember); none == nil || len(none) != 0 {
		t.Fatalf("у ученика своих предложений нет: %+v", none)
	}
}
