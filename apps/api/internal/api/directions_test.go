package api

import (
	"slices"
	"testing"
)

// Сид: «Виртуальные миры» НТО. ВШЭ целиком даёт БВИ, но на Программную
// инженерию — 100 баллов и не на всех программах; на ИБ — БВИ.
const (
	virtualProfile = "p669-5-virtualnye-miry-razrabotka-kompyuternyh-igr-tehnologii-virtualnoy-realnosti-tehnologii-dopolnennoy-realnosti-cifrovye-tehnologii-v-arhitekture"
	dirSE          = "napr-09-03-04"
	dirIS          = "napr-10-03-01"
	dirBio         = "napr-06-03-01"
)

func rowOf(t *testing.T, rows []map[string]any, uni string) map[string]any {
	t.Helper()
	for _, r := range rows {
		if r["university_id"] == uni {
			return r
		}
	}
	t.Fatalf("нет строки вуза %s: %v", uni, rows)
	return nil
}

func TestDirections_AllWithPopular(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")
	items := list(t, e.do("GET", "/api/v1/directions", token, nil).body["items"])
	popular := 0
	for _, it := range items {
		if it["popular"] == true {
			popular++
		}
		if it["id"] == dirSE && (it["code"] != "09.03.04" || len(it["groups"].([]any)) == 0 || it["popular"] != true) {
			t.Fatalf("ПИ: %v", it)
		}
	}
	if len(items) != 72 || popular != 16 {
		t.Fatalf("направлений %d, основных %d", len(items), popular)
	}
}

// Льгота в моих вузах — на мои направления (F65): цель, а выбор в вузе
// важнее цели.
func TestOlympiad_BenefitOnMyDirections(t *testing.T) {
	e := newEnv(t)
	e.kidCreator() // цель ПИ; вузы Иннополис, ВШЭ, КФУ
	token := e.login(900000001, "Артём")

	benefits := list(t, e.do("GET", "/api/v1/olympiads/"+virtualProfile, token, nil).body["benefits"])
	hse := rowOf(t, benefits, "hse")
	if hse["benefit"] != "score100" || !slices.Equal(strs(hse["directions"]), []string{"Программная инженерия"}) ||
		hse["unverified"] != false {
		t.Fatalf("ВШЭ на ПИ: %v", hse)
	}

	// «Где ещё даёт льготу» — на скольких направлениях вуза.
	body := e.do("GET", "/api/v1/olympiads/"+virtualProfile, token, nil).body
	if itmo := rowOf(t, list(t, body["benefit_universities"]), "itmo"); itmo["directions_count"] != float64(18) ||
		itmo["directions_total"] != float64(24) {
		t.Fatalf("ИТМО: на 18 из 24 направлений: %v", itmo)
	}

	r := e.do("PUT", "/api/v1/profile/universities/hse/directions", token, map[string]any{"direction_ids": []string{dirIS}})
	if r.code != 200 {
		t.Fatalf("выбор направлений: %d %s", r.code, r.raw)
	}
	var uni map[string]any
	for _, u := range list(t, r.body["universities"]) {
		if u["id"] == "hse" {
			uni = u
		}
	}
	if chosen := list(t, uni["chosen_directions"]); len(chosen) != 1 || chosen[0]["id"] != dirIS ||
		uni["target_basis"] != "chosen" {
		t.Fatalf("выбор в профиле: %v", uni)
	}
	if goal := list(t, r.body["directions"]); len(goal) != 2 || goal[1]["id"] != dirIS {
		t.Fatalf("направление дописалось в цель: %v", goal)
	}

	benefits = list(t, e.do("GET", "/api/v1/olympiads/"+virtualProfile, token, nil).body["benefits"])
	if hse := rowOf(t, benefits, "hse"); hse["benefit"] != "bvi" ||
		!slices.Equal(strs(hse["directions"]), []string{"Информационная безопасность"}) {
		t.Fatalf("ВШЭ на ИБ: %v", hse)
	}
}

func TestUniversityDirections_Errors(t *testing.T) {
	e := newEnv(t)
	f := e.withParent(e.kidCreator())
	token := e.login(900000001, "Артём")
	put := func(token, uni string, body any) resp {
		return e.do("PUT", "/api/v1/profile/universities/"+uni+"/directions", token, body)
	}
	if r := put(token, "innopolis", map[string]any{"direction_ids": []string{dirBio}}); r.code != 400 {
		t.Fatalf("направления нет в вузе — 400: %d %s", r.code, r.raw)
	}
	if r := put(token, "нет-такого", map[string]any{"direction_ids": []string{dirSE}}); r.code != 404 {
		t.Fatalf("нет вуза — 404: %d %s", r.code, r.raw)
	}
	if r := put(token, "hse", map[string]any{}); r.code != 400 {
		t.Fatalf("без списка — 400: %d %s", r.code, r.raw)
	}
	_ = f
	// Профиль правят все участники — направления в вузе тоже.
	parent := e.login(900000002, "Ольга")
	if r := put(parent, "hse", map[string]any{"direction_ids": []string{dirSE}}); r.code != 200 {
		t.Fatalf("родитель выбирает направление: %d %s", r.code, r.raw)
	}
}

// Карточка вуза (F65): направления с отметкой «моё» и цели, у олимпиад —
// льгота на мои направления и на скольких направлениях вуза она есть.
func TestUniversity_Directions(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")

	b := e.do("GET", "/api/v1/universities/hse", token, nil).body
	ds := list(t, b["offered_directions"])
	if len(ds) != 19 || ds[0]["id"] != dirSE || ds[0]["is_goal"] != true || ds[0]["is_mine"] != false ||
		ds[0]["code"] != "09.03.04" || ds[0]["status"] != "offered" || ds[0]["benefit_olympiads_count"].(float64) == 0 {
		t.Fatalf("направления ВШЭ: %v", ds[:2])
	}
	if b["target_basis"] != "goal" {
		t.Fatalf("льготы по цели: %v", b["target_basis"])
	}
	var virtual map[string]any
	for _, o := range list(t, b["olympiads"]) {
		if o["olympiad_profile_id"] == virtualProfile {
			virtual = o
		}
	}
	if virtual == nil || virtual["benefit"] != "bvi" || virtual["my_benefit"] != "score100" ||
		virtual["directions_count"].(float64) < 2 || virtual["directions_total"] != float64(19) {
		t.Fatalf("олимпиада в ВШЭ: %v", virtual)
	}
}
