package bot

import (
	"errors"
	"strings"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/match"
	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/core/pick"
	"github.com/ArthurBabkin/max-hackathon/packages/core/refdata"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// finished — онбординг пройден: профиль списком и итог (SPEC 9, F10, F11).
func (b *Bot) finished(t *turn) error {
	m, ok, err := b.member(t)
	if err != nil || !ok {
		return err
	}
	v, tr, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	lines, err := b.profileLines(t, v, tr, true)
	if err != nil {
		return err
	}
	msg := maxapi.WithKeyboard(v.T("bot.summary.title", nil)+"\n"+strings.Join(lines, "\n"),
		maxapi.Keyboard{maxapi.Row(b.app(v.T("bot.summary.edit", nil), "profile"))})
	msg.Format = "markdown"
	if _, err := b.send(t, msg); err != nil {
		return err
	}
	return b.sendResult(t, m, v, tr, "")
}

// profileLines — профиль списком: класс и место, предметы, цель, опыт,
// где учиться, вузы. withEmptyVuz = false — без строки «Вузы», если их нет.
func (b *Bot) profileLines(t *turn, v voice.Voice, tr store.Trajectory, withEmptyVuz bool) ([]string, error) {
	subs, err := b.store.TrajectorySubjects(t.ctx, tr.ID)
	if err != nil {
		return nil, err
	}
	unis, err := b.store.TrajectoryUniversities(t.ctx, tr.ID)
	if err != nil {
		return nil, err
	}
	subjects := make([]string, len(subs))
	for i, s := range subs {
		subjects[i] = strings.ToLower(s.Name)
	}
	place := refdata.Short(tr.RegionCode)
	if tr.HomeCity != nil && *tr.HomeCity != "" {
		place = *tr.HomeCity
	}
	lines := []string{
		v.T("bot.summary.grade", voice.Vars{"grade": tr.Grade, "place": place}),
		v.T("bot.summary.subjects", voice.Vars{"subjects": strings.Join(subjects, ", ")}),
		v.T("bot.summary.goal", voice.Vars{"direction": goalText(v, tr)}),
	}
	if tr.Experience != "" {
		lines = append(lines, v.T("bot.summary.experience", voice.Vars{"experience": v.T("bot.exp.label."+tr.Experience, nil)}))
	}
	lines = append(lines, v.T("bot.summary.places", voice.Vars{"places": placesText(v, tr.Places)}))
	if len(unis) > 0 || withEmptyVuz {
		names := make([]string, len(unis))
		for i, u := range unis {
			names[i] = uniLabel(u)
		}
		vuzy := strings.Join(names, ", ")
		if len(names) == 0 {
			vuzy = v.T("bot.vuz.noneChosen", nil)
		}
		lines = append(lines, v.T("bot.summary.universities", voice.Vars{"names": vuzy}))
	}
	return lines, nil
}

// goalText — цель в профиле: направления, «подобрали вместе», если их
// предложил бот, «ждёт ответа», если родитель передал вопрос ребёнку.
func goalText(v voice.Voice, tr store.Trajectory) string {
	if len(tr.Directions) == 0 && tr.GoalByKid {
		return v.T("bot.summary.waitKid", nil)
	}
	text := directionText(v, tr)
	if tr.GoalStatus == "suggested" && len(tr.Directions) > 0 {
		text += " · " + v.T("bot.summary.together", nil)
	}
	return text
}

// placesText — «Казань, Москва»; мест нет — «не важно».
func placesText(v voice.Voice, places []store.Place) string {
	if len(places) == 0 {
		return v.T("bot.summary.placesAny", nil)
	}
	names := make([]string, len(places))
	for i, p := range places {
		names[i] = refdata.Short(p.RegionCode)
		if p.City != "" {
			names[i] = p.City
		}
	}
	return strings.Join(names, ", ")
}

func directionText(v voice.Voice, tr store.Trajectory) string {
	if len(tr.Directions) == 0 {
		return v.T("bot.join.noGoal", nil)
	}
	names := make([]string, len(tr.Directions))
	for i, d := range tr.Directions {
		names[i] = lowerFirst(d.Name)
	}
	return strings.Join(names, ", ")
}

// resultText — «Под твою цель подходят 4 олимпиады и ВсОШ.», затем
// стартовая олимпиада (SPEC 2.6, 9), ближайшая по сроку, если это другая, и
// «В мини-приложении…». Цифры — из того же подбора, что экран «Подбор»
// (core/pick).
func (b *Bot) resultText(t *turn, v voice.Voice, tr store.Trajectory) (string, pick.Result, error) {
	res, err := pick.Recommend(t.ctx, b.store, tr, b.now(), "all")
	if err != nil {
		return "", res, err
	}
	perechen, vsosh := 0, false
	var nearest *match.Result
	for i, r := range res.Items {
		switch r.Kind {
		case "vsosh":
			vsosh = true
		default:
			perechen++
		}
		// Ближайшая — с самым ранним сроком, до которого ещё есть хотя бы день.
		if r.Deadline != nil && voice.DaysUntil(*r.Deadline, b.now(), tzOf(tr)) >= 1 &&
			(nearest == nil || r.Deadline.Before(*nearest.Deadline)) {
			nearest = &res.Items[i]
		}
	}
	if perechen == 0 && !vsosh {
		return v.T("bot.result.empty", nil), res, nil
	}
	count := voice.Olympiads(perechen)
	switch {
	case perechen > 0 && vsosh:
		count += " и ВсОШ"
	case vsosh:
		count = "ВсОШ"
	}
	key := "bot.result.many"
	if perechen == 1 && !vsosh || perechen == 0 {
		key = "bot.result.one"
	}
	if len(tr.Directions) == 0 {
		// Цели нет — подбор по предметам.
		key += "Subjects"
	}
	blocks := []string{v.T(key, voice.Vars{"count": count})}
	start := pick.Start(res)
	if start != nil && nearest != nil && start.ProfileID == nearest.ProfileID {
		// Стартовая и есть ближайшая — один блок.
		blocks = append(blocks, b.olympiadText(v, tr, res, *start, "bot.result.startNearest"))
	} else {
		if start != nil {
			blocks = append(blocks, b.olympiadText(v, tr, res, *start, "bot.result.start"))
		}
		if nearest != nil {
			blocks = append(blocks, b.olympiadText(v, tr, res, *nearest, "bot.result.nearest"))
		}
	}
	tail := "bot.result.tail"
	if len(res.Set.Universities) == 0 {
		tail = "bot.result.tailNoVuz"
	}
	blocks = append(blocks, v.T(tail, nil))
	if tr.GoalByKid && len(tr.Directions) == 0 {
		blocks = append(blocks, v.T("bot.result.kidInvite", nil))
	}
	return strings.Join(blocks, "\n\n"), res, nil
}

// olympiadText — заголовок («Для старта советую эту:», «Ближайшая
// олимпиада:»), олимпиада с предметом и строка «II уровень · до 5 октября,
// 4 дня · онлайн».
func (b *Bot) olympiadText(v voice.Voice, tr store.Trajectory, res pick.Result, r match.Result, title string) string {
	p := res.Profiles[r.ProfileID]
	lines := []string{
		v.T(title, nil),
		v.T("bot.result.olympiad", voice.Vars{"olympiad": notify.Short(p.OlympiadName), "subject": p.SubjectName}),
	}
	var info []string
	if r.Kind != "vsosh" && r.Level != nil {
		info = append(info, v.T("bot.result.level", voice.Vars{"level": *r.Level}))
	}
	if r.Deadline != nil {
		days := voice.DaysUntil(*r.Deadline, b.now(), tzOf(tr))
		info = append(info, v.T("bot.result.until", voice.Vars{
			"date": stages.Day(r.Deadline.In(tzOf(tr))), "days": voice.Days(days)}))
	}
	if r.Online {
		info = append(info, v.T("bot.result.online", nil))
	}
	if len(info) > 0 {
		lines = append(lines, strings.Join(info, " · "))
	}
	return strings.Join(lines, "\n")
}

// resultKeyboard — «Открыть подборку», «Напоминать о сроках», «Пригласить…»
// (F11, SPEC 9). Родитель передал вопросы ребёнку — приглашение первым.
func (b *Bot) resultKeyboard(v voice.Voice, m store.Member, tr store.Trajectory, remindersOn bool) maxapi.Keyboard {
	remind := maxapi.CallbackButton(v.T("bot.btn.remind", nil), "rem:on")
	if remindersOn {
		remind = maxapi.CallbackButton(v.T("bot.btn.remindOn", nil), "noop")
	}
	kb := maxapi.Keyboard{maxapi.Row(b.app(v.T("bot.btn.open", nil), "match")), maxapi.Row(remind)}
	if permissions(m).Invite {
		label := v.T("bot.btn.invite", nil)
		if m.Role == "parent" && !m.HasKid {
			label = v.T("bot.btn.inviteKid", nil)
		}
		invite := maxapi.Row(maxapi.CallbackButton(label, "inv:new"))
		if tr.GoalByKid && len(tr.Directions) == 0 && !m.HasKid {
			return append(maxapi.Keyboard{invite}, kb...)
		}
		kb = append(kb, invite)
	}
	return kb
}

// sendResult — итог с кнопками; prefix — «Отлично!» для приглашённого.
func (b *Bot) sendResult(t *turn, m store.Member, v voice.Voice, tr store.Trajectory, prefix string) error {
	text, _, err := b.resultText(t, v, tr)
	if err != nil {
		return err
	}
	if prefix != "" {
		text = prefix + " " + text
	}
	_, err = b.send(t, maxapi.WithKeyboard(text, b.resultKeyboard(v, m, tr, false)))
	return err
}

// remindOn — «Напоминать о сроках» (F11): все четыре порога. Если трекер
// пуст и пользователь может его пополнять, в него попадает подборка —
// иначе напоминать было бы не о чем. Лишнее убирается в мини-приложении.
func (b *Bot) remindOn(t *turn, cb *maxapi.Callback, question *maxapi.Message) error {
	m, ok, err := b.member(t)
	if err != nil {
		return err
	}
	if !ok {
		return b.stale(t, cb, nil)
	}
	v, tr, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	if err := b.store.EnableAllReminders(t.ctx, m.MemberID); err != nil {
		return err
	}
	added := 0
	if permissions(m).AddToTracker {
		state, err := b.store.TrackerState(t.ctx, m.TrajectoryID)
		if err != nil {
			return err
		}
		if len(state.InTracker) == 0 {
			_, res, err := b.resultText(t, v, tr)
			if err != nil {
				return err
			}
			for _, r := range res.Items {
				if _, created, err := b.store.AddTrackerItem(t.ctx, m.TrajectoryID, r.ProfileID, m.MemberID); err != nil {
					return err
				} else if created {
					added++
				}
			}
		}
	}
	if err := b.store.SyncReminders(t.ctx, m.TrajectoryID, b.cfg.ReminderHour, b.now()); err != nil {
		return err
	}
	answer := maxapi.CallbackAnswer{Notification: v.T("bot.btn.remindOn", nil)}
	if question != nil {
		msg := maxapi.WithKeyboard(question.Body.Text, b.resultKeyboard(v, m, tr, true))
		answer.Message = &msg
	}
	if err := b.max.Answer(t.ctx, cb.CallbackID, answer); err != nil {
		return err
	}
	text := v.T("bot.remind.enabled", nil)
	if added > 0 {
		text += " " + v.T("bot.remind.added", voice.Vars{"count": voice.Olympiads(added)})
	}
	_, err = b.send(t, maxapi.WithKeyboard(text, maxapi.Keyboard{maxapi.Row(b.app(v.T("bot.menu.tracker", nil), "tracker"))}))
	return err
}

// invite — «Пригласить в семью» (F39, F40): роль приглашённого — ученик,
// если его в траектории ещё нет и приглашает родитель; иначе родитель.
func (b *Bot) invite(t *turn, cb *maxapi.Callback) error {
	m, ok, err := b.member(t)
	if err != nil {
		return err
	}
	if !ok {
		return b.stale(t, cb, nil)
	}
	v, _, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	if !permissions(m).Invite {
		return b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Notification: v.T("bot.invite.forbidden", nil)})
	}
	role, key := "parent", "bot.invite.forParent"
	if m.Role == "parent" && !m.HasKid {
		role, key = "kid", "bot.invite.forKid"
	}
	token, err := store.NewInviteToken()
	if err != nil {
		return err
	}
	if _, err := b.store.CreateInvite(t.ctx, m.TrajectoryID, m.MemberID, role, token); err != nil {
		if errors.Is(err, store.ErrKidExists) {
			role, key = "parent", "bot.invite.forParent"
			if _, err = b.store.CreateInvite(t.ctx, m.TrajectoryID, m.MemberID, role, token); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	if err := b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Notification: "👋"}); err != nil {
		return err
	}
	return b.say(t, v.T(key, voice.Vars{"link": notify.InviteURL(b.cfg.BotName, token)}))
}

// join — вход по ссылке-приглашению (F39, F42).
func (b *Bot) join(t *turn, token string) error {
	v := kidVoice(t)
	m, err := b.store.JoinByInvite(t.ctx, token, t.userID)
	switch {
	case errors.Is(err, store.ErrInviteInvalid):
		return b.say(t, v.T("bot.invite.invalid", nil))
	case errors.Is(err, store.ErrKidExists):
		return b.say(t, v.T("bot.invite.kidExists", nil))
	case errors.Is(err, store.ErrAlreadyMember):
		if err := b.say(t, v.T("bot.invite.already", nil)); err != nil {
			return err
		}
		return b.menu(t, false)
	case err != nil:
		return err
	}
	b.notify.Joined(t.ctx, m)
	inviter, err := b.store.InviterName(t.ctx, token)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	mv, tr, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	if err := b.say(t, mv.T("bot.join.hello", voice.Vars{"name": inviter})); err != nil {
		return err
	}
	if m.Role == "parent" {
		if _, err := b.store.StartDialog(t.ctx, store.Dialog{UserID: t.userID, Step: stepDone, Role: m.Role}); err != nil {
			return err
		}
		return b.sendResult(t, m, mv, tr, "")
	}
	// Предметы — чтобы в «Изменить цель» первыми шли подходящие направления.
	subs, err := b.store.TrajectorySubjects(t.ctx, tr.ID)
	if err != nil {
		return err
	}
	draft := store.Draft{Name: tr.StudentName}
	for _, s := range subs {
		draft.SubjectCodes = append(draft.SubjectCodes, s.Code)
	}
	for _, d := range tr.Directions {
		draft.DirectionIDs = append(draft.DirectionIDs, d.ID)
	}
	d, err := b.store.StartDialog(t.ctx, store.Dialog{UserID: t.userID, Step: stepJoinConfirm, Role: m.Role, Draft: draft})
	if err != nil {
		return err
	}
	return b.ask(t, d)
}

// joinCheck — «Проверь, всё ли верно» профилем списком (F42, SPEC 10).
func (b *Bot) joinCheck(t *turn, d store.Dialog) (maxapi.NewMessage, error) {
	m, ok, err := b.member(t)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	if !ok {
		return maxapi.NewMessage{}, store.ErrNotFound
	}
	v, tr, err := b.voiceOf(t, m)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	lines, err := b.profileLines(t, v, tr, false)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	msg := maxapi.WithKeyboard(v.T("bot.join.checkTitle", nil)+"\n"+strings.Join(lines, "\n"),
		maxapi.Keyboard{maxapi.Row(maxapi.CallbackButton(v.T("bot.join.ok", nil), "join:ok"),
			maxapi.CallbackButton(v.T("bot.join.goal", nil), "join:goal"))})
	msg.Format = "markdown"
	return msg, nil
}

// joinCallback — «Да, всё верно» и «Изменить цель» приглашённого ученика.
// Если родитель передал вопросы об интересах ученику, после «Всё верно»
// бот ведёт по В1 → В2 → предложенным направлениям (SPEC 10).
func (b *Bot) joinCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, action string) error {
	v := kidVoice(t)
	if action == "goal" {
		return b.transition(t, cb, question, v.T("bot.join.goal", nil), func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepJoinConfirm); err != nil {
				return err
			}
			d.Step = stepJoinDirection
			return nil
		})
	}
	m, ok, err := b.member(t)
	if err != nil {
		return err
	}
	if !ok {
		return b.stale(t, cb, nil)
	}
	tr, err := b.store.Trajectory(t.ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	return b.transition(t, cb, question, v.T("bot.join.ok", nil), func(_ *store.Store, d *store.Dialog) error {
		if err := expect(d, stepJoinConfirm); err != nil {
			return err
		}
		d.Step = stepDone
		d.Draft.Joined = true
		if tr.GoalByKid && len(tr.Directions) == 0 && m.Role == "kid" {
			d.Step, d.Draft.Interests, d.Draft.Work = stepInterest, nil, ""
		}
		return nil
	})
}

// kidChoseGoal — родителям: ученик ответил за них на вопросы об интересах.
func (b *Bot) kidChoseGoal(t *turn) error {
	m, ok, err := b.member(t)
	if err != nil || !ok {
		return err
	}
	tr, err := b.store.Trajectory(t.ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	names := make([]string, len(tr.Directions))
	for i, d := range tr.Directions {
		names[i] = lowerFirst(d.Name)
	}
	b.notify.KidChoseGoal(t.ctx, m, strings.Join(names, ", "))
	return nil
}

// joinedResult — итог приглашённому ученику после подтверждения.
func (b *Bot) joinedResult(t *turn) error {
	m, ok, err := b.member(t)
	if err != nil || !ok {
		return err
	}
	v, tr, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	return b.sendResult(t, m, v, tr, v.T("bot.join.great", nil))
}

func tzOf(tr store.Trajectory) *time.Location {
	if loc, err := time.LoadLocation(tr.TZ); err == nil {
		return loc
	}
	return time.UTC
}
