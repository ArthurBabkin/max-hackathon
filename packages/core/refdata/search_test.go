package refdata

import (
	"testing"
	"time"
)

func TestSearch(t *testing.T) {
	type want struct {
		first Match
		n     int // 0 — не проверять число
	}
	cases := []struct {
		q    string
		want *want // nil — пусто
	}{
		{"казань", &want{Match{Kind: "city", RegionCode: "16", City: "Казань", Name: "Казань"}, 1}},
		{"Казнь", &want{Match{Kind: "city", RegionCode: "16", City: "Казань", Name: "Казань", Fuzzy: true}, 0}},
		{"питер", &want{Match{Kind: "region", RegionCode: "78", Name: "Санкт-Петербург"}, 1}},
		{"г. Москва", &want{Match{Kind: "region", RegionCode: "77", Name: "Москва"}, 1}},
		{"мо", &want{Match{Kind: "region", RegionCode: "50", Name: "Московская область"}, 1}},
		{"советск", &want{Match{Kind: "city", RegionCode: "39", City: "Советск", Name: "Советск"}, 3}},
		{"екб", &want{Match{Kind: "city", RegionCode: "66", City: "Екатеринбург", Name: "Екатеринбург"}, 1}},
		{"югра", &want{Match{Kind: "region", RegionCode: "86", Name: "Ханты-Мансийский автономный округ — Югра"}, 1}},
		{"Татарстан республика", &want{Match{Kind: "region", RegionCode: "16", Name: "Республика Татарстан"}, 1}},
		{"набережные челны", &want{Match{Kind: "city", RegionCode: "16", City: "Набережные Челны", Name: "Набережные Челны"}, 1}},
		{"Набережные  Челны!", &want{Match{Kind: "city", RegionCode: "16", City: "Набережные Челны", Name: "Набережные Челны"}, 1}},
		{"орёл", &want{Match{Kind: "city", RegionCode: "57", City: "Орёл", Name: "Орёл"}, 1}},
		{"орел", &want{Match{Kind: "city", RegionCode: "57", City: "Орёл", Name: "Орёл"}, 1}},
		{"Калининградская обл", &want{Match{Kind: "region", RegionCode: "39", Name: "Калининградская область"}, 1}},
		{"новосиб", &want{Match{Kind: "region", RegionCode: "54", Name: "Новосибирская область"}, 0}},
		{"", nil},
		{"   ", nil},
		{"asdf", nil},
		{"г", nil},
	}
	for _, c := range cases {
		got := Search(c.q, 0)
		if c.want == nil {
			if len(got) != 0 {
				t.Errorf("%q: ждали пусто, нашли %+v", c.q, got)
			}
			continue
		}
		if len(got) == 0 || got[0] != c.want.first {
			t.Errorf("%q: первым ждали %+v, нашли %+v", c.q, c.want.first, got)
			continue
		}
		if c.want.n > 0 && len(got) != c.want.n {
			t.Errorf("%q: ждали %d вариантов, нашли %+v", c.q, c.want.n, got)
		}
	}
}

func TestSearch_RegionsAboveCitiesAndLimit(t *testing.T) {
	got := Search("моск", 0)
	if len(got) == 0 || got[0].Kind != "region" {
		t.Fatalf("регионы выше городов: %+v", got)
	}
	if got := Search("ка", 3); len(got) != 0 {
		t.Fatalf("две буквы — только точное совпадение: %+v", got)
	}
	if got := Search("ово", 4); len(got) != 4 {
		t.Fatalf("лимит: %+v", got)
	}
}

func TestSearch_IsFast(t *testing.T) {
	start := time.Now()
	const runs = 50
	for i := 0; i < runs; i++ {
		Search("Набережные Чолны", 0) // до опечатки проходит все шаги
	}
	if per := time.Since(start) / runs; per > 5*time.Millisecond {
		t.Fatalf("поиск %v, ждали < 5 мс", per)
	}
}

func TestDamerau(t *testing.T) {
	cases := []struct {
		a, b string
		d    int
	}{{"казань", "казань", 0}, {"казнь", "казань", 1}, {"казнаь", "казань", 1}, {"пермь", "перм", 1}, {"абв", "где", 3}}
	for _, c := range cases {
		if got := damerau(c.a, c.b); got != c.d {
			t.Errorf("damerau(%q, %q) = %d, ждали %d", c.a, c.b, got, c.d)
		}
	}
}

func TestNearestCity(t *testing.T) {
	c, km := NearestCity(55.79, 49.11)
	if c.Name != "Казань" || km > CityRadiusKm {
		t.Fatalf("Казань: %+v в %.1f км", c, km)
	}
	// Посреди тайги ближайший город дальше 30 км.
	if _, km := NearestCity(63.0, 100.0); km < CityRadiusKm {
		t.Fatalf("тайга: %.1f км", km)
	}
}

func TestShortAndLetters(t *testing.T) {
	for code, want := range map[string]string{"16": "Татарстан", "39": "Калининградская обл.", "09": "Карачаево-Черкесия",
		"77": "Москва", "23": "Краснодарский край"} {
		if got := Short(code); got != want {
			t.Errorf("Short(%s) = %q, ждали %q", code, got, want)
		}
	}
	letters := Letters()
	if len(letters) < 15 || letters[0] != "А" {
		t.Fatalf("буквы: %v", letters)
	}
	total := 0
	for _, l := range letters {
		total += len(OnLetter(l))
	}
	if total != 89 {
		t.Fatalf("все регионы на своих буквах: %d", total)
	}
	found := false
	for _, r := range OnLetter("Т") {
		found = found || r.Code == "16"
	}
	if !found {
		t.Fatal("Татарстан на «Т»")
	}
}

func TestLocate(t *testing.T) {
	cases := []struct {
		lat, lon     float64
		region, city string
	}{
		{55.79, 49.11, "16", "Казань"},
		{55.75, 37.62, "77", ""}, // центр Москвы
		{59.94, 30.31, "78", ""}, // Петербург
		{55.03, 82.92, "54", "Новосибирск"},
		{63.0, 100.0, Nearest(63.0, 100.0).Code, ""}, // тайга — только регион
	}
	for _, c := range cases {
		if r, city := Locate(c.lat, c.lon); r != c.region || city != c.city {
			t.Errorf("Locate(%v, %v) = %s %q, ждали %s %q", c.lat, c.lon, r, city, c.region, c.city)
		}
	}
	if !Federal("77") || Federal("16") {
		t.Fatal("Federal")
	}
}
