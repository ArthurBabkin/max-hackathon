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
	// Направления подготовки: для поиска в вопросе и карточки направления.
	directions []Direction
	direction  map[string]Direction
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
	ds, err := a.Store.AllDirections(ctx)
	if err != nil {
		return b, err
	}
	b.direction = make(map[string]Direction, len(ds))
	for _, d := range ds {
		x := Direction{ID: d.ID, Code: d.Code, Name: d.Name}
		b.directions = append(b.directions, x)
		b.direction[d.ID] = x
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

// scope — чьи условия показывает карточка олимпиады: вузы из вопроса и
// предметы ученика на «в моих вузах»; пусто — все. Строки соседних вузов и
// других предметов модель путает с теми, о которых спросили.
type scope struct{ universities, subjects []string }

// subjectsOf — предметы охвата, которые у олимпиады есть; нет ни одного —
// пусто, то есть все.
func (s scope) subjectsOf(ps []store.Profile) []string {
	return slices.DeleteFunc(slices.Clone(s.subjects), func(code string) bool {
		return !slices.ContainsFunc(ps, func(x store.Profile) bool { return x.SubjectCode == code })
	})
}

// olympiadText — карточка олимпиады, как её лист в приложении, только сразу
// по всем профилям и всем вузам базы. s — охват: условия только вузов из
// вопроса (остальные вузы с льготами — списком) и только по предметам
// ученика.
func (a *Assistant) olympiadText(ctx context.Context, b base, oid string, s scope, c clock) (string, []store.BenefitRow, dates, error) {
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
	// Как плашка в приложении: по одним датам модель советовала олимпиаду,
	// в которую уже не вступить.
	var closed []string
	for _, x := range ps {
		if !stages.Joinable(byProfile[x.ID], c.now) {
			closed = append(closed, profileTitle(x))
		}
	}
	switch {
	case len(closed) > 0 && len(closed) == len(ps):
		lines = append(lines, "Регистрация закрыта: срок первого этапа прошёл, в этом сезоне в олимпиаду не вступить.")
	case len(closed) > 0:
		lines = append(lines, "Регистрация закрыта по профилям: "+strings.Join(closed, ", ")+" — срок первого этапа прошёл.")
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
	subjects := s.subjectsOf(ps)
	if len(subjects) > 0 && len(benefits) > 0 {
		var titles []string
		for _, code := range subjects {
			if i := slices.IndexFunc(ps, func(x store.Profile) bool { return x.SubjectCode == code }); i >= 0 {
				titles = append(titles, strings.ToLower(ps[i].SubjectName))
			}
		}
		lines = append(lines, "Показаны условия по предметам ученика: "+strings.Join(titles, ", ")+".")
	}
	byUni := map[string][]grouped{}
	var nicks, others []string
	for _, x := range ps {
		if len(subjects) > 0 && !slices.Contains(subjects, x.SubjectCode) {
			continue
		}
		for _, bn := range benefits {
			if bn.ProfileID != x.ID {
				continue
			}
			nick := pick.Nick(bn.UniversityID, bn.UniversityShort)
			if len(s.universities) > 0 && !slices.Contains(s.universities, bn.UniversityID) {
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
		nothing := " — по этой олимпиаде ничего не даёт"
		if len(subjects) > 0 {
			nothing = " — по этим предметам ничего не даёт"
		}
		for _, id := range s.universities {
			if u, ok := b.uni[id]; ok && !slices.Contains(nicks, pick.Nick(id, u.ShortName)) {
				lines = append(lines, "  "+pick.Nick(id, u.ShortName)+nothing)
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
	states := stages.States(st, stages.Progress{}, c.now)
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
			// модель пишет «регистрация открыта», а «ещё не идёт» читает как
			// «ещё идёт».
			if s.StartsAt == nil || !s.StartsAt.After(c.now) {
				tags = append(tags, "идёт сейчас")
			} else {
				tags = append(tags, "сейчас не идёт, начнётся "+c.day(*s.StartsAt))
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
	g := grants(bn)
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

// grants — что получат победитель и призёр: «победителю — БВИ; призёру —
// 100 баллов».
func grants(bn store.BenefitRow) string {
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
	switch w, p := pick.Grants(bn); {
	case w == "":
		return "льготы нет"
	case w == p:
		return "победителю и призёру — " + label(w)
	case p == "":
		return "победителю — " + label(w) + "; призёру льготы нет"
	default:
		return "победителю — " + label(w) + "; призёру — " + label(p)
	}
}

// universityText — карточка вуза, как его лист в приложении: направления
// с программами и олимпиадами, на что ученику здесь считаются льготы
// (core/targets). full — со всеми олимпиадами и условиями: сначала на
// направления ученика, остальные — отдельно; иначе только шапка со сводкой
// по уровням: условия вуза по олимпиаде вопроса уже есть в её карточке.
// asked — направления из вопроса: тогда льготы — на них, а не на цель
// ученика; нет их в вузе — только шапка.
func (a *Assistant) universityText(ctx context.Context, b base, trajectoryID, id string, asked []string, full bool, c clock) (string, error) {
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
	dirs, err := a.Store.UniversityDirections(ctx, trajectoryID, id)
	if err != nil {
		return "", err
	}
	l := lens{who: "направления ученика", mark: "направление ученика"}
	var tg map[string]store.UniversityTarget
	if len(asked) > 0 {
		l = lens{who: "направление из вопроса", mark: "направление из вопроса", asked: b.directionNames(asked)}
		tg, err = a.Store.TargetsOn(ctx, asked, []string{id})
	} else {
		tg, err = a.Store.TargetsOf(ctx, trajectoryID, []string{id})
	}
	if err != nil {
		return "", err
	}
	t := tg[id]
	lines = append(lines, directionLines(dirs, t, l)...)
	if l.asked != nil && t.Basis == "university" {
		return strings.Join(lines, "\n"), nil
	}
	if d.EgeNote != nil {
		lines = append(lines, "Порог ЕГЭ для подтверждения олимпиадной льготы: "+*d.EgeNote)
	}
	rows, err := a.Store.Benefits(ctx, b.profileIDs(), []string{id})
	if err != nil {
		return "", err
	}
	// Льготы на направления ученика (или из вопроса) — когда они в вузе
	// есть и проверены; иначе — вуза целиком, как «Все» в приложении.
	onDirs := t.Basis != "university" && !t.Unverified
	var mine []store.BenefitRow
	if onDirs {
		if l.asked != nil {
			mine, err = a.Store.BenefitsOn(ctx, asked, b.profileIDs(), []string{id})
		} else {
			mine, err = a.Store.TargetBenefits(ctx, trajectoryID, b.profileIDs(), []string{id})
		}
		if err != nil {
			return "", err
		}
		lines = append(lines, b.byLevel(mine, " на "+l.who)...)
	} else {
		lines = append(lines, b.byLevel(rows, "")...)
	}
	if !full {
		return strings.Join(lines, "\n"), nil
	}
	cov, err := a.Store.DirectionCoverage(ctx, b.profileIDs(), []string{id})
	if err != nil {
		return "", err
	}
	coverage := func(profileID string) string {
		x, ok := cov[profileID+"/"+id]
		if !ok || x.Count >= x.Total {
			return ""
		}
		return fmt.Sprintf(" (на %d из %d направлений)", x.Count, x.Total)
	}
	short := func(oid string) string { return names.Olympiad(b.profiles[oid][0].OlympiadName) }
	if !onDirs {
		byOlympiad, order := b.byOlympiad(rows, func(bn store.BenefitRow) (string, string) {
			p := b.profile[bn.ProfileID]
			return grantText(bn), profileTitle(p) + coverage(bn.ProfileID)
		})
		if len(order) == 0 {
			lines = append(lines, "Олимпиады с льготами: в базе нет ни одной.")
		} else {
			lines = append(lines, fmt.Sprintf("Олимпиады с льготами или баллами в вузе целиком (%d олимпиад, %d профилей) — профили (на скольких направлениях вуза, если не на всех): условие:", len(order), len(rows)))
		}
		for _, oid := range order {
			lines = append(lines, "  "+short(oid)+" — "+joinGroups(byOlympiad[oid]))
		}
		return strings.Join(lines, "\n"), nil
	}
	byOlympiad, order := b.byOlympiad(mine, func(bn store.BenefitRow) (string, string) {
		return grantText(bn) + directionNotes(bn, true), profileTitle(b.profile[bn.ProfileID])
	})
	who := strings.Join(t.DirectionNames, ", ")
	if len(order) == 0 {
		lines = append(lines, "Олимпиады с льготой на "+l.who+" ("+who+"): в базе нет ни одной.")
	} else {
		lines = append(lines, fmt.Sprintf("Олимпиады с льготой на %s (%s) (%d олимпиад, %d профилей) — профили: условие:", l.who, who, len(order), len(mine)))
	}
	for _, oid := range order {
		lines = append(lines, "  "+short(oid)+" — "+joinGroups(byOlympiad[oid]))
	}
	// Остальные олимпиады вуза — с тем, на скольких направлениях они дают
	// льготу: у профилей одной олимпиады берётся самый широкий охват.
	best := map[string]store.Coverage{}
	var others []string
	for _, bn := range rows {
		p, ok := b.profile[bn.ProfileID]
		if !ok || byOlympiad[p.OlympiadID] != nil {
			continue
		}
		if _, seen := best[p.OlympiadID]; !seen {
			others = append(others, p.OlympiadID)
		}
		if x := cov[bn.ProfileID+"/"+id]; x.Count >= best[p.OlympiadID].Count {
			best[p.OlympiadID] = x
		}
	}
	b.sortOlympiads(others)
	if len(others) > 0 {
		parts := make([]string, len(others))
		for i, oid := range others {
			parts[i] = short(oid)
			if x := best[oid]; x.Total > 0 {
				parts[i] += fmt.Sprintf(" (на %d из %d направлений)", x.Count, x.Total)
			}
		}
		lines = append(lines, fmt.Sprintf("Только на другие направления вуза, не на %s (%d олимпиад): %s", l.who, len(others), strings.Join(parts, ", ")))
	}
	return strings.Join(lines, "\n"), nil
}

// lens — на чьи направления смотрит карточка вуза: ученика или из вопроса.
type lens struct {
	who, mark string
	asked     []string // названия направлений из вопроса; nil — цель ученика
}

// directionLines — направления вуза: код, программы, бюджетные места,
// олимпиады с льготой или «льготы уточняются»; направления ученика (или из
// вопроса) помечены, у них — названия программ. Дальше — на что здесь
// считаются льготы.
func directionLines(dirs []store.UniversityDirection, t store.UniversityTarget, l lens) []string {
	var lines []string
	if len(dirs) > 0 {
		lines = append(lines, fmt.Sprintf("Направления вуза (%d) — программ, олимпиад с льготой:", len(dirs)))
	}
	for _, d := range dirs {
		var parts []string
		if d.Programs > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", d.Programs, voice.Plural(d.Programs, "программа", "программы", "программ")))
		}
		if d.BudgetPlaces != nil {
			parts = append(parts, fmt.Sprintf("%d %s", *d.BudgetPlaces, voice.Plural(*d.BudgetPlaces, "бюджетное место", "бюджетных места", "бюджетных мест")))
		}
		if d.Status == "to_check" {
			parts = append(parts, "льготы уточняются")
		} else {
			parts = append(parts, fmt.Sprintf("%d %s с льготой", d.BenefitOlympiads, voice.Plural(d.BenefitOlympiads, "олимпиада", "олимпиады", "олимпиад")))
		}
		line := "  " + d.Code + " " + d.Name + " — " + strings.Join(parts, ", ")
		target := slices.Contains(t.DirectionIDs, d.ID)
		switch {
		case target && l.asked != nil:
			line += "; " + l.mark
		case target:
			line += "; " + l.mark + " (" + basisText[t.Basis] + ")"
		}
		lines = append(lines, line)
		if target && len(d.ProgramNames) > 0 {
			lines = append(lines, "    программы: "+strings.Join(d.ProgramNames, "; "))
		}
	}
	names := strings.Join(t.DirectionNames, ", ")
	switch {
	case l.asked != nil && (t.Basis == "university" || len(t.DirectionIDs) == 0):
		lines = append(lines, "Направления из вопроса ("+strings.Join(l.asked, ", ")+") в этом вузе нет.")
	case l.asked != nil && t.Unverified:
		lines = append(lines, "Льготы здесь считаются на направление из вопроса: "+names+" — льготы на него ещё уточняются; ниже — льготы вуза целиком.")
	case l.asked != nil:
		lines = append(lines, "Льготы здесь считаются на направление из вопроса: "+names+".")
	case t.Basis == "university" || len(t.DirectionIDs) == 0:
		lines = append(lines, "Ни выбранных в вузе, ни из цели ученика направлений здесь нет — льготы указаны по вузу целиком.")
	case t.Unverified:
		lines = append(lines, "Льготы ученику здесь считаются на направления: "+strings.Join(t.DirectionNames, ", ")+
			" ("+basisText[t.Basis]+") — льготы на них ещё уточняются; ниже — льготы вуза целиком.")
	default:
		lines = append(lines, "Льготы ученику здесь считаются на направления: "+strings.Join(t.DirectionNames, ", ")+" ("+basisText[t.Basis]+").")
	}
	return lines
}

// directionNotes — к льготе на направления ученика: зависит ли она от
// программы (если показанное примечание вуза этого не сказало) и более
// слабая льгота на других его направлениях.
func directionNotes(r store.BenefitRow, noteShown bool) string {
	var s string
	if r.Varies && (!noteShown || r.Note == nil || !strings.Contains(*r.Note, "Зависит от программы")) {
		s += "; зависит от программы"
	}
	for _, o := range r.OtherDirections {
		s += "; " + otherText[o.Benefit] + " — на " + strings.Join(o.Names, ", ")
	}
	return s
}

// byOlympiad группирует профили каждой олимпиады по условию: text — условие
// и подпись профиля. Олимпиады — в порядке sortOlympiads.
func (b base) byOlympiad(rows []store.BenefitRow, text func(store.BenefitRow) (string, string)) (map[string][]grouped, []string) {
	out := map[string][]grouped{}
	var order []string
	for _, bn := range rows {
		p, ok := b.profile[bn.ProfileID]
		if !ok {
			continue
		}
		if _, ok := out[p.OlympiadID]; !ok {
			order = append(order, p.OlympiadID)
		}
		grant, name := text(bn)
		out[p.OlympiadID] = group(out[p.OlympiadID], grant, name)
	}
	b.sortOlympiads(order)
	return out, order
}

// sortOlympiads — ВсОШ первой, вне перечня — в конце, внутри — по алфавиту.
func (b base) sortOlympiads(order []string) {
	rank := map[string]int{"vsosh": 0, "perechen": 1, "other": 2}
	short := func(oid string) string { return names.Olympiad(b.profiles[oid][0].OlympiadName) }
	slices.SortFunc(order, func(x, y string) int {
		if r := rank[b.profiles[x][0].Kind] - rank[b.profiles[y][0].Kind]; r != 0 {
			return r
		}
		return strings.Compare(names.Key(short(x)), names.Key(short(y)))
	})
}

// directionNames — названия направлений по id.
func (b base) directionNames(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = b.direction[id].Name
	}
	return out
}

// askedText — льгота профилей олимпиады на направления из вопроса: в вузах
// из вопроса, иначе во всех вузах базы с этими направлениями. Вузы из
// вопроса без направления — списком.
func (a *Assistant) askedText(ctx context.Context, b base, asked, named []string, ps []store.Profile) ([]string, error) {
	unis := named
	if len(unis) == 0 {
		for _, u := range b.unis {
			unis = append(unis, u.ID)
		}
	}
	tg, err := a.Store.TargetsOn(ctx, asked, unis)
	if err != nil {
		return nil, err
	}
	var offered, without []string
	for _, u := range unis {
		if tg[u].Basis == "university" {
			without = append(without, pick.Nick(u, b.uni[u].ShortName))
			continue
		}
		offered = append(offered, u)
	}
	lines := []string{"Льгота на направление из вопроса (" + strings.Join(b.directionNames(asked), ", ") + ") — главнее льготы вуза целиком:"}
	if len(offered) > 0 {
		ids := make([]string, len(ps))
		for i, p := range ps {
			ids[i] = p.ID
		}
		rows, err := a.Store.BenefitsOn(ctx, asked, ids, offered)
		if err != nil {
			return nil, err
		}
		byPair := map[string]store.BenefitRow{}
		for _, r := range rows {
			byPair[r.ProfileID+"/"+r.UniversityID] = r
		}
		for _, p := range ps {
			indent := "  "
			if len(ps) > 1 {
				lines = append(lines, "  "+profileTitle(p)+":")
				indent = "    "
			}
			for _, u := range offered {
				who := indent + pick.Nick(u, b.uni[u].ShortName) + ": "
				switch r, ok := byPair[p.ID+"/"+u]; {
				case !ok:
					lines = append(lines, who+"льготы нет")
				case r.Unverified:
					lines = append(lines, who+"льготы на направление уточняются")
				default:
					line := who + grants(r)
					if r.EgeMin != nil {
						line += fmt.Sprintf(", ЕГЭ от %d", *r.EgeMin)
					}
					if n := strings.TrimPrefix(directionNotes(r, false), "; "); n != "" {
						line += " (" + n + ")"
					}
					lines = append(lines, line)
				}
			}
		}
	}
	if len(named) > 0 && len(without) > 0 {
		lines = append(lines, "  Нет этого направления: "+strings.Join(without, ", "))
	}
	return lines, nil
}

// directionText — карточка направления: в каких вузах базы оно есть (с
// укрупнённой группой — тоже), сколько олимпиад дают на нём льготу или
// «льготы уточняются», вузы ученика помечены; вузы без направления —
// списком.
func (a *Assistant) directionText(ctx context.Context, b base, t store.Trajectory, myUnis []string, id string) (string, error) {
	d := b.direction[id]
	head := "Направление " + d.Code + " " + d.Name
	if slices.ContainsFunc(t.Directions, func(x store.Direction) bool { return x.ID == id }) {
		head += " — в цели ученика"
	}
	lines := []string{head}
	unis, match, err := a.Store.UniversitiesByDirection(ctx, t.ID, "", "", id)
	if err != nil {
		return "", err
	}
	if len(unis) == 0 {
		lines = append(lines, "В вузах базы этого направления нет.")
	} else {
		lines = append(lines, fmt.Sprintf("Вузы базы с этим направлением (%d), олимпиад с льготой на нём:", len(unis)))
	}
	for _, u := range unis {
		m := match[u.ID]
		line := "  " + pick.Nick(u.ID, u.ShortName)
		if u.City != nil && *u.City != line[2:] {
			line += ", " + *u.City
		}
		line += " — "
		for _, other := range m.DirectionIDs {
			if other == id {
				continue
			}
			x := b.direction[other]
			line += "как " + x.Code + " " + x.Name
			if strings.HasSuffix(x.Code, ".00.00") {
				line += " (укрупнённая группа)"
			}
			line += ", "
		}
		if m.Unverified {
			line += "льготы уточняются"
		} else {
			line += fmt.Sprintf("%d %s с льготой", m.Olympiads, voice.Plural(m.Olympiads, "олимпиада", "олимпиады", "олимпиад"))
		}
		if slices.Contains(myUnis, u.ID) {
			line += "; вуз ученика"
		}
		lines = append(lines, line)
	}
	var without []string
	for _, u := range b.unis {
		if _, ok := match[u.ID]; !ok {
			without = append(without, pick.Nick(u.ID, u.ShortName))
		}
	}
	if len(without) > 0 {
		lines = append(lines, "Этого направления нет: "+strings.Join(without, ", "))
	}
	return strings.Join(lines, "\n"), nil
}

// byLevel — льготы вуза по уровням перечня: сколько профилей что дают. Без
// сводки на «какой минимальный уровень даёт БВИ?» модель обобщала по одной
// олимпиаде из контекста.
func (b base) byLevel(rows []store.BenefitRow, scope string) []string {
	type key struct{ level, grant string }
	count := map[key]int{}
	var keys []key
	for _, bn := range rows {
		p, ok := b.profile[bn.ProfileID]
		if !ok {
			continue
		}
		level := "вне перечня"
		switch {
		case p.Kind == "vsosh":
			level = "ВсОШ"
		case p.Level != nil:
			level = *p.Level + " уровень"
		}
		k := key{level, grants(bn)}
		if count[k] == 0 {
			keys = append(keys, k)
		}
		count[k]++
	}
	if len(keys) == 0 {
		return nil
	}
	rank := map[string]int{"I уровень": 0, "II уровень": 1, "III уровень": 2, "ВсОШ": 3, "вне перечня": 4}
	slices.SortFunc(keys, func(x, y key) int {
		if r := rank[x.level] - rank[y.level]; r != 0 {
			return r
		}
		if r := count[y] - count[x]; r != 0 {
			return r
		}
		return strings.Compare(x.grant, y.grant)
	})
	lines := []string{"Льготы по уровню олимпиады" + scope + " (I — самый высокий), число профилей:"}
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("  %s: %s (%d)", k.level, k.grant, count[k]))
	}
	return lines
}

// catalogText — что есть в базе: на «какие вузы у тебя есть» и поиск
// олимпиад по предмету, уровню, классу, городу. subjects — предметы из
// вопроса: тогда вместо всех олимпиад — только их профили по этим предметам,
// с уровнем и классами в строке олимпиады. grade — класс из вопроса: только
// профили для него. Сводить классы из двух списков по всей базе и
// фильтровать их модель не умеет: называет олимпиады не для того класса.
func (b base) catalogText(subjects []string, grade int) string {
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
	fits := func(p store.Profile) bool { return grade == 0 || (p.GradesFrom <= grade && grade <= p.GradesTo) }
	forGrade := ""
	if grade > 0 {
		forGrade = fmt.Sprintf(" для %d класса", grade)
	}
	order := slices.DeleteFunc(slices.Clone(b.olympiads), func(oid string) bool { return !slices.ContainsFunc(b.profiles[oid], fits) })
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
			if i := slices.IndexFunc(b.profiles[oid], func(p store.Profile) bool { return p.SubjectCode == code && fits(p) }); i >= 0 {
				own = append(own, oid)
				name = b.profiles[oid][i].SubjectName
			}
		}
		if len(own) == 0 {
			continue
		}
		found = true
		lines = append(lines, fmt.Sprintf("Олимпиады по предмету «%s»%s: %d. По уровням перечня:", name, forGrade, len(own)))
		lines = append(lines, b.bySubject(own, code, grade)...)
		lines = append(lines, "По олимпиадам (вид; профили по предмету — уровень, классы; формат; финал):")
		for _, oid := range own {
			ps := b.profiles[oid]
			var profiles []string
			for _, p := range ps {
				if p.SubjectCode != code || !fits(p) {
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
	if grade > 0 {
		lines = append(lines, fmt.Sprintf("Олимпиады%s: %d.", forGrade, len(order)))
	}
	lines = append(lines, "Олимпиады по предметам и уровням (в скобках — профиль, если он называется иначе, чем предмет):")
	lines = append(lines, b.bySubject(order, "", grade)...)
	lines = append(lines, "Олимпиады (вид; классы; формат; финал):")
	for _, oid := range order {
		ps := slices.DeleteFunc(slices.Clone(b.profiles[oid]), func(p store.Profile) bool { return !fits(p) })
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
// этого предмета, "" — все; grade — только профили для этого класса, 0 — все.
func (b base) bySubject(order []string, subject string, grade int) []string {
	type key struct{ subject, level string }
	rows := map[key][]string{}
	var keys []key
	for _, oid := range order {
		for _, p := range b.profiles[oid] {
			if (subject != "" && p.SubjectCode != subject) || (grade > 0 && (grade < p.GradesFrom || grade > p.GradesTo)) {
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

// resultText — итог этапа для модели.
var resultText = map[string]string{
	stages.Passed:      "прошёл дальше",
	stages.Failed:      "не прошёл",
	stages.Winner:      "победитель",
	stages.Prizer:      "призёр",
	stages.Participant: "участник без диплома",
}

// studentCard — карточка ученика: класс, цель, предметы, вузы и трекер с
// ближайшими этапами. Без имени: в модель уходит только то, что нужно для
// ответа. Источники — сайты олимпиад трекера, ближайшие по срокам первыми:
// там регистрация и даты.
// otherText — более слабая льгота на других направлениях ученика в вузе.
var otherText = map[string]string{"bvi": "БВИ", "bvi_winners": "БВИ победителю", "score100": "100 баллов"}

// basisText — почему льготы в вузе смотрятся на эти направления.
var basisText = map[string]string{"chosen": "выбрано в вузе", "goal": "по цели"}

// targetText — льгота профилей ученика в его вузах на его направления
// (core/targets): она главнее льготы вуза целиком из списка выше. Вузы, где
// направлений ученика не нашлось, не показываются — там верна льгота вуза.
func (a *Assistant) targetText(ctx context.Context, b base, trajectoryID string, myUnis []string, ps []store.Profile) ([]string, error) {
	if trajectoryID == "" || len(myUnis) == 0 || len(ps) == 0 {
		return nil, nil
	}
	tg, err := a.Store.TargetsOf(ctx, trajectoryID, myUnis)
	if err != nil {
		return nil, err
	}
	unis := slices.DeleteFunc(slices.Clone(myUnis), func(u string) bool { return tg[u].Basis == "university" })
	if len(unis) == 0 {
		return nil, nil
	}
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	rows, err := a.Store.TargetBenefits(ctx, trajectoryID, ids, unis)
	if err != nil {
		return nil, err
	}
	byPair := map[string]store.BenefitRow{}
	for _, r := range rows {
		byPair[r.ProfileID+"/"+r.UniversityID] = r
	}
	lines := []string{"Льгота в вузах ученика на его направления (главнее льготы вуза целиком):"}
	for _, p := range ps {
		var parts []string
		for _, u := range unis {
			t, x := tg[u], b.uni[u]
			who := pick.Nick(u, x.ShortName) + ": "
			r, ok := byPair[p.ID+"/"+u]
			switch {
			case !ok:
				parts = append(parts, who+"льготы нет (направления ученика: "+strings.Join(t.DirectionNames, ", ")+")")
			case r.Unverified:
				parts = append(parts, who+"льготы на направления ученика уточняются ("+strings.Join(t.DirectionNames, ", ")+")")
			default:
				line := who + grants(r) + " (направления ученика: " + strings.Join(r.DirectionNames, ", ")
				if r.Varies {
					line += "; зависит от программы"
				}
				for _, o := range r.OtherDirections {
					line += "; " + otherText[o.Benefit] + " — на " + strings.Join(o.Names, ", ")
				}
				parts = append(parts, line+")")
			}
		}
		if len(ps) > 1 {
			lines = append(lines, "  "+profileTitle(p)+":")
			for _, x := range parts {
				lines = append(lines, "    "+x)
			}
			continue
		}
		for _, x := range parts {
			lines = append(lines, "  "+x)
		}
	}
	return lines, nil
}

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
	unis, err := a.Store.TrajectoryUniversities(ctx, t.ID)
	if err != nil {
		return err
	}
	uniIDs := make([]string, len(unis))
	for i, u := range unis {
		uniIDs[i] = u.ID
	}
	tg, err := a.Store.TargetsOf(ctx, t.ID, uniIDs)
	if err != nil {
		return err
	}
	var dirs []string
	for _, u := range unis {
		if x := tg[u.ID]; x.Basis != "university" {
			dirs = append(dirs, pick.Nick(u.ID, u.ShortName)+": "+strings.Join(x.DirectionNames, ", ")+" ("+basisText[x.Basis]+")")
		}
	}
	if len(dirs) > 0 {
		lines = append(lines, "Направления в вузах (на них считаются льготы): "+strings.Join(dirs, "; "))
	}
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
	itemIDs := make([]string, len(items))
	for i, x := range items {
		itemIDs[i] = x.ID
	}
	marks, err := a.Store.StageMarks(ctx, itemIDs)
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
		p := stages.Progress{Registered: x.RegisteredAt != nil, Marks: marks[x.ID]}
		first := stages.FirstRegistration(st)
		for i, s := range st {
			m := p.Marks[s.ID]
			if m.Registered && i != first {
				line += "; " + strings.ToLower(stageName(s)) + " — отмечена"
			}
			if m.Result != "" {
				line += "; " + strings.ToLower(stageName(s)) + " — " + resultText[m.Result]
			}
		}
		// Статус — как группа в трекере приложения.
		status, outcome := stages.Status(st, p, c.clock.now)
		switch {
		case outcome == stages.OutcomeMissed:
			line += "; регистрация закрылась без отметки"
		case outcome == stages.OutcomeUnknown:
			line += "; сезон прошёл, итог не отмечен"
		case status == stages.StatusFinished:
			line += "; участие завершено"
		}
		// Регистрация закрылась без отметки, а ближайший этап всё равно
		// нужен: вдруг ученик записался и не отметил.
		if i := slices.IndexFunc(stages.States(st, p, c.clock.now), func(s string) bool { return s != "past" }); i >= 0 {
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
