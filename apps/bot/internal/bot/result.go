package bot

import (
	"errors"
	"strings"
	"time"

	"github.com/ArthurBabkin/max-hackathon/packages/core/notify"
	"github.com/ArthurBabkin/max-hackathon/packages/core/pick"
	"github.com/ArthurBabkin/max-hackathon/packages/core/refdata"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// finished — траектория создана по «Готово» в карточке профиля: итог с
// числом подобранных олимпиад (SPEC 9).
func (b *Bot) finished(t *turn) error {
	m, ok, err := b.member(t)
	if err != nil || !ok {
		return err
	}
	v, tr, err := b.voiceOf(t, m)
	if err != nil {
		return err
	}
	return b.sendResult(t, m, v, tr, "")
}

// profile — что показывает карточка профиля: из траектории или, до её
// создания, из черновика онбординга.
type profile struct {
	Grade        int
	RegionCode   string
	HomeCity     string
	Subjects     []string // названия со строчной
	Directions   []store.Direction
	Suggested    bool // направления предложил бот
	GoalByKid    bool
	Experience   string
	Places       []store.Place
	Universities []store.University
}

// trajectoryProfile — профиль созданной траектории.
func (b *Bot) trajectoryProfile(t *turn, tr store.Trajectory) (profile, error) {
	subs, err := b.store.TrajectorySubjects(t.ctx, tr.ID)
	if err != nil {
		return profile{}, err
	}
	unis, err := b.store.TrajectoryUniversities(t.ctx, tr.ID)
	if err != nil {
		return profile{}, err
	}
	p := profile{
		Grade: tr.Grade, RegionCode: tr.RegionCode, Directions: tr.Directions,
		Suggested: tr.GoalStatus == "suggested", GoalByKid: tr.GoalByKid,
		Experience: tr.Experience, Places: tr.Places, Universities: unis,
	}
	if tr.HomeCity != nil {
		p.HomeCity = *tr.HomeCity
	}
	for _, s := range subs {
		p.Subjects = append(p.Subjects, strings.ToLower(s.Name))
	}
	return p, nil
}

// profileLines — профиль списком: класс и место, предметы, цель, опыт,
// где учиться, вузы. withEmptyVuz = false — без строки «Вузы», если их нет.
func profileLines(v voice.Voice, p profile, withEmptyVuz bool) []string {
	place := refdata.Short(p.RegionCode)
	if p.HomeCity != "" {
		place = p.HomeCity
	}
	grade := v.T("bot.summary.grade", voice.Vars{"grade": p.Grade, "place": place})
	if place == "" {
		grade = v.T("bot.summary.gradeOnly", voice.Vars{"grade": p.Grade})
	}
	lines := []string{
		grade,
		v.T("bot.summary.subjects", voice.Vars{"subjects": strings.Join(p.Subjects, ", ")}),
		v.T("bot.summary.goal", voice.Vars{"direction": goalText(v, p)}),
	}
	if p.Experience != "" {
		lines = append(lines, v.T("bot.summary.experience", voice.Vars{"experience": v.T("bot.exp.label."+p.Experience, nil)}))
	}
	lines = append(lines, v.T("bot.summary.places", voice.Vars{"places": placesText(v, p.Places)}))
	if len(p.Universities) > 0 || withEmptyVuz {
		names := make([]string, len(p.Universities))
		for i, u := range p.Universities {
			names[i] = uniLabel(u)
		}
		vuzy := strings.Join(names, ", ")
		if len(names) == 0 {
			vuzy = v.T("bot.vuz.noneChosen", nil)
		}
		lines = append(lines, v.T("bot.summary.universities", voice.Vars{"names": vuzy}))
	}
	return lines
}

// goalText — цель в профиле: направления, «подобрали вместе», если их
// предложил бот, «ждёт ответа», если родитель передал вопрос ребёнку.
func goalText(v voice.Voice, p profile) string {
	if len(p.Directions) == 0 && p.GoalByKid {
		return v.T("bot.summary.waitKid", nil)
	}
	if len(p.Directions) == 0 {
		return v.T("bot.join.noGoal", nil)
	}
	names := make([]string, len(p.Directions))
	for i, d := range p.Directions {
		names[i] = lowerFirst(d.Name)
	}
	text := strings.Join(names, ", ")
	if p.Suggested {
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

// resultText — «Подобрали 7 олимпиад под тебя.» (SPEC 9). Число — сколько
// карточек в «Подборе» мини-приложения: тот же расчёт core/pick, ВсОШ
// входит в него. Уровни, сроки и стартовую олимпиаду бот не называет:
// о них рассказывает мини-приложение.
func (b *Bot) resultText(t *turn, v voice.Voice, tr store.Trajectory) (string, pick.Result, error) {
	res, err := pick.Recommend(t.ctx, b.store, tr, b.now(), "all")
	if err != nil {
		return "", res, err
	}
	if len(res.Items) == 0 {
		return v.T("bot.result.empty", nil), res, nil
	}
	key := "bot.result.found"
	if len(tr.Directions) == 0 {
		// Цели нет — подбор по предметам.
		key = "bot.result.foundSubjects"
	}
	return v.T(key, voice.Vars{"count": voice.OlympiadsAcc(len(res.Items))}), res, nil
}

// waitsForKid — родитель передал вопросы об интересах ребёнку, а ребёнка в
// траектории ещё нет: бот обещал ссылку-приглашение.
func waitsForKid(m store.Member, tr store.Trajectory) bool {
	return m.Role == "parent" && !m.HasKid && tr.GoalByKid && len(tr.Directions) == 0 && permissions(m).Invite
}

// resultKeyboard — одна кнопка: мини-приложение на главной, где новичку
// покажут обучение. Напоминания и приглашения — там же, о них бот пока
// не говорит. Исключение — родитель ждёт ответа ребёнка: приглашение первым.
func (b *Bot) resultKeyboard(v voice.Voice, m store.Member, tr store.Trajectory) maxapi.Keyboard {
	kb := maxapi.Keyboard{maxapi.Row(b.app(v.T("bot.btn.openApp", nil), "home"))}
	if waitsForKid(m, tr) {
		invite := maxapi.Row(maxapi.CallbackButton(v.T("bot.btn.inviteKid", nil), "inv:new"))
		return append(maxapi.Keyboard{invite}, kb...)
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
	if waitsForKid(m, tr) {
		text += "\n\n" + v.T("bot.result.kidInvite", nil)
	}
	_, err = b.send(t, maxapi.WithKeyboard(text, b.resultKeyboard(v, m, tr)))
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
	// Кнопка — из итога прежних версий: сообщение не трогаем.
	if err := b.max.Answer(t.ctx, cb.CallbackID, maxapi.CallbackAnswer{Notification: v.T("bot.btn.remindOn", nil)}); err != nil {
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
	p, err := b.trajectoryProfile(t, tr)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	msg := maxapi.WithKeyboard(v.T("bot.join.checkTitle", nil)+"\n"+strings.Join(profileLines(v, p, false), "\n"),
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
