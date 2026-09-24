import { describe, expect, it } from 'vitest'
import {
  daysLabel,
  daysLeft,
  deadlineTone,
  formatDay,
  formatMonthTitle,
  nearestDeadlineMonth,
  plural,
} from './deadline'

describe('plural', () => {
  const форма = (n: number) => plural(n, 'день', 'дня', 'дней')

  it.each([
    [1, 'день'],
    [2, 'дня'],
    [4, 'дня'],
    [5, 'дней'],
    [11, 'дней'],
    [12, 'дней'],
    [14, 'дней'],
    [21, 'день'],
    [22, 'дня'],
    [25, 'дней'],
    [101, 'день'],
    [111, 'дней'],
    [0, 'дней'],
  ])('%i → %s', (n, ожидаем) => {
    expect(форма(n)).toBe(ожидаем)
  })
})

describe('daysLabel', () => {
  it('склеивает число с нужной формой', () => {
    expect(daysLabel(1)).toBe('1 день')
    expect(daysLabel(4)).toBe('4 дня')
    expect(daysLabel(10)).toBe('10 дней')
  })
})

describe('daysLeft', () => {
  // Считаем по календарным суткам, а не по часам: «завтра в 9 утра» — это
  // один день, даже если до него 15 часов.
  const now = new Date('2026-09-21T23:30:00')

  it('считает разницу в календарных днях, а не в часах', () => {
    expect(daysLeft('2026-09-22T00:30:00', now)).toBe(1)
  })

  it('сегодняшний срок — ноль', () => {
    expect(daysLeft('2026-09-21T08:00:00', now)).toBe(0)
  })

  it('прошедший срок — отрицательное число', () => {
    expect(daysLeft('2026-09-19T08:00:00', now)).toBe(-2)
  })

  it('без срока возвращает null', () => {
    expect(daysLeft(null, now)).toBeNull()
  })
})

describe('deadlineTone', () => {
  // Пороги из ТЗ §7.3: не больше 7 дней — розовая, не больше 16 — жёлтая,
  // дальше серая, «готово» — зелёная.
  it('отметка о регистрации важнее срока', () => {
    expect(deadlineTone(3, true)).toBe('done')
    expect(deadlineTone(null, true)).toBe('done')
  })

  it.each([
    [0, 'hot'],
    [7, 'hot'],
    [8, 'soon'],
    [16, 'soon'],
    [17, 'ok'],
    [100, 'ok'],
  ])('%i дней → %s', (дней, ожидаем) => {
    expect(deadlineTone(дней, false)).toBe(ожидаем)
  })

  it('прошедший срок красный, а не серый', () => {
    expect(deadlineTone(-1, false)).toBe('hot')
  })

  it('без срока плашки нет', () => {
    expect(deadlineTone(null, false)).toBeNull()
  })
})

describe('formatDay', () => {
  it('даёт «25 сентября»', () => {
    expect(formatDay('2026-09-25T20:59:00')).toBe('25 сентября')
  })

  it('без даты возвращает null', () => {
    expect(formatDay(null)).toBeNull()
  })
})

describe('formatMonthTitle', () => {
  it('заголовок календаря — с заглавной буквы и с годом', () => {
    expect(formatMonthTitle('2026-10')).toBe('Октябрь 2026')
  })

  it('предложный падеж для строки «В октябре»', () => {
    expect(formatMonthTitle('2026-10', 'in')).toBe('октябре')
  })
})

describe('nearestDeadlineMonth — с какого месяца открыть календарь', () => {
  const now = new Date('2026-09-24T09:00:00Z')
  const item = (deadline_at: string | null) => ({ deadline_at })

  it('месяц ближайшего срока, а не первого в списке', () => {
    const items = [item('2026-12-01T20:59:00Z'), item('2026-11-10T20:59:00Z'), item('2027-01-15T20:59:00Z')]
    expect(nearestDeadlineMonth(items, now)).toBe('2026-11')
  })

  it('прошедшие сроки пропускает', () => {
    expect(nearestDeadlineMonth([item('2026-09-01T20:59:00Z'), item('2026-10-05T20:59:00Z')], now)).toBe('2026-10')
  })

  it('срок сегодня — ближайший, даже если его час уже прошёл', () => {
    expect(nearestDeadlineMonth([item('2026-09-24T06:00:00Z'), item('2026-10-05T20:59:00Z')], now)).toBe('2026-09')
  })

  it('месяц считает по Москве, как календарь на сервере', () => {
    // 21:30 UTC 31 октября — это уже 00:30 1 ноября по Москве.
    expect(nearestDeadlineMonth([item('2026-10-31T21:30:00Z')], now)).toBe('2026-11')
  })

  it('без будущих сроков — текущий месяц', () => {
    expect(nearestDeadlineMonth([item(null), item('2026-08-01T20:59:00Z')], now)).toBe('2026-09')
    expect(nearestDeadlineMonth([], now)).toBe('2026-09')
  })

  it('текущий месяц тоже по Москве', () => {
    expect(nearestDeadlineMonth([], new Date('2026-09-30T21:30:00Z'))).toBe('2026-10')
  })
})
