// Package notify — сообщения бота, которые рождаются не в диалоге:
// напоминания о сроках и уведомления семьи (F41, F44–F46, F50). Их шлют и
// бот, и воркер, и REST, поэтому тексты и клавиатуры собраны в одном месте,
// а каждому получателю — в его голосе (F3).
package notify

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/schedule"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// Notifier рассылает уведомления. Max == nil — уведомления выключены
// (локальный запуск без токена бота): сценарии работают, сообщения не уходят.
type Notifier struct {
	Store        *store.Store
	Max          maxapi.Sender
	BotName      string
	BotID        int64
	ReminderHour int
}

// App — кнопка мини-приложения с разделом в start_param.
func (n *Notifier) App(text, payload string) maxapi.Button {
	return maxapi.OpenAppButton(text, n.BotName, n.BotID, payload)
}

// Voice — голос получателя в траектории.
func Voice(r store.Recipient, t store.Trajectory) voice.Voice {
	return voice.New(voice.Role(r.Role), t.StudentName, r.Name)
}

// Send отправляет и не роняет сценарий: изменение уже сохранено, а
// недоставленное уведомление — не повод отвечать пользователю ошибкой.
// Остановивший бота пользователь — обычное дело, не ошибка.
func (n *Notifier) Send(ctx context.Context, userID int64, m maxapi.NewMessage) string {
	if n.Max == nil || userID == 0 {
		return ""
	}
	mid, err := n.Max.Send(ctx, userID, m)
	switch {
	case maxapi.IsBlocked(err):
		slog.Info("получатель недоступен", "err", err)
	case err != nil:
		slog.Warn("отправка уведомления", "err", err)
	}
	return mid
}

// Detached — контекст для рассылки после ответа: запрос пользователя уже
// завершён, но рассылка должна дойти, пусть и с верхней границей времени.
func Detached(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
}

// Short — название для кавычек в тексте: «Всероссийская олимпиада
// школьников «Высшая проба»» превращается в «Высшая проба», чтобы не было
// кавычек в кавычках. Без кавычек в названии — как есть.
func Short(name string) string {
	open := strings.Index(name, "«")
	if open < 0 {
		return name
	}
	rest := name[open+len("«"):]
	if end := strings.Index(rest, "»"); end > 0 {
		return rest[:end]
	}
	return name
}

// StagePhrase — «регистрация на олимпиаду «Высшая проба»». Название
// олимпиады не склоняем — оно в кавычках после родового слова.
func StagePhrase(v voice.Voice, kind, olympiad string) string {
	key := "bot.stage." + kind
	switch kind {
	case "registration", "qualifying", "final", "school", "municipal", "regional":
	default:
		key = "bot.stage.any"
	}
	return v.T(key, voice.Vars{"title": Short(olympiad)})
}

// NextDeadline — ближайший будущий срок профиля: этап и дата.
func NextDeadline(st []stages.Stage, registered bool, now time.Time) (stages.Stage, bool) {
	for _, s := range st {
		if registered && stages.RegistrationLike(s.Kind) {
			continue
		}
		if s.DeadlineAt != nil && s.DeadlineAt.After(now) {
			return s, true
		}
	}
	return stages.Stage{}, false
}

func location(tz string) *time.Location {
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}
	return time.UTC
}

// Reminder — текст и кнопки напоминания для одного получателя (ТЗ §5.3).
// Формулировка — по реальному числу дней до срока, а не по порогу: у
// догоняющего и «напомнить завтра» порог условный.
func (n *Notifier) Reminder(d store.DueReminder, r store.Recipient, t store.Trajectory, now time.Time) maxapi.NewMessage {
	v := Voice(r, t)
	loc := location(d.TZ)
	stage := StagePhrase(v, d.StageKind, d.OlympiadName)
	var text string
	switch days := voice.DaysUntil(d.Deadline, now, loc); {
	case days <= 0:
		text = v.T("bot.remind.today", voice.Vars{"stage": stage})
	case days == 1:
		text = v.T("bot.remind.tomorrow", voice.Vars{"stage": stage})
	case days == 7:
		text = v.T("bot.remind.week", voice.Vars{"stage": stage})
	case days >= 28 && days <= 31:
		text = v.T("bot.remind.month", voice.Vars{"stage": stage})
	default:
		text = v.T("bot.remind.days", voice.Vars{"stage": stage, "days": voice.Days(days)})
	}
	registration := stages.RegistrationLike(d.StageKind)
	if registration && v.Role() == voice.Parent {
		text += " " + v.T("bot.remind.consent", nil)
	}

	var kb maxapi.Keyboard
	if d.OfficialURL != nil && strings.HasPrefix(*d.OfficialURL, "https://") {
		label := v.T("bot.remind.openSite", nil)
		if registration {
			label = v.T("bot.remind.openRegistration", nil)
		}
		kb = append(kb, maxapi.Row(maxapi.LinkButton(label, *d.OfficialURL)))
	}
	if registration && !d.Registered {
		kb = append(kb, maxapi.Row(maxapi.CallbackButton(v.T("bot.remind.markRegistered", nil), "rem:done:"+d.TrackerItemID)))
	}
	if _, ok := schedule.Tomorrow(d.Deadline, loc, n.ReminderHour, now); ok {
		kb = append(kb, maxapi.Row(maxapi.CallbackButton(v.T("bot.remind.snooze", nil), "rem:snooze:"+d.ID)))
	}
	return maxapi.WithKeyboard(text, kb)
}

// recipients — активные участники, кроме автора действия, и траектория
// для голоса. Ошибка чтения — пустой список: уведомление не критично.
func (n *Notifier) recipients(ctx context.Context, trajectoryID, except string) ([]store.Recipient, store.Trajectory, bool) {
	t, err := n.Store.Trajectory(ctx, trajectoryID)
	if err != nil {
		slog.Warn("уведомление: траектория", "err", err)
		return nil, t, false
	}
	rs, err := n.Store.ActiveRecipients(ctx, trajectoryID, except)
	if err != nil {
		slog.Warn("уведомление: участники", "err", err)
		return nil, t, false
	}
	return rs, t, true
}

// Registered — «✅ Ольга отмечает: регистрация на «…» пройдена» остальным (F46).
func (n *Notifier) Registered(ctx context.Context, trajectoryID, itemID string, by store.Member) {
	if n.Max == nil {
		return
	}
	item, err := n.Store.TrackerItem(ctx, trajectoryID, itemID)
	if err != nil {
		return
	}
	rs, t, ok := n.recipients(ctx, trajectoryID, by.MemberID)
	if !ok {
		return
	}
	for _, r := range rs {
		v := Voice(r, t)
		n.Send(ctx, r.MaxUserID, maxapi.WithKeyboard(
			v.T("notify.registered", voice.Vars{"name": by.FirstName, "title": Short(item.OlympiadName)}),
			maxapi.Keyboard{maxapi.Row(n.App(v.T("bot.menu.tracker", nil), "tracker"))}))
	}
}

// ProposalMessage — предложение ученику с кнопками ответа (F45).
func (n *Notifier) ProposalMessage(ctx context.Context, p store.ProposalRow, r store.Recipient, t store.Trajectory, now time.Time) maxapi.NewMessage {
	v := Voice(r, t)
	by := ""
	if p.ProposedBy != nil {
		by = p.ProposedBy.Name
	}
	text := v.T("notify.proposal", voice.Vars{"name": by, "title": Short(p.OlympiadName)})
	if st, err := n.Store.StagesFor(ctx, []string{p.ProfileID}); err == nil {
		if s, ok := NextDeadline(st[p.ProfileID], false, now); ok {
			text += " " + v.T("bot.deadlineLine", voice.Vars{"date": stages.Day(s.DeadlineAt.In(location(t.TZ)))})
		}
	}
	return maxapi.WithKeyboard(text, maxapi.Keyboard{
		maxapi.Row(maxapi.CallbackButton(v.T("bot.prop.accept", nil), "prop:acc:"+p.ID)),
		maxapi.Row(n.App(v.T("bot.prop.card", nil), "o_"+p.OlympiadID)),
		maxapi.Row(maxapi.CallbackButton(v.T("bot.prop.decline", nil), "prop:dec:"+p.ID)),
	})
}

// ProposalCreated — ученику приходит предложение родителя (F45).
func (n *Notifier) ProposalCreated(ctx context.Context, trajectoryID, proposalID string, by store.Member, now time.Time) {
	if n.Max == nil {
		return
	}
	p, err := n.Store.Proposal(ctx, trajectoryID, proposalID)
	if err != nil {
		return
	}
	rs, t, ok := n.recipients(ctx, trajectoryID, by.MemberID)
	if !ok {
		return
	}
	for _, r := range rs {
		if r.Role == "kid" {
			n.Send(ctx, r.MaxUserID, n.ProposalMessage(ctx, p, r, t, now))
		}
	}
}

// ProposalResolved — автору предложения: ученик добавил или отложил (F45).
func (n *Notifier) ProposalResolved(ctx context.Context, trajectoryID, proposalID string, accepted bool) {
	if n.Max == nil {
		return
	}
	p, err := n.Store.Proposal(ctx, trajectoryID, proposalID)
	if err != nil || p.ProposedBy == nil {
		return
	}
	rs, t, ok := n.recipients(ctx, trajectoryID, "")
	if !ok {
		return
	}
	key := "notify.proposalDeclined"
	if accepted {
		key = "notify.proposalAccepted"
	}
	for _, r := range rs {
		if r.MemberID == p.ProposedBy.ID {
			v := Voice(r, t)
			n.Send(ctx, r.MaxUserID, maxapi.WithKeyboard(v.T(key, voice.Vars{"title": Short(p.OlympiadName)}),
				maxapi.Keyboard{maxapi.Row(n.App(v.T("bot.menu.tracker", nil), "tracker"))}))
		}
	}
}

// MemberRemoved — удалённому участнику (F41): доступа больше нет.
func (n *Notifier) MemberRemoved(ctx context.Context, trajectoryID string, removed store.FamilyMember, by store.Member) {
	if n.Max == nil {
		return
	}
	t, err := n.Store.Trajectory(ctx, trajectoryID)
	if err != nil {
		return
	}
	v := voice.New(voice.Role(removed.Role), t.StudentName, removed.Name)
	n.Send(ctx, removed.MaxUserID, maxapi.Text(v.T("notify.removed", voice.Vars{"name": by.FirstName})))
}

// Left — остальным: участник вышел сам (F48).
func (n *Notifier) Left(ctx context.Context, trajectoryID string, left store.FamilyMember) {
	n.broadcast(ctx, trajectoryID, left.ID, "notify.left", voice.Vars{"name": left.Name})
}

// Joined — остальным: по приглашению вошёл новый участник (F42).
func (n *Notifier) Joined(ctx context.Context, m store.Member) {
	n.broadcast(ctx, m.TrajectoryID, m.MemberID, "notify.joined", voice.Vars{"name": m.FirstName})
}

func (n *Notifier) broadcast(ctx context.Context, trajectoryID, except, key string, vars voice.Vars) {
	if n.Max == nil {
		return
	}
	rs, t, ok := n.recipients(ctx, trajectoryID, except)
	if !ok {
		return
	}
	for _, r := range rs {
		n.Send(ctx, r.MaxUserID, maxapi.Text(Voice(r, t).T(key, vars)))
	}
}

// Deleted — бывшим участникам удалённой траектории (F50). Список получают
// до удаления: после него траектория уже не «живая».
func (n *Notifier) Deleted(ctx context.Context, t store.Trajectory, rs []store.Recipient) {
	for _, r := range rs {
		n.Send(ctx, r.MaxUserID, maxapi.Text(Voice(r, t).T("notify.deleted", nil)))
	}
}
