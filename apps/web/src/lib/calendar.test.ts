import { describe, expect, it } from 'vitest'
import { daysInMonth, leadingBlanks, shiftMonth } from '@/screens/Calendar'

describe('shiftMonth', () => {
  it('идёт вперёд и назад внутри года', () => {
    expect(shiftMonth('2026-09', 1)).toBe('2026-10')
    expect(shiftMonth('2026-09', -1)).toBe('2026-08')
  })

  it('перескакивает через границу года', () => {
    expect(shiftMonth('2026-12', 1)).toBe('2027-01')
    expect(shiftMonth('2026-01', -1)).toBe('2025-12')
  })

  it('сохраняет ведущий ноль', () => {
    expect(shiftMonth('2026-10', -1)).toBe('2026-09')
  })
})

describe('leadingBlanks — неделя начинается с понедельника', () => {
  it.each([
    ['2026-09', 1], // 1 сентября 2026 — вторник
    ['2026-10', 3], // 1 октября 2026 — четверг
    ['2026-11', 6], // 1 ноября 2026 — воскресенье
  ])('%s → %i пустых клеток', (month, expected) => {
    expect(leadingBlanks(month)).toBe(expected)
  })
})

describe('daysInMonth', () => {
  it.each([
    ['2026-09', 30],
    ['2026-10', 31],
    ['2026-02', 28],
    ['2028-02', 29], // високосный
  ])('%s → %i', (month, expected) => {
    expect(daysInMonth(month)).toBe(expected)
  })
})
