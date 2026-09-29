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
// объединяются без повторов; пустой список — «пока не решил». Места «Где
// учиться» — в порядке выбора, пустой список — «не важно» (онбординг v2).
func TestUpdateTrajectory_DirectionsAndPlaces(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	places := []Place{{RegionCode: "16", City: "Казань"}, {RegionCode: "77"}}
	err := s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{
		DirectionIDs: []string{"napr-38-03-01", "napr-09-03-04"}, Places: places,
		Experience: ptr("school"), HomeCity: ptr("Казань")})
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
	if !slices.Equal(tr.Places, places) || tr.Experience != "school" || tr.HomeCity == nil || *tr.HomeCity != "Казань" {
		t.Fatalf("места, опыт, город: %+v", tr)
	}

	// Имя без направлений — направления и места не трогаются.
	if err := s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{StudentName: ptr("Тёма")}); err != nil {
		t.Fatal(err)
	}
	if tr, _ = s.Trajectory(ctx, f.trajectoryID); len(tr.Directions) != 2 || len(tr.Places) != 2 || tr.Experience != "school" {
		t.Fatalf("правка имени задела цель: %+v", tr)
	}

	err = s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{
		DirectionIDs: []string{"napr-09-03-04"}, GoalStatus: ptr("suggested"), GoalByKid: ptr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if tr, _ = s.Trajectory(ctx, f.trajectoryID); tr.GoalStatus != "suggested" || !tr.GoalByKid {
		t.Fatalf("предложенная цель: %+v", tr)
	}

	err = s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{DirectionIDs: []string{}, Places: []Place{}})
	if err != nil {
		t.Fatal(err)
	}
	tr, _ = s.Trajectory(ctx, f.trajectoryID)
	if len(tr.Directions) != 0 || tr.GoalStatus != "exploring" || len(tr.Places) != 0 || len(tr.DirectionSubjects) != 0 {
		t.Fatalf("«пока не решил» и «не важно»: %+v", tr)
	}
	err = s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{Experience: ptr("pro")})
	if err == nil {
		t.Fatal("неизвестный опыт принят")
	}
	err = s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{DirectionIDs: []string{"нет-такого"}})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("неизвестное направление — ErrNotFound, получили %v", err)
	}
}

// Вузы для онбординга (SPEC 8): Москва вместе с областью, город отдельно от
// региона, по направлению, страницами.
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
	moscow, err := s.SuggestUniversities(ctx, []string{"napr-01-03-02"}, []Place{{RegionCode: "77"}}, 6, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(moscow); !slices.Contains(got, "mipt") || !slices.Contains(got, "msu") || slices.Contains(got, "sechenov") ||
		slices.Contains(got, "spbu") {
		t.Fatalf("ПМИ в Москве с областью: %v", got)
	}
	kazan, _ := s.SuggestUniversities(ctx, nil, []Place{{RegionCode: "16"}}, 6, 0)
	if got := ids(kazan); len(got) != 3 || !slices.Contains(got, "kazan-gmu") {
		t.Fatalf("все вузы Татарстана: %v", got)
	}
	city, _ := s.SuggestUniversities(ctx, nil, []Place{{RegionCode: "16", City: "Иннополис"}}, 6, 0)
	if got := ids(city); !slices.Equal(got, []string{"innopolis"}) {
		t.Fatalf("только Иннополис: %v", got)
	}
	anywhere, _ := s.SuggestUniversities(ctx, nil, nil, 4, 0)
	next, _ := s.SuggestUniversities(ctx, nil, nil, 4, 4)
	if len(anywhere) != 4 || len(next) != 4 || slices.Contains(ids(next), anywhere[0].ID) {
		t.Fatalf("страницы: %v, %v", ids(anywhere), ids(next))
	}
}

// Направления, по которым есть вузы в выбранных местах (SPEC 8.3, V3).
func TestDirectionsIn(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	tatarstan, err := s.DirectionsIn(ctx, []Place{{RegionCode: "16"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(tatarstan, "napr-31-05-01") || !slices.Contains(tatarstan, "napr-09-03-04") ||
		slices.Contains(tatarstan, "napr-19-03-01") {
		t.Fatalf("направления Татарстана: %v", tatarstan)
	}
	spb, _ := s.DirectionsIn(ctx, []Place{{RegionCode: "78"}})
	if !slices.Contains(spb, "napr-19-03-01") || !slices.Contains(spb, "napr-03-03-02") {
		t.Fatalf("направления Петербурга: %v", spb)
	}
	// Подсказка бота — только из основных направлений клавиатуры.
	anywhere, _ := s.DirectionsIn(ctx, nil)
	popular, _ := s.Directions(ctx)
	if len(anywhere) > len(popular) {
		t.Fatalf("подсказка шире клавиатуры: %d из %d", len(anywhere), len(popular))
	}
}

// Согласие с политикой конфиденциальности (152-ФЗ): бот спрашивает его до
// анкеты и до входа по приглашению. Отзыв — /delete создателем или выход из
// траектории: вернувшегося бот спросит снова.
func TestPrivacy_AcceptAndWithdraw(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	accepted := func(userID string) bool {
		t.Helper()
		ok, err := s.PrivacyAccepted(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}

	f := seedTrajectory(t, s, 900000001, "kid")
	if accepted(f.creatorUser) {
		t.Fatal("согласия ещё не давали")
	}
	if err := s.AcceptPrivacy(ctx, f.creatorUser); err != nil || !accepted(f.creatorUser) {
		t.Fatalf("согласие не записано: %v", err)
	}

	olga, _ := s.UpsertUser(ctx, 900000002, "Ольга")
	igor, _ := s.UpsertUser(ctx, 900000003, "Игорь")
	olgaMember := addMember(t, s, f.trajectoryID, olga, "parent", false)
	igorMember := addMember(t, s, f.trajectoryID, igor, "parent", false)
	for _, u := range []string{olga, igor} {
		_ = s.AcceptPrivacy(ctx, u)
	}

	// Вместе с согласием уходит и черновик анкеты: в нём имя ученика, класс, регион.
	draft := func(userID string) bool {
		t.Helper()
		_, err := s.Dialog(ctx, userID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		return err == nil
	}
	for _, u := range []string{f.creatorUser, olga, igor} {
		if _, err := s.StartDialog(ctx, Dialog{UserID: u, Step: "done", Draft: Draft{Name: "Артём", Grade: 9}}); err != nil {
			t.Fatal(err)
		}
	}

	// Вышла сама — отозвала. Удалил создатель — её решения тут нет.
	if _, err := s.LeaveTrajectory(ctx, f.trajectoryID, olgaMember); err != nil || accepted(olga) || draft(olga) {
		t.Fatalf("выход из траектории отзывает согласие и стирает черновик: %v", err)
	}
	if _, err := s.RemoveMember(ctx, f.trajectoryID, igorMember, f.creatorMember); err != nil || !accepted(igor) || !draft(igor) {
		t.Fatalf("удалённый создателем согласия не отзывал: %v", err)
	}

	if err := s.DeleteTrajectory(ctx, f.trajectoryID, f.creatorMember); err != nil || accepted(f.creatorUser) || draft(f.creatorUser) {
		t.Fatalf("/delete отзывает согласие создателя и стирает черновик: %v", err)
	}

	// Анкета брошена без траектории — отозвать можно и так; второй раз стирать нечего.
	lena, _ := s.UpsertUser(ctx, 900000004, "Лена")
	_ = s.AcceptPrivacy(ctx, lena)
	_, _ = s.StartDialog(ctx, Dialog{UserID: lena, Step: "grade"})
	if changed, err := s.WithdrawPrivacy(ctx, lena); err != nil || !changed || accepted(lena) || draft(lena) {
		t.Fatalf("отзыв без траектории: %v %v", changed, err)
	}
	if changed, err := s.WithdrawPrivacy(ctx, lena); err != nil || changed {
		t.Fatalf("повторный отзыв: %v %v", changed, err)
	}
}
