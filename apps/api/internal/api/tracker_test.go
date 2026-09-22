package api

import (
	"context"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

func TestTracker_AddIsIdempotentAndRespectsRoles(t *testing.T) {
	e := newEnv(t)
	e.withParent(e.kidCreator())
	kid := e.login(900000001, "Артём")
	parent := e.login(900000002, "Ольга")
	add := func(token, profileID string) resp {
		return e.do("POST", "/api/v1/tracker", token, map[string]any{"olympiad_profile_id": profileID})
	}

	r := add(kid, "p669-8-informatika")
	if r.code != 201 {
		t.Fatalf("первое добавление — 201: %d %s", r.code, r.raw)
	}
	id := r.body["id"].(string)
	added, _ := r.body["added_by"].(map[string]any)
	if added["name"] != "Артём" || added["role"] != "kid" || r.body["registered_at"] != nil ||
		r.body["next_stage_title"] != "Регистрация" || r.body["olympiad_name"] == nil {
		t.Fatalf("пункт: %s", r.raw)
	}
	if again := add(kid, "p669-8-informatika"); again.code != 200 || again.body["id"] != id {
		t.Fatalf("повторное добавление — 200 с тем же пунктом: %d %s", again.code, again.raw)
	}
	if r := add(kid, "nope"); r.code != 404 {
		t.Fatalf("неизвестный профиль — 404: %d", r.code)
	}
	if r := e.do("POST", "/api/v1/tracker", kid, map[string]any{}); r.code != 400 {
		t.Fatalf("без профиля — 400: %d", r.code)
	}

	// Родитель при ученике только предлагает (ТЗ §3.1).
	if r := add(parent, "vsosh-informatika"); r.code != 403 || r.errCode() != "FORBIDDEN" {
		t.Fatalf("родитель при ученике — 403: %d %s", r.code, r.raw)
	}
	if r := e.do("DELETE", "/api/v1/tracker/"+id, parent, nil); r.code != 403 {
		t.Fatalf("и удалять не может: %d", r.code)
	}
}

func TestTracker_ParentWithoutKidOwnsTracker(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	uid, _ := e.st.UpsertUser(ctx, 900000002, "Ольга")
	_, err := e.st.CreateTrajectory(ctx, store.NewTrajectory{
		CreatorUserID: uid, Role: "parent", StudentName: "Артём", Grade: 9, RegionCode: "16",
		TZ: "Europe/Moscow", GoalStatus: "suggested", SubjectCodes: []string{"inf"}, UniversityIDs: []string{"kfu"},
	})
	if err != nil {
		t.Fatal(err)
	}
	token := e.login(900000002, "Ольга")
	r := e.do("POST", "/api/v1/tracker", token, map[string]any{"olympiad_profile_id": "vsosh-informatika"})
	if r.code != 201 {
		t.Fatalf("родитель без ученика ведёт трекер сам: %d %s", r.code, r.raw)
	}
	if d := e.do("DELETE", "/api/v1/tracker/"+r.body["id"].(string), token, nil); d.code != 204 {
		t.Fatalf("и удаляет: %d %s", d.code, d.raw)
	}
}

func TestTracker_RegisteredMarkAndDelete(t *testing.T) {
	e := newEnv(t)
	e.withParent(e.kidCreator())
	kid := e.login(900000001, "Артём")
	parent := e.login(900000002, "Ольга")
	id := e.do("POST", "/api/v1/tracker", kid, map[string]any{"olympiad_profile_id": "p669-8-informatika"}).body["id"].(string)

	r := e.do("PUT", "/api/v1/tracker/"+id+"/registered", parent, nil)
	by, _ := r.body["registered_by"].(map[string]any)
	if r.code != 200 || r.body["registered_at"] == nil || by["name"] != "Ольга" ||
		r.body["next_stage_title"] != "Отборочный этап, 1 тур" {
		t.Fatalf("отметку ставит любой участник: %d %s", r.code, r.raw)
	}
	r = e.do("PUT", "/api/v1/tracker/"+id+"/registered", kid, nil)
	if by, _ := r.body["registered_by"].(map[string]any); r.code != 200 || by["name"] != "Ольга" {
		t.Fatalf("повторная отметка не перезаписывает, кто отметил: %s", r.raw)
	}
	r = e.do("DELETE", "/api/v1/tracker/"+id+"/registered", kid, nil)
	if r.code != 200 || r.body["registered_at"] != nil || r.body["registered_by"] != nil {
		t.Fatalf("снятие отметки: %d %s", r.code, r.raw)
	}

	for _, bad := range []string{"not-a-uuid", "00000000-0000-4000-8000-000000000999"} {
		if r := e.do("PUT", "/api/v1/tracker/"+bad+"/registered", kid, nil); r.code != 404 {
			t.Fatalf("%s — 404: %d %s", bad, r.code, r.raw)
		}
	}

	// Чужой пункт — как несуществующий.
	uid, _ := e.st.UpsertUser(context.Background(), 900000005, "Мария")
	_, err := e.st.CreateTrajectory(context.Background(), store.NewTrajectory{
		CreatorUserID: uid, Role: "kid", StudentName: "Мария", Grade: 10, RegionCode: "77",
		TZ: "Europe/Moscow", GoalStatus: "suggested", SubjectCodes: []string{"inf"}, UniversityIDs: []string{"hse"},
	})
	if err != nil {
		t.Fatal(err)
	}
	stranger := e.login(900000005, "Мария")
	if r := e.do("DELETE", "/api/v1/tracker/"+id, stranger, nil); r.code != 404 {
		t.Fatalf("чужой пункт — 404: %d", r.code)
	}

	if r := e.do("DELETE", "/api/v1/tracker/"+id, kid, nil); r.code != 204 || len(r.raw) != 0 {
		t.Fatalf("удаление — 204 без тела: %d %s", r.code, r.raw)
	}
	if r := e.do("DELETE", "/api/v1/tracker/"+id, kid, nil); r.code != 404 {
		t.Fatalf("второе удаление — 404: %d", r.code)
	}
}

func TestTracker_ListWithProposals(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	kid := e.login(900000001, "Артём")
	parent := e.login(900000002, "Ольга")
	e.track(f, "vsosh-informatika", false)  // школьный этап до 28.10
	e.track(f, "p669-8-informatika", false) // регистрация до 22.09
	if _, err := e.pool.Exec(context.Background(), `
		INSERT INTO proposals (trajectory_id, olympiad_profile_id, proposed_by_member_id) VALUES ($1, 'p669-34-informatika', $2)`,
		f.trajectoryID, f.parent.MemberID); err != nil {
		t.Fatal(err)
	}

	r := e.do("GET", "/api/v1/tracker", kid, nil)
	items := list(t, r.body["items"])
	if r.code != 200 || len(items) != 2 || items[0]["olympiad_profile_id"] != "p669-8-informatika" {
		t.Fatalf("пункты по сроку: %d %s", r.code, r.raw)
	}
	props := list(t, r.body["proposals"])
	if len(props) != 1 || props[0]["status"] != "pending" || props[0]["olympiad_profile_id"] != "p669-34-informatika" ||
		props[0]["proposed_by"].(map[string]any)["name"] != "Ольга" || props[0]["resolved_at"] != nil ||
		props[0]["deadline_at"] == nil || props[0]["olympiad_name"] == nil {
		t.Fatalf("ученику — предложения, ждущие ответа: %v", props)
	}
	if props := list(t, e.do("GET", "/api/v1/tracker", parent, nil).body["proposals"]); len(props) != 1 {
		t.Fatalf("родителю — его ждущие подтверждения предложения: %v", props)
	}

	// Ученик добавил предложенное сам — предложение закрыто.
	e.do("POST", "/api/v1/tracker", kid, map[string]any{"olympiad_profile_id": "p669-34-informatika"})
	if props := list(t, e.do("GET", "/api/v1/tracker", kid, nil).body["proposals"]); len(props) != 0 {
		t.Fatalf("предложение закрыто добавлением: %v", props)
	}

	// Автор пункта остаётся подписью и после удаления участника (ТЗ §3.2).
	if _, err := e.pool.Exec(context.Background(), `UPDATE members SET removed_at = now() WHERE id = $1`,
		f.kid.MemberID); err != nil {
		t.Fatal(err)
	}
	items = list(t, e.do("GET", "/api/v1/tracker", parent, nil).body["items"])
	for _, it := range items {
		if it["added_by"].(map[string]any)["name"] != "Артём" {
			t.Fatalf("added_by выживает: %v", it)
		}
	}
}
