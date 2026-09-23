// Package refdata — справочник субъектов РФ: код, название, часовой пояс,
// координаты административного центра и федеральный округ.
//
// Таблицы в базе под это нет сознательно: на trajectories.region_code нет
// внешнего ключа, пояс траектории денормализован в trajectories.tz, а поиск
// региона по координатам (F6) всё равно живёт только в Go. Справочник
// статичен и встроен в бинарь.
//
// Коды — двузначные коды субъектов (как в кодах ФНС): Татарстан — "16",
// Москва — "77". Их же ждёт фронт.
package refdata

import (
	_ "embed"
	"encoding/json"
	"math"
	"sort"
	"strings"

	// База часовых поясов встраивается в бинарь (~450 КБ): в рантайме Yandex
	// Cloud Functions системного tzdata может не оказаться, и тогда
	// time.LoadLocation("Asia/Vladivostok") вернёт ошибку, а напоминания
	// уйдут не в 10:00 по местному времени.
	_ "time/tzdata"
)

// Region — субъект РФ.
type Region struct {
	Code string `json:"code"`
	Name string `json:"name"`
	// TZ — пояс IANA административного центра. Напоминания ставятся в 10:00 по нему.
	TZ string `json:"tz"`
	// Lat, Lon — административный центр, по нему ищется ближайший регион.
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	// District — номер федерального округа из Districts().
	District int `json:"district"`
}

// District — федеральный округ. Нужен боту: 89 кнопок в одну клавиатуру не
// помещаются, поэтому регион выбирается в два шага — округ, потом субъект.
type District struct {
	N    int    `json:"n"`
	Name string `json:"name"`
}

//go:embed regions.json
var regionsJSON []byte

var (
	regions   []Region // отсортированы по sortKey
	districts []District
	byCode    map[string]Region
)

func init() {
	var file struct {
		Districts []District `json:"districts"`
		Regions   []Region   `json:"regions"`
	}
	if err := json.Unmarshal(regionsJSON, &file); err != nil {
		panic("refdata: битый regions.json: " + err.Error())
	}
	districts = file.Districts
	regions = file.Regions
	sort.SliceStable(regions, func(i, j int) bool {
		return sortKey(regions[i].Name) < sortKey(regions[j].Name)
	})
	byCode = make(map[string]Region, len(regions))
	for _, r := range regions {
		byCode[r.Code] = r
	}
}

// sortKey — ключ сортировки названий. «Республика Татарстан» стоит на «Т»:
// так её ищут глазами, а не среди двух десятков «Республика …». «Ё»
// приравнивается к «Е», как в словарях.
func sortKey(name string) string {
	k := strings.ToLower(strings.TrimPrefix(name, "Республика "))
	return strings.ReplaceAll(k, "ё", "е")
}

// All — все субъекты, по алфавиту. Возвращает копию.
func All() []Region {
	return append([]Region(nil), regions...)
}

// ByCode ищет субъект по двузначному коду.
func ByCode(code string) (Region, bool) {
	r, ok := byCode[code]
	return r, ok
}

// Districts — федеральные округа по порядку номеров.
func Districts() []District {
	return append([]District(nil), districts...)
}

// InDistrict — субъекты округа n по алфавиту; для неизвестного n — пусто.
func InDistrict(n int) []Region {
	var out []Region
	for _, r := range regions {
		if r.District == n {
			out = append(out, r)
		}
	}
	return out
}

// metro — город федерального значения и область вокруг него: МФТИ в
// Долгопрудном для школьника — московский вуз.
var metro = map[string][]string{"77": {"77", "50"}, "50": {"77", "50"}, "78": {"78", "47"}, "47": {"78", "47"}}

// Metro — субъекты, вузы которых подходят тому, кто хочет учиться в code:
// Москва вместе с областью, Петербург — с Ленинградской, остальные — сами по себе.
func Metro(code string) []string {
	if m, ok := metro[code]; ok {
		return append([]string(nil), m...)
	}
	return []string{code}
}

// Nearest — субъект с ближайшим к точке административным центром. Это
// приближение: у крупного региона окраина может оказаться ближе к центру
// соседа. Поэтому бот не записывает регион молча, а показывает найденный и
// просит подтвердить.
func Nearest(lat, lon float64) Region {
	best, bestDist := regions[0], math.Inf(1)
	for _, r := range regions {
		if d := haversine(lat, lon, r.Lat, r.Lon); d < bestDist {
			best, bestDist = r, d
		}
	}
	return best
}

// haversine — расстояние по дуге большого круга, км.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(a))
}
