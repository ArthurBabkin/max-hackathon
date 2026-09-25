package store

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

// Направления из сида: ПИ — цель Артёма, ПМИ и ИБ — рядом, 09.00.00 —
// укрупнённая группа Иннополиса.
const (
	dirSE   = "napr-09-03-04"
	dirAMI  = "napr-01-03-02"
	dirIS   = "napr-10-03-01"
	dirIVT  = "napr-09-00-00"
	dirBio  = "napr-06-03-01"
	virtual = "p669-5-virtualnye-miry-razrabotka-kompyuternyh-igr-tehnologii-virtualnoy-realnosti-tehnologii-dopolnennoy-realnosti-cifrovye-tehnologii-v-arhitekture"
	infosec = "p669-22-informacionnaya-bezopasnost"
)

func chosenAt(t *testing.T, s *Store, trajectoryID, universityID string) []string {
	t.Helper()
	var ids []string
	if err := s.db.QueryRow(context.Background(), `
		SELECT COALESCE(array_agg(direction_id ORDER BY direction_id), '{}') FROM trajectory_university_directions
		WHERE trajectory_id = $1 AND university_id = $2`, trajectoryID, universityID).Scan(&ids); err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestSetUniversityDirections(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")

	// Вуз не из моих — выбор добавляет его, а направление — в цель.
	if err := s.SetUniversityDirections(ctx, f.trajectoryID, "spbu", f.creatorMember, []string{dirAMI}); err != nil {
		t.Fatal(err)
	}
	tr, _ := s.Trajectory(ctx, f.trajectoryID)
	var goal []string
	for _, d := range tr.Directions {
		goal = append(goal, d.ID)
	}
	mine, _ := s.TrajectoryUniversities(ctx, f.trajectoryID)
	if !slices.Equal(goal, []string{dirSE, dirAMI}) || tr.GoalStatus != "known" ||
		!slices.ContainsFunc(mine, func(u University) bool { return u.ID == "spbu" }) {
		t.Fatalf("цель %v (%s), вузы %v", goal, tr.GoalStatus, mine)
	}
	if got := chosenAt(t, s, f.trajectoryID, "spbu"); !slices.Equal(got, []string{dirAMI}) {
		t.Fatalf("выбор в СПбГУ: %v", got)
	}

	// Снятие выбора цель не трогает, вуз остаётся моим.
	if err := s.SetUniversityDirections(ctx, f.trajectoryID, "spbu", f.creatorMember, []string{}); err != nil {
		t.Fatal(err)
	}
	tr, _ = s.Trajectory(ctx, f.trajectoryID)
	if len(tr.Directions) != 2 || len(chosenAt(t, s, f.trajectoryID, "spbu")) != 0 {
		t.Fatalf("после снятия: цель %v, выбор %v", tr.Directions, chosenAt(t, s, f.trajectoryID, "spbu"))
	}

	if err := s.SetUniversityDirections(ctx, f.trajectoryID, "innopolis", f.creatorMember, []string{dirBio}); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("направления нет в вузе — ErrNotAllowed: %v", err)
	}
	if err := s.SetUniversityDirections(ctx, f.trajectoryID, "нет-такого", f.creatorMember, []string{dirSE}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("нет вуза — ErrNotFound: %v", err)
	}
}

// Сохранение профиля не стирает выбранные у вуза направления: они уходят,
// только когда уходит сам вуз.
func TestReplaceUniversities_KeepsChosenDirections(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	if err := s.SetUniversityDirections(ctx, f.trajectoryID, "hse", f.creatorMember, []string{dirSE, dirAMI}); err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{UniversityIDs: []string{"hse", "mipt"}}); err != nil {
		t.Fatal(err)
	}
	if got := chosenAt(t, s, f.trajectoryID, "hse"); len(got) != 2 {
		t.Fatalf("выбор в ВШЭ пропал при сохранении профиля: %v", got)
	}
	if err := s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{UniversityIDs: []string{"mipt"}}); err != nil {
		t.Fatal(err)
	}
	if got := chosenAt(t, s, f.trajectoryID, "hse"); len(got) != 0 {
		t.Fatalf("вуз убрали — выбор ушёл с ним: %v", got)
	}
}

func TestUniversityDirections(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	_ = s.SetUniversityDirections(ctx, f.trajectoryID, "hse", f.creatorMember, []string{dirAMI})

	ds, err := s.UniversityDirections(ctx, f.trajectoryID, "hse")
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 19 || ds[0].ID != dirAMI || !ds[0].IsMine || ds[0].BenefitOlympiads == 0 {
		t.Fatalf("выбранное — первым: %+v", ds[:2])
	}
	// За выбранными — покрывающие цель, дальше — по числу олимпиад.
	if !ds[1].IsGoal {
		t.Fatalf("вторым — направление цели: %+v", ds[1])
	}
	for i := 3; i < len(ds); i++ {
		if ds[i-1].BenefitOlympiads < ds[i].BenefitOlympiads {
			t.Fatalf("дальше — по числу олимпиад: %+v, %+v", ds[i-1], ds[i])
		}
	}
	se := ds[slices.IndexFunc(ds, func(d UniversityDirection) bool { return d.ID == dirSE })]
	if !se.IsGoal || se.IsMine || se.Code != "09.03.04" || se.Programs == 0 || se.Status != "offered" {
		t.Fatalf("ПИ — в цели, не выбрано: %+v", se)
	}

	// Укрупнённая группа Иннополиса покрывает цель 09.03.04.
	inno, _ := s.UniversityDirections(ctx, f.trajectoryID, "innopolis")
	if len(inno) != 1 || inno[0].ID != dirIVT || !inno[0].IsGoal {
		t.Fatalf("Иннополис: %+v", inno)
	}
}

// Льгота в моих вузах — на мои направления: выбранные, иначе цель, иначе
// вуз целиком (core/targets).
func TestTargetBenefits(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid") // цель ПИ, вузы Иннополис, ВШЭ, КФУ

	byPair := func(rows []BenefitRow) map[string]BenefitRow {
		out := map[string]BenefitRow{}
		for _, b := range rows {
			out[b.ProfileID+"/"+b.UniversityID] = b
		}
		return out
	}
	unis := []string{"innopolis", "hse", "kfu", "mipt"}
	rows, err := s.TargetBenefits(ctx, f.trajectoryID, []string{virtual, infosec}, unis)
	if err != nil {
		t.Fatal(err)
	}
	got := byPair(rows)

	// ВШЭ целиком даёт БВИ, но на ПИ — только 100 баллов и не на всех программах.
	if b := got[virtual+"/hse"]; b.Benefit != "score100" || !slices.Equal(b.DirectionNames, []string{"Программная инженерия"}) ||
		!b.Varies || b.Basis != "goal" {
		t.Fatalf("ВШЭ на ПИ: %+v", b)
	}
	// Иннополис: 09.00.00 покрывает ПИ.
	if b := got[virtual+"/innopolis"]; b.Benefit != "bvi" || b.Basis != "goal" {
		t.Fatalf("Иннополис: %+v", b)
	}
	// ИБ в ВШЭ даёт БВИ, но не на ПИ — строки нет.
	if b, ok := got[infosec+"/hse"]; ok {
		t.Fatalf("на ПИ в ВШЭ льготы нет: %+v", b)
	}

	// Выбор в вузе важнее цели: ИБ в ВШЭ — БВИ.
	_ = s.SetUniversityDirections(ctx, f.trajectoryID, "hse", f.creatorMember, []string{dirIS})
	rows, _ = s.TargetBenefits(ctx, f.trajectoryID, []string{virtual, infosec}, unis)
	got = byPair(rows)
	if b := got[virtual+"/hse"]; b.Benefit != "bvi" || b.Basis != "chosen" ||
		!slices.Equal(b.DirectionNames, []string{"Информационная безопасность"}) {
		t.Fatalf("ВШЭ по выбору: %+v", b)
	}

	// Льгота на выбранных направлениях разная: лучшая — в строке, прочие —
	// отдельно, от сильной к слабой.
	_ = s.SetUniversityDirections(ctx, f.trajectoryID, "hse", f.creatorMember, []string{dirSE, dirIS})
	rows, _ = s.TargetBenefits(ctx, f.trajectoryID, []string{virtual}, unis)
	got = byPair(rows)
	if b := got[virtual+"/hse"]; b.Benefit != "bvi" || !slices.Equal(b.DirectionNames, []string{"Информационная безопасность"}) ||
		len(b.OtherDirections) != 1 || b.OtherDirections[0].Benefit != "score100" ||
		!slices.Equal(b.OtherDirections[0].Names, []string{"Программная инженерия"}) {
		t.Fatalf("ВШЭ на ПИ и ИБ: %+v", b)
	}

	// Цели нет — льгота вуза целиком, как раньше.
	_ = s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{DirectionIDs: []string{}})
	rows, _ = s.TargetBenefits(ctx, f.trajectoryID, []string{virtual}, unis)
	got = byPair(rows)
	if b := got[virtual+"/mipt"]; b.Benefit != "score100" || b.Basis != "university" || len(b.DirectionNames) != 0 {
		t.Fatalf("МФТИ без цели: %+v", b)
	}
}

func TestTargetsOf(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	_ = s.SetUniversityDirections(ctx, f.trajectoryID, "hse", f.creatorMember, []string{dirAMI})

	tg, err := s.TargetsOf(ctx, f.trajectoryID, []string{"hse", "innopolis", "kazan-gmu"})
	if err != nil {
		t.Fatal(err)
	}
	if x := tg["hse"]; x.Basis != "chosen" || !slices.Equal(x.DirectionNames, []string{"Прикладная математика и информатика"}) {
		t.Fatalf("ВШЭ: %+v", x)
	}
	if x := tg["innopolis"]; x.Basis != "goal" || len(x.DirectionNames) != 1 || x.Unverified {
		t.Fatalf("Иннополис: %+v", x)
	}
	if x := tg["kazan-gmu"]; x.Basis != "university" || len(x.DirectionNames) != 0 {
		t.Fatalf("Медвуз без ПИ — вуз целиком: %+v", x)
	}
}

// На скольких направлениях вуза олимпиада даёт льготу — «на 5 из 19».
func TestDirectionCoverage(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	cov, err := s.DirectionCoverage(ctx, []string{virtual}, []string{"hse", "innopolis"})
	if err != nil {
		t.Fatal(err)
	}
	if c := cov[virtual+"/hse"]; c.Count != 5 || c.Total != 19 {
		t.Fatalf("ВШЭ: %+v", c)
	}
	if c := cov[virtual+"/innopolis"]; c.Count != 1 || c.Total != 1 {
		t.Fatalf("Иннополис: %+v", c)
	}
}

// Каталог вузов по направлению (F67): вузы, где есть направление, покрывающее
// искомое, и сколько олимпиад дают на нём льготу.
func TestUniversitiesByDirection(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid") // вузы Иннополис, ВШЭ, КФУ

	us, m, err := s.UniversitiesByDirection(ctx, f.trajectoryID, "", "", dirSE)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(us))
	for i, u := range us {
		ids[i] = u.ID
	}
	if slices.Contains(ids, "kazan-gmu") || !slices.Contains(ids, "hse") || !slices.Contains(ids, "innopolis") {
		t.Fatalf("вузы с ПИ: %v", ids)
	}
	// Иннополис — через укрупнённую группу 09.00.00.
	if x := m["innopolis"]; !slices.Equal(x.DirectionIDs, []string{dirIVT}) || x.Olympiads == 0 || x.Unverified {
		t.Fatalf("Иннополис: %+v", x)
	}
	hseDirs, _ := s.UniversityDirections(ctx, f.trajectoryID, "hse")
	se := hseDirs[slices.IndexFunc(hseDirs, func(d UniversityDirection) bool { return d.ID == dirSE })]
	if x := m["hse"]; !slices.Equal(x.DirectionIDs, []string{dirSE}) || x.Olympiads != se.BenefitOlympiads {
		t.Fatalf("ВШЭ: %+v, ждём %d олимпиад", x, se.BenefitOlympiads)
	}
	if !us[slices.Index(ids, "hse")].IsMine {
		t.Fatal("отметка «мой» сохраняется")
	}
	// Сначала — где больше олимпиад на это направление.
	for i := 1; i < len(us); i++ {
		if m[us[i-1].ID].Olympiads < m[us[i].ID].Olympiads {
			t.Fatalf("по числу олимпиад: %v", ids)
		}
	}

	// Льготы на ПМИ в НГУ ещё проверяются — вуз в конце, «уточняется».
	us, m, _ = s.UniversitiesByDirection(ctx, f.trajectoryID, "", "", dirAMI)
	if x := m["nsu"]; !x.Unverified || x.Olympiads != 0 || us[len(us)-1].ID != "nsu" {
		t.Fatalf("НГУ: %+v, последний %s", x, us[len(us)-1].ID)
	}

	// Поиск и город — вместе с направлением.
	us, _, _ = s.UniversitiesByDirection(ctx, f.trajectoryID, "", "Казань", dirSE)
	if len(us) != 1 || us[0].ID != "kfu" {
		t.Fatalf("Казань и ПИ: %+v", us)
	}

	if _, _, err := s.UniversitiesByDirection(ctx, f.trajectoryID, "", "", "нет-такого"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("нет направления — ErrNotFound: %v", err)
	}
}
