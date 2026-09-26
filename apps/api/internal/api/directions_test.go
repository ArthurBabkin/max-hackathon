package api

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/dbtest"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

// Сид: «Виртуальные миры» НТО. ВШЭ на Программную инженерию даёт БВИ не на
// всех программах, на ИБ — БВИ.
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

// Слабее льгота на другом моём направлении — со своими победителем и
// призёром. «Высшая проба» по математике в ИТМО: на ПИ — БВИ всем, на ПМИ —
// БВИ победителю, призёру 100 баллов (а не «ничего»).
func TestOlympiad_OtherDirectionGrants(t *testing.T) {
	e := newEnv(t)
	f := e.kidCreator()
	token := e.login(900000001, "Артём")
	if err := e.st.ReplaceUniversities(context.Background(), f.trajectoryID, []string{"itmo"}); err != nil {
		t.Fatal(err)
	}
	if r := e.do("PUT", "/api/v1/profile/universities/itmo/directions", token,
		map[string]any{"direction_ids": []string{dirSE, "napr-01-03-02"}}); r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	itmo := rowOf(t, list(t, e.do("GET", "/api/v1/olympiads/p669-2-matematika", token, nil).body["benefits"]), "itmo")
	others := list(t, itmo["other_directions"])
	if itmo["benefit"] != "bvi" || len(others) != 1 || others[0]["benefit"] != "bvi_winners" ||
		grant(others[0]["winner"]) != "bvi БВИ" || grant(others[0]["prizer"]) != "score100 100 баллов" {
		t.Fatalf("ИТМО на ПИ и ПМИ: %v", itmo)
	}
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
	if hse["benefit"] != "bvi" || !slices.Equal(strs(hse["directions"]), []string{"Программная инженерия"}) ||
		hse["unverified"] != false || hse["varies"] != true {
		t.Fatalf("ВШЭ на ПИ: %v", hse)
	}
	// Льгота не на всех программах направления — плашкой у вуза.
	if c := strs(hse["conditions"]); !slices.ContainsFunc(c, func(x string) bool {
		return strings.HasPrefix(x, "Зависит от программы: льгота только на «Компьютерные науки и технологии»")
	}) {
		t.Fatalf("ВШЭ: зависит от программы: %v", hse["conditions"])
	}

	// «Где ещё даёт льготу» — на скольких направлениях вуза.
	body := e.do("GET", "/api/v1/olympiads/"+virtualProfile, token, nil).body
	// ИТМО: «учитывается только на 11.03.02» (перечень БВИ, стр. 6).
	if itmo := rowOf(t, list(t, body["benefit_universities"]), "itmo"); itmo["directions_count"] != float64(1) ||
		itmo["directions_total"] != float64(24) {
		t.Fatalf("ИТМО: на 1 из 24 направлений: %v", itmo)
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
	if len(ds) != 18 || ds[0]["id"] != dirSE || ds[0]["is_goal"] != true || ds[0]["is_mine"] != false ||
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
	if virtual == nil || virtual["benefit"] != "bvi" || virtual["my_benefit"] != "bvi" ||
		virtual["directions_count"].(float64) < 2 || virtual["directions_total"] != float64(18) {
		t.Fatalf("олимпиада в ВШЭ: %v", virtual)
	}
}

// Льготы на выбранные направления в вузе ещё проверяются: строка — льгота
// вуза целиком с плашкой «уточняется».
func TestOlympiad_BenefitUnverified(t *testing.T) {
	e := newEnv(t)
	dbtest.ToCheck(t, e.pool, "hse", "napr-40-03-01", "Юриспруденция")
	f := e.kidCreator()
	token := e.login(900000001, "Артём")
	if err := e.st.ReplaceUniversities(context.Background(), f.trajectoryID, []string{"hse"}); err != nil {
		t.Fatal(err)
	}
	if r := e.do("PUT", "/api/v1/profile/universities/hse/directions", token, map[string]any{"direction_ids": []string{"napr-40-03-01"}}); r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	var row map[string]any
	for _, b := range list(t, e.do("GET", "/api/v1/olympiads/p669-8-informatika", token, nil).body["benefits"]) {
		if b["university_id"] == "hse" {
			row = b
		}
	}
	if row == nil || row["unverified"] != true || !slices.Contains(strs(row["conditions"]),
		"Льгота на «Юриспруденция» ещё уточняется — пока показана льгота вуза целиком") {
		t.Fatalf("ВШЭ: %v", row)
	}
}

// «Ведут в мои вузы и на мои направления» (F66): олимпиады, у которых есть
// БВИ, БВИ победителям или 100 баллов в моих вузах — на мои направления
// по правилу целей, а не в вузе целиком.
func TestOlympiadsCatalog_Mine(t *testing.T) {
	e := newEnv(t)
	f := e.kidCreator()
	token := e.login(900000001, "Артём")
	ctx := context.Background()
	// Только ВШЭ, в ней выбрана ИБ: часть олимпиад даёт льготу в ВШЭ, но не на ИБ.
	unis := []string{"hse"}
	if err := e.st.ReplaceUniversities(ctx, f.trajectoryID, unis); err != nil {
		t.Fatal(err)
	}
	if r := e.do("PUT", "/api/v1/profile/universities/hse/directions", token, map[string]any{"direction_ids": []string{dirIS}}); r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}

	// Оракул — store: профили по информатике с сильной льготой.
	profiles, err := e.st.Profiles(ctx, store.ProfileQuery{SubjectCodes: []string{"inf"}})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(profiles))
	olympiadOf := map[string]string{}
	for i, p := range profiles {
		ids[i], olympiadOf[p.ID] = p.ID, p.OlympiadID
	}
	strong := func(rows []store.BenefitRow) map[string]bool {
		out := map[string]bool{}
		for _, b := range rows {
			if b.Benefit == "bvi" || b.Benefit == "bvi_winners" || b.Benefit == "score100" {
				out[olympiadOf[b.ProfileID]] = true
			}
		}
		return out
	}
	targetRows, _ := e.st.TargetBenefits(ctx, f.trajectoryID, ids, unis)
	wholeRows, _ := e.st.Benefits(ctx, ids, unis)
	want, whole := strong(targetRows), strong(wholeRows)
	if len(want) == 0 || len(whole) <= len(want) {
		t.Fatalf("в сиде есть олимпиады с льготой в ВШЭ, но не на ИБ: %d / %d", len(whole), len(want))
	}

	items := list(t, e.do("GET", "/api/v1/olympiads?mine=true&subject=inf", token, nil).body["items"])
	got := map[string]bool{}
	rank := map[string]int{"bvi": 0, "bvi_winners": 1, "score100": 2}
	for _, it := range items {
		got[it["olympiad_id"].(string)] = true
		if it["primary_profile"].(map[string]any)["subject_code"] != "inf" {
			t.Fatalf("с предметом — «и»: %v", it)
		}
		groups := list(t, it["my_benefits"])
		if len(groups) == 0 {
			t.Fatalf("у строки — льгота в моих вузах: %v", it)
		}
		for i, g := range groups {
			if _, ok := rank[g["benefit"].(string)]; !ok || g["benefit_label"] == "" || len(strs(g["universities"])) == 0 {
				t.Fatalf("группа льготы: %v", g)
			}
			if i > 0 && rank[groups[i-1]["benefit"].(string)] >= rank[g["benefit"].(string)] {
				t.Fatalf("от сильной льготы к слабой: %v", groups)
			}
			for _, u := range strs(g["universities"]) {
				if u != "ВШЭ" {
					t.Fatalf("вузы — мои, короткими названиями: %v", g)
				}
			}
			// На ИБ в ВШЭ льготы от программы не зависят.
			if p, ok := g["partial_universities"]; !ok || len(strs(p)) != 0 {
				t.Fatalf("льгота на все программы — partial_universities пуст: %v", g)
			}
		}
	}
	if len(got) != len(want) {
		t.Fatalf("олимпиады с льготой на мои направления: %d, ждём %d", len(got), len(want))
	}
	for id := range want {
		if !got[id] {
			t.Fatalf("нет %s", id)
		}
	}

	// Без фильтра льгота в строке не считается.
	for _, it := range list(t, e.do("GET", "/api/v1/olympiads?subject=inf", token, nil).body["items"]) {
		if len(list(t, it["my_benefits"])) != 0 {
			t.Fatalf("без mine — пусто: %v", it)
		}
	}

	// На ПИ в ВШЭ льгота зависит от программы — вуз помечен: «БВИ» не
	// должно читаться как «на любую программу направления».
	if r := e.do("PUT", "/api/v1/profile/universities/hse/directions", token, map[string]any{"direction_ids": []string{dirSE}}); r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	targetRows, _ = e.st.TargetBenefits(ctx, f.trajectoryID, ids, unis)
	// По профилю строки (основному по предмету), а не по олимпиаде целиком:
	// у «Высшей пробы» от программы зависит математика, а не информатика.
	varies := map[string]bool{}
	for _, b := range targetRows {
		varies[b.ProfileID] = varies[b.ProfileID] || (b.Varies && !b.Unverified)
	}
	marked := 0
	for _, it := range list(t, e.do("GET", "/api/v1/olympiads?mine=true&subject=inf", token, nil).body["items"]) {
		for _, g := range list(t, it["my_benefits"]) {
			partial := strs(g["partial_universities"])
			if len(partial) > 0 {
				marked++
			}
			pid := it["primary_profile"].(map[string]any)["olympiad_profile_id"].(string)
			if varies[pid] != slices.Equal(partial, []string{"ВШЭ"}) {
				t.Fatalf("%s: зависит от программы — %v, в ответе %v", pid, varies[pid], g)
			}
		}
	}
	if marked == 0 {
		t.Fatal("в сиде на ПИ в ВШЭ льготы зависят от программы — пометка должна быть")
	}

	// Льготы на выбранные в ВШЭ направления ещё уточняются: вести туда
	// нечем — как и в карточке вуза на «На мои направления».
	dbtest.ToCheck(t, e.pool, "hse", "napr-40-03-01", "Юриспруденция")
	if err := e.st.ReplaceUniversities(ctx, f.trajectoryID, []string{"hse"}); err != nil {
		t.Fatal(err)
	}
	if r := e.do("PUT", "/api/v1/profile/universities/hse/directions", token, map[string]any{"direction_ids": []string{"napr-40-03-01"}}); r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	if items := list(t, e.do("GET", "/api/v1/olympiads?mine=true&subject=math", token, nil).body["items"]); len(items) != 0 {
		t.Fatalf("льготы уточняются — не «ведут»: %d, первая %v", len(items), items[0]["my_benefits"])
	}

	// Вузов нет — никуда не ведут.
	if err := e.st.ReplaceUniversities(ctx, f.trajectoryID, []string{}); err != nil {
		t.Fatal(err)
	}
	if items := list(t, e.do("GET", "/api/v1/olympiads?mine=true", token, nil).body["items"]); len(items) != 0 {
		t.Fatalf("без вузов пусто: %d", len(items))
	}
}

// Каталог вузов по направлению (F67): вузы, где оно есть (с укрупнёнными
// группами), сколько олимпиад дают на нём льготу и проверены ли льготы.
func TestUniversitiesCatalog_Direction(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")
	get := func(query string) resp { return e.do("GET", "/api/v1/universities"+query, token, nil) }

	items := list(t, get("?direction=" + dirSE).body["items"])
	inno := rowOf(t, withID(items), "innopolis")
	match, _ := inno["direction_match"].(map[string]any)
	if match == nil || !slices.Equal(strs(match["direction_ids"]), []string{"napr-09-00-00"}) ||
		match["olympiads_count"].(float64) == 0 || match["status"] != "offered" {
		t.Fatalf("Иннополис по ПИ: %v", inno)
	}
	for _, u := range items {
		if u["id"] == "kazan-gmu" {
			t.Fatal("медвуза без ПИ нет")
		}
	}

	dbtest.ToCheck(t, e.pool, "hse", "napr-40-03-01", "Юриспруденция")
	items = list(t, get("?direction=napr-40-03-01").body["items"])
	last := items[len(items)-1]
	if m := last["direction_match"].(map[string]any); last["id"] != "hse" || m["status"] != "to_check" || m["olympiads_count"] != float64(0) {
		t.Fatalf("ВШЭ по юриспруденции — льготы уточняются, в конце: %v", last)
	}

	for _, u := range list(t, get("").body["items"]) {
		if _, ok := u["direction_match"]; ok {
			t.Fatalf("без фильтра — без direction_match: %v", u)
		}
	}
	if r := get("?direction=нет-такого"); r.code != 400 {
		t.Fatalf("нет направления — 400: %d %s", r.code, r.raw)
	}
}

// withID — строки вузов с university_id, как у строк льгот, для rowOf.
func withID(items []map[string]any) []map[string]any {
	for _, it := range items {
		it["university_id"] = it["id"]
	}
	return items
}
