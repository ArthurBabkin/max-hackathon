package api

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/core/pick"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
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
	if b["name"] != "Высшая проба" || b["kind"] != "perechen" ||
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

	// Таблица льгот (F18, F19): вузы строками, самые выгодные первыми, дальше
	// по алфавиту короткого имени.
	benefits := list(t, b["benefits"])
	var names []string
	for _, row := range benefits {
		names = append(names, row["university_nick"].(string))
		if row["benefit"] != "bvi" || row["benefit_label"] != "БВИ" || row["ege_min"] != float64(75) ||
			grant(row["winner"]) != "bvi БВИ" || grant(row["prizer"]) != "bvi БВИ" {
			t.Fatalf("льгота в вузе ученика: %v", row)
		}
	}
	if strings.Join(names, ",") != "ВШЭ,Иннополис,КФУ" {
		t.Fatalf("по строке на каждый вуз ученика: %v", names)
	}
	// У ВШЭ в ключе есть демо-запись: строка без источника, и весь блок
	// честно помечается «данные уточняются».
	if benefits[0]["source"] != nil || b["benefits_source"] != nil {
		t.Fatalf("демо-строка снимает метку «Факт»: %v", b["benefits_source"])
	}

	// У ВШЭ порог зависит от программы — он в столбце «Порог ЕГЭ», а не в
	// условиях строки; у других вузов верхней границы нет. Порог — на
	// направление цели (Программная инженерия), а не по вузу целиком.
	if benefits[0]["ege_max"] != float64(85) || benefits[1]["ege_max"] != nil || benefits[2]["ege_max"] != nil {
		t.Fatalf("разброс порога ВШЭ: %v", benefits)
	}
	// Столбцы показали всё, кроме того, что льгота ВШЭ на Программную
	// инженерию есть не на всех программах (F65).
	for i, row := range benefits {
		var want []string
		if i == 0 {
			want = []string{"Зависит от программы: на «Программирование и инжиниринг компьютерных игр» и «Разработка информационных систем для бизнеса» льготы нет"}
		}
		var got []string
		if row["conditions"] != nil {
			got = strs(row["conditions"])
		}
		if !slices.Equal(got, want) {
			t.Fatalf("свои условия вуза: %v", row)
		}
	}
	if fmt.Sprint(b["benefit_columns"]) != "[winner prizer ege]" {
		t.Fatalf("пороги разные — столбец ЕГЭ есть: %v", b["benefit_columns"])
	}

	// Общие условия — то, что верно для всех вузов ученика. Порог уже в
	// таблице, поэтому здесь только предмет.
	conds, _ := b["conditions"].([]any)
	want := []any{
		"Нужен диплом победителя или призёра",
		"Льготу подтверждает ЕГЭ по предмету «Информатика»",
		"БВИ можно использовать только в одном вузе",
	}
	if fmt.Sprint(conds) != fmt.Sprint(want) {
		t.Fatalf("общие условия: %v", conds)
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

	// «Где ещё даёт льготу» — вузы базы, кроме вузов ученика: они уже в
	// блоке льгот выше.
	where := list(t, b["benefit_universities"])
	if len(where) != 5 || where[len(where)-1]["benefit"] != "score100" {
		t.Fatalf("где ещё даёт льготу — остальные вузы, сильные льготы первыми: %v", where)
	}
	for _, row := range where {
		if id := row["university_id"]; id == "kfu" || id == "hse" || id == "innopolis" {
			t.Fatalf("вуз ученика повторён в «Где ещё даёт льготу»: %v", row)
		}
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

// grant — «вид подпись» того, что получит победитель или призёр; nil — "".
func grant(v any) string {
	g, _ := v.(map[string]any)
	if g == nil {
		return ""
	}
	return fmt.Sprintf("%v %v", g["kind"], g["label"])
}

// rowConditions — условия вуза из строки льготы через « | ».
func rowConditions(t *testing.T, row map[string]any) string {
	t.Helper()
	raw, _ := row["conditions"].([]any)
	out := make([]string, len(raw))
	for i, c := range raw {
		out[i] = c.(string)
	}
	return strings.Join(out, " | ")
}

// У МГУ и МФТИ по «Высшей пробе» свои правила: у МГУ призёру — 100 баллов и
// засчитывается только диплом 11 класса. МФТИ на Программную инженерию (цель
// Артёма) даёт 100 баллов, а не БВИ, как по вузу целиком. В МГУ этого
// направления нет — там льгота вуза.
func TestOlympiad_ConditionsByUniversity(t *testing.T) {
	e := newEnv(t)
	f := e.kidCreator()
	if err := e.st.ReplaceUniversities(context.Background(), f.trajectoryID, []string{"msu", "mipt", "kfu"}); err != nil {
		t.Fatal(err)
	}
	b := e.do("GET", "/api/v1/olympiads/p669-8-informatika", e.login(900000001, "Артём"), nil).body

	// Призёр и порог — в столбцах таблицы, в условиях строки остаётся только
	// то, для чего столбца нет: класс диплома.
	var got []string
	for _, row := range list(t, b["benefits"]) {
		got = append(got, fmt.Sprintf("%s: %s / %s / %v–%v / %s", row["university_nick"], grant(row["winner"]),
			grant(row["prizer"]), row["ege_min"], row["ege_max"], rowConditions(t, row)))
	}
	want := []string{
		"КФУ: bvi БВИ / bvi БВИ / 75–<nil> / ",
		"МГУ: bvi БВИ / score100 100 баллов / 75–<nil> / Засчитывает только диплом 11 класса — диплом за 9 класс не подойдёт",
		"МФТИ: score100 100 баллов / score100 100 баллов / 75–<nil> / ",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("строки таблицы:\n%s", strings.Join(got, "\n"))
	}
	for _, c := range b["conditions"].([]any) {
		if strings.Contains(c.(string), "МГУ") || strings.Contains(c.(string), "порог выше") {
			t.Fatalf("своё у вуза не дублируется в общих условиях: %v", b["conditions"])
		}
	}
}

// Вузы ученика льготы не дают — условия собираются по всем вузам базы, и
// особенности вузов остаются в общих условиях: строк с вузами, где их
// показать, в карточке нет.
func TestOlympiad_ConditionsFromAllUniversities(t *testing.T) {
	e := newEnv(t)
	f := e.kidCreator()
	if err := e.st.ReplaceUniversities(context.Background(), f.trajectoryID, []string{"spbu"}); err != nil {
		t.Fatal(err)
	}
	b := e.do("GET", "/api/v1/olympiads/p669-8-informatika", e.login(900000001, "Артём"), nil).body

	conds, _ := b["conditions"].([]any)
	want := []any{
		"Нужен диплом победителя или призёра",
		"ЕГЭ по предмету «Информатика» не ниже 75",
		"На некоторых программах порог выше, проверь правила вуза",
		"МГУ, МФТИ: победителю — БВИ, призёру — 100 баллов",
		"БВИ можно использовать только в одном вузе",
	}
	if fmt.Sprint(conds) != fmt.Sprint(want) {
		t.Fatalf("условия по всем вузам: %v", conds)
	}
	if spbu := list(t, b["benefits"])[0]; spbu["benefit"] != nil || spbu["conditions"] != nil ||
		spbu["winner"] != nil || spbu["prizer"] != nil || spbu["university_nick"] != "СПбГУ" {
		t.Fatalf("у вуза без льготы своих условий нет: %v", spbu)
	}
	if fmt.Sprint(b["benefit_columns"]) != "[winner prizer]" {
		t.Fatalf("без засчитанных строк сравнивать пороги не с чем: %v", b["benefit_columns"])
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
	rows := list(t, b["benefits"])
	kfu := rows[len(rows)-1]
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
	// Вне перечня — один столбец «Доп. баллы», больше баллов — выше.
	var extra []string
	for _, row := range list(t, b["benefits"]) {
		extra = append(extra, fmt.Sprintf("%s %s", row["university_nick"], grant(row["prizer"])))
	}
	if fmt.Sprint(b["benefit_columns"]) != "[extra_points]" ||
		strings.Join(extra, ", ") != "КФУ extra_points +3 балла, Иннополис extra_points +2 балла, ВШЭ " {
		t.Fatalf("доп. баллы по вузам: %v %v", b["benefit_columns"], extra)
	}

	if r := e.do("GET", "/api/v1/olympiads/nope", token, nil); r.code != 404 || r.errCode() != "NOT_FOUND" {
		t.Fatalf("неизвестный профиль — 404: %d %s", r.code, r.raw)
	}
}

// Правила вуза, которых нет у демо-ученика: призёр без льготы, свой порог
// без разброса по программам, класс диплома, который ученику не мешает.
func TestUniConditions(t *testing.T) {
	cs := cardSet{
		Set:   pick.Set{Trajectory: store.Trajectory{Grade: 10}},
		voice: voice.New(voice.Role("parent"), "Артём", "Ольга"),
	}
	for _, tc := range []struct {
		name string
		row  store.BenefitRow
		want string
	}{
		// Призёр и порог видны в столбцах таблицы — в условиях строки их нет.
		{"призёру ничего", store.BenefitRow{Note: ptr("БВИ только победителю. Подтвердить ЕГЭ: Физика")}, ""},
		{"порог выше общего", store.BenefitRow{EgeMin: ptr(80)}, ""},
		{"класс ученика засчитывается", store.BenefitRow{DiplomaGrades: []int32{10, 11}}, ""},
		{"классы не подряд", store.BenefitRow{DiplomaGrades: []int32{9, 11}},
			"Засчитывает только диплом 9, 11 класса — диплом за 10 класс не подойдёт"},
		{"другой предмет ЕГЭ", store.BenefitRow{Note: ptr("Подтвердить ЕГЭ: Астрономия")},
			"Льготу подтверждает ЕГЭ по предмету «Астрономия», а не «Физика»"},
		{"предмет среди вариантов", store.BenefitRow{Note: ptr("Подтвердить ЕГЭ: Математика или Физика")}, ""},
		{"предмет с «и ИКТ»", store.BenefitRow{Note: ptr("Подтвердить ЕГЭ: Физика и ИКТ")}, ""},
		// Хвост разбора правил, а не предмет ЕГЭ: в карточку такое не выводим.
		{"не предмет", store.BenefitRow{Note: ptr("Подтвердить ЕГЭ: фотоника, приборостроение, машиностроение")}, ""},
	} {
		if got := strings.Join(cs.uniConditions(tc.row, "Физика"), " | "); got != tc.want {
			t.Errorf("%s: %q, ждали %q", tc.name, got, tc.want)
		}
	}
}

// Что получат победитель и призёр в вузе — из вида льготы и примечания.
func TestBenefitGrants(t *testing.T) {
	for _, tc := range []struct {
		name   string
		row    store.BenefitRow
		winner string
		prizer string
	}{
		{"БВИ обоим", store.BenefitRow{Benefit: "bvi"}, "bvi БВИ", "bvi БВИ"},
		{"БВИ только победителю", store.BenefitRow{Benefit: "bvi_winners", Note: ptr("БВИ только победителю")}, "bvi БВИ", ""},
		{"призёру 100 баллов", store.BenefitRow{Benefit: "bvi_winners",
			Note: ptr("Победителю — БВИ, призёру — 100 баллов. Подтвердить ЕГЭ: Физика")}, "bvi БВИ", "score100 100 баллов"},
		{"100 баллов обоим", store.BenefitRow{Benefit: "score100"}, "score100 100 баллов", "score100 100 баллов"},
		{"100 баллов только победителю", store.BenefitRow{Benefit: "score100", Note: ptr("100 баллов только победителю")},
			"score100 100 баллов", ""},
		{"1 балл", store.BenefitRow{Benefit: "extra_points", ExtraPoints: ptr(1)}, "extra_points +1 балл", "extra_points +1 балл"},
		{"5 баллов", store.BenefitRow{Benefit: "extra_points", ExtraPoints: ptr(5)}, "extra_points +5 баллов", "extra_points +5 баллов"},
		{"баллы без числа", store.BenefitRow{Benefit: "extra_points"}, "extra_points доп. баллы", "extra_points доп. баллы"},
	} {
		row := benefitRowOf(tc.row)
		if got := grantOf(row.Winner) + " / " + grantOf(row.Prizer); got != tc.winner+" / "+tc.prizer {
			t.Errorf("%s: %q, ждали %q", tc.name, got, tc.winner+" / "+tc.prizer)
		}
	}
}

func grantOf(g *benefitGrant) string {
	if g == nil {
		return ""
	}
	return g.Kind + " " + g.Label
}

// Регистрация закрылась, а отборочный ещё идёт: срок в карточке — уже про
// отборочный, и без пометки казалось бы, что вступить ещё можно.
func TestOlympiad_RegistrationClosed(t *testing.T) {
	e := newEnv(t)
	e.kidCreator()
	token := e.login(900000001, "Артём")
	for id, want := range map[string]bool{"p669-54-informatika-i-programmirovanie": true, "p669-8-informatika": false} {
		r := e.do("GET", "/api/v1/olympiads/"+id, token, nil)
		if r.code != 200 || r.body["registration_closed"] != want {
			t.Fatalf("%s: registration_closed = %v, ждали %v", id, r.body["registration_closed"], want)
		}
	}
	items := list(t, e.do("GET", "/api/v1/olympiads?q="+url.QueryEscape("Физтех"), token, nil).body["items"])
	if len(items) == 0 || items[0]["registration_closed"] != true {
		t.Fatalf("в каталоге — та же пометка: %v", items)
	}
	hse := list(t, e.do("GET", "/api/v1/olympiads?q="+url.QueryEscape("Высшая проба"), token, nil).body["items"])
	if len(hse) == 0 || hse[0]["registration_closed"] != false {
		t.Fatalf("открытая регистрация: %v", hse)
	}
}
