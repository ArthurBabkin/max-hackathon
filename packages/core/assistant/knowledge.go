package assistant

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ArthurBabkin/max-hackathon/packages/core/names"
	"github.com/ArthurBabkin/max-hackathon/packages/core/pick"
	"github.com/ArthurBabkin/max-hackathon/packages/core/stages"
	"github.com/ArthurBabkin/max-hackathon/packages/core/voice"
	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

// Карточки базы знаний — текст для модели, собранный из тех же данных, что
// страницы приложения: одна карточка на олимпиаду со всеми профилями,
// этапами и льготами во всех вузах базы, одна на вуз со всеми олимпиадами.
// Текст, а не JSON с полями: так карточка короче, а модель читает её как
// справку.

// base — каталог базы на один вопрос: все олимпиады с профилями и все вузы.
// Из него — классы Jev, поиск названий, каталог и карточки.
type base struct {
	olympiads []string                   // id олимпиад в порядке официальных названий
	profiles  map[string][]store.Profile // олимпиада → профили
	profile   map[string]store.Profile   // профиль по id
	unis      []store.University
	uni       map[string]store.University
}

func (a *Assistant) loadBase(ctx context.Context) (base, error) {
	b := base{profiles: map[string][]store.Profile{}, profile: map[string]store.Profile{}, uni: map[string]store.University{}}
	ps, err := a.Store.Profiles(ctx, store.ProfileQuery{})
	if err != nil {
		return b, err
	}
	for _, p := range ps {
		if _, ok := b.profiles[p.OlympiadID]; !ok {
			b.olympiads = append(b.olympiads, p.OlympiadID)
		}
		b.profiles[p.OlympiadID] = append(b.profiles[p.OlympiadID], p)
		b.profile[p.ID] = p
	}
	if b.unis, err = a.Store.FindUniversities(ctx, ""); err != nil {
		return b, err
	}
	for _, u := range b.unis {
		b.uni[u.ID] = u
	}
	return b, nil
}

// named — олимпиады и вузы для поиска названий в тексте вопроса.
func (b base) named() (olympiads, universities []Named) {
	for _, oid := range b.olympiads {
		olympiads = append(olympiads, Named{ID: oid, Name: b.profiles[oid][0].OlympiadName})
	}
	for _, u := range b.unis {
		universities = append(universities, Named{ID: u.ID, Name: u.Name, Aliases: []string{u.ShortName}})
	}
	return olympiads, universities
}

func (b base) profileIDs() []string {
	ids := make([]string, 0, len(b.profile))
	for _, oid := range b.olympiads {
		for _, p := range b.profiles[oid] {
			ids = append(ids, p.ID)
		}
	}
	return ids
}

// clock — «сейчас» и часовой пояс ученика: даты этапов и что уже прошло.
type clock struct {
	now time.Time
	loc *time.Location
}

func (c clock) day(t time.Time) string { return t.In(c.loc).Format("02.01.2006") }

var kindText = map[string]string{
	"perechen": "перечневая олимпиада: входит в перечень олимпиад школьников Минобрнауки",
	"vsosh":    "Всероссийская олимпиада школьников (ВсОШ)",
	"other":    "олимпиада вне перечня Минобрнауки",
}

var conditionsText = map[string]string{
	"perechen": "Условия: нужен диплом победителя или призёра; БВИ можно использовать только в одном вузе.",
	"vsosh":    "Условия: нужен диплом победителя или призёра заключительного этапа; БВИ можно использовать только в одном вузе.",
	"other":    "Условия: льгот при поступлении не даёт; дополнительные баллы — не больше 10 в сумме за все достижения.",
}

// olympiadText — карточка олимпиады, как её лист в приложении, только сразу
// по всем профилям и всем вузам базы.
func (a *Assistant) olympiadText(ctx context.Context, b base, oid string, c clock) (string, []store.BenefitRow, error) {
	ps := slices.Clone(b.profiles[oid])
	slices.SortStableFunc(ps, func(x, y store.Profile) int { return strings.Compare(profileTitle(x), profileTitle(y)) })
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	byProfile, err := a.Store.StagesFor(ctx, ids)
	if err != nil {
		return "", nil, err
	}
	benefits, err := a.Store.Benefits(ctx, ids, nil)
	if err != nil {
		return "", nil, err
	}
	p := ps[0]
	short := names.Olympiad(p.OlympiadName)
	lines := []string{short}
	if strings.Trim(p.OlympiadName, "«»") != short {
		lines = append(lines, "Официальное название: "+p.OlympiadName)
	}
	head := []string{kindText[p.Kind]}
	if p.Organizer != nil {
		head = append(head, "организатор "+*p.Organizer)
	}
	if p.Format != nil {
		head = append(head, "формат: "+strings.ToLower(*p.Format))
	}
	if p.FinalCity != nil {
		head = append(head, "финал: "+*p.FinalCity)
	} else {
		head = append(head, "город финала не указан")
	}
	lines = append(lines, strings.Join(head, "; "))
	if p.OfficialURL != nil {
		lines = append(lines, "Сайт: "+*p.OfficialURL)
	}
	if p.Description != nil {
		lines = append(lines, "Описание: "+*p.Description)
	}
	profiles := make([]string, len(ps))
	for i, x := range ps {
		profiles[i] = profileTitle(x)
		if x.Level != nil {
			profiles[i] += " — " + *x.Level + " уровень"
		}
		profiles[i] += ", " + gradesText(x.GradesFrom, x.GradesTo)
	}
	lines = append(lines, "Профили: "+strings.Join(profiles, "; "))

	demo := false
	var schedules []grouped
	for _, x := range ps {
		st := byProfile[x.ID]
		demo = demo || len(st) == 0 || slices.ContainsFunc(st, func(s stages.Stage) bool { return s.IsDemo })
		schedules = group(schedules, c.schedule(st), profileTitle(x))
	}
	if demo {
		lines = append(lines, "Этапы (даты предварительные, по прошлому году):")
	} else {
		lines = append(lines, "Этапы:")
	}
	for _, g := range schedules {
		who := strings.Join(g.names, ", ")
		if len(schedules) == 1 {
			who = "Все профили"
		}
		lines = append(lines, "  "+who+": "+g.text)
	}

	year := 0
	for _, bn := range benefits {
		year = max(year, bn.AdmissionYear)
	}
	switch {
	case len(benefits) == 0 && p.Kind != "other":
		lines = append(lines, "Льготы при поступлении: ни один вуз из базы льготу не даёт.")
	case len(benefits) > 0 && p.Kind == "other":
		lines = append(lines, fmt.Sprintf("Дополнительные баллы при поступлении в %d году (вуз — профили: условие):", year))
	case len(benefits) > 0:
		lines = append(lines, fmt.Sprintf("Льготы при поступлении в %d году (вуз — профили: условие):", year))
	}
	byUni := map[string][]grouped{}
	var nicks []string
	for _, x := range ps {
		for _, bn := range benefits {
			if bn.ProfileID != x.ID {
				continue
			}
			nick := pick.Nick(bn.UniversityID, bn.UniversityShort)
			if _, ok := byUni[nick]; !ok {
				nicks = append(nicks, nick)
			}
			byUni[nick] = group(byUni[nick], grantText(bn), profileTitle(x))
		}
	}
	slices.SortFunc(nicks, func(x, y string) int { return strings.Compare(names.Key(x), names.Key(y)) })
	for _, nick := range nicks {
		lines = append(lines, "  "+nick+" — "+joinGroups(byUni[nick]))
	}
	lines = append(lines, conditionsText[p.Kind])
	return strings.Join(lines, "\n"), benefits, nil
}

// grouped — одинаковый текст у нескольких профилей: пишем его один раз.
type grouped struct {
	text  string
	names []string
}

func group(gs []grouped, text, name string) []grouped {
	i := slices.IndexFunc(gs, func(g grouped) bool { return g.text == text })
	if i < 0 {
		return append(gs, grouped{text: text, names: []string{name}})
	}
	gs[i].names = append(gs[i].names, name)
	return gs
}

func joinGroups(gs []grouped) string {
	parts := make([]string, len(gs))
	for i, g := range gs {
		parts[i] = strings.Join(g.names, ", ") + ": " + g.text
	}
	return strings.Join(parts, " | ")
}

// schedule — этапы профиля одной строкой: даты, онлайн, прошёл ли этап.
func (c clock) schedule(st []stages.Stage) string {
	if len(st) == 0 {
		return "этапов нет"
	}
	states := stages.States(st, false, c.now)
	parts := make([]string, len(st))
	for i, s := range st {
		from, to := s.StartsAt, s.EndsAt
		if to == nil {
			to = s.DeadlineAt
		}
		var when string
		switch {
		case from != nil && to != nil && c.day(*from) != c.day(*to):
			when = c.day(*from) + "–" + c.day(*to)
		case to != nil:
			when = c.day(*to)
		case from != nil:
			when = c.day(*from)
		default:
			when = "дата не указана"
		}
		var tags []string
		if s.IsOnline {
			tags = append(tags, "онлайн")
		}
		switch states[i] {
		case "past":
			tags = append(tags, "прошёл")
		case "current":
			tags = append(tags, "идёт сейчас")
		}
		parts[i] = stageName(s) + " " + when
		if len(tags) > 0 {
			parts[i] += " (" + strings.Join(tags, ", ") + ")"
		}
	}
	return strings.Join(parts, " → ")
}

// grantNotes — оговорки примечания, которые grantText уже сказал словами
// «победителю … призёру …».
var grantNotes = []string{"Победителю — БВИ, призёру — 100 баллов", "БВИ только победителю", "100 баллов только победителю"}

// grantText — условие льготы в вузе: что получат победитель и призёр,
// порог ЕГЭ, класс диплома и оговорки вуза.
func grantText(bn store.BenefitRow) string {
	label := func(kind string) string {
		switch kind {
		case "extra_points":
			if bn.ExtraPoints == nil {
				return "дополнительные баллы"
			}
			return fmt.Sprintf("+%d %s", *bn.ExtraPoints, voice.Plural(*bn.ExtraPoints, "балл", "балла", "баллов"))
		case "score100":
			return "100 баллов"
		}
		return "БВИ"
	}
	var g string
	switch w, p := pick.Grants(bn); {
	case w == "":
		g = "льготы нет"
	case w == p:
		g = "победителю и призёру — " + label(w)
	case p == "":
		g = "победителю — " + label(w) + "; призёру льготы нет"
	default:
		g = "победителю — " + label(w) + "; призёру — " + label(p)
	}
	if bn.EgeMin != nil {
		g += fmt.Sprintf(", ЕГЭ от %d", *bn.EgeMin)
	}
	if len(bn.DiplomaGrades) > 0 {
		g += ", диплом за " + gradesList(bn.DiplomaGrades) + " класс"
	}
	var notes []string
	for _, s := range pick.NoteSentences(bn.Note) {
		if s != "" && !slices.ContainsFunc(grantNotes, func(n string) bool { return strings.HasPrefix(s, n) }) {
			notes = append(notes, s)
		}
	}
	if len(notes) > 0 {
		g += ". " + strings.Join(notes, ". ")
	}
	if bn.Source == nil {
		g += " [источник не указан]"
	}
	return g
}

// universityText — карточка вуза. full — со всеми олимпиадами и условиями;
// иначе только шапка: условия вуза уже есть в карточке олимпиады.
func (a *Assistant) universityText(ctx context.Context, b base, trajectoryID, id string, full bool, c clock) (string, error) {
	d, err := a.Store.University(ctx, trajectoryID, id)
	if err != nil {
		return "", err
	}
	lines := []string{uniTitle(d.University)}
	if n, ok := universityNames[id]; ok && n.full != d.Name {
		lines = append(lines, "Полное название: "+n.full)
	}
	if d.Description != nil {
		lines = append(lines, "О вузе: "+*d.Description)
	}
	if d.SiteURL != nil {
		lines = append(lines, "Сайт: "+*d.SiteURL)
	}
	if d.RulesVerifiedAt != nil {
		lines = append(lines, "Правила приёма: проверены "+c.day(*d.RulesVerifiedAt))
	} else {
		lines = append(lines, "Правила приёма: не проверены")
	}
	if len(d.Directions) > 0 {
		lines = append(lines, "Направления: "+strings.Join(d.Directions, ", "))
	}
	if d.EgeNote != nil {
		lines = append(lines, "Порог ЕГЭ для подтверждения олимпиадной льготы: "+*d.EgeNote)
	}
	if !full {
		return strings.Join(lines, "\n"), nil
	}
	rows, err := a.Store.Benefits(ctx, b.profileIDs(), []string{id})
	if err != nil {
		return "", err
	}
	byOlympiad := map[string][]grouped{}
	var order []string
	for _, bn := range rows {
		p, ok := b.profile[bn.ProfileID]
		if !ok {
			continue
		}
		if _, ok := byOlympiad[p.OlympiadID]; !ok {
			order = append(order, p.OlympiadID)
		}
		byOlympiad[p.OlympiadID] = group(byOlympiad[p.OlympiadID], grantText(bn), profileTitle(p))
	}
	// ВсОШ первой, вне перечня — в конце, внутри — по алфавиту.
	rank := map[string]int{"vsosh": 0, "perechen": 1, "other": 2}
	short := func(oid string) string { return names.Olympiad(b.profiles[oid][0].OlympiadName) }
	slices.SortFunc(order, func(x, y string) int {
		if r := rank[b.profiles[x][0].Kind] - rank[b.profiles[y][0].Kind]; r != 0 {
			return r
		}
		return strings.Compare(names.Key(short(x)), names.Key(short(y)))
	})
	if len(order) == 0 {
		lines = append(lines, "Олимпиады с льготами: в базе нет ни одной.")
	} else {
		lines = append(lines, fmt.Sprintf("Олимпиады с льготами или баллами (%d олимпиад, %d профилей) — профили: условие:", len(order), len(rows)))
	}
	for _, oid := range order {
		lines = append(lines, "  "+short(oid)+" — "+joinGroups(byOlympiad[oid]))
	}
	return strings.Join(lines, "\n"), nil
}

// catalogText — что есть в базе: на «какие вузы у тебя есть» и поиск
// олимпиад по предмету, уровню, классу, городу.
func (b base) catalogText() string {
	count := map[string]int{}
	for _, oid := range b.olympiads {
		count[b.profiles[oid][0].Kind]++
	}
	lines := []string{
		fmt.Sprintf("В базе сервиса %d %s и %d %s. Льготы при поступлении собраны только для этих вузов.",
			len(b.unis), voice.Plural(len(b.unis), "вуз", "вуза", "вузов"),
			len(b.olympiads), voice.Plural(len(b.olympiads), "олимпиада", "олимпиады", "олимпиад")),
		"Вузы (олимпиад с БВИ или 100 баллами):",
	}
	for _, u := range b.unis {
		lines = append(lines, "  "+uniTitle(u)+fmt.Sprintf(" — %d", u.BenefitOlympiads))
	}
	lines = append(lines, fmt.Sprintf("Олимпиад в базе: %d — из перечня Минобрнауки %d, ВсОШ %d, вне перечня %d.",
		len(b.olympiads), count["perechen"], count["vsosh"], count["other"]),
		"Олимпиады (вид; профили с уровнем; классы; формат; финал):")
	order := slices.Clone(b.olympiads)
	short := func(oid string) string { return names.Olympiad(b.profiles[oid][0].OlympiadName) }
	slices.SortFunc(order, func(x, y string) int { return strings.Compare(names.Key(short(x)), names.Key(short(y))) })
	kinds := map[string]string{"perechen": "перечень", "vsosh": "ВсОШ", "other": "вне перечня"}
	for _, oid := range order {
		ps := b.profiles[oid]
		from, to := ps[0].GradesFrom, ps[0].GradesTo
		var profiles []string
		for _, p := range ps {
			label := strings.ToLower(profileTitle(p))
			if p.Level != nil {
				label += " " + *p.Level
			}
			profiles = append(profiles, label)
			from, to = min(from, p.GradesFrom), max(to, p.GradesTo)
		}
		slices.Sort(profiles)
		parts := []string{kinds[ps[0].Kind], strings.Join(profiles, ", "), gradesText(from, to)}
		if ps[0].Format != nil {
			parts = append(parts, strings.ToLower(*ps[0].Format))
		}
		if ps[0].FinalCity != nil {
			parts = append(parts, "финал: "+*ps[0].FinalCity)
		}
		lines = append(lines, "  "+short(oid)+" — "+strings.Join(parts, "; "))
	}
	return strings.Join(lines, "\n")
}

// uniTitle — «НИУ ВШЭ (ВШЭ), Москва»; сокращение — если оно другое.
func uniTitle(u store.University) string {
	title := u.Name
	if u.ShortName != u.Name {
		title += " (" + u.ShortName + ")"
	}
	if u.City != nil {
		title += ", " + *u.City
	}
	return title
}

var goalText = map[string]string{
	"known":     "цель выбрана",
	"suggested": "цель подобрана сервисом по интересам",
	"exploring": "ученик пока выбирает цель",
}

// studentText — карточка ученика: класс, цель, предметы, вузы и трекер с
// ближайшими этапами. Без имени: в модель уходит только то, что нужно для
// ответа.
func (a *Assistant) studentText(ctx context.Context, t store.Trajectory, subjects, unis []string, c clock) (string, error) {
	lines := []string{fmt.Sprintf("Класс: %d", t.Grade)}
	goal := goalText[t.GoalStatus]
	if len(t.Directions) > 0 {
		directions := make([]string, len(t.Directions))
		for i, d := range t.Directions {
			directions[i] = d.Name
		}
		goal += "; направления: " + strings.Join(directions, ", ")
	}
	lines = append(lines, "Цель: "+goal, "Предметы: "+orDash(strings.Join(subjects, ", ")), "Вузы: "+orDash(strings.Join(unis, ", ")))
	items, err := a.Store.TrackerItems(ctx, t.ID)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return strings.Join(append(lines, "Трекер: пусто"), "\n"), nil
	}
	ids := make([]string, len(items))
	for i, x := range items {
		ids[i] = x.ProfileID
	}
	byProfile, err := a.Store.StagesFor(ctx, ids)
	if err != nil {
		return "", err
	}
	lines = append(lines, "Трекер:")
	for _, x := range items {
		line := "  " + names.Olympiad(x.OlympiadName) + ", " + profileLabel(store.Profile{SubjectName: x.SubjectName, ProfileName: x.ProfileName})
		if x.RegisteredAt != nil {
			line += " (регистрация отмечена)"
		}
		st := byProfile[x.ProfileID]
		if i := slices.IndexFunc(stages.States(st, x.RegisteredAt != nil, c.now), func(s string) bool { return s != "past" }); i >= 0 {
			line += "; ближайший этап: " + c.schedule(st[i:i+1])
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), nil
}

// profileTitle — «Информатика», «Промышленное программирование»: название
// профиля, а если оно совпадает с предметом — предмет.
func profileTitle(p store.Profile) string {
	if sameAsSubject(p) || *p.ProfileName == "" {
		return p.SubjectName
	}
	r, size := utf8.DecodeRuneInString(*p.ProfileName)
	return string(unicode.ToUpper(r)) + (*p.ProfileName)[size:]
}

// gradesText — «9–11 классы», «11 класс».
func gradesText(from, to int) string {
	if from == to {
		return fmt.Sprintf("%d класс", from)
	}
	return fmt.Sprintf("%d–%d классы", from, to)
}

// gradesList — классы подряд через тире, иначе через запятую: «11», «9–11», «9, 11».
func gradesList(grades []int32) string {
	contiguous := true
	for i := 1; i < len(grades); i++ {
		contiguous = contiguous && grades[i] == grades[i-1]+1
	}
	if len(grades) > 1 && contiguous {
		return fmt.Sprintf("%d–%d", grades[0], grades[len(grades)-1])
	}
	parts := make([]string, len(grades))
	for i, g := range grades {
		parts[i] = fmt.Sprint(g)
	}
	return strings.Join(parts, ", ")
}
