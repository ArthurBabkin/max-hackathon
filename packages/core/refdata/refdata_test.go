package refdata

import (
	"strings"
	"testing"
	"time"
)

func TestAll_HasEveryFederalSubjectOnce(t *testing.T) {
	all := All()
	if len(all) != 89 {
		t.Fatalf("субъектов РФ 89, в справочнике %d", len(all))
	}
	seen := map[string]bool{}
	for _, r := range all {
		if seen[r.Code] {
			t.Errorf("код %s повторяется", r.Code)
		}
		seen[r.Code] = true
		if len(r.Code) != 2 || r.Name == "" {
			t.Errorf("битая запись: %+v", r)
		}
		if r.Lat < 41 || r.Lat > 72 || r.Lon < 19 || r.Lon > 180 {
			t.Errorf("%s: координаты центра вне России: %v, %v", r.Name, r.Lat, r.Lon)
		}
		if _, err := time.LoadLocation(r.TZ); err != nil {
			t.Errorf("%s: часовой пояс %q не загружается: %v", r.Name, r.TZ, err)
		}
	}
}

func TestAll_SortedByNameIgnoringRepublicPrefix(t *testing.T) {
	all := All()
	for i := 1; i < len(all); i++ {
		if sortKey(all[i-1].Name) > sortKey(all[i].Name) {
			t.Fatalf("порядок нарушен: %q перед %q", all[i-1].Name, all[i].Name)
		}
	}
	// Татарстан ищут на «Т», а не среди двух десятков «Республика …».
	if sortKey("Республика Татарстан") != sortKey("Татарстан") {
		t.Error("префикс «Республика» не должен влиять на порядок")
	}
}

func TestAll_ReturnsCopy(t *testing.T) {
	a := All()
	a[0].Name = "испорчено"
	if All()[0].Name == "испорчено" {
		t.Fatal("All должен отдавать копию, а не внутренний срез")
	}
}

func TestByCode(t *testing.T) {
	cases := map[string]string{
		"16": "Республика Татарстан",
		"77": "Москва",
		"78": "Санкт-Петербург",
		"02": "Республика Башкортостан",
	}
	for code, name := range cases {
		r, ok := ByCode(code)
		if !ok || r.Name != name {
			t.Errorf("ByCode(%q) = %q, %v; ждали %q", code, r.Name, ok, name)
		}
	}
	if _, ok := ByCode("00"); ok {
		t.Error("несуществующий код должен давать ok=false")
	}
	if _, ok := ByCode("16 "); ok {
		t.Error("код сравнивается строго")
	}
}

func TestTimeZonesOfRegionsOffMoscowTime(t *testing.T) {
	cases := map[string]string{
		"39": "Europe/Kaliningrad",
		"63": "Europe/Samara",
		"16": "Europe/Moscow",
		"66": "Asia/Yekaterinburg",
		"54": "Asia/Novosibirsk",
		"25": "Asia/Vladivostok",
		"41": "Asia/Kamchatka",
	}
	for code, tz := range cases {
		if r, _ := ByCode(code); r.TZ != tz {
			t.Errorf("%s (%s): пояс %q, ждали %q", r.Name, code, r.TZ, tz)
		}
	}
}

func TestNearest(t *testing.T) {
	cases := []struct {
		name     string
		lat, lon float64
		code     string
	}{
		{"Казань", 55.79, 49.11, "16"},
		{"Москва, центр", 55.75, 37.62, "77"},
		{"Санкт-Петербург, центр", 59.94, 30.31, "78"},
		{"Владивосток", 43.12, 131.89, "25"},
		{"Екатеринбург", 56.84, 60.60, "66"},
		{"Новосибирск", 55.03, 82.92, "54"},
	}
	for _, c := range cases {
		if got := Nearest(c.lat, c.lon); got.Code != c.code {
			t.Errorf("%s: Nearest = %s (%s), ждали %s", c.name, got.Code, got.Name, c.code)
		}
	}
	if r := Nearest(43.12, 131.89); r.TZ != "Asia/Vladivostok" {
		t.Errorf("Приморский край должен жить по Asia/Vladivostok, а не %s", r.TZ)
	}
}

func TestDistricts(t *testing.T) {
	ds := Districts()
	if len(ds) != 8 {
		t.Fatalf("федеральных округов 8, получили %d", len(ds))
	}
	// Состав округов по указам о федеральных округах, включая изменения 2023 года.
	want := []int{18, 11, 12, 7, 14, 6, 10, 11}
	total := 0
	for i, d := range ds {
		if d.N != i+1 || d.Name == "" {
			t.Errorf("округ %d: %+v", i+1, d)
		}
		in := InDistrict(d.N)
		if len(in) != want[i] {
			t.Errorf("в округе %q %d субъектов, ждали %d", d.Name, len(in), want[i])
		}
		for j := 1; j < len(in); j++ {
			if sortKey(in[j-1].Name) > sortKey(in[j].Name) {
				t.Errorf("%s: порядок нарушен", d.Name)
			}
		}
		total += len(in)
	}
	if total != 89 {
		t.Fatalf("сумма по округам %d, ждали 89 — у кого-то нет округа", total)
	}
	if len(InDistrict(0)) != 0 || len(InDistrict(9)) != 0 {
		t.Error("несуществующий округ — пустой список")
	}
	tat, _ := ByCode("16")
	if ds[tat.District-1].Name != "Приволжский" {
		t.Errorf("Татарстан в округе %d", tat.District)
	}
}

func TestMetro(t *testing.T) {
	for code, want := range map[string]string{"77": "77,50", "50": "77,50", "78": "78,47", "47": "78,47", "16": "16"} {
		if got := strings.Join(Metro(code), ","); got != want {
			t.Errorf("Metro(%s) = %s, ждали %s", code, got, want)
		}
	}
	m := Metro("77")
	m[0] = "xx"
	if Metro("77")[0] != "77" {
		t.Error("Metro должен возвращать копию")
	}
}
