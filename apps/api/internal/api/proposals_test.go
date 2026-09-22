package api

import (
	"testing"
)

func TestProposals_ProposeAcceptDecline(t *testing.T) {
	e := newEnv(t)
	e.withParent(e.kidCreator())
	kid := e.login(900000001, "Артём")
	parent := e.login(900000002, "Ольга")
	propose := func(token, profileID string) resp {
		return e.do("POST", "/api/v1/proposals", token, map[string]any{"olympiad_profile_id": profileID})
	}

	r := propose(parent, "p669-8-informatika")
	if r.code != 201 || r.body["status"] != "pending" || r.body["proposed_by"].(map[string]any)["name"] != "Ольга" {
		t.Fatalf("предложение: %d %s", r.code, r.raw)
	}
	id := r.body["id"].(string)
	if again := propose(parent, "p669-8-informatika"); again.code != 200 || again.body["id"] != id {
		t.Fatalf("второе по тому же профилю — 200 с существующим: %d %s", again.code, again.raw)
	}
	if r := propose(kid, "vsosh-informatika"); r.code != 403 {
		t.Fatalf("ученик не предлагает, а добавляет сам: %d", r.code)
	}
	if r := propose(parent, "nope"); r.code != 404 {
		t.Fatalf("неизвестный профиль — 404: %d", r.code)
	}

	// Карточка в подборе знает о предложении.
	card := e.do("GET", "/api/v1/olympiads/p669-8-informatika", kid, nil).body
	if card["proposal_status"] != "pending" {
		t.Fatalf("proposal_status в карточке: %v", card["proposal_status"])
	}

	if r := e.do("POST", "/api/v1/proposals/"+id+"/accept", parent, nil); r.code != 403 {
		t.Fatalf("отвечает только ученик: %d", r.code)
	}
	r = e.do("POST", "/api/v1/proposals/"+id+"/accept", kid, nil)
	if r.code != 200 {
		t.Fatalf("принятие: %d %s", r.code, r.raw)
	}
	prop := r.body["proposal"].(map[string]any)
	item := r.body["tracker_item"].(map[string]any)
	if prop["status"] != "accepted" || prop["resolved_at"] == nil || item["olympiad_profile_id"] != "p669-8-informatika" ||
		item["added_by"].(map[string]any)["name"] != "Ольга" {
		t.Fatalf("принятие кладёт олимпиаду в трекер: %s", r.raw)
	}
	if r := e.do("POST", "/api/v1/proposals/"+id+"/accept", kid, nil); r.code != 409 || r.errCode() != "CONFLICT" {
		t.Fatalf("повторный ответ — 409: %d %s", r.code, r.raw)
	}
	if r := propose(parent, "p669-8-informatika"); r.code != 409 {
		t.Fatalf("то, что уже в трекере, не предлагается: %d", r.code)
	}

	id2 := propose(parent, "vsosh-informatika").body["id"].(string)
	r = e.do("POST", "/api/v1/proposals/"+id2+"/decline", kid, nil)
	if r.code != 200 || r.body["status"] != "declined" || r.body["resolved_at"] == nil {
		t.Fatalf("отказ: %d %s", r.code, r.raw)
	}
	if items := list(t, e.do("GET", "/api/v1/tracker", kid, nil).body["items"]); len(items) != 1 {
		t.Fatalf("отказ в трекер ничего не кладёт: %v", items)
	}
	for _, bad := range []string{"nope", "00000000-0000-4000-8000-000000000999"} {
		if r := e.do("POST", "/api/v1/proposals/"+bad+"/decline", kid, nil); r.code != 404 {
			t.Fatalf("%s — 404: %d", bad, r.code)
		}
	}
}
