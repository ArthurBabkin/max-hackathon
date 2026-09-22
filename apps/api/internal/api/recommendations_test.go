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

	vsosh := byProfile(items, "vsosh-informatika")
	if vsosh == nil || byProfile(items, "vsosh-matematika") == nil {
		t.Fatalf("ВсОШ по предметам цели в подборе всегда: %s", r.raw)
	}
	if vsosh["reason"] != "Главная олимпиада страны, первый этап — в школе" ||
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

func TestRecommendations_TrackerStateAndParentVoice(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	e.track(f, "vsosh-informatika", false)
	if _, err := e.pool.Exec(context.Background(),
		`INSERT INTO proposals (trajectory_id, olympiad_profile_id, proposed_by_member_id) VALUES ($1, 'vsosh-matematika', $2)`,
		f.trajectoryID, f.parent.MemberID); err != nil {
		t.Fatal(err)
	}
	r := e.do("GET", "/api/v1/recommendations", e.login(900000002, "Ольга"), nil)
	items := cards(t, r, "items")
	if c := byProfile(items, "vsosh-informatika"); c["in_tracker"] != true || c["proposal_status"] != nil {
		t.Fatalf("пункт трекера: %v", c)
	}
	if c := byProfile(items, "vsosh-matematika"); c["in_tracker"] != false || c["proposal_status"] != "pending" {
		t.Fatalf("ожидающее предложение: %v", c)
	}
	for _, c := range items {
		s := c["benefits_summary"].(string)
		if strings.Contains(s, "твоих") {
			t.Fatalf("родителю — голос родителя: %q", s)
		}
		if strings.HasPrefix(s, "В вузах") && s != "В вузах Артёма льгот нет" {
			t.Fatalf("нет льгот — с именем ученика: %q", s)
		}
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
