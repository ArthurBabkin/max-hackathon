package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

type fixture struct {
	trajectoryID, creatorMember, creatorUser string
}

func seedTrajectory(t *testing.T, s *Store, creatorMax int64, role string) fixture {
	t.Helper()
	ctx := context.Background()
	uid, err := s.UpsertUser(ctx, creatorMax, "Артём")
	if err != nil {
		t.Fatal(err)
	}
	n := demoTrajectory(uid)
	n.Role = role
	m, err := s.CreateTrajectory(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{trajectoryID: m.TrajectoryID, creatorMember: m.MemberID, creatorUser: uid}
}

func addMember(t *testing.T, s *Store, trajectoryID, userID, role string, creator bool) string {
	t.Helper()
	id, err := s.addMember(context.Background(), trajectoryID, userID, role, creator)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestUpsertUser_KeepsIDAndRefreshesName(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	a, err := s.UpsertUser(ctx, 900000001, "Артём")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.UpsertUser(ctx, 900000001, "Тёма")
	if err != nil || a != b {
		t.Fatalf("повторный вход должен вернуть того же пользователя: %s, %s, %v", a, b, err)
	}
	var name string
	_ = s.db.QueryRow(ctx, "SELECT first_name FROM users WHERE id = $1", a).Scan(&name)
	if name != "Тёма" {
		t.Fatalf("имя не обновилось: %q", name)
	}
}

func TestCurrentMember_FindsActiveAndComputesHasKid(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	if _, err := s.CurrentMember(ctx, 900000009); !errors.Is(err, ErrNotFound) {
		t.Fatalf("без траектории ждём ErrNotFound, получили %v", err)
	}
	f := seedTrajectory(t, s, 900000002, "parent")
	m, err := s.CurrentMember(ctx, 900000002)
	if err != nil {
		t.Fatal(err)
	}
	if m.MemberID != f.creatorMember || !m.IsCreator || m.Role != "parent" || m.HasKid {
		t.Fatalf("неверное участие: %+v", m)
	}
	if len(m.ReminderOffsets) != 4 {
		t.Fatalf("пороги по умолчанию 30/7/3/1, получили %v", m.ReminderOffsets)
	}

	kid, _ := s.UpsertUser(ctx, 900000001, "Артём")
	addMember(t, s, f.trajectoryID, kid, "kid", false)
	m, _ = s.CurrentMember(ctx, 900000002)
	if !m.HasKid {
		t.Fatal("после подключения ученика has_kid должен стать true")
	}
}

func TestActiveMember_RemovedOrDeletedIsNotFound(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	parent, _ := s.UpsertUser(ctx, 900000002, "Ольга")
	pm := addMember(t, s, f.trajectoryID, parent, "parent", false)

	if _, err := s.ActiveMember(ctx, pm); err != nil {
		t.Fatalf("живой участник: %v", err)
	}
	_, _ = s.db.Exec(ctx, "UPDATE members SET removed_at = now() WHERE id = $1", pm)
	if _, err := s.ActiveMember(ctx, pm); !errors.Is(err, ErrNotFound) {
		t.Fatalf("удалённый участник должен терять доступ, получили %v", err)
	}
	_, _ = s.db.Exec(ctx, "UPDATE trajectories SET deleted_at = now() WHERE id = $1", f.trajectoryID)
	if _, err := s.ActiveMember(ctx, f.creatorMember); !errors.Is(err, ErrNotFound) {
		t.Fatalf("после удаления траектории доступа нет ни у кого, получили %v", err)
	}
}

func TestTrajectory_Summary(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	tr, err := s.Trajectory(ctx, f.trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if tr.StudentName != "Артём" || tr.Grade != 9 || tr.RegionCode != "16" || !tr.HasKid || tr.MembersCount != 1 {
		t.Fatalf("сводка: %+v", tr)
	}
	if tr.DirectionName == nil || *tr.DirectionName != "Программная инженерия" {
		t.Fatalf("название направления из сида: %v", tr.DirectionName)
	}
}
