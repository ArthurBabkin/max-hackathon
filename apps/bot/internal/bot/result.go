package bot

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/match"
	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/core/pick"
	"github.com/ArthurBabkin/max-hackathon/packages/core/refdata"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// finished — онбординг пройден: сводка ответов и итог (F10, F11).
func (b *Bot) finished(t *turn) error {
	m, ok, err := b.member(t)
	if err != nil || !ok {
		return err
	}
	v, tr, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	summary, err := b.summary(t, v, tr)
	if err != nil {
		return err
	}
	if err := b.say(t, summary); err != nil {
		return err
	}
	return b.sendResult(t, m, v, tr, "")
}

// summary — «Готово: Артём, 9 класс, Татарстан. Предметы: …».
func (b *Bot) summary(t *turn, v voice.Voice, tr store.Trajectory) (string, error) {
	subs, err := b.store.TrajectorySubjects(t.ctx, tr.ID)
	if err != nil {
		return "", err
	}
	unis, err := b.store.TrajectoryUniversities(t.ctx, tr.ID)
	if err != nil {
		return "", err
	}
	subjects := make([]string, len(subs))
	for i, s := range subs {
		subjects[i] = strings.ToLower(s.Name)
	}
	names := make([]string, len(unis))
	for i, u := range unis {
		names[i] = uniLabel(u)
	}
	vuzy := strings.Join(names, ", ")
	if len(names) == 0 {
		vuzy = v.T("bot.vuz.noneChosen", nil)
	}
	region := tr.RegionCode
	if reg, ok := refdata.ByCode(tr.RegionCode); ok {
		region = reg.Name
	}
	return v.T("bot.summary", voice.Vars{"grade": tr.Grade, "region": region,
		"subject": strings.Join(subjects, ", "), "direction": directionText(v, tr), "names": vuzy}), nil
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

// resultText — «Под твою цель подходят 4 олимпиады и ВсОШ. Ближайший срок —
// регистрация на олимпиаду «Высшая проба», осталось 4 дня. В навигаторе…»
// Цифры — из того же подбора, что экран «Подбор» (core/pick).
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
		if r.Deadline != nil && (nearest == nil || r.Deadline.Before(*nearest.Deadline)) {
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
	parts := []string{v.T(key, voice.Vars{"count": count})}
	if nearest != nil && nearest.Stage != nil {
		days := voice.DaysUntil(*nearest.Deadline, b.now(), tzOf(tr))
		if days >= 1 {
			name := res.Profiles[nearest.ProfileID].OlympiadName
			parts = append(parts, v.T("bot.result.deadline", voice.Vars{
				"stage": notify.StagePhrase(v, nearest.Stage.Kind, name), "days": voice.Days(days)}))
		}
	}
	tail := "bot.result.tail"
	if len(res.Set.Universities) == 0 {
		tail = "bot.result.tailNoVuz"
	}
	parts = append(parts, v.T(tail, nil))
	return strings.Join(parts, " "), res, nil
}

// resultKeyboard — «Открыть навигатор», «Напоминать о сроках», «Пригласить…» (F11).
func (b *Bot) resultKeyboard(v voice.Voice, m store.Member, remindersOn bool) maxapi.Keyboard {
	remind := maxapi.CallbackButton(v.T("bot.btn.remind", nil), "rem:on")
	if remindersOn {
		remind = maxapi.CallbackButton(v.T("bot.btn.remindOn", nil), "noop")
	}
	kb := maxapi.Keyboard{maxapi.Row(b.app(v.T("bot.btn.open", nil), "home")), maxapi.Row(remind)}
	if permissions(m).Invite {
		label := v.T("bot.btn.invite", nil)
		if m.Role == "parent" && !m.HasKid {
			label = v.T("bot.btn.inviteKid", nil)
		}
		kb = append(kb, maxapi.Row(maxapi.CallbackButton(label, "inv:new")))
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
	_, err = b.send(t, maxapi.WithKeyboard(text, b.resultKeyboard(v, m, false)))
	return err
}

// remindOn — «Напоминать о сроках» (F11): все четыре порога. Если трекер
// пуст и пользователь может его пополнять, в него попадает подборка —
// иначе напоминать было бы не о чем. Лишнее убирается в навигаторе.
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
		msg := maxapi.WithKeyboard(question.Body.Text, b.resultKeyboard(v, m, true))
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

// joinCheck — «Проверь, всё ли верно: имя, класс, цель» (F42).
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
	return maxapi.WithKeyboard(v.T("bot.join.check", voice.Vars{"grade": strconv.Itoa(tr.Grade), "direction": directionText(v, tr)}),
		maxapi.Keyboard{maxapi.Row(maxapi.CallbackButton(v.T("bot.join.ok", nil), "join:ok"),
			maxapi.CallbackButton(v.T("bot.join.goal", nil), "join:goal"))}), nil
}

// joinCallback — «Да, всё верно» и «Изменить цель» приглашённого ученика.
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
	before, err := b.store.Dialog(t.ctx, t.userID)
	if err != nil {
		return b.stale(t, cb, nil)
	}
	if _, err := b.store.UpdateDialog(t.ctx, t.userID, func(_ *store.Store, d *store.Dialog) error {
		if err := expect(d, stepJoinConfirm); err != nil {
			return err
		}
		d.Step = stepDone
		return nil
	}); errors.Is(err, store.ErrStale) {
		return b.stale(t, cb, &before)
	} else if err != nil {
		return err
	}
	if err := b.max.Answer(t.ctx, cb.CallbackID, chosen(question, v.T("bot.join.ok", nil))); err != nil {
		return err
	}
	return b.joinedResult(t)
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
