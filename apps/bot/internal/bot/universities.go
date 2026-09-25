package bot

import (
	"slices"
	"sort"
	"strings"

	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/core/refdata"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// offerLimit — сколько вузов бот предлагает кнопками за раз (F9).
const offerLimit = 6

// nearbyLimit — сколько ближайших вузов предложить, если в выбранных местах
// нет направления (SPEC 8.3).
const nearbyLimit = 3

// similarLimit — сколько похожих направлений предложить там же.
const similarLimit = 2

// universityAliases — как вузы называют в разговоре (SPEC 8.2). Ключи —
// в виде refdata.Normalize.
var universityAliases = map[string]string{
	"вышка": "hse", "вшэ": "hse",
	"мгу":   "msu",
	"спбгу": "spbu", "питерский университет": "spbu",
	"физтех": "mipt", "мфти": "mipt",
	"итмо": "itmo",
	"нгу":  "nsu",
	"кфу":  "kfu", "казанский федеральный": "kfu",
	"иннополис": "innopolis", "уи": "innopolis",
	"сеченовка": "sechenov", "первый мед": "sechenov",
	"кгму": "kazan-gmu", "казанский медицинский": "kazan-gmu",
}

// aliasUniversity — id вуза по народному названию: «вышка», «Казанский
// федеральный университет». "" — такого названия нет.
func aliasUniversity(q string) string {
	q = refdata.Normalize(q)
	if id, ok := universityAliases[q]; ok {
		return id
	}
	for alias, id := range universityAliases {
		if strings.HasPrefix(q, alias+" ") {
			return id
		}
	}
	return ""
}

// uniLabel — как вуз подписан в чате: коротко, но узнаваемо.
func uniLabel(u store.University) string { return notify.UniversityLabel(u.ID, u.ShortName) }

// uniButton — «ВШЭ · Москва» (SPEC 8.1).
func uniButton(u store.University) string {
	if u.City == nil || *u.City == "" || *u.City == uniLabel(u) {
		return uniLabel(u)
	}
	return uniLabel(u) + " · " + *u.City
}

// offer — вузы для кнопок шага «Вузы»: подходящие по местам и направлениям
// (с «Показать ещё» — несколько страниц) плюс уже отмеченные. Если
// подходящих нет, а места и направления выбраны, — ближайшие вузы с
// направлением и похожие направления в выбранных местах (V3).
func offer(t *turn, tx *store.Store, d *store.Dialog) error {
	places, _ := draftPlaces(d.Draft)
	if d.Draft.VuzAnywhere {
		places = nil
	}
	shown := (d.Draft.OfferPage + 1) * offerLimit
	unis, err := tx.SuggestUniversities(t.ctx, d.Draft.DirectionIDs, places, shown+1, 0)
	if err != nil {
		return err
	}
	d.Draft.OfferMore = len(unis) > shown
	d.Draft.Offered, d.Draft.Nearby, d.Draft.Similar = nil, nil, nil
	for _, u := range unis[:min(len(unis), shown)] {
		d.Draft.Offered = append(d.Draft.Offered, u.ID)
	}
	if len(unis) == 0 && len(places) > 0 && len(d.Draft.DirectionIDs) > 0 {
		if err := noDirectionHere(t, tx, d, places); err != nil {
			return err
		}
	}
	for _, id := range d.Draft.UniversityIDs {
		if !slices.Contains(d.Draft.Offered, id) {
			d.Draft.Offered = append(d.Draft.Offered, id)
		}
	}
	return nil
}

// noDirectionHere — V3: ближайшие к ученику вузы с направлением (по
// центрам регионов) и направления с общими ключевыми предметами, которые
// в выбранных местах есть.
func noDirectionHere(t *turn, tx *store.Store, d *store.Dialog, places []store.Place) error {
	all, err := tx.SuggestUniversities(t.ctx, d.Draft.DirectionIDs, nil, 100, 0)
	if err != nil {
		return err
	}
	home, _ := refdata.ByCode(d.Draft.RegionCode)
	lat, lon := home.Lat, home.Lon
	if la, lo, ok := refdata.CityCoords(d.Draft.HomeCity, d.Draft.RegionCode); ok {
		lat, lon = la, lo
	}
	dist := func(u store.University) float64 {
		r, ok := refdata.ByCode(u.RegionCode)
		if !ok {
			return 1e9
		}
		return refdata.Distance(lat, lon, r.Lat, r.Lon)
	}
	sort.SliceStable(all, func(i, j int) bool { return dist(all[i]) < dist(all[j]) })
	for _, u := range all[:min(len(all), nearbyLimit)] {
		d.Draft.Nearby = append(d.Draft.Nearby, u.ID)
	}
	here, err := tx.DirectionsIn(t.ctx, places)
	if err != nil {
		return err
	}
	dirs, err := tx.Directions(t.ctx)
	if err != nil {
		return err
	}
	d.Draft.Similar = similarDirections(dirs, d.Draft.DirectionIDs, here, similarLimit)
	return nil
}

// similarDirections — до limit направлений из here, не выбранных, у которых
// с каким-нибудь выбранным не меньше двух общих ключевых предметов: одной
// математики мало, она есть почти везде. Порядок справочника.
func similarDirections(dirs []store.Direction, chosen, here []string, limit int) []string {
	byID := map[string]store.Direction{}
	for _, d := range dirs {
		byID[d.ID] = d
	}
	var out []string
	for _, d := range dirs {
		if len(out) == limit {
			break
		}
		if !slices.Contains(here, d.ID) || slices.Contains(chosen, d.ID) {
			continue
		}
		for _, id := range chosen {
			common := 0
			for _, c := range byID[id].SubjectCodes {
				if slices.Contains(d.SubjectCodes, c) {
					common++
				}
			}
			if common >= 2 {
				out = append(out, d.ID)
				break
			}
		}
	}
	return out
}

// directionNames — названия направлений по id через запятую, со строчной.
func directionNames(dirs []store.Direction, ids []string) string {
	var names []string
	for _, id := range ids {
		if i := slices.IndexFunc(dirs, func(x store.Direction) bool { return x.ID == id }); i >= 0 {
			names = append(names, lowerFirst(dirs[i].Name))
		}
	}
	return strings.Join(names, ", ")
}

// universitiesPrompt — подборка вузов с «✓» (V1) или, если в выбранных
// местах направления нет, ближайшие вузы с ним (V3). Выбирать не
// обязательно: без вузов подбор покажет льготы в вузах с направлением.
func (b *Bot) universitiesPrompt(t *turn, v voice.Voice, d store.Dialog) (maxapi.NewMessage, error) {
	cb := maxapi.CallbackButton
	unis, err := b.store.FindUniversities(t.ctx, "")
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	dirs, err := b.store.Directions(t.ctx)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	byID := func(id string) (store.University, bool) {
		i := slices.IndexFunc(unis, func(u store.University) bool { return u.ID == id })
		if i < 0 {
			return store.University{}, false
		}
		return unis[i], true
	}
	var buttons []maxapi.Button
	for _, id := range d.Draft.Offered {
		if u, ok := byID(id); ok {
			buttons = append(buttons, cb(check(slices.Contains(d.Draft.UniversityIDs, id), uniButton(u)), "vuz:t:"+id))
		}
	}
	kb := grid(buttons, 2)
	finish := v.T("bot.vuz.skip", nil)
	if len(d.Draft.UniversityIDs) > 0 {
		finish = v.T("bot.btn.done", nil)
	}
	if len(d.Draft.Nearby) > 0 {
		text := v.T("bot.vuz.noDirections", nil)
		allDir := "bot.vuz.allDirs"
		if len(d.Draft.DirectionIDs) == 1 {
			text = v.T("bot.vuz.noDirection", voice.Vars{"direction": directionTitle(dirs, d.Draft.DirectionIDs[0])})
			allDir = "bot.vuz.allDir"
		}
		for _, id := range d.Draft.Nearby {
			if u, ok := byID(id); ok && !slices.Contains(d.Draft.Offered, id) {
				kb = append(kb, maxapi.Row(cb("+ "+uniButton(u), "vuz:add:"+id)))
			}
		}
		kb = append(kb, maxapi.Row(cb(v.T(allDir, nil), "vuz:alldir")))
		if len(d.Draft.Similar) > 0 {
			kb = append(kb, maxapi.Row(cb(v.T("bot.vuz.similar", voice.Vars{"directions": directionNames(dirs, d.Draft.Similar)}), "vuz:similar")))
		}
		if len(d.Draft.UniversityIDs) == 0 {
			finish = v.T("bot.vuz.skipAll", nil)
		}
		kb = append(kb, maxapi.Row(cb(finish, "vuz:done")))
		return maxapi.WithKeyboard(text, kb), nil
	}
	text := v.T("bot.vuz.askAny", nil)
	if len(d.Draft.DirectionIDs) > 0 {
		text = v.T("bot.vuz.ask", voice.Vars{"directions": directionNames(dirs, d.Draft.DirectionIDs)})
	}
	if len(buttons) == 0 {
		text = v.T("bot.vuz.none", nil)
	}
	if d.Draft.OfferMore {
		kb = append(kb, maxapi.Row(cb(v.T("bot.vuz.more", nil), "vuz:more")))
	}
	kb = append(kb, maxapi.Row(cb(v.T("bot.vuz.places", nil), "vuz:places"), cb(finish, "vuz:done")))
	return maxapi.WithKeyboard(text, kb), nil
}

func directionTitle(dirs []store.Direction, id string) string {
	if i := slices.IndexFunc(dirs, func(x store.Direction) bool { return x.ID == id }); i >= 0 {
		return dirs[i].Name
	}
	return id
}

func (b *Bot) universityCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, action, id string) error {
	v := b.dialogVoiceOf(t)
	onStep := func(fn func(tx *store.Store, d *store.Dialog) error) func(tx *store.Store, d *store.Dialog) error {
		return func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepUniversities, stepUniSearch); err != nil {
				return err
			}
			d.Step = stepUniversities
			return fn(tx, d)
		}
	}
	switch action {
	case "t":
		return b.transition(t, cb, question, "", onStep(func(_ *store.Store, d *store.Dialog) error {
			if !slices.Contains(d.Draft.Offered, id) {
				return store.ErrStale
			}
			d.Draft.UniversityIDs = toggle(d.Draft.UniversityIDs, id)
			return nil
		}))
	case "more":
		return b.transition(t, cb, question, "", onStep(func(tx *store.Store, d *store.Dialog) error {
			if !d.Draft.OfferMore {
				return store.ErrStale
			}
			d.Draft.OfferPage++
			return offer(t, tx, d)
		}))
	case "places":
		return b.transition(t, cb, question, v.T("bot.vuz.places", nil), onStep(func(_ *store.Store, d *store.Dialog) error {
			d.Step, d.Draft.PlacesAny, d.Draft.Found = stepTarget, false, nil
			d.Draft.PlaceOptions = placeOptions(d.Draft)
			if places, _ := draftPlaces(d.Draft); len(d.Draft.Places) == 0 {
				// Диалог v1: место было в Target.
				d.Draft.Places = places
			}
			d.Draft.Target = ""
			return nil
		}))
	case "add":
		return b.transition(t, cb, question, "", onStep(func(tx *store.Store, d *store.Dialog) error {
			if !slices.Contains(d.Draft.Nearby, id) {
				return store.ErrStale
			}
			unis, err := tx.FindUniversities(t.ctx, "")
			if err != nil {
				return err
			}
			i := slices.IndexFunc(unis, func(u store.University) bool { return u.ID == id })
			if i < 0 || unis[i].RegionCode == "" {
				return store.ErrStale
			}
			// Место вуза добавляется к выбранным: подборка покажет и его.
			o := store.PlaceOption{Kind: "region", RegionCode: unis[i].RegionCode, Label: refdata.Short(unis[i].RegionCode)}
			if c := unis[i].City; c != nil && *c != "" && !refdata.Federal(unis[i].RegionCode) {
				o = store.PlaceOption{Kind: "city", RegionCode: unis[i].RegionCode, City: *c, Label: *c + " · " + refdata.Short(unis[i].RegionCode)}
			}
			if places, _ := draftPlaces(d.Draft); len(d.Draft.Places) == 0 {
				d.Draft.Places = places
			}
			d.Draft.Target = ""
			addPlace(&d.Draft, o)
			if !slices.Contains(d.Draft.UniversityIDs, id) {
				d.Draft.UniversityIDs = append(d.Draft.UniversityIDs, id)
			}
			return offer(t, tx, d)
		}))
	case "alldir":
		return b.transition(t, cb, question, "", onStep(func(tx *store.Store, d *store.Dialog) error {
			d.Draft.VuzAnywhere, d.Draft.OfferPage = true, 0
			return offer(t, tx, d)
		}))
	case "similar":
		return b.transition(t, cb, question, "", onStep(func(tx *store.Store, d *store.Dialog) error {
			if len(d.Draft.Similar) == 0 {
				return store.ErrStale
			}
			for _, s := range d.Draft.Similar {
				if !slices.Contains(d.Draft.DirectionIDs, s) {
					d.Draft.DirectionIDs = append(d.Draft.DirectionIDs, s)
				}
			}
			return offer(t, tx, d)
		}))
	case "done":
		label := v.T("bot.btn.done", nil)
		return b.transition(t, cb, question, label, onStep(func(tx *store.Store, d *store.Dialog) error {
			return b.createTrajectory(t, tx, d)
		}))
	case "other", "city":
		// Кнопки шага вузов из v1.
		return b.staleDialog(t, cb)
	}
	return b.stale(t, cb, nil)
}

// universityText — поиск вуза по названию и народным именам (V2).
// Найденный вуз появляется на кнопках, до трёх найденных — сразу с «✓».
func (b *Bot) universityText(t *turn, d store.Dialog, text string) error {
	v := dialogVoice(t, d)
	q := strings.TrimSpace(text)
	var found []store.University
	all, err := b.store.FindUniversities(t.ctx, "")
	if err != nil {
		return err
	}
	if id := aliasUniversity(q); id != "" {
		if i := slices.IndexFunc(all, func(u store.University) bool { return u.ID == id }); i >= 0 {
			found = append(found, all[i])
		}
	}
	if len(found) == 0 && q != "" {
		if found, err = b.store.FindUniversities(t.ctx, q); err != nil {
			return err
		}
	}
	d, err = b.store.UpdateDialog(t.ctx, t.userID, func(_ *store.Store, d *store.Dialog) error {
		if err := expect(d, stepUniversities, stepUniSearch); err != nil {
			return err
		}
		d.Step = stepUniversities
		for i, u := range found {
			if i < offerLimit && !slices.Contains(d.Draft.Offered, u.ID) {
				d.Draft.Offered = append(d.Draft.Offered, u.ID)
			}
			if len(found) <= 3 && !slices.Contains(d.Draft.UniversityIDs, u.ID) {
				d.Draft.UniversityIDs = append(d.Draft.UniversityIDs, u.ID)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	switch {
	case len(found) == 0:
		err = b.say(t, v.T("bot.vuz.notFound", voice.Vars{"query": q}))
	case len(found) <= 3:
		names := make([]string, len(found))
		for i, u := range found {
			names[i] = uniLabel(u)
		}
		err = b.say(t, v.T("bot.vuz.added", voice.Vars{"uni": strings.Join(names, ", ")}))
	}
	if err != nil {
		return err
	}
	return b.ask(t, d)
}
