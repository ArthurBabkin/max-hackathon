package bot

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/access"
	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// Config — то, что бот знает о себе.
type Config struct {
	BotName      string // для ссылок https://max.ru/<bot>?start=...
	BotID        int64  // contact_id кнопки open_app
	ReminderHour int
}

// Bot — сценарии чата поверх store: онбординг, команды, кнопки из
// напоминаний и уведомлений. Транспорт (вебхук или long polling) снаружи.
type Bot struct {
	store  *store.Store
	max    maxapi.Sender
	notify *notify.Notifier
	cfg    Config
	now    func() time.Time
}

func New(st *store.Store, max maxapi.Sender, cfg Config) *Bot {
	return &Bot{store: st, max: max, cfg: cfg, now: time.Now,
		notify: &notify.Notifier{Store: st, Max: max, BotName: cfg.BotName, BotID: cfg.BotID, ReminderHour: cfg.ReminderHour}}
}

// turn — одно обновление от одного пользователя.
type turn struct {
	ctx    context.Context
	user   maxapi.User
	userID string // users.id
}

// Handle обрабатывает обновление. Повторную доставку того же обновления
// (MAX повторяет, если не дождался 200) отсекает ключ в bot_updates_seen.
func (b *Bot) Handle(ctx context.Context, u maxapi.Update) error {
	from, ok := u.From()
	if !ok || from.IsBot {
		return nil
	}
	// Бот работает только в личном диалоге: в групповом чате пришлось бы
	// отвечать всем сразу, а сценарии личные.
	if u.Message != nil && u.Message.Recipient.ChatType != "" && u.Message.Recipient.ChatType != "dialog" {
		return nil
	}
	first, err := b.store.MarkUpdateSeen(ctx, u.DedupKey())
	if err != nil || !first {
		return err
	}
	name := strings.TrimSpace(from.FirstName)
	if name == "" {
		name = "Друг"
	}
	uid, err := b.store.UpsertUser(ctx, from.UserID, name)
	if err != nil {
		return err
	}
	from.FirstName = name
	t := &turn{ctx: ctx, user: from, userID: uid}

	switch u.UpdateType {
	case maxapi.UpdateBotStarted:
		payload := ""
		if u.Payload != nil {
			payload = *u.Payload
		}
		return b.start(t, payload)
	case maxapi.UpdateMessageCreated:
		return b.message(t, u.Message)
	case maxapi.UpdateMessageCallback:
		return b.callback(t, u.Callback, u.Message)
	case maxapi.UpdateBotStopped, maxapi.UpdateDialogRemoved:
		slog.Info("пользователь остановил бота", "update_type", u.UpdateType)
	}
	return nil
}

// member — текущее участие пользователя; ok = false, если траектории нет.
func (b *Bot) member(t *turn) (store.Member, bool, error) {
	m, err := b.store.CurrentMember(t.ctx, t.user.UserID)
	if errors.Is(err, store.ErrNotFound) {
		return m, false, nil
	}
	return m, err == nil, err
}

func permissions(m store.Member) access.Permissions {
	return access.For(access.Member{Role: access.Role(m.Role), IsCreator: m.IsCreator, HasKid: m.HasKid})
}

// voiceOf — голос участника траектории.
func (b *Bot) voiceOf(t *turn, m store.Member) (voice.Voice, store.Trajectory, error) {
	tr, err := b.store.Trajectory(t.ctx, m.TrajectoryID)
	if err != nil {
		return voice.Voice{}, tr, err
	}
	return voice.New(voice.Role(m.Role), tr.StudentName, t.user.FirstName), tr, nil
}

// kidVoice — голос до выбора роли: к незнакомому пользователю бот
// обращается как к ученику, на «ты» (основная аудитория).
func kidVoice(t *turn) voice.Voice { return voice.New(voice.Kid, "", t.user.FirstName) }

func (b *Bot) send(t *turn, m maxapi.NewMessage) (string, error) {
	return b.max.Send(t.ctx, t.user.UserID, m)
}

func (b *Bot) say(t *turn, text string) error {
	_, err := b.send(t, maxapi.Text(text))
	return err
}

func (b *Bot) app(text, payload string) maxapi.Button {
	return maxapi.OpenAppButton(text, b.cfg.BotName, b.cfg.BotID, payload)
}

// message — текст, команда или геопозиция.
func (b *Bot) message(t *turn, msg *maxapi.Message) error {
	if msg == nil {
		return nil
	}
	if lat, lon, ok := msg.Location(); ok {
		return b.location(t, lat, lon)
	}
	text := strings.TrimSpace(msg.Body.Text)
	if strings.HasPrefix(text, "/") {
		cmd, arg, _ := strings.Cut(text, " ")
		// «/start@bot» в группах; в диалоге суффикса нет, но срезать не вредно.
		cmd, _, _ = strings.Cut(cmd, "@")
		return b.command(t, strings.ToLower(cmd), strings.TrimSpace(arg))
	}
	return b.freeText(t, text)
}

func (b *Bot) command(t *turn, cmd, arg string) error {
	switch cmd {
	case "/start":
		return b.start(t, arg)
	case "/menu":
		return b.menu(t, false)
	case "/help":
		return b.help(t)
	case "/settings":
		return b.settings(t)
	case "/delete":
		return b.deleteCommand(t)
	}
	return b.unknown(t)
}

func (b *Bot) unknown(t *turn) error {
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
	return b.say(t, v.T("bot.unknown", nil))
}

// start — /start и bot_started (F1). Параметр inv_<token> — вход по
// приглашению; любой другой сохраняется как источник, битый — игнорируется.
// Без согласия с политикой конфиденциальности — сначала оно, параметр ждёт
// в диалоге.
func (b *Bot) start(t *turn, payload string) error {
	token, invite := strings.CutPrefix(payload, "inv_")
	invite = invite && inviteTokenRe.MatchString(token)
	if !invite {
		_, ok, err := b.member(t)
		if err != nil {
			return err
		}
		if ok {
			return b.menu(t, true)
		}
	}
	var source *string
	if sourceRe.MatchString(payload) {
		source = &payload
	}
	accepted, err := b.store.PrivacyAccepted(t.ctx, t.userID)
	if err != nil {
		return err
	}
	if !accepted {
		d, err := b.store.StartDialog(t.ctx, store.Dialog{UserID: t.userID, Step: stepConsent, SourcePayload: source})
		if err != nil {
			return err
		}
		return b.ask(t, d)
	}
	if invite {
		return b.join(t, token)
	}
	d, err := b.store.StartDialog(t.ctx, store.Dialog{UserID: t.userID, Step: stepRole, SourcePayload: source})
	if err != nil {
		return err
	}
	return b.ask(t, d)
}

var (
	// Параметр ссылки MAX: до 128 символов; берём только безопасные.
	sourceRe      = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	inviteTokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]{12,64}$`)
	uuidRe        = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// menu — /menu (F51); welcome — приветствие вернувшегося по /start.
func (b *Bot) menu(t *turn, welcome bool) error {
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
	title := v.T("bot.menu.title", nil)
	if welcome {
		title = v.T("bot.welcomeBack", nil)
	}
	_, err = b.send(t, maxapi.WithKeyboard(title, maxapi.Keyboard{
		maxapi.Row(b.app(v.T("bot.menu.app", nil), "home")),
		maxapi.Row(b.app(v.T("bot.menu.tracker", nil), "tracker"), b.app(v.T("bot.menu.family", nil), "family")),
		maxapi.Row(maxapi.CallbackButton(v.T("bot.menu.settings", nil), "set:open")),
	}))
	return err
}

func (b *Bot) help(t *turn) error {
	m, ok, err := b.member(t)
	if err != nil {
		return err
	}
	v := kidVoice(t)
	if ok {
		if v, _, err = b.voiceOf(t, m); err != nil {
			return err
		}
	}
	// Политика конфиденциальности — по ссылке из /help (ТЗ §12).
	_, err = b.send(t, maxapi.WithKeyboard(v.T("bot.help", nil), maxapi.Keyboard{
		maxapi.Row(maxapi.LinkButton(v.T("privacy.link", nil), privacyURL))}))
	return err
}

// Commands — подсказки меню бота (F51), их ставит cmd/setup.
func Commands() []maxapi.Command {
	v := voice.New(voice.Kid, "", "")
	return []maxapi.Command{
		{Name: "menu", Description: v.T("bot.cmd.menu", nil)},
		{Name: "settings", Description: v.T("bot.cmd.settings", nil)},
		{Name: "help", Description: v.T("bot.cmd.help", nil)},
		{Name: "delete", Description: v.T("bot.cmd.delete", nil)},
	}
}
