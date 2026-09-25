package bot

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ArthurBabkin/max-hackathon/packages/core/refdata"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// Шаги онбординга (ТЗ §5.1, docs/onboarding-v2/SPEC.md). Шаг — условие
// перехода: кнопка из старого сообщения не совпадает с текущим шагом и
// ничего не меняет.
const (
	stepRole        = "role"
	stepNameConfirm = "name_confirm"
	stepNameInput   = "name_input"
	stepGrade       = "grade"
	stepRegion      = "region"
	stepSubjects    = "subjects"
	stepDirection   = "direction"
	// stepInterest, stepWork, stepSuggest — «Не знаю — помоги выбрать»:
	// В1, В2 и предложенные направления (SPEC 5).
	stepInterest     = "interest"
	stepWork         = "work"
	stepSuggest      = "suggest"
	stepExperience   = "experience"
	stepTarget       = "target"
	stepUniversities = "universities"
	// stepUniSearch — шаг поиска вуза из v1: поиск теперь прямо на шаге
	// вузов, старые диалоги ведут себя так же.
	stepUniSearch     = "university_search"
	stepJoinConfirm   = "join_confirm"
	stepJoinDirection = "join_direction"
	// stepSummary — профиль на подтверждение: траектории ещё нет, «Готово»
	// создаёт её, «Изменить» возвращает к нужному вопросу.
	stepSummary = "summary"
	stepDone    = "done"
)

// multiSelect — шаги, где «Готово» оставляет у вопроса отмеченное.
var multiSelect = []string{stepSubjects, stepDirection, stepJoinDirection, stepSuggest, stepInterest, stepUniversities, stepTarget}

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

// stepNumbers — номер шага онбординга в счётчике «Шаг N из M». Номера
// постоянные: пропущенный шаг не сдвигает остальные, счётчик просто
// перескакивает. «Помоги выбрать» (interest, work, suggest) — часть шага
// направления, как и поиск вуза — часть шага вузов.
var stepNumbers = map[string]int{
	stepRole: 1, stepNameConfirm: 2, stepNameInput: 2, stepGrade: 3, stepRegion: 4, stepSubjects: 5,
	stepDirection: 6, stepInterest: 6, stepWork: 6, stepSuggest: 6,
	stepExperience: 7, stepTarget: 8, stepUniversities: 9, stepUniSearch: 9,
}

// stepsTotal — сколько шагов в счётчике.
const stepsTotal = 9

// withStep ставит «Шаг N из M» первой строкой вопроса. У приглашённого
// ученика своя короткая анкета, счётчика там нет; при правке из карточки
// профиля — тоже.
func withStep(v voice.Voice, d store.Dialog, msg maxapi.NewMessage) maxapi.NewMessage {
	n, ok := stepNumbers[d.Step]
	if !ok || d.Draft.Joined || d.Draft.EditStage > 0 {
		return msg
	}
	msg.Text = maxapi.Truncate(v.T("bot.step", voice.Vars{"count": n, "total": stepsTotal})+"\n\n"+msg.Text, maxapi.MaxTextLen)
	return msg
}

// prompt — вопрос текущего шага со счётчиком шагов.
func (b *Bot) prompt(t *turn, d store.Dialog) (maxapi.NewMessage, error) {
	msg, err := b.question(t, d)
	if err != nil {
		return msg, err
	}
	return withStep(dialogVoice(t, d), d, msg), nil
}

// question — вопрос текущего шага с клавиатурой, собранной из черновика:
// отмеченное — с «✓» (F7, F9).
func (b *Bot) question(t *turn, d store.Dialog) (maxapi.NewMessage, error) {
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
		ask := "bot.grade.ask"
		if d.Draft.EditStage > 0 {
			// Правка из карточки профиля: знакомились уже.
			ask = "bot.grade.askAgain"
		}
		return maxapi.WithKeyboard(v.T(ask, nil), maxapi.Keyboard{row}), nil
	case stepRegion:
		return regionPrompt(v, d), nil
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
	case stepDirection, stepJoinDirection, stepSuggest:
		return b.directionPrompt(t, v, d)
	case stepInterest:
		return interestPrompt(v, d), nil
	case stepWork:
		return workPrompt(v), nil
	case stepExperience:
		return experiencePrompt(v, d), nil
	case stepTarget:
		return targetPrompt(v, d, ""), nil
	case stepUniversities, stepUniSearch:
		return b.universitiesPrompt(t, v, d)
	case stepJoinConfirm:
		return b.joinCheck(t, d)
	case stepSummary:
		return b.summaryPrompt(t, v, d)
	}
	return maxapi.NewMessage{}, errors.New("bot: у шага нет вопроса: " + d.Step)
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

// directionPrompt — направления с «✓» (F8): сначала подходящие под предметы
// (на шаге suggest — предложенные по интересам), по кнопке — все.
// Выбранное из полного списка остаётся на виду.
func (b *Bot) directionPrompt(t *turn, v voice.Voice, d store.Dialog) (maxapi.NewMessage, error) {
	cb := maxapi.CallbackButton
	dirs, err := b.store.Directions(t.ctx)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	var shown []store.Direction
	text := "bot.direction.ask"
	if d.Step == stepSuggest {
		for _, id := range d.Draft.Suggested {
			if i := slices.IndexFunc(dirs, func(x store.Direction) bool { return x.ID == id }); i >= 0 {
				shown = append(shown, dirs[i])
			}
		}
		text = "bot.suggest.ask"
	} else {
		shown = matchingDirections(dirs, d.Draft.SubjectCodes)
	}
	if len(shown) == 0 || d.Draft.AllDirections && d.Step != stepSuggest {
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
	switch d.Step {
	case stepSuggest:
		kb = append(kb, maxapi.Row(cb(v.T("bot.btn.done", nil), "dir:done")),
			maxapi.Row(cb(v.T("bot.suggest.later", nil), "dir:later")))
	case stepDirection:
		kb = append(kb, maxapi.Row(cb(v.T("bot.btn.done", nil), "dir:done"), cb(v.T("bot.direction.later", nil), "dir:later")),
			maxapi.Row(cb(v.T("bot.direction.help", nil), "dir:help")))
		if d.Role == "parent" {
			kb = append(kb, maxapi.Row(cb(v.T("bot.direction.askKid", nil), "dir:kid")))
		}
	default:
		kb = append(kb, maxapi.Row(cb(v.T("bot.btn.done", nil), "dir:done"), cb(v.T("bot.direction.later", nil), "dir:later")))
	}
	return maxapi.WithKeyboard(v.T(text, nil), kb), nil
}

// interestPrompt — В1 «что нравится», до двух вариантов (SPEC 5.2).
func interestPrompt(v voice.Voice, d store.Dialog) maxapi.NewMessage {
	var kb maxapi.Keyboard
	for _, code := range interestCodes {
		kb = append(kb, maxapi.Row(maxapi.CallbackButton(
			check(slices.Contains(d.Draft.Interests, code), v.T("bot.interest."+code, nil)), "int:t:"+code)))
	}
	kb = append(kb, maxapi.Row(maxapi.CallbackButton(v.T("bot.btn.done", nil), "int:done")))
	return maxapi.WithKeyboard(v.T("bot.interest.ask", nil), kb)
}

// workPrompt — В2 «какая работа ближе» (SPEC 5.3). «Своими словами» пока
// только отвечает «скоро».
func workPrompt(v voice.Voice) maxapi.NewMessage {
	var kb maxapi.Keyboard
	for _, code := range workCodes {
		kb = append(kb, maxapi.Row(maxapi.CallbackButton(v.T("bot.work."+code, nil), "work:"+code)))
	}
	kb = append(kb, maxapi.Row(maxapi.CallbackButton(v.T("bot.work.text", nil), "work:text")))
	return maxapi.WithKeyboard(v.T("bot.work.ask", nil), kb)
}

// experiencePrompt — опыт в олимпиадах (SPEC 6); у родителя есть «Не знаю».
func experiencePrompt(v voice.Voice, d store.Dialog) maxapi.NewMessage {
	cb := maxapi.CallbackButton
	kb := maxapi.Keyboard{
		maxapi.Row(cb(v.T("bot.exp.none", nil), "exp:none")),
		maxapi.Row(cb(v.T("bot.exp.school", nil), "exp:school")),
		maxapi.Row(cb(v.T("bot.exp.region", nil), "exp:region")),
	}
	if d.Role == "parent" {
		kb = append(kb, maxapi.Row(cb(v.T("bot.exp.idk", nil), "exp:none:idk")))
	}
	return maxapi.WithKeyboard(v.T("bot.exp.ask", nil), kb)
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
	return b.sendPrompt(t, msg)
}

func (b *Bot) sendPrompt(t *turn, msg maxapi.NewMessage) error {
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
	return b.transitionSay(t, cb, question, label, apply, nil)
}

// transitionSay — transition, после которого до следующего вопроса бот
// что-то говорит: «Казань → Татарстан ✓» с кнопкой «Не то».
func (b *Bot) transitionSay(t *turn, cb *maxapi.Callback, question *maxapi.Message, label string,
	apply func(tx *store.Store, d *store.Dialog) error, say func(d store.Dialog) error) error {
	before, err := b.store.Dialog(t.ctx, t.userID)
	if errors.Is(err, store.ErrNotFound) {
		return b.stale(t, cb, nil)
	}
	if err != nil {
		return err
	}
	after, err := b.updateDialog(t, apply)
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
	if question != nil && slices.Contains(multiSelect, before.Step) {
		// После «Готово» у вопроса остаётся то, что выбрали, а не «✓ Готово».
		d := after
		d.Step = before.Step
		msg, err := b.prompt(t, d)
		if err != nil {
			return err
		}
		if p := picked(msg); len(p.Keyboard()) > 0 {
			answer = maxapi.CallbackAnswer{Message: p}
		}
	}
	if err := b.max.Answer(t.ctx, cb.CallbackID, answer); err != nil {
		return err
	}
	if say != nil {
		if err := say(after); err != nil {
			return err
		}
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
		if d.Draft.Joined {
			return b.joinedResult(t)
		}
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
	case "int":
		return true, b.interestCallback(t, cb, question, arg(0), arg(1))
	case "work":
		return true, b.workCallback(t, cb, question, arg(0))
	case "exp":
		return true, b.experienceCallback(t, cb, question, arg(0), arg(1) == "idk")
	case "place":
		return true, b.placeCallback(t, cb, question, arg(0), arg(1))
	case "target":
		// Кнопки «Где учиться» из v1: места теперь выбираются по-другому.
		return true, b.staleDialog(t, cb)
	case "vuz":
		return true, b.universityCallback(t, cb, question, arg(0), arg(1))
	case "sum":
		return true, b.summaryCallback(t, cb, question, arg(0), arg(1))
	}
	return false, nil
}

// staleDialog — stale с переспросом текущего шага.
func (b *Bot) staleDialog(t *turn, cb *maxapi.Callback) error {
	d, err := b.store.Dialog(t.ctx, t.userID)
	if err != nil {
		return b.stale(t, cb, nil)
	}
	return b.stale(t, cb, &d)
}

// dialogVoiceOf — голос текущего диалога; до диалога — голос ученика.
func (b *Bot) dialogVoiceOf(t *turn) voice.Voice {
	if d, err := b.store.Dialog(t.ctx, t.userID); err == nil {
		return dialogVoice(t, d)
	}
	return kidVoice(t)
}

func (b *Bot) directionCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, action, id string) error {
	// «Решу позже» у родителя звучит иначе: «Пока не знаем».
	v := b.dialogVoiceOf(t)
	switch action {
	case "t":
		return b.transition(t, cb, question, "", func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepDirection, stepJoinDirection, stepSuggest); err != nil {
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
			if err := expect(d, stepDirection, stepJoinDirection, stepSuggest); err != nil {
				return err
			}
			d.Draft.AllDirections = true
			return nil
		})
	case "help":
		return b.transition(t, cb, question, v.T("bot.direction.help", nil), func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepDirection); err != nil {
				return err
			}
			d.Draft.DirectionIDs, d.Draft.AllDirections, d.Draft.Interests, d.Draft.Work = nil, false, nil, ""
			d.Step = stepInterest
			return nil
		})
	case "kid":
		return b.transitionSay(t, cb, question, v.T("bot.direction.askKid", nil), func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepDirection); err != nil || d.Role != "parent" {
				return store.ErrStale
			}
			d.Draft.DirectionIDs, d.Draft.GoalByKid, d.Step = nil, true, stepExperience
			return nil
		}, func(d store.Dialog) error { return b.say(t, dialogVoice(t, d).T("bot.direction.kidLater", nil)) })
	case "done", "later":
		label := v.T("bot.btn.done", nil)
		if action == "later" {
			label = v.T("bot.direction.later", nil)
			if d, err := b.store.Dialog(t.ctx, t.userID); err == nil && d.Step == stepSuggest {
				label = v.T("bot.suggest.later", nil)
			}
		}
		var chose []string
		return b.transitionSay(t, cb, question, label, func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepDirection, stepJoinDirection, stepSuggest); err != nil {
				return err
			}
			if action == "later" {
				d.Draft.DirectionIDs = nil
			} else if len(d.Draft.DirectionIDs) == 0 {
				if d.Step == stepSuggest {
					return errNotNow{"bot.suggest.need"}
				}
				return errNotNow{"bot.direction.need"}
			}
			switch {
			case d.Step == stepJoinDirection:
				// Приглашённый ученик поправил цель уже созданной траектории (F42).
				m, err := tx.CurrentMember(t.ctx, t.user.UserID)
				if err != nil {
					return err
				}
				if err := tx.SetDirections(t.ctx, m.TrajectoryID, m.MemberID, d.Draft.DirectionIDs); err != nil {
					return err
				}
				d.Step = stepDone
			case d.Draft.Joined:
				// Приглашённый ученик ответил за родителя (SPEC 10).
				m, err := tx.CurrentMember(t.ctx, t.user.UserID)
				if err != nil {
					return err
				}
				goal, off := store.GoalStatusOf(d.Draft.DirectionIDs), false
				if len(d.Draft.DirectionIDs) > 0 {
					goal = "suggested"
				}
				if err := tx.UpdateTrajectory(t.ctx, m.TrajectoryID, m.MemberID, store.TrajectoryPatch{
					DirectionIDs: nonNilIDs(d.Draft.DirectionIDs), GoalStatus: &goal, GoalByKid: &off}); err != nil {
					return err
				}
				chose = d.Draft.DirectionIDs
				d.Step = stepDone
			default:
				d.Step = stepExperience
			}
			return nil
		}, func(d store.Dialog) error {
			if len(chose) == 0 {
				return nil
			}
			return b.kidChoseGoal(t)
		})
	}
	return b.stale(t, cb, nil)
}

func nonNilIDs(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

// interestCallback — В1: до двух вариантов, «Готово» от одного.
func (b *Bot) interestCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, action, code string) error {
	v := b.dialogVoiceOf(t)
	if action == "done" {
		return b.transition(t, cb, question, v.T("bot.btn.done", nil), func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepInterest); err != nil {
				return err
			}
			if len(d.Draft.Interests) == 0 {
				return errNotNow{"bot.interest.need"}
			}
			d.Step = stepWork
			return nil
		})
	}
	if action != "t" || !slices.Contains(interestCodes, code) {
		return b.stale(t, cb, nil)
	}
	return b.transition(t, cb, question, "", func(_ *store.Store, d *store.Dialog) error {
		if err := expect(d, stepInterest); err != nil {
			return err
		}
		if !slices.Contains(d.Draft.Interests, code) && len(d.Draft.Interests) >= maxInterests {
			return errNotNow{"bot.interest.max"}
		}
		d.Draft.Interests = toggle(d.Draft.Interests, code)
		return nil
	})
}

// workCallback — В2 и подсчёт предложенных направлений (SPEC 5.3–5.4).
func (b *Bot) workCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, code string) error {
	if code == "text" {
		d, err := b.store.Dialog(t.ctx, t.userID)
		if err != nil || d.Step != stepWork {
			return b.staleDialog(t, cb)
		}
		// TODO(onboarding-v2): Jev сопоставит ответ своими словами с направлениями.
		return b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Notification: dialogVoice(t, d).T("bot.work.textSoon", nil)})
	}
	if !slices.Contains(workCodes, code) {
		return b.stale(t, cb, nil)
	}
	return b.transition(t, cb, question, b.dialogVoiceOf(t).T("bot.work."+code, nil), func(tx *store.Store, d *store.Dialog) error {
		if err := expect(d, stepWork); err != nil {
			return err
		}
		dirs, err := tx.Directions(t.ctx)
		if err != nil {
			return err
		}
		d.Draft.Work = code
		d.Draft.Suggested = suggestDirections(d.Draft.Interests, code, d.Draft.SubjectCodes, dirs)
		d.Draft.DirectionIDs = slices.Clone(d.Draft.Suggested)
		d.Draft.AllDirections = false
		d.Step = stepSuggest
		return nil
	})
}

// experienceCallback — опыт в олимпиадах (SPEC 6), затем «Где учиться».
func (b *Bot) experienceCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, code string, idk bool) error {
	if !slices.Contains([]string{"none", "school", "region"}, code) || idk && code != "none" {
		return b.stale(t, cb, nil)
	}
	key := "bot.exp." + code
	if idk {
		key = "bot.exp.idk"
	}
	return b.transition(t, cb, question, b.dialogVoiceOf(t).T(key, nil), func(_ *store.Store, d *store.Dialog) error {
		if err := expect(d, stepExperience); err != nil {
			return err
		}
		d.Draft.Experience, d.Step = code, stepTarget
		d.Draft.PlaceOptions, d.Draft.Found = defaultPlaces(d.Draft), nil
		return nil
	})
}

// createTrajectory — последний шаг: траектория создаётся один раз, в той
// же транзакции, что закрывает диалог (F2, F38). Регион «не важен» —
// пустой код и московское время.
func (b *Bot) createTrajectory(t *turn, tx *store.Store, d *store.Dialog) error {
	reg, ok := refdata.ByCode(d.Draft.RegionCode)
	if d.Draft.RegionCode == "" {
		reg, ok = refdata.Region{TZ: refdata.DefaultTZ}, true
	}
	places, placesOK := draftPlaces(d.Draft)
	if !ok || d.Draft.Grade == 0 || d.Draft.Name == "" || len(d.Draft.SubjectCodes) == 0 || !placesOK {
		return store.ErrStale
	}
	n := store.NewTrajectory{
		CreatorUserID: t.userID, Role: d.Role, StudentName: d.Draft.Name, Grade: d.Draft.Grade,
		RegionCode: reg.Code, TZ: reg.TZ, SourcePayload: d.SourcePayload,
		DirectionIDs: d.Draft.DirectionIDs, SubjectCodes: d.Draft.SubjectCodes, UniversityIDs: d.Draft.UniversityIDs,
		Places: places, Experience: d.Draft.Experience, HomeCity: d.Draft.HomeCity,
		GoalByKid: d.Draft.GoalByKid && len(d.Draft.DirectionIDs) == 0,
	}
	if len(d.Draft.Suggested) > 0 && len(d.Draft.DirectionIDs) > 0 {
		n.GoalStatus = "suggested"
	}
	if _, err := tx.CreateTrajectory(t.ctx, n); err != nil {
		return err
	}
	d.Step = stepDone
	return nil
}

// freeText — имя (F4), поиск места (шаги региона и «Где учиться») или
// вуза; на остальных шагах — подсказка.
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
		d, err = b.updateDialog(t, func(_ *store.Store, d *store.Dialog) error {
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
	case stepRegion:
		return b.regionText(t, d, text)
	case stepTarget:
		return b.targetText(t, d, text)
	case stepUniversities, stepUniSearch:
		return b.universityText(t, d, text)
	}
	return b.unknown(t)
}
