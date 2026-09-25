package assistant

import (
	"cmp"
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
// по всем профилям и всем вузам базы. asked — вузы из вопроса: тогда условия
// только их, остальные вузы с льготами — списком. Строки соседних вузов
// модель путает с условиями того, о котором спросили.
func (a *Assistant) olympiadText(ctx context.Context, b base, oid string, asked []string, c clock) (string, []store.BenefitRow, dates, error) {
	ps := slices.Clone(b.profiles[oid])
	slices.SortStableFunc(ps, func(x, y store.Profile) int { return strings.Compare(profileTitle(x), profileTitle(y)) })
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	byProfile, err := a.Store.StagesFor(ctx, ids)
	if err != nil {
		return "", nil, dates{}, err
	}
	benefits, err := a.Store.Benefits(ctx, ids, nil)
	if err != nil {
		return "", nil, dates{}, err
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

	var schedules []grouped
	var d dates
	for _, x := range ps {
		schedules = group(schedules, c.dated(byProfile[x.ID]), profileTitle(x))
		d.add(short, byProfile[x.ID])
	}
	lines = append(lines, "Этапы:")
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
	var nicks, others []string
	for _, x := range ps {
		for _, bn := range benefits {
			if bn.ProfileID != x.ID {
				continue
			}
			nick := pick.Nick(bn.UniversityID, bn.UniversityShort)
			if len(asked) > 0 && !slices.Contains(asked, bn.UniversityID) {
				if !slices.Contains(others, nick) {
					others = append(others, nick)
				}
				continue
			}
			if _, ok := byUni[nick]; !ok {
				nicks = append(nicks, nick)
			}
			byUni[nick] = group(byUni[nick], grantText(bn), profileTitle(x))
		}
	}
	byName := func(x, y string) int { return strings.Compare(names.Key(x), names.Key(y)) }
	slices.SortFunc(nicks, byName)
	for _, nick := range nicks {
		lines = append(lines, "  "+nick+" — "+joinGroups(byUni[nick]))
	}
	if len(benefits) > 0 {
		for _, id := range asked {
			if u, ok := b.uni[id]; ok && !slices.Contains(nicks, pick.Nick(id, u.ShortName)) {
				lines = append(lines, "  "+pick.Nick(id, u.ShortName)+" — по этой олимпиаде ничего не даёт")
			}
		}
	}
	if len(others) > 0 {
		slices.SortFunc(others, byName)
		lines = append(lines, "Льготы по ней есть и в других вузах базы (условия не показаны, вопрос не про них): "+strings.Join(others, ", "))
	}
	lines = append(lines, conditionsText[p.Kind])
	return strings.Join(lines, "\n"), benefits, d, nil
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

// dated — расписание с пометкой, фактические даты или примерные: модель
// обязана сказать это в ответе.
func (c clock) dated(st []stages.Stage) string {
	switch {
	case len(st) == 0:
		return "этапов нет"
	case approximate(st):
		return "даты примерные, по прошлому году: " + c.schedule(st)
	}
	return "даты фактические: " + c.schedule(st)
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
		case from == nil && to != nil:
			// Известен только крайний срок — «до», как в приложении.
			when = "до " + c.day(*to)
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
			// Текущий в приложении — ближайший непрошедший; идёт он, только
			// если уже начался. Не начался — говорим прямо: по одним датам
			// модель пишет «регистрация открыта».
			if s.StartsAt == nil || !s.StartsAt.After(c.now) {
				tags = append(tags, "идёт сейчас")
			} else {
				tags = append(tags, "ещё не идёт")
			}
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
		g += " (данные уточняются)"
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
// олимпиад по предмету, уровню, классу, городу. subjects — предметы из
// вопроса: тогда вместо всех олимпиад — только их профили по этим предметам,
// с уровнем и классами в строке олимпиады. Сводить классы из двух списков по
// всей базе модель не умеет: называет олимпиады не для того класса.
func (b base) catalogText(subjects []string) string {
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
		len(b.olympiads), count["perechen"], count["vsosh"], count["other"]))
	order := slices.Clone(b.olympiads)
	short := func(oid string) string { return names.Olympiad(b.profiles[oid][0].OlympiadName) }
	slices.SortFunc(order, func(x, y string) int { return strings.Compare(names.Key(short(x)), names.Key(short(y))) })
	kinds := map[string]string{"perechen": "перечень", "vsosh": "ВсОШ", "other": "вне перечня"}
	// tail — формат и город финала в конце строки олимпиады.
	tail := func(p store.Profile) []string {
		var parts []string
		if p.Format != nil {
			parts = append(parts, strings.ToLower(*p.Format))
		}
		if p.FinalCity != nil {
			parts = append(parts, "финал: "+*p.FinalCity)
		}
		return parts
	}
	found := false
	for _, code := range subjects {
		var own []string
		name := ""
		for _, oid := range order {
			if i := slices.IndexFunc(b.profiles[oid], func(p store.Profile) bool { return p.SubjectCode == code }); i >= 0 {
				own = append(own, oid)
				name = b.profiles[oid][i].SubjectName
			}
		}
		if len(own) == 0 {
			continue
		}
		found = true
		lines = append(lines, fmt.Sprintf("Олимпиады по предмету «%s»: %d. По уровням перечня:", name, len(own)))
		lines = append(lines, b.bySubject(own, code)...)
		lines = append(lines, "По олимпиадам (вид; профили по предмету — уровень, классы; формат; финал):")
		for _, oid := range own {
			ps := b.profiles[oid]
			var profiles []string
			for _, p := range ps {
				if p.SubjectCode != code {
					continue
				}
				about := []string{gradesText(p.GradesFrom, p.GradesTo)}
				if p.Level != nil {
					about = slices.Insert(about, 0, *p.Level+" уровень")
				}
				profiles = append(profiles, strings.ToLower(profileTitle(p))+" ("+strings.Join(about, ", ")+")")
			}
			parts := append([]string{kinds[ps[0].Kind], "профили: " + strings.Join(profiles, ", ")}, tail(ps[0])...)
			lines = append(lines, "  "+short(oid)+" — "+strings.Join(parts, "; "))
		}
	}
	if found {
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "Олимпиады по предметам и уровням (в скобках — профиль, если он называется иначе, чем предмет):")
	lines = append(lines, b.bySubject(order, "")...)
	lines = append(lines, "Олимпиады (вид; классы; формат; финал):")
	for _, oid := range order {
		ps := b.profiles[oid]
		from, to := ps[0].GradesFrom, ps[0].GradesTo
		for _, p := range ps {
			from, to = min(from, p.GradesFrom), max(to, p.GradesTo)
		}
		parts := append([]string{kinds[ps[0].Kind], gradesText(from, to)}, tail(ps[0])...)
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

// bySubject — строки «Физика, I уровень: Физтех, Росатом, …»: на поиск
// по предмету и уровню модель отвечает одной строкой, а не перебором всех
// олимпиад. order — олимпиады в порядке вывода; subject — только профили
// этого предмета, "" — все.
func (b base) bySubject(order []string, subject string) []string {
	type key struct{ subject, level string }
	rows := map[key][]string{}
	var keys []key
	for _, oid := range order {
		for _, p := range b.profiles[oid] {
			if subject != "" && p.SubjectCode != subject {
				continue
			}
			k := key{p.SubjectName, "вне перечня"}
			switch {
			case p.Level != nil:
				k.level = *p.Level + " уровень"
			case p.Kind == "vsosh":
				k.level = "ВсОШ"
			}
			name := names.Olympiad(p.OlympiadName)
			if !sameAsSubject(p) {
				name += " (" + strings.ToLower(profileTitle(p)) + ")"
			}
			if _, ok := rows[k]; !ok {
				keys = append(keys, k)
			}
			if !slices.Contains(rows[k], name) {
				rows[k] = append(rows[k], name)
			}
		}
	}
	rank := map[string]int{"ВсОШ": 0, "I уровень": 1, "II уровень": 2, "III уровень": 3}
	slices.SortFunc(keys, func(x, y key) int {
		return cmp.Or(strings.Compare(names.Key(x.subject), names.Key(y.subject)), cmp.Compare(rankOr(rank, x.level), rankOr(rank, y.level)))
	})
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = "  " + k.subject + ", " + k.level + ": " + strings.Join(rows[k], ", ")
	}
	return out
}

func rankOr(rank map[string]int, level string) int {
	if r, ok := rank[level]; ok {
		return r
	}
	return len(rank)
}

var goalText = map[string]string{
	"known":     "цель выбрана",
	"suggested": "цель подобрана сервисом по интересам",
	"exploring": "ученик пока выбирает цель",
}

// studentCard — карточка ученика: класс, цель, предметы, вузы и трекер с
// ближайшими этапами. Без имени: в модель уходит только то, что нужно для
// ответа. Источники — сайты олимпиад трекера, ближайшие по срокам первыми:
// там регистрация и даты.
func (a *Assistant) studentCard(ctx context.Context, b base, t store.Trajectory, c *collected) error {
	lines := []string{fmt.Sprintf("Класс: %d", t.Grade)}
	goal := goalText[t.GoalStatus]
	if len(t.Directions) > 0 {
		directions := make([]string, len(t.Directions))
		for i, d := range t.Directions {
			directions[i] = d.Name
		}
		goal += "; направления: " + strings.Join(directions, ", ")
	}
	lines = append(lines, "Цель: "+goal, "Предметы: "+orDash(strings.Join(c.subjects, ", ")), "Вузы: "+orDash(strings.Join(c.universities, ", ")))
	items, err := a.Store.TrackerItems(ctx, t.ID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		c.cards = append(c.cards, card{id: "student", text: strings.Join(append(lines, "Трекер: пусто"), "\n")})
		return nil
	}
	ids := make([]string, len(items))
	for i, x := range items {
		ids[i] = x.ProfileID
	}
	byProfile, err := a.Store.StagesFor(ctx, ids)
	if err != nil {
		return err
	}
	var d dates
	type next struct {
		p    store.Profile
		ends time.Time
	}
	var soon []next
	lines = append(lines, "Трекер:")
	for _, x := range items {
		line := "  " + names.Olympiad(x.OlympiadName) + ", " + profileLabel(store.Profile{SubjectName: x.SubjectName, ProfileName: x.ProfileName})
		if x.RegisteredAt != nil {
			line += " (регистрация отмечена)"
		}
		st := byProfile[x.ProfileID]
		if i := slices.IndexFunc(stages.States(st, x.RegisteredAt != nil, c.clock.now), func(s string) bool { return s != "past" }); i >= 0 {
			line += "; ближайший этап — " + c.clock.dated(st[i:i+1])
			d.add(names.Olympiad(x.OlympiadName), st[i:i+1])
			if p, ok := b.profile[x.ProfileID]; ok {
				soon = append(soon, next{p, *cmp.Or(st[i].EndsAt, st[i].DeadlineAt, st[i].StartsAt, &c.clock.now)})
			}
		}
		lines = append(lines, line)
	}
	slices.SortStableFunc(soon, func(x, y next) int { return x.ends.Compare(y.ends) })
	var sources []store.Source
	for _, n := range soon {
		if !slices.ContainsFunc(sources, func(s store.Source) bool { return s.ID == "site-"+n.p.OlympiadID }) {
			sources = appendSite(sources, n.p)
		}
	}
	c.cards = append(c.cards, card{id: "student", text: strings.Join(lines, "\n"), dates: d, sources: sources})
	return nil
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
