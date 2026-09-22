package bot

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/core/schedule"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// callback — нажатие кнопки. Payload по схеме ТЗ §5.2: «вид:аргументы».
func (b *Bot) callback(t *turn, cb *maxapi.Callback, question *maxapi.Message) error {
	if cb == nil {
		return nil
	}
	parts := strings.Split(cb.Payload, ":")
	kind, args := parts[0], parts[1:]
	if handled, err := b.onboardingCallback(t, cb, question, kind, args); handled {
		return err
	}
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return ""
	}
	switch kind {
	case "join":
		return b.joinCallback(t, cb, question, arg(0))
	case "rem":
		switch arg(0) {
		case "on":
			return b.remindOn(t, cb, question)
		case "done":
			return b.markRegistered(t, cb, arg(1))
		case "snooze":
			return b.snooze(t, cb, arg(1))
		}
	case "inv":
		return b.invite(t, cb)
	case "prop":
		return b.resolveProposal(t, cb, question, arg(0) == "acc", arg(1))
	case "set":
		if arg(0) == "open" {
			if err := b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Notification: "⚙️"}); err != nil {
				return err
			}
			return b.settings(t)
		}
		return b.toggleOffset(t, cb, arg(1))
	case "del":
		return b.deleteCallback(t, cb, question, arg(0) == "yes")
	case "leave":
		return b.leave(t, cb, question)
	}
	return b.stale(t, cb, nil)
}

// --- /settings (F51) ----------------------------------------------------------

var offsetKeys = []int{30, 7, 3, 1}

func (b *Bot) settingsMessage(v voice.Voice, offsets []int32) maxapi.NewMessage {
	var buttons []maxapi.Button
	for _, o := range offsetKeys {
		on := slices.Contains(offsets, int32(o))
		buttons = append(buttons, maxapi.CallbackButton(check(on, v.T("bot.settings."+strconv.Itoa(o), nil)), "set:rem:"+strconv.Itoa(o)))
	}
	text := v.T("bot.settings.ask", nil)
	if len(offsets) == 0 {
		text += "\n\n" + v.T("bot.settings.off", nil)
	}
	return maxapi.WithKeyboard(text, grid(buttons, 2))
}

func (b *Bot) settings(t *turn) error {
	m, ok, err := b.member(t)
	if err != nil {
		return err
	}
	if !ok {
		return b.say(t, kidVoice(t).T("bot.noTrajectory", nil))
	}
	v, _, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	_, err = b.send(t, b.settingsMessage(v, m.ReminderOffsets))
	return err
}

func (b *Bot) toggleOffset(t *turn, cb *maxapi.Callback, raw string) error {
	offset, err := strconv.Atoi(raw)
	m, ok, merr := b.member(t)
	if merr != nil {
		return merr
	}
	if err != nil || !ok || !slices.Contains(offsetKeys, offset) {
		return b.stale(t, cb, nil)
	}
	offsets, err := b.store.ToggleReminderOffset(t.ctx, m.MemberID, offset)
	if err != nil {
		return err
	}
	v, _, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	msg := b.settingsMessage(v, offsets)
	return b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Message: &msg})
}

// --- /delete и выход (F48, F50) ----------------------------------------------

func (b *Bot) deleteCommand(t *turn) error {
	m, ok, err := b.member(t)
	if err != nil {
		return err
	}
	if !ok {
		return b.say(t, kidVoice(t).T("bot.noTrajectory", nil))
	}
	v, _, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	cb := maxapi.CallbackButton
	if m.IsCreator {
		others, err := b.store.OtherMemberNames(t.ctx, m.TrajectoryID, m.MemberID)
		if err != nil {
			return err
		}
		text := v.T("bot.delete.confirmAlone", nil)
		if len(others) > 0 {
			text = v.T("bot.delete.confirm", voice.Vars{"names": strings.Join(others, ", ")})
		}
		_, err = b.send(t, maxapi.WithKeyboard(text, maxapi.Keyboard{maxapi.Row(
			cb(v.T("bot.delete.yes", nil), "del:yes"), cb(v.T("bot.delete.no", nil), "del:no"))}))
		return err
	}
	members, err := b.store.FamilyMembers(t.ctx, m.TrajectoryID)
	if err != nil {
		return err
	}
	creator := ""
	if len(members) > 0 && members[0].IsCreator {
		creator = members[0].Name
	}
	_, err = b.send(t, maxapi.WithKeyboard(v.T("bot.delete.notCreator", voice.Vars{"creator": creator}), maxapi.Keyboard{maxapi.Row(
		cb(v.T("bot.leave.yes", nil), "leave:yes"), cb(v.T("bot.delete.no", nil), "del:no"))}))
	return err
}

// closeQuestion — ответ на нажатие: вопрос остаётся, клавиатура уходит.
func closeQuestion(question *maxapi.Message, label string) maxapi.CallbackAnswer {
	if a := chosen(question, label); a.Message != nil {
		return a
	}
	return maxapi.CallbackAnswer{Notification: label}
}

func (b *Bot) deleteCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, yes bool) error {
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
	if !yes {
		if err := b.max.Answer(t.ctx, cb.CallbackID, closeQuestion(question, v.T("bot.delete.no", nil))); err != nil {
			return err
		}
		return b.say(t, v.T("bot.delete.cancelled", nil))
	}
	if !m.IsCreator {
		return b.stale(t, cb, nil)
	}
	others, err := b.store.ActiveRecipients(t.ctx, m.TrajectoryID, m.MemberID)
	if err != nil {
		return err
	}
	if err := b.store.DeleteTrajectory(t.ctx, m.TrajectoryID, m.MemberID); err != nil {
		return err
	}
	if err := b.max.Answer(t.ctx, cb.CallbackID, closeQuestion(question, v.T("bot.delete.yes", nil))); err != nil {
		return err
	}
	b.notify.Deleted(t.ctx, tr, others)
	return b.say(t, v.T("bot.delete.done", nil))
}

func (b *Bot) leave(t *turn, cb *maxapi.Callback, question *maxapi.Message) error {
	m, ok, err := b.member(t)
	if err != nil {
		return err
	}
	if !ok || m.IsCreator {
		return b.stale(t, cb, nil)
	}
	v, _, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	left, err := b.store.LeaveTrajectory(t.ctx, m.TrajectoryID, m.MemberID)
	if err != nil {
		return err
	}
	if err := b.max.Answer(t.ctx, cb.CallbackID, closeQuestion(question, v.T("bot.leave.yes", nil))); err != nil {
		return err
	}
	b.notify.Left(t.ctx, m.TrajectoryID, left)
	return b.say(t, v.T("bot.leave.done", nil))
}

// --- Кнопки напоминаний (F30, F46) ------------------------------------------

func (b *Bot) markRegistered(t *turn, cb *maxapi.Callback, itemID string) error {
	notice := func(text string) error {
		return b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Notification: text})
	}
	if !uuidRe.MatchString(itemID) {
		return b.stale(t, cb, nil)
	}
	m, err := b.store.MemberFor(t.ctx, t.user.UserID, "tracker_item", itemID)
	if errors.Is(err, store.ErrNotFound) {
		return notice(kidVoice(t).T("bot.remind.gone", nil))
	}
	if err != nil {
		return err
	}
	v, _, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	item, err := b.store.TrackerItem(t.ctx, m.TrajectoryID, itemID)
	if err != nil {
		return err
	}
	changed, err := b.store.SetRegistered(t.ctx, m.TrajectoryID, itemID, m.MemberID, true)
	if err != nil {
		return err
	}
	if changed {
		if err := b.store.SyncReminders(t.ctx, m.TrajectoryID, b.cfg.ReminderHour, b.now()); err != nil {
			return err
		}
		b.notify.Registered(t.ctx, m.TrajectoryID, itemID, m)
	}
	return notice(v.T("bot.remind.marked", voice.Vars{"title": notify.Short(item.OlympiadName)}))
}

// snooze — «Напомнить завтра» (F30): разовое напоминание тому, кто нажал,
// завтра в REMINDER_HOUR, если срок ещё не пройдёт.
func (b *Bot) snooze(t *turn, cb *maxapi.Callback, reminderID string) error {
	notice := func(text string) error {
		return b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Notification: text})
	}
	if !uuidRe.MatchString(reminderID) {
		return b.stale(t, cb, nil)
	}
	m, err := b.store.MemberFor(t.ctx, t.user.UserID, "reminder", reminderID)
	if errors.Is(err, store.ErrNotFound) {
		return notice(kidVoice(t).T("bot.remind.gone", nil))
	}
	if err != nil {
		return err
	}
	d, err := b.store.Reminder(t.ctx, reminderID)
	if errors.Is(err, store.ErrNotFound) {
		return notice(kidVoice(t).T("bot.remind.gone", nil))
	}
	if err != nil {
		return err
	}
	v, _, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(d.TZ)
	if err != nil {
		loc = time.UTC
	}
	at, ok := schedule.Tomorrow(d.Deadline, loc, b.cfg.ReminderHour, b.now())
	if !ok {
		return notice(v.T("bot.remind.tooLate", nil))
	}
	if _, err := b.store.RemindTomorrow(t.ctx, d.TrackerItemID, d.StageID, m.MemberID, at); err != nil {
		return err
	}
	return notice(v.T("bot.remind.snoozed", voice.Vars{"date": "завтра в " + at.In(loc).Format("15:04")}))
}

// --- Предложения родителя (F45) ---------------------------------------------

func (b *Bot) resolveProposal(t *turn, cb *maxapi.Callback, question *maxapi.Message, accept bool, id string) error {
	if !uuidRe.MatchString(id) {
		return b.stale(t, cb, nil)
	}
	m, err := b.store.MemberFor(t.ctx, t.user.UserID, "proposal", id)
	if errors.Is(err, store.ErrNotFound) {
		return b.stale(t, cb, nil)
	}
	if err != nil {
		return err
	}
	v, _, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	if !permissions(m).ResolveProposals {
		return b.stale(t, cb, nil)
	}
	p, err := b.store.Proposal(t.ctx, m.TrajectoryID, id)
	if err != nil {
		return err
	}
	_, err = b.store.ResolveProposal(t.ctx, m.TrajectoryID, id, m.MemberID, accept)
	if errors.Is(err, store.ErrProposalClosed) {
		return b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Notification: v.T("bot.prop.closed", nil)})
	}
	if err != nil {
		return err
	}
	label, key := v.T("bot.prop.decline", nil), "bot.prop.declined"
	if accept {
		label, key = v.T("bot.prop.accept", nil), "bot.prop.accepted"
		if err := b.store.SyncReminders(t.ctx, m.TrajectoryID, b.cfg.ReminderHour, b.now()); err != nil {
			return err
		}
	}
	if err := b.max.Answer(t.ctx, cb.CallbackID, closeQuestion(question, label)); err != nil {
		return err
	}
	b.notify.ProposalResolved(t.ctx, m.TrajectoryID, id, accept)
	by := ""
	if p.ProposedBy != nil {
		by = p.ProposedBy.Name
	}
	return b.say(t, v.T(key, voice.Vars{"name": by}))
}
