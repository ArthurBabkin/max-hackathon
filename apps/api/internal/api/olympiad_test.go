package api

import (
	"strings"
	"testing"
)

func list(t *testing.T, v any) []map[string]any {
	t.Helper()
	raw, ok := v.([]any)
	if !ok {
		t.Fatalf("не массив: %v", v)
	}
	out := make([]map[string]any, len(raw))
	for i, x := range raw {
		out[i] = x.(map[string]any)
	}
	return out
}

func contains(xs []any, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func TestOlympiad_PerechenCard(t *testing.T) {
	e := newEnv(t)
	f := e.kidCreator()
	token := e.login(900000001, "Артём")

	r := e.do("GET", "/api/v1/olympiads/p669-8-informatika", token, nil)
	if r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	b := r.body
	if b["name"] != "Всероссийская олимпиада школьников «Высшая проба»" || b["kind"] != "perechen" ||
		b["level"] != "I" || b["official_url"] == nil || b["stages_are_demo"] != false {
		t.Fatalf("шапка карточки: %s", r.raw)
	}

	profiles := list(t, b["profiles"])
	if len(profiles) != 12 {
		t.Fatalf("все 12 профилей «Высшей пробы», получили %d", len(profiles))
	}
	for _, p := range profiles {
		if (p["olympiad_profile_id"] == "p669-8-informatika") != (p["is_mine"] == true) {
			t.Fatalf("is_mine — только у открытого профиля: %v", p)
		}
	}
	if src, _ := b["profiles_source"].(map[string]any); src == nil || src["kind"] != "order" {
		t.Fatalf("уровни — из перечня: %v", b["profiles_source"])
	}

	benefits := list(t, b["benefits"])
	var names []string
	for _, row := range benefits {
		names = append(names, row["university_id"].(string))
		if row["benefit"] != "bvi" || row["benefit_label"] != "БВИ" || row["ege_min"] != float64(75) {
			t.Fatalf("льгота в вузе ученика: %v", row)
		}
	}
	if strings.Join(names, ",") != "kfu,hse,innopolis" {
		t.Fatalf("по строке на каждый вуз ученика: %v", names)
	}
	// У ВШЭ в ключе есть демо-запись: строка без источника, и весь блок
	// честно помечается «данные уточняются».
	if benefits[1]["source"] != nil || b["benefits_source"] != nil {
		t.Fatalf("демо-строка снимает метку «Факт»: %v", b["benefits_source"])
	}

	conds, _ := b["conditions"].([]any)
	for _, want := range []string{
		"Нужен диплом победителя или призёра",
		"ЕГЭ по предмету «Информатика» — не ниже 75",
		"На некоторых программах порог выше — проверь правила вуза",
		"БВИ можно использовать только в одном вузе",
	} {
		if !contains(conds, want) {
			t.Fatalf("нет условия %q: %v", want, conds)
		}
	}
	why, _ := b["why"].(string)
	if !strings.Contains(why, "Профиль «информатика» совпадает с направлением «Программная инженерия».") ||
		!strings.Contains(why, "БВИ в твоих вузах: КФУ, ВШЭ, Иннополис.") {
		t.Fatalf("почему подходит: %q", why)
	}

	stages := list(t, b["stages"])
	if len(stages) != 4 || stages[0]["kind"] != "registration" || stages[0]["state"] != "current" ||
		stages[1]["state"] != "future" || stages[0]["subtitle"] == nil {
		t.Fatalf("этапы: %v", stages)
	}

	where := list(t, b["benefit_universities"])
	if len(where) != 8 || where[len(where)-1]["benefit"] != "score100" {
		t.Fatalf("где даёт льготу — все вузы, сильные льготы первыми: %v", where)
	}

	// После отметки о регистрации она пройдена, текущий — отборочный.
	e.track(f, "p669-8-informatika", true)
	b = e.do("GET", "/api/v1/olympiads/p669-8-informatika", token, nil).body
	stages = list(t, b["stages"])
	if stages[0]["state"] != "past" || stages[1]["state"] != "current" || b["in_tracker"] != true ||
		b["next_stage_title"] != "Отборочный этап, 1 тур" {
		t.Fatalf("после отметки: %v", stages)
	}
}

func TestOlympiad_BenefitsSourceWhenEveryRowIsSourced(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	b := e.do("GET", "/api/v1/olympiads/p669-34-informatika", e.login(900000001, "Артём"), nil).body
	src, _ := b["benefits_source"].(map[string]any)
	if src == nil || src["kind"] != "rules" || src["verified_at"] == nil ||
		!strings.HasSuffix(src["title"].(string), ": КФУ, ВШЭ, Иннополис") {
		t.Fatalf("все строки с источником — блок с меткой «Факт» и правилами всех вузов: %v", b["benefits_source"])
	}
}

func TestOlympiad_VsoshDemoBenefitsAndParentVoice(t *testing.T) {
	e := newEnv(t)
	e.withParent(e.kidCreator())
	r := e.do("GET", "/api/v1/olympiads/vsosh-informatika", e.login(900000002, "Ольга"), nil)
	if r.code != 200 {
		t.Fatalf("%d %s", r.code, r.raw)
	}
	b := r.body
	if b["stages_are_demo"] != true || len(list(t, b["stages"])) != 4 {
		t.Fatalf("у ВсОШ четыре демо-этапа: %s", r.raw)
	}
	if b["benefits_source"] != nil {
		t.Fatalf("льготы из демо-записей — без метки «Факт»: %v", b["benefits_source"])
	}
	kfu := list(t, b["benefits"])[0]
	if kfu["university_id"] != "kfu" || kfu["benefit"] != nil || kfu["benefit_label"] != nil || kfu["source"] != nil {
		t.Fatalf("вуз ученика без льготы — строка «не учитывает»: %v", kfu)
	}
	why, _ := b["why"].(string)
	if !strings.HasPrefix(why, "Школьный этап проходит в школе Артёма") || !strings.Contains(why, "в вузах Артёма") {
		t.Fatalf("родителю — голос родителя: %q", why)
	}
	if conds, _ := b["conditions"].([]any); !contains(conds, "Нужен диплом победителя или призёра заключительного этапа") {
		t.Fatalf("условия ВсОШ: %v", conds)
	}
}

func TestOlympiad_NoStagesOutsideAndNotFound(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")

	b := e.do("GET", "/api/v1/olympiads/p669-5-yadernye-tehnologii", token, nil).body
	if len(list(t, b["stages"])) != 0 || b["stages_are_demo"] != true || b["deadline_at"] != nil {
		t.Fatalf("профиль без дат — «данные уточняются»: %v", b)
	}

	b = e.do("GET", "/api/v1/olympiads/other-tyk-inf", token, nil).body
	conds, _ := b["conditions"].([]any)
	if b["kind"] != "other" || b["reason"] != "Льгот нет, только дополнительные баллы" ||
		!contains(conds, "Льгот при поступлении не даёт") {
		t.Fatalf("вне перечня: %v", b)
	}

	if r := e.do("GET", "/api/v1/olympiads/nope", token, nil); r.code != 404 || r.errCode() != "NOT_FOUND" {
		t.Fatalf("неизвестный профиль — 404: %d %s", r.code, r.raw)
	}
}
