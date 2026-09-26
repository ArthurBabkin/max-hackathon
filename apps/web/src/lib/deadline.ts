/**
 * Сроки: сколько осталось, какого цвета плашка, как назвать дату.
 *
 * Логика без React и без запросов — её и покрывают тесты. Пороги плашек
 * заданы в ТЗ §7.3 и повторены в прототипе (функция `pill`).
 */

/** Цвет плашки срока. `null` — срока нет, плашку не рисуем. */
export type DeadlineTone = 'done' | 'hot' | 'soon' | 'ok'

/** Не больше стольких дней — плашка розовая (ТЗ §7.3). */
const HOT_DAYS = 7
/** Не больше стольких дней — жёлтая. Этот же порог у фильтра «Срок скоро» (F15). */
const SOON_DAYS = 16

/**
 * Русская форма слова по числу: 1 день, 2 дня, 5 дней.
 * Перенесено из прототипа (`plural`), чтобы «4 дня» в интерфейсе читалось так же.
 */
export function plural(n: number, one: string, few: string, many: string): string {
  return { one, few, many }[pluralForm(n)]
}

/** Форма слова при числе: 1 олимпиада — `one`, 2 олимпиады — `few`, 5 олимпиад — `many`. */
export function pluralForm(n: number): 'one' | 'few' | 'many' {
  const hundreds = Math.abs(n) % 100
  const units = hundreds % 10
  if (hundreds > 10 && hundreds < 20) return 'many'
  if (units > 1 && units < 5) return 'few'
  if (units === 1) return 'one'
  return 'many'
}

/** «4 дня», «1 день», «10 дней». */
export function daysLabel(n: number): string {
  return `${n} ${plural(n, 'день', 'дня', 'дней')}`
}

/** Локальная полночь — чтобы считать календарные сутки, а не часы. */
function startOfDay(d: Date): number {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
}

/**
 * Сколько календарных дней осталось до срока. Отрицательное — срок прошёл,
 * `null` — срока нет.
 *
 * Считаем по суткам, а не по часам: пользователю «завтра» — это завтра,
 * даже если до дедлайна 15 часов. Округление, а не отбрасывание, — из-за
 * перехода на летнее время, где в сутках выходит 23 или 25 часов.
 */
export function daysLeft(deadline: string | null | undefined, now: Date = new Date()): number | null {
  if (!deadline) return null
  const parsed = new Date(deadline)
  if (Number.isNaN(parsed.getTime())) return null
  return Math.round((startOfDay(parsed) - startOfDay(now)) / 86_400_000)
}

/**
 * Цвет плашки срока. Отметка о регистрации важнее срока: если пункт отмечен,
 * плашка зелёная независимо от того, сколько осталось.
 *
 * Прошедший срок остаётся розовым, а не становится серым: серый читается как
 * «не срочно», и пропущенный дедлайн в трекере так потерялся бы.
 */
export function deadlineTone(days: number | null, registered: boolean): DeadlineTone | null {
  if (registered) return 'done'
  if (days === null) return null
  if (days <= HOT_DAYS) return 'hot'
  if (days <= SOON_DAYS) return 'soon'
  return 'ok'
}

/** Попадает ли срок под фильтр «Срок скоро» (F15). */
export function isSoon(days: number | null): boolean {
  return days !== null && days <= SOON_DAYS
}

const MONTHS_IN = [
  'январе', 'феврале', 'марте', 'апреле', 'мае', 'июне',
  'июле', 'августе', 'сентябре', 'октябре', 'ноябре', 'декабре',
]
const MONTHS_NOMINATIVE = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
]

const dayFormatter = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long' })

/** «25 сентября». Родительный падеж Intl даёт сам, поэтому здесь он и используется. */
export function formatDay(date: string | null | undefined): string | null {
  if (!date) return null
  const parsed = new Date(date)
  if (Number.isNaN(parsed.getTime())) return null
  return dayFormatter.format(parsed)
}

/**
 * Название месяца по строке `YYYY-MM`.
 *
 * `case: 'in'` даёт предложный падеж для заголовка «В октябре» — Intl такой
 * формы не умеет, поэтому здесь два коротких списка вместо форматтера.
 */
export function formatMonthTitle(month: string, grammaticalCase: 'title' | 'in' = 'title'): string {
  const [year, monthNumber] = month.split('-')
  const index = Number(monthNumber) - 1
  if (!year || Number.isNaN(index) || index < 0 || index > 11) return month
  if (grammaticalCase === 'in') return MONTHS_IN[index] as string
  return `${MONTHS_NOMINATIVE[index] as string} ${year}`
}

const moscowDayFormatter = new Intl.DateTimeFormat('en-CA', { timeZone: 'Europe/Moscow' })

/** `YYYY-MM-DD` по Москве — в этом поясе сервер раскладывает сроки по дням календаря. */
export function moscowDay(date: Date): string {
  return moscowDayFormatter.format(date)
}

/**
 * С какого месяца открыть календарь: там, где ближайший срок, чтобы его было
 * видно сразу. У пункта трекера `deadline_at` — срок ближайшего этапа, так что
 * ближайшая точка календаря — самый ранний из них. Будущих сроков нет —
 * текущий месяц. Сегодняшний срок считается будущим до конца суток.
 */
export function nearestDeadlineMonth(
  items: ReadonlyArray<{ deadline_at: string | null }>,
  now: Date = new Date(),
): string {
  const today = moscowDay(now)
  let nearest: string | null = null
  for (const { deadline_at } of items) {
    if (!deadline_at) continue
    const parsed = new Date(deadline_at)
    if (Number.isNaN(parsed.getTime())) continue
    const day = moscowDay(parsed)
    if (day >= today && (nearest === null || day < nearest)) nearest = day
  }
  return (nearest ?? today).slice(0, 7)
}

const todayFormatter = new Intl.DateTimeFormat('ru-RU', {
  weekday: 'long',
  day: 'numeric',
  month: 'long',
})

/** «Понедельник, 21 сентября» — подзаголовок приветствия на главной. */
export function formatToday(now: Date = new Date()): string {
  const s = todayFormatter.format(now)
  return s.charAt(0).toUpperCase() + s.slice(1)
}

const shortDateFormatter = new Intl.DateTimeFormat('ru-RU', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
})

/** «15.09.2026» — дата проверки источника рядом с меткой «Факт». */
export function formatShortDate(date: string | null | undefined): string | null {
  if (!date) return null
  const parsed = new Date(date)
  if (Number.isNaN(parsed.getTime())) return null
  return shortDateFormatter.format(parsed)
}
