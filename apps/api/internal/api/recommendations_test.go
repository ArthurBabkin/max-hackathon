package api

import (
	"context"
	"strings"
	"testing"
	"time"
)

func cards(t *testing.T, r resp, field string) []map[string]any {
	t.Helper()
	raw, ok := r.body[field].([]any)
	if !ok {
		t.Fatalf("%s — не массив: %s", field, r.raw)
	}
	out := make([]map[string]any, len(raw))
	for i, x := range raw {
		out[i] = x.(map[string]any)
	}
	return out
}

func byProfile(cs []map[string]any, id string) map[string]any {
	for _, c := range cs {
		if c["olympiad_profile_id"] == id {
			return c
		}
	}
	return nil
}

func TestRecommendations_Composition(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")

	r := e.do("GET", "/api/v1/recommendations", token, nil)
	if r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	if r.body["note"] != "Сначала ближайшие сроки и точное совпадение профиля" {
		t.Fatalf("note: %v", r.body["note"])
	}
	items := cards(t, r, "items")
	perechen, olympiads := 0, map[string]bool{}
	var prev time.Time
	for _, c := range items {
		if c["subject_code"] != "inf" && c["subject_code"] != "math" {
			t.Fatalf("только предметы ученика: %v", c)
		}
		if c["kind"] == "other" {
			t.Fatalf("вне перечня не смешивается с основным списком: %v", c)
		}
		if c["kind"] == "perechen" {
			perechen++
		}
		if olympiads[c["olympiad_id"].(string)] {
			t.Fatalf("одна олимпиада — одна карточка: %v", c["olympiad_id"])
		}
		olympiads[c["olympiad_id"].(string)] = true
		if c["reason"] == "" || c["benefits_summary"] == "" {
			t.Fatalf("причина и льгота заполнены всегда: %v", c)
		}
		if d, ok := c["deadline_at"].(string); ok {
			at, _ := time.Parse(time.RFC3339, d)
			if at.Before(prev) {
				t.Fatalf("items отсортированы по сроку: %v после %v", at, prev)
			}
			prev = at
		}
	}
	if perechen != 5 {
		t.Fatalf("из перечня — топ-5, получили %d", perechen)
	}
	if more := cards(t, r, "more"); len(more) == 0 || r.body["state"] != "ok" {
		t.Fatalf("остальные подходящие — в «ещё»: %s", r.raw)
	} else {
		for _, c := range more {
			if c["kind"] != "perechen" || olympiads[c["olympiad_id"].(string)] {
				t.Fatalf("в «ещё» — перечень, которого нет в топе: %v", c)
			}
		}
	}

	vsosh := byProfile(items, "vsosh-informatika")
	if vsosh == nil || byProfile(items, "vsosh-matematika") == nil {
		t.Fatalf("ВсОШ по предметам цели в подборе всегда: %s", r.raw)
	}
	if vsosh["reason"] != "Всероссийская олимпиада, первый этап в школе" ||
		vsosh["benefits_summary"] != "ВШЭ, Иннополис: БВИ" || vsosh["next_stage_title"] != "Школьный этап" ||
		vsosh["level"] != nil || vsosh["in_tracker"] != false || vsosh["proposal_status"] != nil {
		t.Fatalf("карточка ВсОШ: %v", vsosh)
	}
	if _, ok := vsosh["short_name"]; !ok {
		t.Fatal("поля плитки есть, пусть и null")
	}

	outside := cards(t, r, "outside")
	if len(outside) != 1 || outside[0]["olympiad_profile_id"] != "other-tyk-inf" {
		t.Fatalf("вне перечня — только по предметам ученика: %v", outside)
	}
	if outside[0]["reason"] != "Льгот нет, только дополнительные баллы" ||
		outside[0]["benefits_summary"] != "КФУ, Иннополис: доп. баллы" {
		t.Fatalf("карточка вне перечня: %v", outside[0])
	}
}

// Уже добавленное и предложенное в «Подборе» не показывается (C3): их
// места занимают следующие по скору, а счётчики говорят, сколько спрятано.
func TestRecommendations_HidesTrackedAndProposed(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	parent := e.login(900000002, "Ольга")
	before := e.do("GET", "/api/v1/recommendations", parent, nil)
	perechenBefore := 0
	for _, c := range cards(t, before, "items") {
		if c["kind"] == "perechen" {
			perechenBefore++
		}
	}
	top := ""
	for _, c := range cards(t, before, "items") {
		if c["kind"] == "perechen" {
			top = c["olympiad_profile_id"].(string)
			break
		}
	}
	e.track(f, "vsosh-informatika", false)
	e.track(f, top, false)
	if _, err := e.pool.Exec(context.Background(),
		`INSERT INTO proposals (trajectory_id, olympiad_profile_id, proposed_by_member_id) VALUES ($1, 'vsosh-matematika', $2)`,
		f.trajectoryID, f.parent.MemberID); err != nil {
		t.Fatal(err)
	}
	r := e.do("GET", "/api/v1/recommendations", parent, nil)
	items := cards(t, r, "items")
	for _, id := range []string{"vsosh-informatika", "vsosh-matematika", top} {
		if byProfile(items, id) != nil || byProfile(cards(t, r, "more"), id) != nil {
			t.Fatalf("%s уже в трекере или предложена — в подборе её нет: %s", id, r.raw)
		}
	}
	perechen := 0
	for _, c := range items {
		if c["kind"] == "perechen" {
			perechen++
		}
		if c["in_tracker"] != false || c["proposal_status"] != nil {
			t.Fatalf("в подборе только недобавленное: %v", c)
		}
		s := c["benefits_summary"].(string)
		if strings.Contains(s, "твоих") {
			t.Fatalf("родителю — голос родителя: %q", s)
		}
	}
	if perechen != perechenBefore {
		t.Fatalf("место добавленной заняла следующая: было %d, стало %d", perechenBefore, perechen)
	}
	if r.body["tracked_count"] != 2.0 || r.body["proposed_count"] != 1.0 || r.body["state"] != "ok" {
		t.Fatalf("счётчики: %v %v %v", r.body["tracked_count"], r.body["proposed_count"], r.body["state"])
	}
}

// Добавить всё подходящее — «ты уже следишь за всеми» (C7): список пуст,
// state = all_tracked, «ещё» тоже пусто.
func TestRecommendations_AllTracked(t *testing.T) {
	e := newEnv(t)
	f := e.kidCreator()
	token := e.login(900000001, "Артём")
	r := e.do("GET", "/api/v1/recommendations", token, nil)
	if r.body["state"] != "ok" || r.body["tracked_count"] != 0.0 || r.body["proposed_count"] != 0.0 {
		t.Fatalf("новичок: %s", r.raw)
	}
	all := append(cards(t, r, "items"), cards(t, r, "more")...)
	for _, c := range all {
		e.track(f, c["olympiad_profile_id"].(string), false)
	}
	r = e.do("GET", "/api/v1/recommendations", token, nil)
	if len(cards(t, r, "items")) != 0 || len(cards(t, r, "more")) != 0 || r.body["state"] != "all_tracked" ||
		r.body["tracked_count"] != float64(len(all)) {
		t.Fatalf("всё добавлено (%d): %s", len(all), r.raw)
	}
}

func TestRecommendations_Filters(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")

	for _, tc := range []struct {
		filter string
		ok     func(map[string]any) bool
	}{
		{"level1", func(c map[string]any) bool { return c["level"] == "I" }},
		{"online", func(c map[string]any) bool { return c["is_online"] == true }},
		{"soon", func(c map[string]any) bool {
			d, ok := c["deadline_at"].(string)
			at, _ := time.Parse(time.RFC3339, d)
			return ok && at.Sub(testNow) <= 16*24*time.Hour
		}},
	} {
		r := e.do("GET", "/api/v1/recommendations?filter="+tc.filter, token, nil)
		if r.code != 200 {
			t.Fatalf("%s: %d %s", tc.filter, r.code, r.raw)
		}
		items := cards(t, r, "items")
		if len(items) == 0 && tc.filter != "soon" {
			t.Fatalf("%s: пусто", tc.filter)
		}
		for _, c := range items {
			if !tc.ok(c) {
				t.Fatalf("%s пропустил лишнее: %v", tc.filter, c)
			}
		}
		for _, c := range cards(t, r, "outside") {
			if !tc.ok(c) {
				t.Fatalf("%s: фильтр действует и на блок вне перечня: %v", tc.filter, c)
			}
		}
	}

	if r := e.do("GET", "/api/v1/recommendations?filter=nope", token, nil); r.code != 400 || r.errCode() != "BAD_REQUEST" {
		t.Fatalf("неизвестный фильтр — 400: %d %s", r.code, r.raw)
	}
	if r := e.do("GET", "/api/v1/recommendations", "", nil); r.code != 401 {
		t.Fatalf("без токена — 401: %d", r.code)
	}
}

// Вузы не выбраны — льготы в вузах с направлением ученика в выбранных
// местах, а не «льгот нет» (SPEC 2.3). ИВТ в Татарстане — только Иннополис.
func TestRecommendations_PotentialBenefitsWithoutUniversities(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")
	if r := e.do("PUT", "/api/v1/profile/universities", token, map[string]any{"university_ids": []string{}}); r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	if r := e.do("PATCH", "/api/v1/profile", token, map[string]any{
		"direction_ids": []string{"napr-09-03-01"}, "places": []map[string]any{{"region_code": "16"}}}); r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	r := e.do("GET", "/api/v1/recommendations", token, nil)
	vsosh := byProfile(cards(t, r, "items"), "vsosh-informatika")
	if vsosh == nil || vsosh["benefits_summary"] != "Иннополис: БВИ" {
		t.Fatalf("потенциальная льгота: %v", vsosh)
	}
}
