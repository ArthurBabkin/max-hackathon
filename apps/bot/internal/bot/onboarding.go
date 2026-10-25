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
	// stepConsent — согласие с политикой конфиденциальности (152-ФЗ): до
	// первого вопроса анкеты и до входа по приглашению. Счётчика и «Назад»
	// у него нет, это ещё не анкета.
	stepConsent = "consent"
	stepRole    = "role"
	// stepNameInput — имя: родитель пишет имя ребёнка, ученик — своё, если
	// в профиле MAX его нет. Иначе имя берётся из профиля без вопроса.
	stepNameInput = "name_input"
	stepGrade     = "grade"
	stepSubjects  = "subjects"
	// stepUniversities — вузы, после предметов (#129). Выбирать не
	// обязательно: «Выберу позже».
	stepUniversities = "universities"
	// stepUniSearch — шаг поиска вуза из v1: поиск теперь прямо на шаге
	// вузов, старые диалоги ведут себя так же.
	stepUniSearch = "university_search"
	// stepDirection — последний вопрос: «Готово» или «Решу позже» создаёт
	// траекторию.
	stepDirection = "direction"
	// stepInterest, stepWork, stepSuggest — «Не знаю — помоги выбрать»:
	// В1, В2 и предложенные направления (SPEC 5).
	stepInterest      = "interest"
	stepWork          = "work"
	stepSuggest       = "suggest"
	stepJoinConfirm   = "join_confirm"
	stepJoinDirection = "join_direction"
	stepDone          = "done"
)

// Шаги, убранные из анкеты в #127: подтверждение имени, регион, опыт,
// «Где учиться» и карточка профиля. Брошенный на них диалог продолжается
// со следующего вопроса (resume).
const (
	stepNameConfirm = "name_confirm"
	stepRegion      = "region"
	stepExperience  = "experience"
	stepTarget      = "target"
	stepSummary     = "summary"
)

var retiredSteps = []string{stepNameConfirm, stepRegion, stepExperience, stepTarget, stepSummary}

// multiSelect — шаги, где «Готово» оставляет у вопроса отмеченное.
var multiSelect = []string{stepSubjects, stepDirection, stepJoinDirection, stepSuggest, stepInterest, stepUniversities}

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

// stepNumbers — номер шага онбординга в счётчике «Шаг N из M». Имя —
// часть шага роли («кто вы и как зовут»), поэтому у всех пять шагов.
// Поиск вуза — часть шага вузов, «Помоги выбрать» (interest, work,
// suggest) — часть шага направления.
var stepNumbers = map[string]int{
	stepRole: 1, stepNameInput: 1, stepGrade: 2, stepSubjects: 3,
	stepUniversities: 4, stepUniSearch: 4,
	stepDirection: 5, stepInterest: 5, stepWork: 5, stepSuggest: 5,
}

// stepsTotal — сколько шагов в счётчике.
const stepsTotal = 5

// withStep ставит «Шаг N из M» первой строкой вопроса. У приглашённого
// ученика своя короткая анкета, счётчика там нет.
func withStep(v voice.Voice, d store.Dialog, msg maxapi.NewMessage) maxapi.NewMessage {
	n, ok := stepNumbers[d.Step]
	if !ok || d.Draft.Joined {
		return msg
	}
	msg.Text = maxapi.Truncate(v.T("bot.step", voice.Vars{"count": n, "total": stepsTotal})+"\n\n"+msg.Text, maxapi.MaxTextLen)
	return msg
}

// prompt — вопрос текущего шага со счётчиком шагов; под вопросом, кроме
// первого, — «← Назад».
func (b *Bot) prompt(t *turn, d store.Dialog) (maxapi.NewMessage, error) {
	msg, err := b.question(t, d)
	if err != nil {
		return msg, err
	}
	return withBack(t, d, withStep(dialogVoice(t, d), d, msg)), nil
}

// question — вопрос текущего шага с клавиатурой, собранной из черновика:
// отмеченное — с «✓» (F7, F9).
func (b *Bot) question(t *turn, d store.Dialog) (maxapi.NewMessage, error) {
	v := dialogVoice(t, d)
	cb := maxapi.CallbackButton
	switch d.Step {
	case stepConsent:
		return consentPrompt(v, d, false), nil
	case stepRole:
		return maxapi.WithKeyboard(v.T("bot.welcome", nil), maxapi.Keyboard{maxapi.Row(
			cb(check(d.Role == "kid", v.T("bot.role.kid", nil)), "role:kid"),
			cb(check(d.Role == "parent", v.T("bot.role.parent", nil)), "role:parent"))}), nil
	case stepNameInput:
		// После «Назад» введённое имя — кнопкой: оставить его или написать другое.
		var kb maxapi.Keyboard
		if d.Draft.Name != "" {
			kb = maxapi.Keyboard{maxapi.Row(cb(check(true, d.Draft.Name), "name:keep"))}
		}
		return maxapi.WithKeyboard(v.T("bot.name.ask", nil), kb), nil
	case stepGrade:
		row := []maxapi.Button{}
		for g := 8; g <= 11; g++ {
			row = append(row, cb(check(g == d.Draft.Grade, strconv.Itoa(g)), "grade:"+strconv.Itoa(g)))
		}
		return maxapi.WithKeyboard(v.T("bot.grade.ask", nil), maxapi.Keyboard{row}), nil
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
		return workPrompt(v, d), nil
	case stepUniversities, stepUniSearch:
		return b.universitiesPrompt(t, v, d)
	case stepJoinConfirm:
		return b.joinCheck(t, d)
	}
	return maxapi.NewMessage{}, errors.New("bot: у шага нет вопроса: " + d.Step)
}

// privacyURL — политика конфиденциальности на лендинге (apps/landing).
const privacyURL = "https://traektoriaedu.ru/privacy.html"

// consentPrompt — вопрос о согласии: ссылка на политику и «Принимаю».
// Пришёл по приглашению — о том, что увидят участники траектории. После
// нажатия ссылка остаётся, а «Принимаю» отмечен.
func consentPrompt(v voice.Voice, d store.Dialog, accepted bool) maxapi.NewMessage {
	text := "bot.consent"
	if d.SourcePayload != nil && strings.HasPrefix(*d.SourcePayload, "inv_") {
		text = "bot.consent.join"
	}
	accept := maxapi.CallbackButton(v.T("bot.consent.accept", nil), "consent:ok")
	if accepted {
		accept = maxapi.CallbackButton(check(true, accept.Text), "noop")
	}
	return maxapi.WithKeyboard(v.T(text, nil), maxapi.Keyboard{
		maxapi.Row(maxapi.LinkButton(v.T("privacy.link", nil), privacyURL)),
		maxapi.Row(accept),
	})
}

// consentCallback — «Принимаю»: согласие записано, дальше то, с чем пришли, —
// анкета или вход по приглашению (параметр ссылки ждал в диалоге).
func (b *Bot) consentCallback(t *turn, cb *maxapi.Callback) error {
	d, err := b.store.Dialog(t.ctx, t.userID)
	if errors.Is(err, store.ErrNotFound) {
		return b.stale(t, cb, nil)
	}
	if err != nil {
		return err
	}
	if d.Step != stepConsent {
		return b.stale(t, cb, &d)
	}
	if err := b.store.AcceptPrivacy(t.ctx, t.userID); err != nil {
		return err
	}
	msg := consentPrompt(kidVoice(t), d, true)
	if err := b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Message: &msg}); err != nil {
		return err
	}
	payload := ""
	if d.SourcePayload != nil {
		payload = *d.SourcePayload
	}
	return b.start(t, payload)
}

// directionPrompt — направления с «✓» (F8) по пять, дальше «Показать ещё»
// (#129); порядок и вопрос — directionChoice. Отмеченное остаётся на виду.
func (b *Bot) directionPrompt(t *turn, v voice.Voice, d store.Dialog) (maxapi.NewMessage, error) {
	cb := maxapi.CallbackButton
	dirs, err := b.store.Directions(t.ctx)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	uniDirs, err := b.store.UniversityDirectionIDs(t.ctx)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	pool, text := directionChoice(d, dirs, uniDirs)
	open := (d.Draft.DirPage + 1) * dirLimit
	shown := slices.Clone(pool[:min(len(pool), open)])
	for _, id := range d.Draft.DirectionIDs {
		if i := slices.IndexFunc(dirs, func(x store.Direction) bool { return x.ID == id }); i >= 0 &&
			!slices.ContainsFunc(shown, func(x store.Direction) bool { return x.ID == id }) {
			shown = append(shown, dirs[i])
		}
	}
	var kb maxapi.Keyboard
	for _, dir := range shown {
		kb = append(kb, maxapi.Row(cb(check(slices.Contains(d.Draft.DirectionIDs, dir.ID), dir.Name), "dir:t:"+dir.ID)))
	}
	switch {
	case len(pool) > open:
		kb = append(kb, maxapi.Row(cb(v.T("bot.direction.more", nil), "dir:more")))
	case len(pool) < len(dirs):
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
func workPrompt(v voice.Voice, d store.Dialog) maxapi.NewMessage {
	var kb maxapi.Keyboard
	for _, code := range workCodes {
		kb = append(kb, maxapi.Row(maxapi.CallbackButton(check(code == d.Draft.Work, v.T("bot.work."+code, nil)), "work:"+code)))
	}
	kb = append(kb, maxapi.Row(maxapi.CallbackButton(v.T("bot.work.text", nil), "work:text")))
	return maxapi.WithKeyboard(v.T("bot.work.ask", nil), kb)
}

func lowerFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return strings.ToLower(string(r)) + s[size:]
}

// nameOK — имя от 1 до 40 символов (F4).
func nameOK(name string) bool {
	n := utf8.RuneCountInString(name)
	return n >= 1 && n <= 40
}

// backStep — куда ведёт «← Назад» с текущего шага; "" — назад некуда
// (первый вопрос, шаги приглашённого). Предыдущий шаг выводится из
// черновика: имя спрашивают не всех.
func backStep(t *turn, d store.Dialog) string {
	switch d.Step {
	case stepNameInput:
		return stepRole
	case stepGrade:
		if nameStep(t, d.Role) == stepNameInput {
			return stepNameInput
		}
		return stepRole
	case stepSubjects:
		return stepGrade
	case stepUniversities, stepUniSearch:
		return stepSubjects
	case stepDirection:
		return stepUniversities
	case stepInterest:
		if d.Draft.Joined {
			return ""
		}
		return stepDirection
	case stepWork:
		return stepInterest
	case stepSuggest:
		return stepWork
	}
	return ""
}

// anonName — так бот зовёт пользователя без имени в профиле MAX. В
// траекторию оно не попадает: такой ученик пишет имя сам.
const anonName = "Друг"

// nameStep — шаг после роли: имя ученика берётся из профиля MAX, родитель
// и ученик без имени в MAX его пишут.
func nameStep(t *turn, role string) string {
	if name := strings.TrimSpace(t.user.FirstName); role == "kid" && name != anonName && nameOK(name) {
		return stepGrade
	}
	return stepNameInput
}

// withBack добавляет под вопрос шага «← Назад» (payload back:<шаг>: со
// старого вопроса кнопка ничего не меняет).
func withBack(t *turn, d store.Dialog, msg maxapi.NewMessage) maxapi.NewMessage {
	if backStep(t, d) == "" {
		return msg
	}
	back := maxapi.CallbackButton(dialogVoice(t, d).T("bot.btn.back", nil), "back:"+d.Step)
	out := maxapi.WithKeyboard(msg.Text, append(msg.Keyboard(), maxapi.Row(back)))
	out.Format = msg.Format
	return out
}

// backCallback — «← Назад»: предыдущий вопрос на месте текущего. Ответы
// черновика остаются и отмечены «✓», вперёд их можно пройти заново.
func (b *Bot) backCallback(t *turn, cb *maxapi.Callback, step string) error {
	d, err := b.store.UpdateDialog(t.ctx, t.userID, func(_ *store.Store, d *store.Dialog) error {
		if err := expect(d, step); err != nil {
			return err
		}
		prev := backStep(t, *d)
		if prev == "" {
			return store.ErrStale
		}
		d.Step = prev
		return nil
	})
	if errors.Is(err, store.ErrStale) || errors.Is(err, store.ErrNotFound) {
		return b.staleDialog(t, cb)
	}
	if err != nil {
		return err
	}
	msg, err := b.prompt(t, d)
	if err != nil {
		return err
	}
	return b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Message: &msg})
}

// resume — диалог на шаге, которого больше нет (retiredSteps): дальше
// следующий вопрос новой анкеты, ответы черновика остаются.
func resume(t *turn, tx *store.Store, d *store.Dialog) error {
	switch d.Step {
	case stepNameConfirm:
		d.Step = stepGrade
		if d.Draft.Name == "" {
			d.Step = stepNameInput
		}
	case stepRegion:
		d.Step = stepSubjects
	case stepExperience, stepTarget, stepSummary:
		return toUniversities(t, tx, d)
	}
	return nil
}

// ask отправляет вопрос шага и запоминает сообщение: мультивыбор правит
// его клавиатуру на месте.
func (b *Bot) ask(t *turn, d store.Dialog) error {
	if slices.Contains(retiredSteps, d.Step) {
		var err error
		d, err = b.store.UpdateDialog(t.ctx, t.userID, func(tx *store.Store, d *store.Dialog) error {
			return resume(t, tx, d)
		})
		if err != nil {
			return err
		}
	}
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
// что-то говорит: «Хорошо. Сейчас дам ссылку-приглашение…».
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
		// Мультивыбор: тот же вопрос с новой клавиатурой.
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
	case "consent":
		return true, b.consentCallback(t, cb)
	case "role":
		role := arg(0)
		if role != "kid" && role != "parent" {
			return true, b.stale(t, cb, nil)
		}
		return true, b.transition(t, cb, question, v.T("bot.role."+role, nil), func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepRole); err != nil {
				return err
			}
			d.Step = nameStep(t, role)
			if role == d.Role && d.Draft.Name != "" {
				// Вернулись к роли и выбрали ту же: имя остаётся.
				return nil
			}
			// У родителя имя — ребёнка, у ученика — своё: при смене роли
			// прежнее не годится.
			d.Role, d.Draft.Name = role, ""
			if d.Step == stepGrade {
				d.Draft.Name = strings.TrimSpace(t.user.FirstName)
			}
			return nil
		})
	case "name":
		if arg(0) != "keep" {
			// «Да, это я» и «Изменить имя» из анкеты до #127.
			return true, b.staleDialog(t, cb)
		}
		// Имя, введённое до «Назад».
		name := ""
		if d, err := b.store.Dialog(t.ctx, t.userID); err == nil {
			name = d.Draft.Name
		}
		return true, b.transition(t, cb, question, name, func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepNameInput); err != nil || d.Draft.Name == "" {
				return store.ErrStale
			}
			d.Step = stepGrade
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
			d.Draft.Grade, d.Step = g, stepSubjects
			return nil
		})
	case "subj":
		if arg(0) == "done" {
			return true, b.transition(t, cb, question, v.T("bot.btn.done", nil), func(tx *store.Store, d *store.Dialog) error {
				if err := expect(d, stepSubjects); err != nil {
					return err
				}
				if len(d.Draft.SubjectCodes) == 0 {
					return errNotNow{"bot.subjects.need"}
				}
				return toUniversities(t, tx, d)
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
	case "vuz":
		return true, b.universityCallback(t, cb, question, arg(0), arg(1))
	case "back":
		return true, b.backCallback(t, cb, arg(0))
	case "region", "exp", "place", "target", "sum":
		// Кнопки шагов, убранных из анкеты: региона, опыта, «Где учиться» и
		// карточки профиля. Брошенный там диалог продолжится (resume).
		return true, b.staleDialog(t, cb)
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
	case "more":
		return b.transition(t, cb, question, "", func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepDirection, stepJoinDirection, stepSuggest); err != nil {
				return err
			}
			d.Draft.DirPage++
			return nil
		})
	case "all":
		// Остальные направления — в конец списка, следующей страницей.
		return b.transition(t, cb, question, "", func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepDirection, stepJoinDirection, stepSuggest); err != nil {
				return err
			}
			d.Draft.AllDirections = true
			d.Draft.DirPage++
			return nil
		})
	case "help":
		return b.transition(t, cb, question, v.T("bot.direction.help", nil), func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepDirection); err != nil {
				return err
			}
			// Прежние ответы В1–В2 остаются: после «Назад» они отмечены.
			d.Step = stepInterest
			return nil
		})
	case "kid":
		return b.transitionSay(t, cb, question, v.T("bot.direction.askKid", nil), func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepDirection); err != nil || d.Role != "parent" {
				return store.ErrStale
			}
			d.Draft.DirectionIDs, d.Draft.Suggested, d.Draft.GoalByKid = nil, nil, true
			return b.createTrajectory(t, tx, d)
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
			if d.Step == stepDirection {
				// Выбрано в общем списке, а не из предложенных; «Пусть ребёнок
				// ответит», если его нажали до «Назад», больше не в силе.
				d.Draft.Suggested, d.Draft.GoalByKid = nil, false
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
				return b.createTrajectory(t, tx, d)
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
		d.Draft.AllDirections, d.Draft.DirPage = false, 0
		d.Step = stepSuggest
		return nil
	})
}

// createTrajectory — конец анкеты: траектория создаётся один раз, в той
// же транзакции, что закрывает диалог (F2, F38). Регион, опыт и места
// анкета не спрашивает: регион пустой — московское время, опыта нет,
// учиться где угодно; меняется это в профиле мини-приложения. У диалогов,
// начатых до #127, — то, что успели ответить.
func (b *Bot) createTrajectory(t *turn, tx *store.Store, d *store.Dialog) error {
	reg, ok := refdata.ByCode(d.Draft.RegionCode)
	if d.Draft.RegionCode == "" {
		reg, ok = refdata.Region{TZ: refdata.DefaultTZ}, true
	}
	if !ok || d.Draft.Grade == 0 || d.Draft.Name == "" || len(d.Draft.SubjectCodes) == 0 {
		return store.ErrStale
	}
	n := store.NewTrajectory{
		CreatorUserID: t.userID, Role: d.Role, StudentName: d.Draft.Name, Grade: d.Draft.Grade,
		RegionCode: reg.Code, TZ: reg.TZ, SourcePayload: d.SourcePayload,
		DirectionIDs: d.Draft.DirectionIDs, SubjectCodes: d.Draft.SubjectCodes, UniversityIDs: d.Draft.UniversityIDs,
		Places: draftPlaces(d.Draft), Experience: d.Draft.Experience, HomeCity: d.Draft.HomeCity,
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

// freeText — имя (F4) или поиск вуза; на шаге, которого больше нет, —
// следующий вопрос; на остальных — подсказка.
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
		if !nameOK(name) {
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
	case stepUniversities, stepUniSearch:
		return b.universityText(t, d, text)
	}
	if slices.Contains(retiredSteps, d.Step) {
		return b.ask(t, d)
	}
	return b.unknown(t)
}

// location — геопозиция. Регион анкета больше не спрашивает: если диалог
// остался на этом вопросе, дальше следующий, иначе — подсказка.
func (b *Bot) location(t *turn, _, _ float64) error {
	d, err := b.store.Dialog(t.ctx, t.userID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if err == nil && slices.Contains(retiredSteps, d.Step) {
		return b.ask(t, d)
	}
	return b.unknown(t)
}

// updateDialog — store.UpdateDialog текущего пользователя.
func (b *Bot) updateDialog(t *turn, apply func(tx *store.Store, d *store.Dialog) error) (store.Dialog, error) {
	return b.store.UpdateDialog(t.ctx, t.userID, apply)
}
