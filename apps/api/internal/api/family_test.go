package api

import (
	"regexp"
	"testing"
)

func TestFamily_ListInviteRemoveLeave(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	kid := e.login(900000001, "Артём")
	parent := e.login(900000002, "Ольга")

	r := e.do("GET", "/api/v1/family", kid, nil)
	members := list(t, r.body["members"])
	if r.code != 200 || len(members) != 2 || r.body["trajectory"] == nil || len(list(t, r.body["invites"])) != 0 {
		t.Fatalf("семья: %d %s", r.code, r.raw)
	}
	creator, olga := members[0], members[1]
	if creator["is_creator"] != true || creator["is_me"] != true || creator["can_remove"] != false ||
		olga["name"] != "Ольга" || olga["role"] != "parent" || olga["can_remove"] != true || olga["joined_at"] == nil {
		t.Fatalf("создатель удаляет приглашённых, но не себя: %v", members)
	}
	for _, m := range list(t, e.do("GET", "/api/v1/family", parent, nil).body["members"]) {
		if m["can_remove"] != false {
			t.Fatalf("приглашённый никого не удаляет: %v", m)
		}
	}

	// Приглашения: при ученике — только роль parent.
	if r := e.do("POST", "/api/v1/family/invites", parent, map[string]any{"role": "kid"}); r.code != 409 || r.errCode() != "CONFLICT" {
		t.Fatalf("вторая роль kid — 409: %d %s", r.code, r.raw)
	}
	if r := e.do("POST", "/api/v1/family/invites", parent, map[string]any{"role": "admin"}); r.code != 400 {
		t.Fatalf("неизвестная роль — 400: %d", r.code)
	}
	r = e.do("POST", "/api/v1/family/invites", parent, map[string]any{"role": "parent"})
	token, _ := r.body["token"].(string)
	if r.code != 201 || !regexp.MustCompile(`^[A-Za-z0-9_-]{12,}$`).MatchString(token) ||
		r.body["url"] != "https://max.ru/test_bot?start=inv_"+token || r.body["role"] != "parent" {
		t.Fatalf("ссылка-приглашение: %d %s", r.code, r.raw)
	}
	second := e.do("POST", "/api/v1/family/invites", kid, map[string]any{"role": "parent"}).body["token"]
	if second == token {
		t.Fatal("каждому приглашённому — новая ссылка")
	}
	if invites := list(t, e.do("GET", "/api/v1/family", kid, nil).body["invites"]); len(invites) != 2 {
		t.Fatalf("активные ссылки: %v", invites)
	}

	// Удаление: только создатель и не себя.
	if r := e.do("DELETE", "/api/v1/family/members/"+f.kid.MemberID, parent, nil); r.code != 403 {
		t.Fatalf("приглашённый не удаляет: %d", r.code)
	}
	if r := e.do("DELETE", "/api/v1/family/members/"+f.kid.MemberID, kid, nil); r.code != 403 {
		t.Fatalf("создатель не удаляет себя: %d", r.code)
	}
	if r := e.do("DELETE", "/api/v1/family/members/00000000-0000-4000-8000-000000000999", kid, nil); r.code != 404 {
		t.Fatalf("нет участника — 404: %d", r.code)
	}
	if r := e.do("POST", "/api/v1/family/leave", kid, nil); r.code != 403 {
		t.Fatalf("создатель не выходит: %d", r.code)
	}
	if r := e.do("DELETE", "/api/v1/family/members/"+f.parent.MemberID, kid, nil); r.code != 204 {
		t.Fatalf("удаление: %d %s", r.code, r.raw)
	}
	// Удалённый теряет доступ сразу, хотя его токен ещё жив.
	if r := e.do("GET", "/api/v1/family", parent, nil); r.code != 401 {
		t.Fatalf("удалённый — 401: %d", r.code)
	}
	fam := e.do("GET", "/api/v1/family", kid, nil).body
	if len(list(t, fam["members"])) != 1 || len(list(t, fam["invites"])) != 1 {
		t.Fatalf("ссылка удалённого отозвана, своя осталась: %v", fam)
	}
}

func TestFamily_InvitedParentLeaves(t *testing.T) {
	e := newEnv(t)
	e.withParent(e.kidCreator())
	parent := e.login(900000002, "Ольга")
	if r := e.do("POST", "/api/v1/family/leave", parent, nil); r.code != 204 {
		t.Fatalf("приглашённый выходит сам: %d %s", r.code, r.raw)
	}
	if r := e.do("GET", "/api/v1/home", parent, nil); r.code != 401 {
		t.Fatalf("после выхода — 401: %d", r.code)
	}
}
