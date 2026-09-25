// Package schedule — когда напоминать о сроке (ТЗ §6.3): за 30, 7, 3 и 1
// день в 10:00 по часовому поясу региона ученика. Пакет чистый: кому и как
// отправлять, решает воркер.
package schedule

import "time"

// Offsets — плановые пороги в днях, от дальнего к ближнему.
var Offsets = []int{30, 7, 3, 1}

// AskOffsets — вопросы об итоге этапа: на следующий день после окончания и
// через неделю. Смещение отрицательное — после, а не до.
var AskOffsets = []int{-1, -8}

type Threshold struct {
	Offset int
	FireAt time.Time
}

// localHour — hour:00 того же календарного дня, что t, в зоне loc.
func localHour(t time.Time, loc *time.Location, hour int) time.Time {
	l := t.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), hour, 0, 0, 0, loc)
}

// Thresholds — все пороги срока без оглядки на текущее время. День срока
// берётся в зоне ученика: у Владивостока московский вечер — уже завтра.
func Thresholds(deadline time.Time, loc *time.Location, hour int) []Threshold {
	day := localHour(deadline, loc, hour)
	out := make([]Threshold, len(Offsets))
	for i, d := range Offsets {
		out[i] = Threshold{Offset: d, FireAt: day.AddDate(0, 0, -d)}
	}
	return out
}

// After — когда спрашивать итог этапа, закончившегося в end: hour:00 по
// зоне ученика через день и через восемь дней после дня окончания.
func After(end time.Time, loc *time.Location, hour int) []Threshold {
	day := localHour(end, loc, hour)
	out := make([]Threshold, len(AskOffsets))
	for i, d := range AskOffsets {
		out[i] = Threshold{Offset: d, FireAt: day.AddDate(0, 0, -d)}
	}
	return out
}

// Upcoming — что планировать сейчас. Прошедшие пороги не ставятся: пачка
// запоздалых «за месяц» и «за неделю» сразу после добавления олимпиады
// только раздражает. Исключение — ближайшее будущее: если все пороги уже
// прошли, а до срока ещё будут hour:00, ставится одно напоминание с
// порогом 1 на эти hour:00.
func Upcoming(deadline time.Time, loc *time.Location, hour int, now time.Time) []Threshold {
	if !deadline.After(now) {
		return []Threshold{}
	}
	out := []Threshold{}
	for _, t := range Thresholds(deadline, loc, hour) {
		if t.FireAt.After(now) {
			out = append(out, t)
		}
	}
	if len(out) > 0 {
		return out
	}
	next := localHour(now, loc, hour)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	if next.Before(deadline) {
		out = append(out, Threshold{Offset: 1, FireAt: next})
	}
	return out
}

// Tomorrow — «напомнить завтра» (F30): завтра в hour:00, если это ещё до
// срока; иначе ok = false.
func Tomorrow(deadline time.Time, loc *time.Location, hour int, now time.Time) (time.Time, bool) {
	t := localHour(now, loc, hour).AddDate(0, 0, 1)
	return t, t.Before(deadline)
}
