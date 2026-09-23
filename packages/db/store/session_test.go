package store

import (
	"context"
	"errors"
	"slices"
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
	if len(tr.Directions) != 1 || tr.Directions[0].Name != "Программная инженерия" || tr.GoalStatus != "known" {
		t.Fatalf("направление из сида: %+v", tr.Directions)
	}
	if !slices.Equal(tr.DirectionSubjects, []string{"inf", "math"}) {
		t.Fatalf("предметы направления: %v", tr.DirectionSubjects)
	}
}

// Несколько направлений (F8): порядок выбора сохраняется, ключевые предметы
// объединяются без повторов; пустой список — «пока не решил».
func TestUpdateTrajectory_DirectionsAndTarget(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	err := s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{
		DirectionIDs: []string{"napr-38-03-01", "napr-09-03-04"}, TargetRegionCode: ptr("77")})
	if err != nil {
		t.Fatal(err)
	}
	tr, _ := s.Trajectory(ctx, f.trajectoryID)
	if len(tr.Directions) != 2 || tr.Directions[0].ID != "napr-38-03-01" || tr.Directions[1].ID != "napr-09-03-04" {
		t.Fatalf("порядок направлений: %+v", tr.Directions)
	}
	if !slices.Equal(tr.DirectionSubjects, []string{"econ", "inf", "math", "soc"}) {
		t.Fatalf("объединённые предметы: %v", tr.DirectionSubjects)
	}
	if tr.TargetRegionCode == nil || *tr.TargetRegionCode != "77" {
		t.Fatalf("город: %v", tr.TargetRegionCode)
	}

	// Имя без направлений — направления и город не трогаются.
	if err := s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{StudentName: ptr("Тёма")}); err != nil {
		t.Fatal(err)
	}
	if tr, _ = s.Trajectory(ctx, f.trajectoryID); len(tr.Directions) != 2 || tr.TargetRegionCode == nil {
		t.Fatalf("правка имени задела цель: %+v", tr)
	}

	err = s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{DirectionIDs: []string{}, TargetRegionCode: ptr("")})
	if err != nil {
		t.Fatal(err)
	}
	tr, _ = s.Trajectory(ctx, f.trajectoryID)
	if len(tr.Directions) != 0 || tr.GoalStatus != "exploring" || tr.TargetRegionCode != nil || len(tr.DirectionSubjects) != 0 {
		t.Fatalf("«пока не решил» и «не важно»: %+v", tr)
	}
	err = s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{DirectionIDs: []string{"нет-такого"}})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("неизвестное направление — ErrNotFound, получили %v", err)
	}
}

// Вузы для онбординга (F9): Москва вместе с областью, по направлению.
func TestSuggestUniversities(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	ids := func(us []University) []string {
		out := []string{}
		for _, u := range us {
			out = append(out, u.ID)
		}
		return out
	}
	moscow, err := s.SuggestUniversities(ctx, []string{"napr-01-03-02"}, []string{"77", "50"}, 6)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(moscow); !slices.Contains(got, "mipt") || !slices.Contains(got, "msu") || slices.Contains(got, "sechenov") ||
		slices.Contains(got, "spbu") {
		t.Fatalf("ПМИ в Москве: %v", got)
	}
	kazan, _ := s.SuggestUniversities(ctx, nil, []string{"16"}, 6)
	if got := ids(kazan); len(got) != 3 || !slices.Contains(got, "kazan-gmu") {
		t.Fatalf("все вузы Татарстана: %v", got)
	}
	anywhere, _ := s.SuggestUniversities(ctx, nil, nil, 4)
	if len(anywhere) != 4 {
		t.Fatalf("лимит: %v", ids(anywhere))
	}
}
