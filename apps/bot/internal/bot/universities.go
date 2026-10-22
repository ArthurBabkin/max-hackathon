package bot

import (
	"slices"
	"strings"

	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/core/refdata"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// offerLimit — сколько вузов бот предлагает кнопками за раз (F9).
const offerLimit = 6

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

// offer — вузы для кнопок шага «Вузы»: сначала те, где есть выбранные
// направления (с «Показать ещё» — несколько страниц), плюс уже отмеченные.
func offer(t *turn, tx *store.Store, d *store.Dialog) error {
	shown := (d.Draft.OfferPage + 1) * offerLimit
	unis, err := tx.SuggestUniversities(t.ctx, d.Draft.DirectionIDs, nil, shown+1, 0)
	if err != nil {
		return err
	}
	d.Draft.OfferMore = len(unis) > shown
	d.Draft.Offered = nil
	for _, u := range unis[:min(len(unis), shown)] {
		d.Draft.Offered = append(d.Draft.Offered, u.ID)
	}
	for _, id := range d.Draft.UniversityIDs {
		if !slices.Contains(d.Draft.Offered, id) {
			d.Draft.Offered = append(d.Draft.Offered, id)
		}
	}
	return nil
}

// toUniversities — к шагу вузов с первой страницей подборки.
func toUniversities(t *turn, tx *store.Store, d *store.Dialog) error {
	d.Step, d.Draft.OfferPage = stepUniversities, 0
	return offer(t, tx, d)
}

// draftPlaces — места «Где учиться» диалогов, начатых до #127 (у диалогов
// v1 — Target). Анкета их больше не спрашивает: пусто — «не важно».
func draftPlaces(dr store.Draft) []store.Place {
	switch {
	case dr.PlacesAny || dr.Target == store.TargetAny:
		return nil
	case len(dr.Places) > 0:
		return dr.Places
	case dr.Target != "":
		return []store.Place{{RegionCode: dr.Target}}
	}
	return nil
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

// universitiesPrompt — подборка вузов с «✓» (V1). Выбирать не
// обязательно: без вузов подбор покажет льготы во всех вузах с
// направлением, отметить их можно позже в мини-приложении.
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
	var buttons []maxapi.Button
	for _, id := range d.Draft.Offered {
		if i := slices.IndexFunc(unis, func(u store.University) bool { return u.ID == id }); i >= 0 {
			buttons = append(buttons, cb(check(slices.Contains(d.Draft.UniversityIDs, id), uniButton(unis[i])), "vuz:t:"+id))
		}
	}
	kb := grid(buttons, 2)
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
	kb = append(kb, maxapi.Row(cb(finishLabel(v, d), "vuz:done")))
	return maxapi.WithKeyboard(text, kb), nil
}

// finishLabel — последняя кнопка анкеты: «Готово», если вузы отмечены,
// иначе «Выберу позже».
func finishLabel(v voice.Voice, d store.Dialog) string {
	if len(d.Draft.UniversityIDs) > 0 {
		return v.T("bot.btn.done", nil)
	}
	return v.T("bot.vuz.later", nil)
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
	case "done":
		// «Готово» или «Выберу позже» — конец анкеты: траектория создаётся сразу.
		label := v.T("bot.btn.done", nil)
		if d, err := b.store.Dialog(t.ctx, t.userID); err == nil {
			label = finishLabel(v, d)
		}
		return b.transition(t, cb, question, label, onStep(func(tx *store.Store, d *store.Dialog) error {
			return b.createTrajectory(t, tx, d)
		}))
	case "places", "add", "alldir", "similar", "other", "city":
		// Кнопки шага вузов прежних версий: «Изменить места», ближайшие и
		// похожие (V3), «Другой вуз» и города из v1.
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
	d, err = b.updateDialog(t, func(_ *store.Store, d *store.Dialog) error {
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
