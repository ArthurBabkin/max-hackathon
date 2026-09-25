package bot

import (
	"slices"
	"strconv"

	"github.com/ArthurBabkin/max-hackathon/packages/core/refdata"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// defaultPlaces — варианты «Где учиться» (SPEC 7): «Только <город>», если
// город известен; «Весь регион · <регион>», если это не Москва или
// Петербург; Москва и Петербург, если их не покрывает свой регион
// (Московская область включает Москву, Ленинградская — Петербург).
// Регион «не важен» — только Москва и Петербург.
func defaultPlaces(dr store.Draft) []store.PlaceOption {
	own := dr.RegionCode
	var out []store.PlaceOption
	if dr.HomeCity != "" && !refdata.Federal(own) {
		out = append(out, store.PlaceOption{Kind: "city", RegionCode: own, City: dr.HomeCity, Label: dr.HomeCity})
	}
	if own != "" && own != "77" && own != "78" {
		out = append(out, store.PlaceOption{Kind: "region", RegionCode: own, Label: refdata.Short(own)})
	}
	for _, code := range []string{"77", "78"} {
		if code == own || !slices.Contains(refdata.Metro(own), code) {
			out = append(out, store.PlaceOption{Kind: "region", RegionCode: code, Label: refdata.Short(code)})
		}
	}
	return out
}

// placeOptions — варианты на кнопках шага target. У диалогов, начатых до
// онбординга v2, их нет в черновике — собираются так же, как при переходе.
func placeOptions(dr store.Draft) []store.PlaceOption {
	if len(dr.PlaceOptions) > 0 {
		return dr.PlaceOptions
	}
	return defaultPlaces(dr)
}

// placeButton — подпись варианта: «Только Казань», «Весь регион · Татарстан»,
// «Москва»; добавленные текстом — как нашёл поиск.
func placeButton(v voice.Voice, dr store.Draft, o store.PlaceOption) string {
	switch {
	case o.Kind == "city" && o.City == dr.HomeCity && o.RegionCode == dr.RegionCode:
		return v.T("bot.target.onlyCity", voice.Vars{"place": o.City})
	case o.Kind == "region" && o.RegionCode == dr.RegionCode && !refdata.Federal(o.RegionCode):
		return v.T("bot.target.wholeRegion", voice.Vars{"region": refdata.Short(o.RegionCode)})
	}
	return o.Label
}

// knowsCity — у ученика есть «свой город», кроме самого региона.
func knowsCity(dr store.Draft) bool { return dr.HomeCity != "" || refdata.Federal(dr.RegionCode) }

// targetPrompt — «Где хочешь учиться?» (W1–W3): мультивыбор мест. text —
// вместо вопроса, например «Добавил Казань. Что-то ещё?».
func targetPrompt(v voice.Voice, d store.Dialog, text string) maxapi.NewMessage {
	cb := maxapi.CallbackButton
	if text == "" {
		text = v.T("bot.target.ask", nil)
		if !knowsCity(d.Draft) {
			text += " " + v.T("bot.target.cityHint", nil)
		}
	}
	var kb maxapi.Keyboard
	for i, o := range placeOptions(d.Draft) {
		kb = append(kb, maxapi.Row(cb(check(slices.Contains(d.Draft.Places, o.Place()), placeButton(v, d.Draft, o)),
			"place:t:"+strconv.Itoa(i))))
	}
	kb = append(kb,
		maxapi.Row(cb(v.T("bot.target.other", nil), "place:other"), cb(v.T("bot.target.any", nil), "place:any")),
		maxapi.Row(cb(v.T("bot.btn.done", nil), "place:done")))
	return maxapi.WithKeyboard(text, kb)
}

// addPlace добавляет место в варианты и в выбранное.
func addPlace(dr *store.Draft, o store.PlaceOption) {
	dr.PlaceOptions = placeOptions(*dr)
	if !slices.ContainsFunc(dr.PlaceOptions, func(x store.PlaceOption) bool { return x.Place() == o.Place() }) {
		dr.PlaceOptions = append(dr.PlaceOptions, o)
	}
	if !slices.Contains(dr.Places, o.Place()) {
		dr.Places = append(dr.Places, o.Place())
	}
	dr.PlacesAny = false
}

// draftPlaces — места для траектории и подборки вузов. У диалогов v1
// вместо мест — Target. ok = false — место ещё не выбрано.
func draftPlaces(dr store.Draft) ([]store.Place, bool) {
	switch {
	case dr.PlacesAny || dr.Target == store.TargetAny:
		return nil, true
	case len(dr.Places) > 0:
		return dr.Places, true
	case dr.Target != "":
		return []store.Place{{RegionCode: dr.Target}}, true
	}
	return nil, false
}

// toUniversities — к шагу вузов с подборкой по местам.
func toUniversities(t *turn, tx *store.Store, d *store.Dialog) error {
	d.Step, d.Draft.Found = stepUniversities, nil
	d.Draft.OfferPage, d.Draft.VuzAnywhere = 0, false
	return offer(t, tx, d)
}

func (b *Bot) placeCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, action, arg string) error {
	v := b.dialogVoiceOf(t)
	switch action {
	case "t", "f":
		i, err := strconv.Atoi(arg)
		if err != nil {
			return b.stale(t, cb, nil)
		}
		return b.transition(t, cb, question, "", func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepTarget); err != nil {
				return err
			}
			if action == "f" {
				if i < 0 || i >= len(d.Draft.Found) {
					return store.ErrStale
				}
				addPlace(&d.Draft, d.Draft.Found[i])
				d.Draft.Found = nil
				return nil
			}
			opts := placeOptions(d.Draft)
			if i < 0 || i >= len(opts) {
				return store.ErrStale
			}
			d.Draft.PlaceOptions, d.Draft.PlacesAny = opts, false
			p := opts[i].Place()
			if j := slices.Index(d.Draft.Places, p); j >= 0 {
				d.Draft.Places = slices.Delete(d.Draft.Places, j, j+1)
			} else {
				d.Draft.Places = append(d.Draft.Places, p)
			}
			return nil
		})
	case "other":
		d, err := b.store.Dialog(t.ctx, t.userID)
		if err != nil || d.Step != stepTarget {
			return b.staleDialog(t, cb)
		}
		if err := b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{}); err != nil {
			return err
		}
		return b.say(t, v.T("bot.target.otherHint", nil))
	case "any":
		return b.transition(t, cb, question, v.T("bot.target.any", nil), func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepTarget); err != nil {
				return err
			}
			d.Draft.Places, d.Draft.PlacesAny = nil, true
			return toUniversities(t, tx, d)
		})
	case "done":
		return b.transition(t, cb, question, v.T("bot.btn.done", nil), func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepTarget); err != nil {
				return err
			}
			if len(d.Draft.Places) == 0 {
				return errNotNow{"bot.target.need"}
			}
			return toUniversities(t, tx, d)
		})
	}
	return b.stale(t, cb, nil)
}

// targetText — город или регион текстом на шаге «Где учиться» (W2):
// одно совпадение добавляется с «✓», несколько — кнопки выбора.
func (b *Bot) targetText(t *turn, d store.Dialog, text string) error {
	v := dialogVoice(t, d)
	matches := refdata.Search(text, 0)
	if len(matches) == 0 {
		return b.say(t, v.T("bot.target.notFound", voice.Vars{"query": text}))
	}
	var found []store.PlaceOption
	for _, m := range matches {
		o := placeOption(m)
		if m.Kind == "region" && !refdata.Federal(m.RegionCode) {
			o.Label = v.T("bot.target.wholeRegion", voice.Vars{"region": refdata.Short(m.RegionCode)})
		}
		found = append(found, o)
	}
	single := len(matches) == 1 && !matches[0].Fuzzy
	d, err := b.updateDialog(t, func(_ *store.Store, d *store.Dialog) error {
		if err := expect(d, stepTarget); err != nil {
			return err
		}
		if single {
			addPlace(&d.Draft, found[0])
			d.Draft.Found = nil
		} else {
			d.Draft.Found = found
		}
		return nil
	})
	if err != nil {
		return err
	}
	if single {
		place := matches[0].Name
		if matches[0].Kind == "region" {
			place = refdata.Short(matches[0].RegionCode)
		}
		return b.sendPrompt(t, withStep(v, d, targetPrompt(v, d, v.T("bot.target.added", voice.Vars{"place": place}))))
	}
	var kb maxapi.Keyboard
	for i, o := range found {
		kb = append(kb, maxapi.Row(maxapi.CallbackButton(o.Label, "place:f:"+strconv.Itoa(i))))
	}
	_, err = b.send(t, maxapi.WithKeyboard(v.T("bot.target.many", nil), kb))
	return err
}
