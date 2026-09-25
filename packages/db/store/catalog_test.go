package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
)

func profileIDs(ps []Profile) map[string]Profile {
	m := map[string]Profile{}
	for _, p := range ps {
		m[p.ID] = p
	}
	return m
}

func TestProfiles_FiltersBySubjectAndGrade(t *testing.T) {
	s := New(dbtest.Open(t))
	ps, err := s.Profiles(context.Background(), ProfileQuery{SubjectCodes: []string{"inf"}, Grade: 9})
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) == 0 {
		t.Fatal("по информатике для 9 класса профили есть")
	}
	for _, p := range ps {
		if p.SubjectCode != "inf" || p.GradesFrom > 9 || p.GradesTo < 9 {
			t.Fatalf("профиль не проходит фильтр: %+v", p)
		}
	}
	got := profileIDs(ps)
	vp, ok := got["p669-8-informatika"]
	if !ok {
		t.Fatal("«Высшая проба» по информатике должна попасть в выборку")
	}
	if vp.OlympiadName == "" || vp.Kind != "perechen" || vp.Level == nil || vp.Source == nil {
		t.Fatalf("карточка неполная: %+v", vp)
	}
	if _, ok := got["vsosh-informatika"]; !ok {
		t.Fatal("ВсОШ по информатике тоже кандидат")
	}
}

func TestProfiles_SearchIgnoresCaseAndEscapesPatterns(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	for _, q := range []string{"высшая ПРОБА", "ниу вшэ"} {
		ps, err := s.Profiles(ctx, ProfileQuery{Search: q})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := profileIDs(ps)["p669-8-informatika"]; !ok {
			t.Fatalf("поиск %q должен найти «Высшую пробу»", q)
		}
	}
	ps, err := s.Profiles(ctx, ProfileQuery{Search: "%"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 0 {
		t.Fatalf("символ %% — не шаблон «всё подряд», нашлось %d", len(ps))
	}
	ps, _ = s.Profiles(ctx, ProfileQuery{IDs: []string{}})
	if ps == nil || len(ps) != 0 {
		t.Fatal("пустой список id — пустой результат, а не весь каталог")
	}
}

func TestBenefits_LatestYearAndBestBenefitPerUniversity(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	rows, err := s.Benefits(ctx, []string{"p669-8-informatika", "vsosh-informatika"}, []string{"innopolis", "kfu"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]BenefitRow{}
	for _, b := range rows {
		got[b.ProfileID+"/"+b.UniversityID] = b
	}
	vp := got["p669-8-informatika/innopolis"]
	if vp.Benefit != "bvi" || vp.Source == nil || vp.UniversityShort != "УИ" {
		t.Fatalf("«Высшая проба» в Иннополисе — БВИ с источником: %+v", vp)
	}
	if got["vsosh-informatika/innopolis"].Source != nil {
		t.Fatal("демо-запись не должна получать источник")
	}

	// Контент тесты не чистят, поэтому лишние строки — внутри транзакции с откатом.
	rollback := errors.New("rollback")
	err = s.Tx(ctx, func(tx *Store) error {
		_, err := tx.db.Exec(ctx, `
			INSERT INTO benefits (id, olympiad_profile_id, university_id, admission_year, benefit) VALUES
			  ('t-old', 'p669-8-informatika', 'kfu', 2025, 'bvi'),
			  ('t-100', 'p669-8-informatika', 'kfu', 2027, 'score100'),
			  ('t-win', 'p669-8-informatika', 'kfu', 2027, 'bvi_winners')`)
		if err != nil {
			return err
		}
		rows, err := tx.Benefits(ctx, []string{"p669-8-informatika"}, []string{"kfu"})
		if err != nil {
			return err
		}
		if len(rows) != 1 || rows[0].AdmissionYear != 2027 || rows[0].Benefit != "bvi_winners" {
			t.Errorf("ждали одну строку 2027 bvi_winners, получили %+v", rows)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}

	all, err := s.Benefits(ctx, []string{"p669-8-informatika"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) <= 2 {
		t.Fatalf("без фильтра по вузам — все вузы, получили %d", len(all))
	}
	if none, _ := s.Benefits(ctx, nil, nil); none == nil || len(none) != 0 {
		t.Fatal("без профилей — пустой срез")
	}
}

func TestTrackerState(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")
	_, err := s.db.Exec(ctx, `
		INSERT INTO tracker_items (trajectory_id, olympiad_profile_id, registered_at) VALUES
		  ($1, 'p669-8-informatika', now()), ($1, 'vsosh-informatika', NULL);
		`, f.trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO proposals (trajectory_id, olympiad_profile_id) VALUES ($1, 'p669-2-matematika')`,
		f.trajectoryID); err != nil {
		t.Fatal(err)
	}
	st, err := s.TrackerState(ctx, f.trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if !st.InTracker["p669-8-informatika"] || !st.Registered["p669-8-informatika"] ||
		!st.InTracker["vsosh-informatika"] || st.Registered["vsosh-informatika"] ||
		!st.Pending["p669-2-matematika"] || st.InTracker["p669-2-matematika"] {
		t.Fatalf("неверное состояние: %+v", st)
	}
	// Олимпиады целиком: «Подбор» прячет все профили добавленной олимпиады.
	if !st.TrackedOlympiads["p669-8"] || !st.TrackedOlympiads["vsosh-informatika"] ||
		!st.PendingOlympiads["p669-2"] || st.TrackedOlympiads["p669-2"] {
		t.Fatalf("олимпиады трекера: %+v / %+v", st.TrackedOlympiads, st.PendingOlympiads)
	}
}

func TestUniversities_CatalogAndDetail(t *testing.T) {
	s := New(dbtest.Open(t))
	ctx := context.Background()
	f := seedTrajectory(t, s, 900000001, "kid")

	all, err := s.Universities(ctx, f.trajectoryID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 10 {
		t.Fatalf("в каталоге 10 вузов, получили %d", len(all))
	}
	mine := 0
	for _, u := range all {
		if u.IsMine {
			mine++
		}
	}
	if mine != 3 {
		t.Fatalf("у ученика три вуза, отмечено %d", mine)
	}
	found, _ := s.Universities(ctx, f.trajectoryID, "иннопол", "")
	if len(found) != 1 || found[0].ID != "innopolis" {
		t.Fatalf("поиск по названию без учёта регистра: %+v", found)
	}
	kazan, _ := s.Universities(ctx, f.trajectoryID, "", "Казань")
	if len(kazan) != 2 {
		t.Fatalf("в Казани два вуза, получили %d", len(kazan))
	}

	d, err := s.University(ctx, f.trajectoryID, "innopolis")
	if err != nil {
		t.Fatal(err)
	}
	if !d.IsMine || d.BenefitOlympiads == 0 || len(d.Directions) == 0 {
		t.Fatalf("карточка вуза: %+v", d)
	}
	if _, err := s.University(ctx, f.trajectoryID, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("неизвестный вуз — ErrNotFound, получили %v", err)
	}

	os, err := s.UniversityOlympiads(ctx, "innopolis")
	if err != nil {
		t.Fatal(err)
	}
	if len(os) == 0 {
		t.Fatal("у Иннополиса есть олимпиады с льготой")
	}
	rank := map[string]int{"bvi": 0, "bvi_winners": 1, "score100": 2}
	for i := 1; i < len(os); i++ {
		if rank[os[i-1].Benefit] > rank[os[i].Benefit] {
			t.Fatalf("сильные льготы первыми: %s перед %s", os[i-1].Benefit, os[i].Benefit)
		}
	}
}
