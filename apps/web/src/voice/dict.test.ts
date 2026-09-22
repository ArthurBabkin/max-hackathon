import { describe, expect, it } from 'vitest'
import raw from '@texts'
import { KNOWN_PLACEHOLDERS, interpolate, text } from './texts'

const entries = Object.entries(raw as Record<string, { kid: string; parent: string }>)
const placeholders = (s: string) => [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1] as string)

describe('словарь текстов', () => {
  it('не пуст', () => {
    expect(entries.length).toBeGreaterThan(100)
  })

  it.each(entries)('%s — есть оба варианта и они непустые', (_key, value) => {
    expect(typeof value.kid).toBe('string')
    expect(typeof value.parent).toBe('string')
    expect(value.kid.trim()).not.toBe('')
    expect(value.parent.trim()).not.toBe('')
  })

  it('все плейсхолдеры известны', () => {
    const unknown = new Set<string>()
    for (const [key, value] of entries) {
      for (const name of [...placeholders(value.kid), ...placeholders(value.parent)]) {
        if (!(KNOWN_PLACEHOLDERS as readonly string[]).includes(name)) {
          unknown.add(`${key}: {${name}}`)
        }
      }
    }
    expect([...unknown]).toEqual([])
  })

  it('в текстах нет разметки — словарь читает ещё и Go-бот', () => {
    const withMarkup = entries.filter(
      ([, v]) => /<[a-z/]/i.test(v.kid) || /<[a-z/]/i.test(v.parent),
    )
    expect(withMarkup.map(([k]) => k)).toEqual([])
  })

  it('в ключах с числом нет вшитого склонения — его считает lib/deadline', () => {
    // Строки вида «5 дней» в словаре означают, что форма зашита намертво
    // и сломается на «1 день». Число приходит через {count}.
    //
    // Исключение — перечисление порогов напоминаний: это константы из ТЗ §6.3,
    // они не считаются и меняться не могут.
    const FIXED_NUMBERS = ['tracker.remindNote']
    const hardcoded = entries
      .filter(([k]) => !FIXED_NUMBERS.includes(k))
      .filter(([, v]) =>
        [v.kid, v.parent].some((s) => /\d+\s+(день|дня|дней|олимпиад\w*|вуз\w*)/.test(s)),
      )
    expect(hardcoded.map(([k]) => k)).toEqual([])
  })
})

describe('text', () => {
  it('выбирает вариант по роли', () => {
    expect(text('home.goalLabel', 'kid', {})).toBe('Моя цель')
    expect(text('home.goalLabel', 'parent', { student_gen: 'Артёма' })).toBe('Цель Артёма')
  })

  it('подставляет имя ученика и имя смотрящего', () => {
    expect(text('home.greeting', 'kid', { student: 'Артём' })).toBe('Привет, Артём 👋')
    expect(text('home.greeting', 'parent', { me: 'Ольга' })).toBe('Здравствуйте, Ольга')
  })

  it('родителю предлагает олимпиаду в дательном падеже', () => {
    expect(text('olympiad.proposeCta', 'parent', { student_dat: 'Артёму' })).toBe(
      'Предложить Артёму',
    )
  })
})

describe('interpolate', () => {
  it('подставляет несколько значений', () => {
    expect(interpolate('{a} и {b}', { a: 'раз', b: 'два' })).toBe('раз и два')
  })

  it('принимает числа', () => {
    expect(interpolate('выбрано: {count}', { count: 3 })).toBe('выбрано: 3')
  })

  it('повторяющийся плейсхолдер заменяется везде', () => {
    expect(interpolate('{n}+{n}', { n: 1 })).toBe('1+1')
  })

  it('без значения оставляет плейсхолдер видимым — так ошибку заметно', () => {
    expect(interpolate('Привет, {student}', {})).toBe('Привет, {student}')
  })
})
