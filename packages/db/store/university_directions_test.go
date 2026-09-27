package store

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

// Направления из сида: ПИ — цель Артёма, ПМИ и ИБ — рядом, 09.00.00 —
// укрупнённая группа Иннополиса, 40.03.01 у ВШЭ — льготы уточняются.
const (
	dirSE   = "napr-09-03-04"
	dirAMI  = "napr-01-03-02"
	dirIS   = "napr-10-03-01"
	dirIVT  = "napr-09-00-00"
	dirCS   = "napr-09-03-01"
	dirBio  = "napr-06-03-01"
	dirLaw  = "napr-40-03-01"
	virtual = "p669-5-virtualnye-miry-razrabotka-kompyuternyh-igr-tehnologii-virtualnoy-realnosti-tehnologii-dopolnennoy-realnosti-cifrovye-tehnologii-v-arhitekture"
	infosec = "p669-22-informacionnaya-bezopasnost"
	formula = "p669-2-matematika"
	dirPhys = "napr-03-03-02"
	dirEcon = "napr-38-03-01"
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

// Направление сняли с цели в настройках — оно уходит и из выбора в вузах:
// «мои направления» в каталоге и в настройках не расходятся.
func TestReplaceDirections_DropsFromUniversities(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	if err := s.SetUniversityDirections(ctx, f.trajectoryID, "hse", f.creatorMember, []string{dirSE, dirAMI}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUniversityDirections(ctx, f.trajectoryID, "spbu", f.creatorMember, []string{dirAMI}); err != nil {
		t.Fatal(err)
	}

	// Сохранение профиля с той же целью выбор не трогает.
	if err := s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{DirectionIDs: []string{dirSE, dirAMI}}); err != nil {
		t.Fatal(err)
	}
	if got := chosenAt(t, s, f.trajectoryID, "hse"); len(got) != 2 {
		t.Fatalf("та же цель — выбор в ВШЭ не меняется: %v", got)
	}

	if err := s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{DirectionIDs: []string{dirSE}}); err != nil {
		t.Fatal(err)
	}
	if got := chosenAt(t, s, f.trajectoryID, "hse"); !slices.Equal(got, []string{dirSE}) {
		t.Fatalf("ПМИ сняли с цели — в ВШЭ остаётся ПИ: %v", got)
	}
	if got := chosenAt(t, s, f.trajectoryID, "spbu"); len(got) != 0 {
		t.Fatalf("ПМИ сняли с цели — в СПбГУ выбор пуст: %v", got)
	}
	mine, _ := s.TrajectoryUniversities(ctx, f.trajectoryID)
	if !slices.ContainsFunc(mine, func(u University) bool { return u.ID == "spbu" }) {
		t.Fatalf("вуз остаётся моим: %v", mine)
	}
}

// Двое в семье одновременно отмечают направления в одном вузе: оба запроса
// проходят, выбор — одного из них, цель — без дублей и пропусков.
func TestSetUniversityDirections_Concurrent(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	picks := [][]string{{dirAMI}, {dirAMI, dirIS}, {dirIS}, {dirAMI}, {dirAMI, dirIS}, {dirIS}}
	errs := make([]error, len(picks))
	var wg sync.WaitGroup
	for i, ids := range picks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = s.SetUniversityDirections(ctx, f.trajectoryID, "hse", f.creatorMember, ids)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("запрос %d: %v", i, err)
		}
	}
	tr, _ := s.Trajectory(ctx, f.trajectoryID)
	var goal []string
	for _, d := range tr.Directions {
		goal = append(goal, d.ID)
	}
	slices.Sort(goal)
	if want := []string{dirIS, dirAMI, dirSE}; !slices.Equal(goal, slices.Sorted(slices.Values(want))) {
		t.Fatalf("цель без дублей и пропусков: %v", goal)
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
	if len(ds) != 14 || ds[0].ID != dirAMI || !ds[0].IsMine || ds[0].BenefitOlympiads == 0 {
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

	// На ПИ в ВШЭ «Виртуальные миры» и профиль ИБ льготы не дают: в Москве
	// они на ИБ, ИВТ и ИТСС (стр. 59 и 40 приложения). БВИ на ПИ давали
	// программы Нижнего Новгорода, а это филиал — отдельный вуз.
	for _, p := range []string{virtual, infosec} {
		if b, ok := got[p+"/hse"]; ok {
			t.Fatalf("ВШЭ на ПИ: %+v", b)
		}
	}
	// Иннополис: 09.00.00 покрывает ПИ.
	if b := got[virtual+"/innopolis"]; b.Benefit != "bvi" || b.Basis != "goal" {
		t.Fatalf("Иннополис: %+v", b)
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
	// «Виртуальные миры»: на ИБ — БВИ, на ИВТ — 100 баллов (Москва, стр. 59 и 40).
	_ = s.SetUniversityDirections(ctx, f.trajectoryID, "hse", f.creatorMember, []string{dirCS, dirIS})
	rows, _ = s.TargetBenefits(ctx, f.trajectoryID, []string{virtual}, unis)
	got = byPair(rows)
	if b := got[virtual+"/hse"]; b.Benefit != "bvi" || !slices.Equal(b.DirectionNames, []string{"Информационная безопасность"}) ||
		len(b.OtherDirections) != 1 || b.OtherDirections[0].Benefit != "score100" ||
		!slices.Equal(b.OtherDirections[0].Names, []string{"Информатика и вычислительная техника"}) {
		t.Fatalf("ВШЭ на ИВТ и ИБ: %+v", b)
	}

	// Цели нет — льгота вуза целиком, как раньше.
	_ = s.UpdateTrajectory(ctx, f.trajectoryID, f.creatorMember, TrajectoryPatch{DirectionIDs: []string{}})
	rows, _ = s.TargetBenefits(ctx, f.trajectoryID, []string{virtual}, unis)
	got = byPair(rows)
	if b := got[virtual+"/mipt"]; b.Benefit != "score100" || b.Basis != "university" || len(b.DirectionNames) != 0 {
		t.Fatalf("МФТИ без цели: %+v", b)
	}
}

// Одна льгота на нескольких направлениях — одна строка, но оговорки каждого
// направления в ней остаются. «Формула Единства» по математике в ВШЭ — 100
// баллов и на физике (за 10–11 класс), и на экономике (за 11 класс, только
// на «Мировой экономике»).
func TestTargetBenefits_SameBenefitOnSeveralDirections(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	if err := s.SetUniversityDirections(ctx, f.trajectoryID, "hse", f.creatorMember, []string{dirPhys, dirEcon}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.TargetBenefits(ctx, f.trajectoryID, []string{formula}, []string{"hse"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("%+v (err=%v)", rows, err)
	}
	b := rows[0]
	if b.Benefit != "score100" || !slices.Equal(b.DirectionNames, []string{"Физика", "Экономика"}) {
		t.Fatalf("100 баллов на физике и экономике: %+v", b)
	}
	// Классы — всех направлений: десятикласснику 100 баллов на физике есть.
	if !slices.Equal(b.DiplomaGrades, []int32{10, 11}) {
		t.Fatalf("классы: %v", b.DiplomaGrades)
	}
	// «Зависит от программы» — с названием своего направления, а не
	// флаг без объяснения.
	want := "Зависит от программы (Экономика): льгота только на «Мировая экономика»"
	if !b.Varies || b.Note == nil || !strings.Contains(*b.Note, want) {
		t.Fatalf("оговорка экономики: %v %+v", b.Varies, b)
	}
	if n := strings.Count(*b.Note, "Подтвердить ЕГЭ"); n != 1 {
		t.Fatalf("предмет ЕГЭ — одним предложением: %s", *b.Note)
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

// Вопрос про направление смотрит льготы на него, а не на цель ученика и не
// на выбор в вузе: «что даёт олимпиада на ИБ в ВШЭ?». В вузе без
// направления льготы на него нет; непроверенное — строка вуза с пометкой.
func TestBenefitsOn(t *testing.T) {
	pool := dbtest.Open(t)
	s := New(pool)
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid") // цель ПИ
	_ = s.SetUniversityDirections(ctx, f.trajectoryID, "hse", f.creatorMember, []string{dirSE})

	tg, err := s.TargetsOn(ctx, []string{dirIS}, []string{"hse", "innopolis"})
	if err != nil {
		t.Fatal(err)
	}
	if x := tg["hse"]; x.Basis != "goal" || !slices.Equal(x.DirectionNames, []string{"Информационная безопасность"}) {
		t.Fatalf("ВШЭ на ИБ: %+v", x)
	}
	if x := tg["innopolis"]; x.Basis != "university" || len(x.DirectionIDs) != 0 {
		t.Fatalf("в Иннополисе ИБ нет: %+v", x)
	}

	rows, err := s.BenefitsOn(ctx, []string{dirIS}, []string{virtual, infosec}, []string{"hse", "innopolis"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]BenefitRow{}
	for _, b := range rows {
		got[b.ProfileID+"/"+b.UniversityID] = b
	}
	if b := got[virtual+"/hse"]; b.Benefit != "bvi" || !slices.Equal(b.DirectionNames, []string{"Информационная безопасность"}) {
		t.Fatalf("ВШЭ на ИБ, а не на выбранную ПИ: %+v", b)
	}
	if b := got[infosec+"/hse"]; b.Benefit != "bvi" {
		t.Fatalf("ИБ-профиль на ИБ в ВШЭ: %+v", b)
	}
	for k := range got {
		if strings.HasSuffix(k, "/innopolis") {
			t.Fatalf("в Иннополисе нет ИБ — и льготы на неё нет: %s", k)
		}
	}

	dbtest.ToCheck(t, pool, "hse", dirLaw, "Юриспруденция")
	rows, err = s.BenefitsOn(ctx, []string{dirLaw}, []string{infosec}, []string{"hse"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !rows[0].Unverified || !slices.Equal(rows[0].DirectionNames, []string{"Юриспруденция"}) {
		t.Fatalf("Юриспруденция в ВШЭ уточняется: %+v", rows)
	}
}

// На скольких направлениях вуза олимпиада даёт льготу — «на 3 из 14».
func TestDirectionCoverage(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	cov, err := s.DirectionCoverage(ctx, []string{virtual}, []string{"hse", "innopolis"})
	if err != nil {
		t.Fatal(err)
	}
	if c := cov[virtual+"/hse"]; c.Count != 3 || c.Total != 14 {
		t.Fatalf("ВШЭ: %+v", c)
	}
	if c := cov[virtual+"/innopolis"]; c.Count != 1 || c.Total != 1 {
		t.Fatalf("Иннополис: %+v", c)
	}
}

// Каталог вузов по направлению (F67): вузы, где есть направление, покрывающее
// искомое, и сколько олимпиад дают на нём льготу.
func TestUniversitiesByDirection(t *testing.T) {
	pool := dbtest.Open(t)
	s := New(pool)
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

	// Льготы на юриспруденцию в ВШЭ ещё проверяются — вуз в конце, «уточняется».
	dbtest.ToCheck(t, pool, "hse", dirLaw, "Юриспруденция")
	us, m, _ = s.UniversitiesByDirection(ctx, f.trajectoryID, "", "", dirLaw)
	if x := m["hse"]; !x.Unverified || x.Olympiads != 0 || us[len(us)-1].ID != "hse" {
		t.Fatalf("ВШЭ: %+v, последний %s", x, us[len(us)-1].ID)
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
