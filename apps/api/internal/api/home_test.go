package api

import (
	"context"
	"testing"
)

func TestHome_NextStepIsNearestUnregistered(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	token := e.login(900000001, "Артём")

	r := e.do("GET", "/api/v1/home", token, nil)
	if r.code != 200 || r.body["next_step"] != nil || r.body["tracker_count"].(float64) != 0 {
		t.Fatalf("пустой трекер: %d %s", r.code, r.raw)
	}
	if len(r.body["upcoming"].([]any)) != 0 || r.body["universities_count"].(float64) != 3 {
		t.Fatalf("пустой трекер: %s", r.raw)
	}

	hse := e.track(f, "p669-8-informatika", false) // регистрация до 22.09
	e.track(f, "vsosh-informatika", false)         // школьный этап до 28.10
	r = e.do("GET", "/api/v1/home", token, nil)
	ns := r.body["next_step"].(map[string]any)
	if ns["tracker_item_id"] != hse || ns["stage_title"] != "Регистрация" || ns["stage_kind"] != "registration" {
		t.Fatalf("следующий шаг — регистрация на «Высшую пробу»: %v", ns)
	}

	_, _ = e.pool.Exec(context.Background(), "UPDATE tracker_items SET registered_at = now() WHERE id = $1", hse)
	r = e.do("GET", "/api/v1/home", token, nil)
	ns = r.body["next_step"].(map[string]any)
	if ns["olympiad_profile_id"] != "vsosh-informatika" || ns["stage_kind"] != "school" || ns["stage_title"] != "Школьный этап" {
		t.Fatalf("после отметки — школьный этап ВсОШ: %v", ns)
	}
	if r.body["registered_count"].(float64) != 1 || r.body["tracker_count"].(float64) != 2 {
		t.Fatalf("счётчики: %s", r.raw)
	}
	up := r.body["upcoming"].([]any)
	first := up[0].(map[string]any)
	if first["olympiad_profile_id"] != "p669-8-informatika" || first["next_stage_title"] != "Отборочный этап, 1 тур" {
		t.Fatalf("у отмеченного пункта дальше отборочный этап: %v", first)
	}

	_, _ = e.pool.Exec(context.Background(), "UPDATE tracker_items SET registered_at = now()")
	if r = e.do("GET", "/api/v1/home", token, nil); r.body["next_step"] != nil {
		t.Fatalf("всё отмечено — next_step null: %v", r.body["next_step"])
	}
}

func TestProfile_GetAndPatch(t *testing.T) {
	e := newEnv(t)
	e.withParent(e.kidCreator())
	token := e.login(900000002, "Ольга")

	r := e.do("GET", "/api/v1/profile", token, nil)
	if r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	others := r.body["other_member_names"].([]any)
	if len(others) != 1 || others[0] != "Артём" {
		t.Fatalf("остальные участники без меня: %v", others)
	}
	if len(r.body["subjects"].([]any)) != 2 || len(r.body["universities"].([]any)) != 3 {
		t.Fatalf("профиль: %s", r.raw)
	}
	uni := r.body["universities"].([]any)[0].(map[string]any)
	if uni["benefit_olympiads_count"].(float64) <= 0 || uni["is_mine"] != true {
		t.Fatalf("вуз: %v", uni)
	}

	bad := []map[string]any{
		{},
		{"subject_codes": []string{}},
		{"grade": 12},
		{"student_name": "   "},
		{"student_name": "Абвгдежзийклмнопрстуфхцчшщъыьэюяабвгдежзи"}, // 41 символ
		{"region_code": "999"},
		{"university_ids": []string{"нет-такого"}},
		{"subject_codes": []string{"klingon"}},
	}
	for _, body := range bad {
		if r := e.do("PATCH", "/api/v1/profile", token, body); r.code != 400 || r.errCode() != "BAD_REQUEST" {
			t.Errorf("PATCH %v: ждали 400, получили %d %s", body, r.code, r.raw)
		}
	}

	r = e.do("PATCH", "/api/v1/profile", token, map[string]any{
		"grade": 10, "region_code": "77", "subject_codes": []string{"inf", "math", "phys"},
		"direction_id": "napr-01-03-02", "student_name": " Артём ",
	})
	if r.code != 200 || r.body["grade"].(float64) != 10 || r.body["region_name"] != "Москва" ||
		r.body["direction_name"] != "Прикладная математика и информатика" || r.body["goal_status"] != "known" ||
		len(r.body["subjects"].([]any)) != 3 || r.body["student_name"] != "Артём" {
		t.Fatalf("PATCH: %d %s", r.code, r.raw)
	}
	var tz string
	_ = e.pool.QueryRow(context.Background(), "SELECT tz FROM trajectories").Scan(&tz)
	if tz != "Europe/Moscow" {
		t.Fatalf("часовой пояс вслед за регионом: %s", tz)
	}
	r = e.do("PATCH", "/api/v1/profile", token, map[string]any{"region_code": "25"})
	_ = e.pool.QueryRow(context.Background(), "SELECT tz FROM trajectories").Scan(&tz)
	if r.code != 200 || tz != "Asia/Vladivostok" {
		t.Fatalf("Приморье — Владивосток: %d %s", r.code, tz)
	}

	if r := e.do("PUT", "/api/v1/profile/universities", token, map[string]any{"university_ids": []string{}}); r.code != 400 {
		t.Fatalf("пустой список вузов: %d", r.code)
	}
	r = e.do("PUT", "/api/v1/profile/universities", token, map[string]any{"university_ids": []string{"msu", "itmo"}})
	if r.code != 200 || len(r.body["universities"].([]any)) != 2 {
		t.Fatalf("PUT universities: %d %s", r.code, r.raw)
	}
}
