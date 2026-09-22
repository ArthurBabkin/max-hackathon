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
	stepGoal          = "goal"
	stepInterests     = "interests"
	stepDirection     = "direction"
	stepUniversities  = "universities"
	stepUniSearch     = "university_search"
	stepJoinConfirm   = "join_confirm"
	stepJoinDirection = "join_direction"
	stepDone          = "done"
)

// districtList — в черновике District < 0: открыт список округов.
const districtList = -1

// Вопросы об интересах (F8): каждый ответ голосует за профильную группу,
// побеждает большинство, при равенстве — ответ на первый вопрос.
var (
	interestGroups = []string{"it", "phys", "econ", "bio"}
	// groupDirection — направление, которое бот предлагает группе.
	groupDirection = map[string]string{
		"it": "napr-09-03-04", "phys": "napr-03-03-01", "bio": "napr-31-05-01", "econ": "napr-38-03-01",
	}
)

// suggestDirection — направление по трём ответам.
func suggestDirection(answers []string) string {
	votes := map[string]int{}
	best := ""
	for _, a := range answers {
		votes[a]++
	}
	for _, a := range answers {
		if best == "" || votes[a] > votes[best] {
			best = a
		}
	}
	return groupDirection[best]
}

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
	case stepGoal:
		return maxapi.WithKeyboard(v.T("bot.goal.ask", nil), maxapi.Keyboard{maxapi.Row(
			cb(v.T("bot.goal.known", nil), "goal:known"), cb(v.T("bot.goal.unknown", nil), "goal:unknown"))}), nil
	case stepDirection, stepJoinDirection:
		dirs, err := b.store.Directions(t.ctx)
		if err != nil {
			return maxapi.NewMessage{}, err
		}
		var kb maxapi.Keyboard
		for _, dir := range dirs {
			kb = append(kb, maxapi.Row(cb(dir.Name, "goal:"+dir.ID)))
		}
		return maxapi.WithKeyboard(v.T("bot.direction.ask", nil), kb), nil
	case stepInterests:
		q := len(d.Draft.Interests) + 1
		if q > 3 {
			dirID := suggestDirection(d.Draft.Interests)
			name, err := b.directionName(t, dirID)
			if err != nil {
				return maxapi.NewMessage{}, err
			}
			return maxapi.WithKeyboard(v.T("bot.int.suggest", voice.Vars{"direction": lowerFirst(name)}), maxapi.Keyboard{
				maxapi.Row(cb(v.T("bot.int.yes", nil), "goal:"+dirID), cb(v.T("bot.int.other", nil), "goal:known"))}), nil
		}
		qs := "q" + strconv.Itoa(q)
		var kb maxapi.Keyboard
		for _, g := range interestGroups {
			kb = append(kb, maxapi.Row(cb(v.T("bot.int."+qs+"."+g, nil), "int:"+strconv.Itoa(q)+":"+g)))
		}
		return maxapi.WithKeyboard(v.T("bot.int."+qs, nil), kb), nil
	case stepUniversities:
		unis, err := b.store.FindUniversities(t.ctx, "")
		if err != nil {
			return maxapi.NewMessage{}, err
		}
		var buttons []maxapi.Button
		for _, u := range unis {
			buttons = append(buttons, cb(check(slices.Contains(d.Draft.UniversityIDs, u.ID), uniLabel(u)), "vuz:t:"+u.ID))
		}
		kb := append(grid(buttons, 2),
			maxapi.Row(cb(v.T("bot.vuz.other", nil), "vuz:other")),
			maxapi.Row(cb(v.T("bot.btn.done", nil), "vuz:done")))
		return maxapi.WithKeyboard(v.T("bot.vuz.ask", nil), kb), nil
	case stepUniSearch:
		return maxapi.Text(v.T("bot.vuz.search", nil)), nil
	case stepJoinConfirm:
		return b.joinCheck(t, d)
	}
	return maxapi.NewMessage{}, errors.New("bot: у шага нет вопроса: " + d.Step)
}

func (b *Bot) regionPrompt(v voice.Voice, d store.Dialog) maxapi.NewMessage {
	cb := maxapi.CallbackButton
	switch {
	case d.Draft.District == districtList:
		var buttons []maxapi.Button
		for _, dist := range refdata.Districts() {
			buttons = append(buttons, cb(dist.Name, "region:d:"+strconv.Itoa(dist.N)))
		}
		return maxapi.WithKeyboard(v.T("bot.region.district", nil), grid(buttons, 2))
	case d.Draft.District > 0:
		var buttons []maxapi.Button
		for _, r := range refdata.InDistrict(d.Draft.District) {
			buttons = append(buttons, cb(r.Name, "region:"+r.Code))
		}
		kb := append(grid(buttons, 2), maxapi.Row(cb(v.T("bot.region.back", nil), "region:list")))
		return maxapi.WithKeyboard(v.T("bot.region.pick", nil), kb)
	}
	return maxapi.WithKeyboard(v.T("bot.region.ask", nil), maxapi.Keyboard{
		maxapi.Row(maxapi.GeoButton(v.T("bot.region.geo", nil))),
		maxapi.Row(cb(v.T("bot.region.list", nil), "region:list")),
	})
}

// uniLabel — как вуз подписан на кнопке: коротко, но узнаваемо.
func uniLabel(u store.University) string { return notify.UniversityLabel(u.ID, u.ShortName) }

func (b *Bot) directionName(t *turn, id string) (string, error) {
	dirs, err := b.store.Directions(t.ctx)
	if err != nil {
		return "", err
	}
	for _, d := range dirs {
		if d.ID == id {
			return d.Name, nil
		}
	}
	return "", store.ErrNotFound
}

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
	if after.Step == before.Step && after.Step != stepInterests {
		// Мультивыбор или список регионов: тот же вопрос с новой клавиатурой.
		msg, err := b.prompt(t, after)
		if err != nil {
			return err
		}
		return b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Message: &msg})
	}
	if err := b.max.Answer(t.ctx, cb.CallbackID, chosen(question, label)); err != nil {
		return err
	}
	return b.next(t, after)
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
				d.Step = stepGoal
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
	case "goal":
		return true, b.goalCallback(t, cb, question, arg(0))
	case "int":
		q, err := strconv.Atoi(arg(0))
		answer := arg(1)
		if err != nil || !slices.Contains(interestGroups, answer) {
			return true, b.stale(t, cb, nil)
		}
		return true, b.transition(t, cb, question, v.T("bot.int.q"+arg(0)+"."+answer, nil), func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepInterests); err != nil {
				return err
			}
			if len(d.Draft.Interests) != q-1 {
				return store.ErrStale
			}
			d.Draft.Interests = append(d.Draft.Interests, answer)
			return nil
		})
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
			d.Draft.District = districtList
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

func (b *Bot) goalCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, arg string) error {
	v := kidVoice(t)
	switch arg {
	case "known":
		return b.transition(t, cb, question, v.T("bot.goal.known", nil), func(_ *store.Store, d *store.Dialog) error {
			// «Другое направление» после подсказки по интересам — тоже сюда.
			if err := expect(d, stepGoal, stepInterests); err != nil {
				return err
			}
			if d.Step == stepInterests && len(d.Draft.Interests) < 3 {
				return store.ErrStale
			}
			d.Step = stepDirection
			return nil
		})
	case "unknown":
		return b.transition(t, cb, question, v.T("bot.goal.unknown", nil), func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepGoal); err != nil {
				return err
			}
			d.Draft.Interests, d.Step = nil, stepInterests
			return nil
		})
	}
	name, err := b.directionName(t, arg)
	if errors.Is(err, store.ErrNotFound) {
		return b.stale(t, cb, nil)
	}
	if err != nil {
		return err
	}
	return b.transition(t, cb, question, name, func(tx *store.Store, d *store.Dialog) error {
		switch {
		case d.Step == stepDirection:
			d.Draft.GoalStatus = "known"
		case d.Step == stepInterests && len(d.Draft.Interests) == 3 && suggestDirection(d.Draft.Interests) == arg:
			d.Draft.GoalStatus = "suggested"
		case d.Step == stepJoinDirection:
			// Приглашённый ученик поправил цель уже созданной траектории (F42).
			m, err := tx.CurrentMember(t.ctx, t.user.UserID)
			if err != nil {
				return err
			}
			if err := tx.SetDirection(t.ctx, m.TrajectoryID, m.MemberID, arg); err != nil {
				return err
			}
			d.Step = stepDone
			return nil
		default:
			return store.ErrStale
		}
		d.Draft.DirectionID, d.Step = arg, stepUniversities
		return nil
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
	case "done":
		return b.transition(t, cb, question, v.T("bot.btn.done", nil), func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepUniversities); err != nil {
				return err
			}
			if len(d.Draft.UniversityIDs) == 0 {
				return errNotNow{"bot.vuz.need"}
			}
			return b.createTrajectory(t, tx, d)
		})
	case "t":
		return b.transition(t, cb, question, "", func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepUniversities); err != nil {
				return err
			}
			unis, err := tx.FindUniversities(t.ctx, "")
			if err != nil {
				return err
			}
			if !slices.ContainsFunc(unis, func(u store.University) bool { return u.ID == id }) {
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
	if !ok || d.Draft.Grade == 0 || d.Draft.Name == "" || len(d.Draft.SubjectCodes) == 0 {
		return store.ErrStale
	}
	n := store.NewTrajectory{
		CreatorUserID: t.userID, Role: d.Role, StudentName: d.Draft.Name, Grade: d.Draft.Grade,
		RegionCode: reg.Code, TZ: reg.TZ, GoalStatus: d.Draft.GoalStatus, SourcePayload: d.SourcePayload,
		SubjectCodes: d.Draft.SubjectCodes, UniversityIDs: d.Draft.UniversityIDs,
	}
	if n.GoalStatus == "" {
		n.GoalStatus = "known"
	}
	if d.Draft.DirectionID != "" {
		n.DirectionID = &d.Draft.DirectionID
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
			if len(found) <= 3 {
				for _, u := range found {
					if !slices.Contains(d.Draft.UniversityIDs, u.ID) {
						d.Draft.UniversityIDs = append(d.Draft.UniversityIDs, u.ID)
					}
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
