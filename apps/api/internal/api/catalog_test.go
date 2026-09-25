package api

import (
	"net/url"
	"strings"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/core/names"
)

func olympiadIDs(items []map[string]any) map[string]map[string]any {
	m := map[string]map[string]any{}
	for _, it := range items {
		m[it["olympiad_id"].(string)] = it
	}
	return m
}

func TestOlympiadsCatalog(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")
	get := func(query string) resp {
		t.Helper()
		r := e.do("GET", "/api/v1/olympiads"+query, token, nil)
		if r.code != 200 {
			t.Fatalf("%s: %d %s", query, r.code, r.raw)
		}
		return r
	}

	all := list(t, get("").body["items"])
	if len(all) != 75 {
		t.Fatalf("строка на олимпиаду, а не на профиль: %d", len(all))
	}
	hse := olympiadIDs(all)["p669-8"]
	pp := hse["primary_profile"].(map[string]any)
	if pp["olympiad_profile_id"] != "p669-8-informatika" || pp["level"] != "I" || hse["profiles_count"] != float64(12) ||
		hse["final_city"] != "Москва" {
		t.Fatalf("основной профиль — по предмету ученика и сильнейший: %v", hse)
	}
	if _, ok := hse["short_name"]; !ok {
		t.Fatal("поля плитки есть, пусть и null")
	}
	if hse["name"] != "Высшая проба" {
		t.Fatalf("в списке — короткое название: %v", hse["name"])
	}
	for i := 1; i < len(all); i++ {
		a, b := all[i-1]["name"].(string), all[i]["name"].(string)
		if names.Key(a) > names.Key(b) {
			t.Fatalf("по алфавиту видимого названия: %q перед %q", a, b)
		}
	}

	for _, q := range []string{"высшая ПРОБА", "ниу вшэ"} {
		found := olympiadIDs(list(t, get("?q=" + url.QueryEscape(q)).body["items"]))
		if len(found) != 1 || found["p669-8"] == nil {
			t.Fatalf("поиск %q по названию и организатору без учёта регистра: %v", q, found)
		}
	}

	phys := list(t, get("?subject=phys").body["items"])
	if len(phys) == 0 {
		t.Fatal("по физике олимпиады есть")
	}
	for _, it := range phys {
		if it["primary_profile"].(map[string]any)["subject_code"] != "phys" {
			t.Fatalf("с фильтром по предмету открывается профиль этого предмета: %v", it)
		}
	}

	kazan := list(t, get("?city=" + url.QueryEscape("Казань")).body["items"])
	if len(kazan) != 4 {
		t.Fatalf("финал в Казани — две олимпиады КФУ и два демо-примера: %v", kazan)
	}
	for _, it := range kazan {
		if it["final_city"] != "Казань" {
			t.Fatalf("город финала: %v", it)
		}
	}

	if r := get("?q=" + url.QueryEscape("нет такой олимпиады")); string(r.raw) != `{"items":[]}`+"\n" {
		t.Fatalf("пустой результат — items: [], получили %s", r.raw)
	}
	if r := e.do("GET", "/api/v1/olympiads?q="+strings.Repeat("я", 101), token, nil); r.code != 400 {
		t.Fatalf("запрос длиннее 100 символов — 400: %d", r.code)
	}
}

func TestUniversitiesCatalogAndCard(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")

	items := list(t, e.do("GET", "/api/v1/universities", token, nil).body["items"])
	if len(items) != 10 {
		t.Fatalf("10 вузов, получили %d", len(items))
	}
	mine := 0
	for _, u := range items {
		if u["is_mine"] == true {
			mine++
		}
		if u["benefit_olympiads_count"].(float64) <= 0 {
			t.Fatalf("у каждого вуза базы есть олимпиады с льготой: %v", u)
		}
	}
	if mine != 3 {
		t.Fatalf("вузы ученика отмечены: %d", mine)
	}
	if found := list(t, e.do("GET", "/api/v1/universities?q="+url.QueryEscape("ИННОПОЛ"), token, nil).body["items"]); len(found) != 1 {
		t.Fatalf("поиск вуза: %v", found)
	}
	if kazan := list(t, e.do("GET", "/api/v1/universities?city="+url.QueryEscape("Казань"), token, nil).body["items"]); len(kazan) != 2 {
		t.Fatalf("вузы Казани: %v", kazan)
	}

	r := e.do("GET", "/api/v1/universities/innopolis", token, nil)
	if r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	b := r.body
	if b["is_mine"] != true || b["rules_url"] == nil || b["ege_note"] == nil || len(b["directions"].([]any)) == 0 {
		t.Fatalf("карточка вуза: %s", r.raw)
	}
	if d, _ := b["rules_verified_at"].(string); len(d) != len("2026-09-21") {
		t.Fatalf("rules_verified_at — дата: %v", b["rules_verified_at"])
	}
	olymps := list(t, b["olympiads"])
	if len(olymps) == 0 {
		t.Fatal("у Иннополиса есть олимпиады с льготой")
	}
	first := olymps[0]
	if first["benefit_label"] != "БВИ" || first["olympiad_id"] == nil || first["subject_name"] == nil {
		t.Fatalf("строка олимпиады: %v", first)
	}
	// Олимпиады по предметам ученика — первыми.
	seenOther := false
	for _, o := range olymps {
		ok := o["subject_code"] == "inf" || o["subject_code"] == "math"
		if !ok {
			seenOther = true
		} else if seenOther {
			t.Fatalf("олимпиада по предмету ученика после чужой: %v", o)
		}
	}

	if r := e.do("GET", "/api/v1/universities/nope", token, nil); r.code != 404 || r.errCode() != "NOT_FOUND" {
		t.Fatalf("неизвестный вуз — 404: %d %s", r.code, r.raw)
	}
}

// Направления подготовки: фильтр каталога, олимпиады по направлению и
// сохранение направления к себе.
func TestUniversityPrograms(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")
	const pmi = "msu__prikladnaya-matematika-i-informatika"

	medical := list(t, e.do("GET", "/api/v1/universities?direction=napr-31-05-01", token, nil).body["items"])
	if len(medical) != 6 {
		t.Fatalf("вузы с «Лечебным делом»: %v", medical)
	}
	if r := e.do("GET", "/api/v1/universities?direction=napr-31-05-01&city="+url.QueryEscape("Казань"), token, nil); len(list(t, r.body["items"])) != 2 {
		t.Fatalf("фильтр по направлению и городу: %s", r.raw)
	}

	card := e.do("GET", "/api/v1/universities/msu", token, nil).body
	programs := list(t, card["programs"])
	var found map[string]any
	for _, p := range programs {
		if p["id"] == pmi {
			found = p
		}
	}
	if found == nil || found["code"] != "01.03.02" || found["direction_id"] != "napr-01-03-02" ||
		found["university_short_name"] == "" || found["is_mine"] != false || found["olympiads_count"].(float64) <= 0 {
		t.Fatalf("направление в карточке вуза: %v", found)
	}
	for _, key := range []string{"faculty", "budget_places"} {
		if _, ok := found[key]; !ok {
			t.Fatalf("поле %s есть, пусть и null: %v", key, found)
		}
	}
	all := list(t, card["olympiads"])
	r := e.do("GET", "/api/v1/universities/msu?program="+pmi, token, nil)
	if r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	own := list(t, r.body["olympiads"])
	if len(own) == 0 || len(own) >= len(all) {
		t.Fatalf("олимпиады направления — часть олимпиад вуза: %d из %d", len(own), len(all))
	}
	if r := e.do("GET", "/api/v1/universities/hse?program="+pmi, token, nil); r.code != 404 {
		t.Fatalf("направление другого вуза — 404: %d %s", r.code, r.raw)
	}

	if r := e.do("PUT", "/api/v1/profile/programs", token, map[string]any{}); r.code != 400 {
		t.Fatalf("без списка направлений: %d", r.code)
	}
	if r := e.do("PUT", "/api/v1/profile/programs", token, map[string]any{"program_ids": []string{"нет"}}); r.code != 400 {
		t.Fatalf("неизвестное направление: %d %s", r.code, r.raw)
	}
	r = e.do("PUT", "/api/v1/profile/programs", token, map[string]any{"program_ids": []string{pmi}})
	saved := list(t, r.body["programs"])
	if r.code != 200 || len(saved) != 1 || saved[0]["id"] != pmi || saved[0]["is_mine"] != true {
		t.Fatalf("сохранили направление: %d %s", r.code, r.raw)
	}
	unis := list(t, r.body["universities"])
	if len(unis) != 4 {
		t.Fatalf("вуз направления добавился к вузам ученика: %v", unis)
	}
	if got := list(t, e.do("GET", "/api/v1/profile", token, nil).body["programs"]); len(got) != 1 {
		t.Fatalf("GET /profile отдаёт сохранённые направления: %v", got)
	}
	if first := list(t, e.do("GET", "/api/v1/universities/msu", token, nil).body["programs"])[0]; first["id"] != pmi || first["is_mine"] != true {
		t.Fatalf("сохранённое направление — первым: %v", first)
	}
}

// Карточки рассказывают, что это за вуз и олимпиада, и ведут на их сайты.
func TestCards_DescriptionAndSite(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")

	u := e.do("GET", "/api/v1/universities/hse", token, nil).body
	if u["site_url"] != "https://www.hse.ru/" {
		t.Fatalf("сайт вуза: %v", u["site_url"])
	}
	if d, _ := u["description"].(string); len(d) < 40 {
		t.Fatalf("описание вуза: %v", u["description"])
	}

	o := e.do("GET", "/api/v1/olympiads/p669-8-informatika", token, nil).body
	if _, ok := o["description"]; !ok {
		t.Fatalf("у карточки олимпиады нет поля description: %v", o)
	}
}

// Справочник целей для профиля (F49): цель меняется выбором из направлений.
func TestDirections(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")
	r := e.do("GET", "/api/v1/directions", token, nil)
	if r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	items := list(t, r.body["items"])
	if len(items) != 16 {
		t.Fatalf("направлений %d, ждали 16", len(items))
	}
	found := false
	for _, it := range items {
		if it["id"] == "napr-09-03-04" && it["name"] == "Программная инженерия" {
			found = true
		}
	}
	if !found {
		t.Fatalf("нет «Программной инженерии»: %v", items)
	}
	if r := e.do("GET", "/api/v1/directions", "", nil); r.code != 401 {
		t.Fatalf("без токена — 401, получили %d", r.code)
	}
}
