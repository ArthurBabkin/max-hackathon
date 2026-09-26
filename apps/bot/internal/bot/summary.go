package bot

import (
	"slices"
	"strings"

	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
	"github.com/ArthurBabkin/max-hackathon/packages/shared/maxapi"
)

// editFields — что можно поменять из карточки профиля (SPEC 9): поле и
// шаг, к которому оно возвращает. Роли нет: она меняет голос всех
// текстов, для этого есть /start заново.
var editFields = []struct{ code, step string }{
	{"name", stepNameInput},
	{"grade", stepGrade},
	{"region", stepRegion},
	{"subjects", stepSubjects},
	{"goal", stepDirection},
	{"exp", stepExperience},
	{"places", stepTarget},
	{"vuz", stepUniversities},
}

// updateDialog — store.UpdateDialog для шагов онбординга: при правке из
// карточки профиля диалог возвращается к профилю, как только уходит
// дальше правленого шага.
func (b *Bot) updateDialog(t *turn, apply func(tx *store.Store, d *store.Dialog) error) (store.Dialog, error) {
	return b.store.UpdateDialog(t.ctx, t.userID, func(tx *store.Store, d *store.Dialog) error {
		if err := apply(tx, d); err != nil {
			return err
		}
		backToSummary(d)
		return nil
	})
}

// backToSummary — правка закончена, если шаг ушёл дальше правленого:
// «Помоги выбрать» — тот же шаг направления, «Изменить места» с шага
// вузов — шаг назад, и то и другое правку не заканчивает.
func backToSummary(d *store.Dialog) {
	if d.Draft.EditStage == 0 {
		return
	}
	if n, ok := stepNumbers[d.Step]; ok && n <= d.Draft.EditStage {
		return
	}
	d.Step, d.Draft.EditStage = stepSummary, 0
}

// draftProfile — профиль из черновика: траектории до «Готово» ещё нет.
func (b *Bot) draftProfile(t *turn, dr store.Draft) (profile, error) {
	subs, err := b.store.AllSubjects(t.ctx)
	if err != nil {
		return profile{}, err
	}
	dirs, err := b.store.Directions(t.ctx)
	if err != nil {
		return profile{}, err
	}
	unis, err := b.store.FindUniversities(t.ctx, "")
	if err != nil {
		return profile{}, err
	}
	places, _ := draftPlaces(dr)
	p := profile{
		Grade: dr.Grade, RegionCode: dr.RegionCode, HomeCity: dr.HomeCity,
		// Как в createTrajectory.
		Suggested:  len(dr.Suggested) > 0 && len(dr.DirectionIDs) > 0,
		GoalByKid:  dr.GoalByKid && len(dr.DirectionIDs) == 0,
		Experience: dr.Experience, Places: places,
	}
	for _, s := range subs {
		if slices.Contains(dr.SubjectCodes, s.Code) {
			p.Subjects = append(p.Subjects, strings.ToLower(s.Name))
		}
	}
	for _, id := range dr.DirectionIDs {
		if i := slices.IndexFunc(dirs, func(x store.Direction) bool { return x.ID == id }); i >= 0 {
			p.Directions = append(p.Directions, dirs[i])
		}
	}
	for _, id := range dr.UniversityIDs {
		if i := slices.IndexFunc(unis, func(u store.University) bool { return u.ID == id }); i >= 0 {
			p.Universities = append(p.Universities, unis[i])
		}
	}
	return p, nil
}

// summaryPrompt — «Артём, собрали твой профиль»: профиль списком и
// «Изменить» / «Готово»; в меню правки — поля и «Назад».
func (b *Bot) summaryPrompt(t *turn, v voice.Voice, d store.Dialog) (maxapi.NewMessage, error) {
	p, err := b.draftProfile(t, d.Draft)
	if err != nil {
		return maxapi.NewMessage{}, err
	}
	cb := maxapi.CallbackButton
	title := "bot.summary.title"
	if d.Role == "parent" && strings.TrimSpace(t.user.FirstName) == "" {
		title = "bot.summary.titleNoName"
	}
	text := v.T(title, nil) + "\n" + strings.Join(profileLines(v, p, true), "\n") + "\n\n"
	kb := maxapi.Keyboard{maxapi.Row(cb(v.T("bot.summary.edit", nil), "sum:edit"), cb(v.T("bot.summary.done", nil), "sum:ok"))}
	if d.Draft.EditMenu {
		var buttons []maxapi.Button
		for _, f := range editFields {
			buttons = append(buttons, cb(v.T("bot.edit."+f.code, nil), "sum:f:"+f.code))
		}
		kb = append(grid(buttons, 2), maxapi.Row(cb(v.T("bot.edit.back", nil), "sum:back")))
		text += v.T("bot.edit.ask", nil)
	} else {
		text += v.T("bot.summary.check", nil)
	}
	msg := maxapi.WithKeyboard(text, kb)
	msg.Format = "markdown"
	return msg, nil
}

// summaryCallback — кнопки карточки профиля: меню правки, поле, «Готово».
func (b *Bot) summaryCallback(t *turn, cb *maxapi.Callback, question *maxapi.Message, action, code string) error {
	v := b.dialogVoiceOf(t)
	switch action {
	case "edit", "back":
		// Меню правки открывается на той же карточке.
		return b.transition(t, cb, question, "", func(_ *store.Store, d *store.Dialog) error {
			if err := expect(d, stepSummary); err != nil {
				return err
			}
			d.Draft.EditMenu = action == "edit"
			return nil
		})
	case "ok":
		return b.transition(t, cb, question, v.T("bot.summary.done", nil), func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepSummary); err != nil {
				return err
			}
			return b.createTrajectory(t, tx, d)
		})
	case "f":
		i := slices.IndexFunc(editFields, func(f struct{ code, step string }) bool { return f.code == code })
		if i < 0 {
			return b.stale(t, cb, nil)
		}
		step := editFields[i].step
		return b.transition(t, cb, question, v.T("bot.edit."+code, nil), func(tx *store.Store, d *store.Dialog) error {
			if err := expect(d, stepSummary); err != nil {
				return err
			}
			d.Step, d.Draft.EditMenu, d.Draft.EditStage = step, false, stepNumbers[step]
			switch step {
			case stepRegion:
				d.Draft.Letter = ""
			case stepDirection:
				d.Draft.AllDirections = false
			case stepTarget:
				d.Draft.PlaceOptions, d.Draft.Found = placeOptions(d.Draft), nil
				if places, _ := draftPlaces(d.Draft); len(d.Draft.Places) == 0 && !d.Draft.PlacesAny {
					// Диалог v1: место было в Target.
					d.Draft.Places = places
				}
				d.Draft.PlacesAny = d.Draft.PlacesAny || d.Draft.Target == store.TargetAny
				d.Draft.Target = ""
			case stepUniversities:
				d.Draft.OfferPage, d.Draft.VuzAnywhere, d.Draft.Found = 0, false, nil
				return offer(t, tx, d)
			}
			return nil
		})
	}
	return b.stale(t, cb, nil)
}
