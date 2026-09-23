package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

func ptr[T any](v T) *T { return &v }

func demoTrajectory(userID string) NewTrajectory {
	return NewTrajectory{
		CreatorUserID: userID, Role: "kid", StudentName: "Артём", Grade: 9, RegionCode: "16",
		TZ: "Europe/Moscow", DirectionIDs: []string{"napr-09-03-04"},
		SubjectCodes: []string{"inf", "math"}, UniversityIDs: []string{"innopolis", "hse", "kfu"},
	}
}

func TestCreateTrajectory_AllOrNothing(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	uid, _ := s.UpsertUser(ctx, 900000001, "Артём")

	bad := demoTrajectory(uid)
	bad.UniversityIDs = []string{"innopolis", "нет-такого-вуза"}
	if _, err := s.CreateTrajectory(ctx, bad); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ссылка на несуществующий вуз — ErrNotFound, получили %v", err)
	}
	var n int
	_ = s.db.QueryRow(ctx, "SELECT count(*) FROM trajectories").Scan(&n)
	if n != 0 {
		t.Fatal("после ошибки не должно остаться частичной траектории")
	}

	m, err := s.CreateTrajectory(ctx, demoTrajectory(uid))
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsCreator || m.Role != "kid" || !m.HasKid {
		t.Fatalf("создатель: %+v", m)
	}
	var subjects, vuzes, audit int
	_ = s.db.QueryRow(ctx, "SELECT count(*) FROM trajectory_subjects WHERE trajectory_id = $1", m.TrajectoryID).Scan(&subjects)
	_ = s.db.QueryRow(ctx, "SELECT count(*) FROM trajectory_universities WHERE trajectory_id = $1", m.TrajectoryID).Scan(&vuzes)
	_ = s.db.QueryRow(ctx, "SELECT count(*) FROM audit_log WHERE entity_id = $1", m.TrajectoryID).Scan(&audit)
	if subjects != 2 || vuzes != 3 || audit != 1 {
		t.Fatalf("предметы %d, вузы %d, аудит %d", subjects, vuzes, audit)
	}
}

func TestAddMember_SecondKidIsConflict(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	kid, _ := s.UpsertUser(ctx, 900000001, "Артём")
	m, err := s.CreateTrajectory(ctx, demoTrajectory(kid))
	if err != nil {
		t.Fatal(err)
	}
	other, _ := s.UpsertUser(ctx, 900000003, "Петя")
	_, err = s.AddMember(ctx, m.TrajectoryID, other, "kid")
	if !IsConflictOn(err, "members_single_active_kid_uniq") {
		t.Fatalf("второй ученик должен упереться в members_single_active_kid_uniq, получили %v", err)
	}
	parent, _ := s.UpsertUser(ctx, 900000002, "Ольга")
	pm, err := s.AddMember(ctx, m.TrajectoryID, parent, "parent")
	if err != nil || pm.IsCreator || !pm.HasKid {
		t.Fatalf("родитель: %+v, %v", pm, err)
	}
	if _, err := s.AddMember(ctx, m.TrajectoryID, parent, "parent"); !IsConflictOn(err, "members_active_uniq") {
		t.Fatalf("повторное подключение — members_active_uniq, получили %v", err)
	}
}
