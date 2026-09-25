package bot

import (
	"errors"
	"slices"
	"strconv"

	"github.com/ArthurBabkin/max-hackathon/packages/core/refdata"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// lettersOpen — в черновике Letter = "*": открыт список букв алфавита.
const lettersOpen = "*"

// regionPrompt — шаг региона (SPEC 3.2): вопрос с быстрыми кнопками (R1),
// буквы алфавита или регионы на букву (R5).
func regionPrompt(v voice.Voice, d store.Dialog) maxapi.NewMessage {
	cb := maxapi.CallbackButton
	hint := "\n" + v.T("bot.region.typeHint", nil)
	switch d.Draft.Letter {
	case "":
	case lettersOpen:
		var buttons []maxapi.Button
		for _, l := range refdata.Letters() {
			buttons = append(buttons, cb(l, "region:l:"+l))
		}
		return maxapi.WithKeyboard(v.T("bot.region.letter", nil)+hint, grid(buttons, 5))
	default:
		var buttons []maxapi.Button
		for _, r := range refdata.OnLetter(d.Draft.Letter) {
			buttons = append(buttons, cb(refdata.Short(r.Code), "region:"+r.Code))
		}
		kb := append(grid(buttons, 2), maxapi.Row(cb(v.T("bot.region.otherLetter", nil), "region:abc")))
		return maxapi.WithKeyboard(v.T("bot.region.onLetter", voice.Vars{"letter": d.Draft.Letter})+hint, kb)
	}
	return maxapi.WithKeyboard(v.T("bot.region.ask", nil), maxapi.Keyboard{
		maxapi.Row(maxapi.GeoButton(v.T("bot.region.geo", nil))),
		maxapi.Row(cb(refdata.Short("77"), "region:77"), cb(refdata.Short("78"), "region:78")),
		maxapi.Row(cb(refdata.Short("50"), "region:50"), cb(v.T("bot.region.alphabet", nil), "region:abc")),
	})
}

// placeOption — найденное место как кнопка: город с кратким регионом
// («Советск · Калининградская обл.») или регион.
func placeOption(m refdata.Match) store.PlaceOption {
	o := store.PlaceOption{Kind: m.Kind, RegionCode: m.RegionCode, City: m.City, Label: m.Name}
	if m.Kind == "city" {
		o.Label = m.City + " · " + refdata.Short(m.RegionCode)
	}
	return o
}

// foundMessage — «Казань → Татарстан ✓» с кнопкой «Не то, выбрать другой».
func foundMessage(v voice.Voice, regionCode, city string) maxapi.NewMessage {
	text := v.T("bot.region.foundRegion", voice.Vars{"region": regionName(regionCode)})
	if city != "" {
		text = v.T("bot.region.found", voice.Vars{"place": city, "region": refdata.Short(regionCode)})
	}
	return maxapi.WithKeyboard(text, maxapi.Keyboard{maxapi.Row(
		maxapi.CallbackButton(v.T("bot.region.change", nil), "region:change"))})
}

func regionName(code string) string {
	if r, ok := refdata.ByCode(code); ok {
		return r.Name
	}
	return code
}

// setRegion — регион выбран: дальше предметы.
func setRegion(d *store.Dialog, code, city string) {
	d.Draft.RegionCode, d.Draft.HomeCity = code, city
	d.Draft.Letter, d.Draft.PlaceOptions, d.Draft.District = "", nil, 0
	d.Step = stepSubjects
}

func (b *Bot) regionCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, args []string) error {
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return ""
	}
	v := b.dialogVoiceOf(t)
	switch arg(0) {
	case "abc", "retry":
		// Алфавит или «Написать заново»: тот же шаг, вопрос на месте.
		letter := lettersOpen
		if arg(0) == "retry" {
			letter = ""
		}
		return b.transition(t, cb, question, "", func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepRegion); err != nil {
				return err
			}
			d.Draft.Letter, d.Draft.PlaceOptions = letter, nil
			return nil
		})
	case "l":
		letter := arg(1)
		if !slices.Contains(refdata.Letters(), letter) {
			return b.stale(t, cb, nil)
		}
		return b.transition(t, cb, question, "", func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepRegion); err != nil {
				return err
			}
			d.Draft.Letter = letter
			return nil
		})
	case "o":
		i, err := strconv.Atoi(arg(1))
		if err != nil {
			return b.stale(t, cb, nil)
		}
		var picked store.PlaceOption
		label := ""
		if d, err := b.store.Dialog(t.ctx, t.userID); err == nil && i >= 0 && i < len(d.Draft.PlaceOptions) {
			label = d.Draft.PlaceOptions[i].Label
		}
		return b.transitionSay(t, cb, question, label, func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepRegion); err != nil {
				return err
			}
			if i < 0 || i >= len(d.Draft.PlaceOptions) {
				return store.ErrStale
			}
			picked = d.Draft.PlaceOptions[i]
			setRegion(d, picked.RegionCode, picked.City)
			return nil
		}, func(d store.Dialog) error {
			_, err := b.send(t, foundMessage(dialogVoice(t, d), picked.RegionCode, picked.City))
			return err
		})
	case "change":
		// «Не то»: пока предметы не отмечены — назад к региону, позже — stale.
		return b.transition(t, cb, question, v.T("bot.region.change", nil), func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepSubjects); err != nil || len(d.Draft.SubjectCodes) > 0 {
				return store.ErrStale
			}
			d.Draft.RegionCode, d.Draft.HomeCity, d.Draft.Letter, d.Step = "", "", "", stepRegion
			return nil
		})
	}
	reg, ok := refdata.ByCode(arg(0))
	if !ok {
		return b.stale(t, cb, nil)
	}
	return b.transition(t, cb, question, reg.Name, func(_ *store.Store, d *store.Dialog) error {
		if err := expect(d, stepRegion); err != nil {
			return err
		}
		setRegion(d, reg.Code, "")
		return nil
	})
}

// regionText — город или регион текстом (SPEC 3.2): одно точное совпадение
// сохраняется сразу, несколько — кнопки выбора, опечатка — переспрос.
func (b *Bot) regionText(t *turn, d store.Dialog, text string) error {
	v := dialogVoice(t, d)
	cb := maxapi.CallbackButton
	matches := refdata.Search(text, 0)
	if len(matches) == 1 && !matches[0].Fuzzy {
		m := matches[0]
		d, err := b.store.UpdateDialog(t.ctx, t.userID, func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepRegion); err != nil {
				return err
			}
			setRegion(d, m.RegionCode, m.City)
			return nil
		})
		if err != nil {
			return err
		}
		if _, err := b.send(t, foundMessage(v, m.RegionCode, m.City)); err != nil {
			return err
		}
		return b.ask(t, d)
	}
	var options []store.PlaceOption
	for _, m := range matches {
		options = append(options, placeOption(m))
	}
	if len(matches) > 0 && matches[0].Fuzzy {
		options = options[:1] // переспрашиваем про лучший вариант
	}
	if _, err := b.store.UpdateDialog(t.ctx, t.userID, func(_ *store.Store, d *store.Dialog) error {
		if err := expect(d, stepRegion); err != nil {
			return err
		}
		d.Draft.PlaceOptions, d.Draft.Letter = options, ""
		return nil
	}); err != nil {
		return err
	}
	alphabet := cb(v.T("bot.region.alphabet", nil), "region:abc")
	var msg maxapi.NewMessage
	switch {
	case len(matches) == 0:
		msg = maxapi.WithKeyboard(v.T("bot.region.none", voice.Vars{"query": text}), maxapi.Keyboard{maxapi.Row(alphabet)})
	case matches[0].Fuzzy:
		m := matches[0]
		ask := v.T("bot.region.maybeRegion", voice.Vars{"query": text, "region": m.Name})
		if m.Kind == "city" {
			ask = v.T("bot.region.maybe", voice.Vars{"query": text, "name": m.City, "region": refdata.Short(m.RegionCode)})
		}
		msg = maxapi.WithKeyboard(ask, maxapi.Keyboard{
			maxapi.Row(cb(v.T("bot.region.maybeYes", voice.Vars{"name": m.Name}), "region:o:0")),
			maxapi.Row(cb(v.T("bot.region.retry", nil), "region:retry")),
			maxapi.Row(alphabet),
		})
	default:
		var kb maxapi.Keyboard
		for i, o := range options {
			kb = append(kb, maxapi.Row(cb(o.Label, "region:o:"+strconv.Itoa(i))))
		}
		kb = append(kb, maxapi.Row(cb(v.T("bot.region.notMine", nil), "region:abc")))
		msg = maxapi.WithKeyboard(v.T("bot.region.many", nil), kb)
	}
	return b.sendPrompt(t, msg)
}

// location — геопозиция на шаге региона (F6): регион и город, если он
// ближе 30 км; координаты не храним. Найденное можно поправить кнопкой «Не то».
func (b *Bot) location(t *turn, lat, lon float64) error {
	code, city := refdata.Locate(lat, lon)
	d, err := b.store.UpdateDialog(t.ctx, t.userID, func(_ *store.Store, d *store.Dialog) error {
		if err := expect(d, stepRegion); err != nil {
			return err
		}
		setRegion(d, code, city)
		return nil
	})
	if errors.Is(err, store.ErrStale) || errors.Is(err, store.ErrNotFound) {
		return b.unknown(t)
	}
	if err != nil {
		return err
	}
	if _, err := b.send(t, foundMessage(dialogVoice(t, d), code, city)); err != nil {
		return err
	}
	return b.ask(t, d)
}
