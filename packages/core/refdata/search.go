package refdata

import (
	_ "embed"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// City — город России из hflabs/city (CC BY-SA 4.0), places.json собирает
// datasets/parser/build_places.py.
type City struct {
	Name       string  `json:"name"`
	RegionCode string  `json:"region_code"`
	Population int     `json:"population"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
}

// Match — найденное место: регион целиком или город в регионе.
type Match struct {
	Kind       string // region | city
	RegionCode string
	City       string // только у Kind = city
	Name       string // название региона или города
	// Fuzzy — найдено по опечатке: бот переспрашивает, а не сохраняет сразу.
	Fuzzy bool
}

//go:embed places.json
var placesJSON []byte

//go:embed aliases.json
var aliasesJSON []byte

// entry — строка поискового словаря: регион, алиас или город.
type entry struct {
	norm  string
	words []string
	match Match
	// rank: 0 — регион или алиас, 1 — город; внутри городов — по населению.
	rank       int
	population int
}

var (
	cities  []City
	entries []entry
)

func init() {
	if err := json.Unmarshal(placesJSON, &cities); err != nil {
		panic("refdata: битый places.json: " + err.Error())
	}
	var aliases map[string]string
	if err := json.Unmarshal(aliasesJSON, &aliases); err != nil {
		panic("refdata: битый aliases.json: " + err.Error())
	}
	add := func(name string, m Match, rank, population int) {
		n := Normalize(name)
		entries = append(entries, entry{norm: n, words: strings.Fields(n), match: m, rank: rank, population: population})
	}
	for _, r := range regions {
		m := Match{Kind: "region", RegionCode: r.Code, Name: r.Name}
		add(r.Name, m, 0, 0)
		if s := Short(r.Code); s != r.Name {
			add(s, m, 0, 0)
		}
	}
	keys := make([]string, 0, len(aliases))
	for k := range aliases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, alias := range keys {
		target := aliases[alias]
		if r, ok := byCode[target]; ok {
			add(alias, Match{Kind: "region", RegionCode: r.Code, Name: r.Name}, 0, 0)
			continue
		}
		c, ok := biggestCity(target)
		if !ok {
			panic("refdata: алиас " + alias + " ведёт в неизвестный город " + target)
		}
		add(alias, Match{Kind: "city", RegionCode: c.RegionCode, City: c.Name, Name: c.Name}, 0, c.Population)
	}
	for _, c := range cities {
		if _, ok := byCode[c.RegionCode]; !ok {
			panic("refdata: город " + c.Name + " в неизвестном регионе " + c.RegionCode)
		}
		add(c.Name, Match{Kind: "city", RegionCode: c.RegionCode, City: c.Name, Name: c.Name}, 1, c.Population)
	}
}

func biggestCity(name string) (City, bool) {
	var best City
	found := false
	for _, c := range cities {
		if c.Name == name && (!found || c.Population > best.Population) {
			best, found = c, true
		}
	}
	return best, found
}

// stopWords — слова, без которых название ищется так же: «г. Казань»,
// «Татарстан республика», «Московская обл».
var stopWords = map[string]bool{"г": true, "город": true, "республика": true, "респ": true, "область": true,
	"обл": true, "край": true, "автономный": true, "автономная": true, "округ": true, "ао": true}

// Normalize приводит название к виду для сравнения: нижний регистр, «ё» как
// «е», пунктуация — пробел, без служебных слов «г», «область», «край»…
func Normalize(s string) string {
	s = strings.ReplaceAll(strings.ToLower(s), "ё", "е")
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s)
	var words []string
	for _, w := range strings.Fields(s) {
		if !stopWords[w] {
			words = append(words, w)
		}
	}
	return strings.Join(words, " ")
}

// SearchLimit — сколько вариантов показывает бот.
const SearchLimit = 6

// Search ищет регион или город по тексту пользователя. Порядок: точное
// совпадение с регионом, алиасом или городом; начало слова (от 3 букв);
// подстрока; опечатка (Дамерау–Левенштейн ≤ 1 для слов 4–7 букв, ≤ 2 для
// 8 и длиннее). Первый шаг, давший результат, — ответ. Регионы и алиасы
// выше городов, города — по населению. limit ≤ 0 — SearchLimit.
func Search(q string, limit int) []Match {
	if limit <= 0 {
		limit = SearchLimit
	}
	q = Normalize(q)
	n := utf8.RuneCountInString(q)
	if n == 0 {
		return nil
	}
	steps := []func(e entry) (int, bool){
		func(e entry) (int, bool) { return 0, e.norm == q },
		func(e entry) (int, bool) {
			return 0, n >= 3 && (strings.HasPrefix(e.norm, q) || strings.Contains(" "+e.norm, " "+q))
		},
		func(e entry) (int, bool) { return 0, n >= 3 && strings.Contains(e.norm, q) },
	}
	for _, step := range steps {
		if out := collectMatches(step, limit, false); len(out) > 0 {
			return out
		}
	}
	maxDist := 0
	switch {
	case n >= 8:
		maxDist = 2
	case n >= 4:
		maxDist = 1
	}
	if maxDist == 0 {
		return nil
	}
	return collectMatches(func(e entry) (int, bool) {
		best := damerau(q, e.norm)
		if len(e.words) > 1 {
			for _, w := range e.words {
				best = min(best, damerau(q, w))
			}
		}
		return best, best <= maxDist
	}, limit, true)
}

func collectMatches(ok func(e entry) (int, bool), limit int, fuzzy bool) []Match {
	type hit struct {
		e    entry
		dist int
	}
	var hits []hit
	for _, e := range entries {
		if d, yes := ok(e); yes {
			hits = append(hits, hit{e, d})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.dist != b.dist {
			return a.dist < b.dist
		}
		if a.e.rank != b.e.rank {
			return a.e.rank < b.e.rank
		}
		return a.e.population > b.e.population
	})
	var out []Match
	seen := map[Match]bool{}
	for _, h := range hits {
		m := h.e.match
		m.Fuzzy = fuzzy
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
		if len(out) == limit {
			break
		}
	}
	return out
}

// damerau — расстояние Дамерау–Левенштейна (с перестановкой соседних букв)
// по рунам.
func damerau(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if d := len(ra) - len(rb); d > 2 || d < -2 {
		return 3 // дальше любого допустимого порога — не считаем
	}
	prev2 := make([]int, len(rb)+1)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1)
			}
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(rb)]
}

// NearestCity — ближайший к точке город и расстояние до него, км. Для
// геолокации город считается известным, если он ближе CityRadiusKm.
func NearestCity(lat, lon float64) (City, float64) {
	best, bestDist := cities[0], math.Inf(1)
	for _, c := range cities {
		if d := haversine(lat, lon, c.Lat, c.Lon); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best, bestDist
}

// CityRadiusKm — ближе этого геолокация считается «в городе».
const CityRadiusKm = 30

// federal — города федерального значения: в places.json их нет, это
// регионы, а центр региона — сам город.
var federal = []string{"77", "78", "92"}

// Federal — code — город федерального значения (Москва, Петербург,
// Севастополь): у него нет отдельного «своего города».
func Federal(code string) bool {
	for _, f := range federal {
		if f == code {
			return true
		}
	}
	return false
}

// Locate — регион и город по геолокации. Город известен, если он ближе
// CityRadiusKm; город федерального значения выигрывает, если его центр
// ближе ближайшего города. Иначе — регион с ближайшим центром, город
// пустой.
func Locate(lat, lon float64) (regionCode, city string) {
	c, km := NearestCity(lat, lon)
	for _, code := range federal {
		r := byCode[code]
		if d := haversine(lat, lon, r.Lat, r.Lon); d < km && d <= CityRadiusKm {
			return code, ""
		}
	}
	if km <= CityRadiusKm {
		return c.RegionCode, c.Name
	}
	return Nearest(lat, lon).Code, ""
}

// CityCoords — координаты города в регионе; ok = false — такого нет.
func CityCoords(name, regionCode string) (lat, lon float64, ok bool) {
	for _, c := range cities {
		if c.Name == name && c.RegionCode == regionCode {
			return c.Lat, c.Lon, true
		}
	}
	return 0, 0, false
}

// Distance — расстояние между точками по дуге большого круга, км.
func Distance(lat1, lon1, lat2, lon2 float64) float64 { return haversine(lat1, lon1, lat2, lon2) }

// shortNames — как регион подписан на кнопке, где полное название не
// помещается. Остальные сокращаются по правилу в Short.
var shortNames = map[string]string{
	"07": "Кабардино-Балкария",
	"09": "Карачаево-Черкесия",
	"14": "Якутия",
	"15": "Северная Осетия",
	"18": "Удмуртия",
	"20": "Чечня",
	"21": "Чувашия",
	"42": "Кузбасс",
	"79": "Еврейская АО",
	"83": "Ненецкий АО",
	"86": "ХМАО — Югра",
	"87": "Чукотка",
	"89": "ЯНАО",
	"93": "ДНР",
	"94": "ЛНР",
}

// Short — короткое название региона для кнопок: «Татарстан»,
// «Калининградская обл.», «Карачаево-Черкесия». Неизвестный код — как есть.
func Short(code string) string {
	if s, ok := shortNames[code]; ok {
		return s
	}
	r, ok := byCode[code]
	if !ok {
		return code
	}
	name := strings.TrimPrefix(r.Name, "Республика ")
	return strings.Replace(name, " область", " обл.", 1)
}

// Letters — первые буквы регионов по алфавиту (как их ищут: «Татарстан» на
// «Т»), для выбора региона в два шага.
func Letters() []string {
	var out []string
	for _, r := range regions {
		l := Letter(r)
		if len(out) == 0 || out[len(out)-1] != l {
			out = append(out, l)
		}
	}
	return out
}

// Letter — буква, на которой регион стоит в алфавите.
func Letter(r Region) string {
	first, _ := utf8.DecodeRuneInString(sortKey(r.Name))
	return strings.ToUpper(string(first))
}

// OnLetter — регионы на букву по алфавиту.
func OnLetter(letter string) []Region {
	var out []Region
	for _, r := range regions {
		if Letter(r) == letter {
			out = append(out, r)
		}
	}
	return out
}
