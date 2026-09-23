package bot

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/core/refdata"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// Шаги онбординга (ТЗ §5.1). Шаг — условие перехода: кнопка из старого
// сообщения не совпадает с текущим шагом и ничего не меняет.
const (
	stepRole          = "role"
	stepNameConfirm   = "name_confirm"
	stepNameInput     = "name_input"
	stepGrade         = "grade"
	stepRegion        = "region"
	stepSubjects      = "subjects"
	stepDirection     = "direction"
	stepTarget        = "target"
	stepUniversities  = "universities"
	stepUniSearch     = "university_search"
	stepJoinConfirm   = "join_confirm"
	stepJoinDirection = "join_direction"
	stepDone          = "done"
)

// districtsOpen — в черновике District < 0: открыт список округов.
const districtsOpen = -1

// offerLimit — сколько вузов бот предлагает кнопками на шаге «Вузы» (F9).
const offerLimit = 6

func dialogVoice(t *turn, d store.Dialog) voice.Voice {
	role := voice.Kid
	if d.Role == "parent" {
		role = voice.Parent
	}
	return voice.New(role, d.Draft.Name, t.user.FirstName)
}

func check(on bool, label string) string {
	if on {
		return "✓ " + label
	}
	return label
}

// grid раскладывает кнопки по n в ряд.
func grid(buttons []maxapi.Button, n int) maxapi.Keyboard {
	var kb maxapi.Keyboard
	for i := 0; i < len(buttons); i += n {
		kb = append(kb, buttons[i:min(i+n, len(buttons))])
	}
	return kb
}

// prompt — вопрос текущего шага с клавиатурой, собранной из черновика:
// отмеченные предметы и вузы — с «✓» (F7, F9).
func (b *Bot) prompt(t *turn, d store.Dialog) (maxapi.NewMessage, error) {
	v := dialogVoice(t, d)
	cb := maxapi.CallbackButton
	switch d.Step {
	case stepRole:
		return maxapi.WithKeyboard(v.T("bot.welcome", nil), maxapi.Keyboard{maxapi.Row(
			cb(v.T("bot.role.kid", nil), "role:kid"), cb(v.T("bot.role.parent", nil), "role:parent"))}), nil
	case stepNameConfirm:
		return maxapi.WithKeyboard(v.T("bot.name.confirm", voice.Vars{"me": d.Draft.Name}), maxapi.Keyboard{maxapi.Row(
			cb(v.T("bot.name.yes", nil), "name:ok"), cb(v.T("bot.name.edit", nil), "name:edit"))}), nil
	case stepNameInput:
		return maxapi.Text(v.T("bot.name.ask", nil)), nil
	case stepGrade:
		row := []maxapi.Button{}
		for g := 8; g <= 11; g++ {
			row = append(row, cb(strconv.Itoa(g), "grade:"+strconv.Itoa(g)))
		}
		return maxapi.WithKeyboard(v.T("bot.grade.ask", nil), maxapi.Keyboard{row}), nil
	case stepRegion:
		return b.regionPrompt(v, d), nil
	case stepSubjects:
		subs, err := b.store.AllSubjects(t.ctx)
		if err != nil {
			return maxapi.NewMessage{}, err
		}
		var buttons []maxapi.Button
		for _, s := range subs {
			buttons = append(buttons, cb(check(slices.Contains(d.Draft.SubjectCodes, s.Code), s.Name), "subj:t:"+s.Code))
		}
		kb := append(grid(buttons, 2), maxapi.Row(cb(v.T("bot.btn.done", nil), "subj:done")))
		return maxapi.WithKeyboard(v.T("bot.subjects.ask", nil), kb), nil
	case stepDirection, stepJoinDirection:
		return b.directionPrompt(t, v, d)
	case stepTarget:
		return targetPrompt(v, d), nil
	case stepUniversities:
		return b.universitiesPrompt(t, v, d)
	case stepUniSearch:
		return maxapi.Text(v.T("bot.vuz.search", nil)), nil
	case stepJoinConfirm:
		return b.joinCheck(t, d)
	}
	return maxapi.NewMessage{}, errors.New("bot: у шага нет вопроса: " + d.Step)
}

// districtList — список «округ → субъект» для шагов региона и «Где хочешь
// учиться?»; kind — начало payload: region или target. ok = false — список
// не открыт.
func districtList(v voice.Voice, d store.Dialog, kind string) (maxapi.NewMessage, bool) {
	cb := maxapi.CallbackButton
	switch {
	case d.Draft.District == districtsOpen:
		var buttons []maxapi.Button
		for _, dist := range refdata.Districts() {
			buttons = append(buttons, cb(dist.Name, kind+":d:"+strconv.Itoa(dist.N)))
		}
		return maxapi.WithKeyboard(v.T("bot.region.district", nil), grid(buttons, 2)), true
	case d.Draft.District > 0:
		var buttons []maxapi.Button
		for _, r := range refdata.InDistrict(d.Draft.District) {
			buttons = append(buttons, cb(r.Name, kind+":"+r.Code))
		}
		kb := append(grid(buttons, 2), maxapi.Row(cb(v.T("bot.region.back", nil), kind+":list")))
		return maxapi.WithKeyboard(v.T("bot.region.pick", nil), kb), true
	}
	return maxapi.NewMessage{}, false
}

func (b *Bot) regionPrompt(v voice.Voice, d store.Dialog) maxapi.NewMessage {
	cb := maxapi.CallbackButton
	if msg, ok := districtList(v, d, "region"); ok {
		return msg
	}
	return maxapi.WithKeyboard(v.T("bot.region.ask", nil), maxapi.Keyboard{
		maxapi.Row(maxapi.GeoButton(v.T("bot.region.geo", nil))),
		maxapi.Row(cb(v.T("bot.region.list", nil), "region:list")),
	})
}

// matchingDirections — направления, чьи ключевые предметы пересекаются с
// предметами ученика: сначала те, где совпало больше. Порядок внутри
// равных — как в справочнике.
func matchingDirections(dirs []store.Direction, subjects []string) []store.Direction {
	overlap := func(d store.Direction) int {
		n := 0
		for _, c := range d.SubjectCodes {
			if slices.Contains(subjects, c) {
				n++
			}
		}
		return n
	}
	var out []store.Direction
	for _, d := range dirs {
		if overlap(d) > 0 {
			out = append(out, d)
		}
	}
	slices.SortStableFunc(out, func(a, b store.Direction) int { return overlap(b) - overlap(a) })
	return out
}

// directionPrompt — направления с «✓» (F8): сначала подходящие под предметы,
// по кнопке — все. Выбранное из полного списка остаётся на виду.
func (b *Bot) directionPrompt(t *turn, v voice.Voice, d store.Dialog) (maxapi.NewMessage, error) {
	cb := maxapi.CallbackButton
	dirs, err := b.store.Directions(t.ctx)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	shown := matchingDirections(dirs, d.Draft.SubjectCodes)
	text := "bot.direction.ask"
	if len(shown) == 0 || d.Draft.AllDirections {
		text = "bot.direction.askAll"
	}
	for _, dir := range dirs {
		picked := slices.Contains(d.Draft.DirectionIDs, dir.ID)
		if (d.Draft.AllDirections || len(shown) == 0 || picked) && !slices.ContainsFunc(shown, func(x store.Direction) bool { return x.ID == dir.ID }) {
			shown = append(shown, dir)
		}
	}
	var kb maxapi.Keyboard
	for _, dir := range shown {
		kb = append(kb, maxapi.Row(cb(check(slices.Contains(d.Draft.DirectionIDs, dir.ID), dir.Name), "dir:t:"+dir.ID)))
	}
	if len(shown) < len(dirs) {
		kb = append(kb, maxapi.Row(cb(v.T("bot.direction.all", nil), "dir:all")))
	}
	kb = append(kb, maxapi.Row(cb(v.T("bot.btn.done", nil), "dir:done"), cb(v.T("bot.direction.later", nil), "dir:later")))
	return maxapi.WithKeyboard(v.T(text, nil), kb), nil
}

// targetOption — кнопка «Где хочешь учиться?»: подпись и код субъекта.
type targetOption struct{ key, code string }

// targetOptions — свой регион, Москва и Петербург (если это не свой
// регион) и «не важно».
func targetOptions(own string) []targetOption {
	opts := []targetOption{{"bot.target.own", own}}
	for _, o := range []targetOption{{"bot.target.moscow", "77"}, {"bot.target.spb", "78"}} {
		if !slices.Contains(refdata.Metro(own), o.code) {
			opts = append(opts, o)
		}
	}
	return append(opts, targetOption{"bot.target.any", store.TargetAny})
}

// targetPrompt — «Где хочешь учиться?»; «Другой регион» открывает на месте
// вопроса тот же список «округ → субъект», что и на шаге региона.
func targetPrompt(v voice.Voice, d store.Dialog) maxapi.NewMessage {
	if msg, ok := districtList(v, d, "target"); ok {
		return msg
	}
	var buttons []maxapi.Button
	for _, o := range targetOptions(d.Draft.RegionCode) {
		buttons = append(buttons, maxapi.CallbackButton(v.T(o.key, nil), "target:"+o.code))
	}
	buttons = append(buttons, maxapi.CallbackButton(v.T("bot.target.other", nil), "target:list"))
	return maxapi.WithKeyboard(v.T("bot.target.ask", nil), grid(buttons, 2))
}

// universitiesPrompt — предложенные и найденные вузы с «✓» (F9). Выбирать
// не обязательно: без вузов подбор покажет, где олимпиады дают льготы.
func (b *Bot) universitiesPrompt(t *turn, v voice.Voice, d store.Dialog) (maxapi.NewMessage, error) {
	cb := maxapi.CallbackButton
	unis, err := b.store.FindUniversities(t.ctx, "")
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	var buttons []maxapi.Button
	for _, id := range d.Draft.Offered {
		i := slices.IndexFunc(unis, func(u store.University) bool { return u.ID == id })
		if i >= 0 {
			buttons = append(buttons, cb(check(slices.Contains(d.Draft.UniversityIDs, id), uniLabel(unis[i])), "vuz:t:"+id))
		}
	}
	text := "bot.vuz.ask"
	if len(buttons) == 0 {
		text = "bot.vuz.none"
	}
	finish := v.T("bot.vuz.skip", nil)
	if len(d.Draft.UniversityIDs) > 0 {
		finish = v.T("bot.btn.done", nil)
	}
	kb := append(grid(buttons, 2),
		maxapi.Row(cb(v.T("bot.vuz.other", nil), "vuz:other"), cb(v.T("bot.vuz.city", nil), "vuz:city")),
		maxapi.Row(cb(finish, "vuz:done")))
	return maxapi.WithKeyboard(v.T(text, nil), kb), nil
}

// offer — вузы для кнопок после выбора города: подходящие по городу и
// направлениям плюс уже отмеченные.
func offer(t *turn, tx *store.Store, d *store.Dialog) error {
	var regions []string
	if d.Draft.Target != store.TargetAny {
		regions = refdata.Metro(d.Draft.Target)
	}
	unis, err := tx.SuggestUniversities(t.ctx, d.Draft.DirectionIDs, regions, offerLimit)
	if err != nil {
		return err
	}
	d.Draft.Offered = nil
	for _, u := range unis {
		d.Draft.Offered = append(d.Draft.Offered, u.ID)
	}
	for _, id := range d.Draft.UniversityIDs {
		if !slices.Contains(d.Draft.Offered, id) {
			d.Draft.Offered = append(d.Draft.Offered, id)
		}
	}
	return nil
}

// uniLabel — как вуз подписан на кнопке: коротко, но узнаваемо.
func uniLabel(u store.University) string { return notify.UniversityLabel(u.ID, u.ShortName) }

func lowerFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return strings.ToLower(string(r)) + s[size:]
}

// ask отправляет вопрос шага и запоминает сообщение: мультивыбор правит
// его клавиатуру на месте.
func (b *Bot) ask(t *turn, d store.Dialog) error {
	msg, err := b.prompt(t, d)
	if err != nil {
		return err
	}
	mid, err := b.send(t, msg)
	if err != nil {
		return err
	}
	return b.store.SetPromptMID(t.ctx, t.userID, mid)
}

// errNotNow — переход с этого шага невозможен (например, «Готово» без
// предметов); пользователь получает подсказку, диалог не меняется.
type errNotNow struct{ key string }

func (e errNotNow) Error() string { return "bot: " + e.key }

// transition — переход онбординга по нажатию. apply проверяет шаг и правит
// черновик; ответ на нажатие — вопрос, на котором нажали, с отмеченным
// выбором, а следом — следующий вопрос. Мультивыбор остаётся на шаге и
// правит клавиатуру на месте.
func (b *Bot) transition(t *turn, cb *maxapi.Callback, question *maxapi.Message, label string,
	apply func(tx *store.Store, d *store.Dialog) error) error {
	before, err := b.store.Dialog(t.ctx, t.userID)
	if errors.Is(err, store.ErrNotFound) {
		return b.stale(t, cb, nil)
	}
	if err != nil {
		return err
	}
	after, err := b.store.UpdateDialog(t.ctx, t.userID, apply)
	var notNow errNotNow
	switch {
	case errors.Is(err, store.ErrStale):
		return b.stale(t, cb, &before)
	case errors.As(err, &notNow):
		return b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Notification: dialogVoice(t, before).T(notNow.key, nil)})
	case err != nil:
		return err
	}
	if after.Step == before.Step {
		// Мультивыбор или список регионов: тот же вопрос с новой клавиатурой.
		msg, err := b.prompt(t, after)
		if err != nil {
			return err
		}
		return b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Message: &msg})
	}
	answer := chosen(question, label)
	if question != nil && slices.Contains([]string{stepSubjects, stepDirection, stepJoinDirection, stepUniversities}, before.Step) {
		// После «Готово» у вопроса остаётся то, что выбрали, а не «✓ Готово».
		d := after
		d.Step = before.Step
		msg, err := b.prompt(t, d)
		if err != nil {
			return err
		}
		answer = maxapi.CallbackAnswer{Message: picked(msg)}
	}
	if err := b.max.Answer(t.ctx, cb.CallbackID, answer); err != nil {
		return err
	}
	return b.next(t, after)
}

// picked — вопрос мультивыбора, от клавиатуры которого остались отмеченные
// кнопки; нажимать их уже незачем.
func picked(msg maxapi.NewMessage) *maxapi.NewMessage {
	var buttons []maxapi.Button
	for _, row := range msg.Keyboard() {
		for _, btn := range row {
			if strings.HasPrefix(btn.Text, "✓ ") {
				buttons = append(buttons, maxapi.CallbackButton(btn.Text, "noop"))
			}
		}
	}
	out := maxapi.WithKeyboard(msg.Text, grid(buttons, 2))
	return &out
}

// chosen — ответ на нажатие: тот же вопрос, от клавиатуры остаётся выбор с «✓».
func chosen(question *maxapi.Message, label string) maxapi.CallbackAnswer {
	if question == nil || label == "" {
		return maxapi.CallbackAnswer{}
	}
	msg := maxapi.WithKeyboard(question.Body.Text, maxapi.Keyboard{maxapi.Row(maxapi.CallbackButton("✓ "+label, "noop"))})
	return maxapi.CallbackAnswer{Message: &msg}
}

// next — что сказать после перехода: вопрос нового шага или итог.
func (b *Bot) next(t *turn, d store.Dialog) error {
	if d.Step == stepDone {
		return b.finished(t)
	}
	return b.ask(t, d)
}

// stale — нажатие кнопки из старого сообщения: ничего не меняем,
// подсказываем и переспрашиваем текущий шаг.
func (b *Bot) stale(t *turn, cb *maxapi.Callback, d *store.Dialog) error {
	if err := b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Notification: kidVoice(t).T("bot.stale", nil)}); err != nil {
		return err
	}
	if d == nil || d.Step == stepDone {
		return nil
	}
	return b.ask(t, *d)
}

func expect(d *store.Dialog, steps ...string) error {
	if !slices.Contains(steps, d.Step) {
		return store.ErrStale
	}
	return nil
}

func toggle(list []string, v string) []string {
	if i := slices.Index(list, v); i >= 0 {
		return slices.Delete(list, i, i+1)
	}
	return append(list, v)
}

// onboardingCallback — нажатия онбординга. handled = false — payload не отсюда.
func (b *Bot) onboardingCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, kind string, args []string) (bool, error) {
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return ""
	}
	v := kidVoice(t)
	switch kind {
	case "role":
		role := arg(0)
		if role != "kid" && role != "parent" {
			return true, b.stale(t, cb, nil)
		}
		return true, b.transition(t, cb, question, v.T("bot.role."+role, nil), func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepRole); err != nil {
				return err
			}
			d.Role = role
			if role == "parent" {
				d.Step = stepNameInput
				return nil
			}
			d.Draft.Name = strings.TrimSpace(t.user.FirstName)
			d.Step = stepNameConfirm
			if n := utf8.RuneCountInString(d.Draft.Name); n < 1 || n > 40 {
				d.Draft.Name = ""
				d.Step = stepNameInput
			}
			return nil
		})
	case "name":
		label := v.T("bot.name.yes", nil)
		next := stepGrade
		if arg(0) == "edit" {
			label, next = v.T("bot.name.edit", nil), stepNameInput
		}
		return true, b.transition(t, cb, question, label, func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepNameConfirm); err != nil {
				return err
			}
			d.Step = next
			return nil
		})
	case "grade":
		g, err := strconv.Atoi(arg(0))
		if err != nil || g < 8 || g > 11 {
			return true, b.stale(t, cb, nil)
		}
		return true, b.transition(t, cb, question, arg(0), func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepGrade); err != nil {
				return err
			}
			d.Draft.Grade, d.Step = g, stepRegion
			return nil
		})
	case "region":
		return true, b.regionCallback(t, cb, question, args)
	case "subj":
		if arg(0) == "done" {
			return true, b.transition(t, cb, question, v.T("bot.btn.done", nil), func(_ *store.Store, d *store.Dialog) error {
				if err := expect(d, stepSubjects); err != nil {
					return err
				}
				if len(d.Draft.SubjectCodes) == 0 {
					return errNotNow{"bot.subjects.need"}
				}
				d.Step, d.Draft.AllDirections = stepDirection, false
				return nil
			})
		}
		code := arg(1)
		return true, b.transition(t, cb, question, "", func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepSubjects); err != nil {
				return err
			}
			subs, err := tx.AllSubjects(t.ctx)
			if err != nil {
				return err
			}
			if !slices.ContainsFunc(subs, func(s store.Subject) bool { return s.Code == code }) {
				return store.ErrStale
			}
			d.Draft.SubjectCodes = toggle(d.Draft.SubjectCodes, code)
			return nil
		})
	case "dir":
		return true, b.directionCallback(t, cb, question, arg(0), arg(1))
	case "target":
		return true, b.targetCallback(t, cb, question, args)
	case "vuz":
		return true, b.universityCallback(t, cb, question, arg(0), arg(1))
	}
	return false, nil
}

func (b *Bot) regionCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, args []string) error {
	if len(args) == 0 {
		return b.stale(t, cb, nil)
	}
	switch {
	case args[0] == "list":
		return b.transition(t, cb, question, "", func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepRegion); err != nil {
				return err
			}
			d.Draft.District = districtsOpen
			return nil
		})
	case args[0] == "d" && len(args) > 1:
		n, err := strconv.Atoi(args[1])
		if err != nil || len(refdata.InDistrict(n)) == 0 {
			return b.stale(t, cb, nil)
		}
		return b.transition(t, cb, question, "", func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepRegion); err != nil {
				return err
			}
			d.Draft.District = n
			return nil
		})
	}
	reg, ok := refdata.ByCode(args[0])
	if !ok {
		return b.stale(t, cb, nil)
	}
	return b.transition(t, cb, question, reg.Name, func(_ *store.Store, d *store.Dialog) error {
		if err := expect(d, stepRegion); err != nil {
			return err
		}
		d.Draft.RegionCode, d.Draft.District, d.Step = reg.Code, 0, stepSubjects
		return nil
	})
}

func (b *Bot) directionCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, action, id string) error {
	v := kidVoice(t)
	if d, err := b.store.Dialog(t.ctx, t.userID); err == nil {
		// «Решу позже» у родителя звучит иначе: «Пока не знаем».
		v = dialogVoice(t, d)
	}
	switch action {
	case "t":
		return b.transition(t, cb, question, "", func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepDirection, stepJoinDirection); err != nil {
				return err
			}
			dirs, err := tx.Directions(t.ctx)
			if err != nil {
				return err
			}
			if !slices.ContainsFunc(dirs, func(x store.Direction) bool { return x.ID == id }) {
				return store.ErrStale
			}
			d.Draft.DirectionIDs = toggle(d.Draft.DirectionIDs, id)
			return nil
		})
	case "all":
		return b.transition(t, cb, question, "", func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepDirection, stepJoinDirection); err != nil {
				return err
			}
			d.Draft.AllDirections = true
			return nil
		})
	case "done", "later":
		label := v.T("bot.btn.done", nil)
		if action == "later" {
			label = v.T("bot.direction.later", nil)
		}
		return b.transition(t, cb, question, label, func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepDirection, stepJoinDirection); err != nil {
				return err
			}
			if action == "later" {
				d.Draft.DirectionIDs = nil
			} else if len(d.Draft.DirectionIDs) == 0 {
				return errNotNow{"bot.direction.need"}
			}
			if d.Step == stepJoinDirection {
				// Приглашённый ученик поправил цель уже созданной траектории (F42).
				m, err := tx.CurrentMember(t.ctx, t.user.UserID)
				if err != nil {
					return err
				}
				if err := tx.SetDirections(t.ctx, m.TrajectoryID, m.MemberID, d.Draft.DirectionIDs); err != nil {
					return err
				}
				d.Step = stepDone
				return nil
			}
			d.Step = stepTarget
			return nil
		})
	}
	return b.stale(t, cb, nil)
}

// targetCallback — «Где хочешь учиться?»: город и вузы в нём (F9).
func (b *Bot) targetCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, args []string) error {
	if len(args) == 0 {
		return b.stale(t, cb, nil)
	}
	// Список округов и субъектов правит тот же вопрос.
	district := 0
	switch {
	case args[0] == "list":
		district = districtsOpen
	case args[0] == "d" && len(args) > 1:
		n, err := strconv.Atoi(args[1])
		if err != nil || len(refdata.InDistrict(n)) == 0 {
			return b.stale(t, cb, nil)
		}
		district = n
	}
	if district != 0 {
		return b.transition(t, cb, question, "", func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepTarget); err != nil {
				return err
			}
			d.Draft.District = district
			return nil
		})
	}
	code := args[0]
	label := kidVoice(t).T("bot.target.any", nil)
	if reg, ok := refdata.ByCode(code); ok {
		label = reg.Name
	} else if code != store.TargetAny {
		return b.stale(t, cb, nil)
	}
	return b.transition(t, cb, question, label, func(tx *store.Store, d *store.Dialog) error {
		if err := expect(d, stepTarget); err != nil {
			return err
		}
		d.Draft.Target, d.Draft.District, d.Step = code, 0, stepUniversities
		return offer(t, tx, d)
	})
}

func (b *Bot) universityCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, action, id string) error {
	v := kidVoice(t)
	switch action {
	case "other":
		return b.transition(t, cb, question, v.T("bot.vuz.other", nil), func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepUniversities); err != nil {
				return err
			}
			d.Step = stepUniSearch
			return nil
		})
	case "city":
		return b.transition(t, cb, question, v.T("bot.vuz.city", nil), func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepUniversities); err != nil {
				return err
			}
			d.Step, d.Draft.District = stepTarget, 0
			return nil
		})
	case "done":
		return b.transition(t, cb, question, v.T("bot.btn.done", nil), func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepUniversities); err != nil {
				return err
			}
			return b.createTrajectory(t, tx, d)
		})
	case "t":
		return b.transition(t, cb, question, "", func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepUniversities); err != nil {
				return err
			}
			if !slices.Contains(d.Draft.Offered, id) {
				return store.ErrStale
			}
			d.Draft.UniversityIDs = toggle(d.Draft.UniversityIDs, id)
			return nil
		})
	}
	return b.stale(t, cb, nil)
}

// createTrajectory — последний шаг: траектория создаётся один раз, в той
// же транзакции, что закрывает диалог (F2, F38).
func (b *Bot) createTrajectory(t *turn, tx *store.Store, d *store.Dialog) error {
	reg, ok := refdata.ByCode(d.Draft.RegionCode)
	if !ok || d.Draft.Grade == 0 || d.Draft.Name == "" || len(d.Draft.SubjectCodes) == 0 || d.Draft.Target == "" {
		return store.ErrStale
	}
	n := store.NewTrajectory{
		CreatorUserID: t.userID, Role: d.Role, StudentName: d.Draft.Name, Grade: d.Draft.Grade,
		RegionCode: reg.Code, TZ: reg.TZ, SourcePayload: d.SourcePayload,
		DirectionIDs: d.Draft.DirectionIDs, SubjectCodes: d.Draft.SubjectCodes, UniversityIDs: d.Draft.UniversityIDs,
	}
	if d.Draft.Target != store.TargetAny {
		n.TargetRegionCode = &d.Draft.Target
	}
	if _, err := tx.CreateTrajectory(t.ctx, n); err != nil {
		return err
	}
	d.Step = stepDone
	return nil
}

// location — геопозиция на шаге региона (F6): определяем субъект,
// координаты не храним.
func (b *Bot) location(t *turn, lat, lon float64) error {
	reg := refdata.Nearest(lat, lon)
	d, err := b.store.UpdateDialog(t.ctx, t.userID, func(_ *store.Store, d *store.Dialog) error {
		if err := expect(d, stepRegion); err != nil {
			return err
		}
		d.Draft.RegionCode, d.Draft.District, d.Step = reg.Code, 0, stepSubjects
		return nil
	})
	if errors.Is(err, store.ErrStale) || errors.Is(err, store.ErrNotFound) {
		return b.unknown(t)
	}
	if err != nil {
		return err
	}
	if err := b.say(t, dialogVoice(t, d).T("bot.region.set", voice.Vars{"region": reg.Name})); err != nil {
		return err
	}
	return b.ask(t, d)
}

// freeText — имя (F4) или поиск вуза (F9); вне этих шагов — подсказка.
func (b *Bot) freeText(t *turn, text string) error {
	d, err := b.store.Dialog(t.ctx, t.userID)
	if errors.Is(err, store.ErrNotFound) {
		return b.unknown(t)
	}
	if err != nil {
		return err
	}
	switch d.Step {
	case stepNameInput:
		name := strings.Join(strings.Fields(text), " ")
		if n := utf8.RuneCountInString(name); n < 1 || n > 40 {
			return b.say(t, dialogVoice(t, d).T("bot.name.invalid", nil))
		}
		d, err = b.store.UpdateDialog(t.ctx, t.userID, func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepNameInput); err != nil {
				return err
			}
			d.Draft.Name, d.Step = name, stepGrade
			return nil
		})
		if err != nil {
			return err
		}
		return b.ask(t, d)
	case stepUniSearch:
		found, err := b.store.FindUniversities(t.ctx, strings.TrimSpace(text))
		if err != nil {
			return err
		}
		d, err = b.store.UpdateDialog(t.ctx, t.userID, func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepUniSearch); err != nil {
				return err
			}
			for i, u := range found {
				if i < offerLimit && !slices.Contains(d.Draft.Offered, u.ID) {
					d.Draft.Offered = append(d.Draft.Offered, u.ID)
				}
				if len(found) <= 3 && !slices.Contains(d.Draft.UniversityIDs, u.ID) {
					d.Draft.UniversityIDs = append(d.Draft.UniversityIDs, u.ID)
				}
			}
			d.Step = stepUniversities
			return nil
		})
		if err != nil {
			return err
		}
		if len(found) == 0 {
			if err := b.say(t, dialogVoice(t, d).T("bot.vuz.notFound", nil)); err != nil {
				return err
			}
		}
		return b.ask(t, d)
	}
	return b.unknown(t)
}
