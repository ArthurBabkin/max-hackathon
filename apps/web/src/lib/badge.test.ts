import { describe, expect, it } from 'vitest'
import { badgeColor, badgeLogo, badgeShortName } from './badge'

/**
 * Контракт разрешает `short_name` и `color` пустыми: в схеме БД таких колонок
 * нет, и заполнять их сервер не обязан. Плитка всё равно должна выглядеть
 * осмысленно, поэтому есть запасной вариант.
 */
describe('badgeShortName', () => {
  it('готовое значение сервера важнее вычисленного', () => {
    expect(badgeShortName('Высшая проба', 'ВП')).toBe('ВП')
  })

  it('из двух слов берёт по первой букве', () => {
    expect(badgeShortName('Высшая проба', null)).toBe('ВП')
    expect(badgeShortName('Innopolis Open', null)).toBe('IO')
  })

  it('из одного слова берёт две буквы, вторую строчной', () => {
    expect(badgeShortName('Технокубок', null)).toBe('Те')
    expect(badgeShortName('Ломоносов', null)).toBe('Ло')
  })

  it('длинное название сокращает до трёх букв, не больше', () => {
    expect(badgeShortName('Турнир юных программистов Казани', null)).toBe('ТЮП')
  })

  it('служебные слова пропускает', () => {
    expect(badgeShortName('Олимпиада по информатике', null)).toBe('ОИ')
  })

  it('пустое название не роняет плитку', () => {
    expect(badgeShortName('', null)).toBe('?')
    expect(badgeShortName('   ', null)).toBe('?')
  })
})

describe('badgeColor', () => {
  it('готовый цвет сервера важнее вычисленного', () => {
    expect(badgeColor('hse', '#123456')).toBe('#123456')
  })

  it('один и тот же идентификатор всегда даёт один и тот же цвет', () => {
    expect(badgeColor('hse', null)).toBe(badgeColor('hse', null))
  })

  it('разные идентификаторы обычно различаются', () => {
    const colors = new Set(['hse', 'inno', 'tk', 'lomo', 'vsosh-inf'].map((id) => badgeColor(id, null)))
    expect(colors.size).toBeGreaterThan(2)
  })

  it('всегда возвращает корректный hex', () => {
    expect(badgeColor('что угодно', null)).toMatch(/^#[0-9A-Fa-f]{6}$/)
  })
})

describe('badgeLogo', () => {
  it('находит логотип вуза и олимпиады по идентификатору', () => {
    expect(badgeLogo('hse')).toBe('/logos/hse.png')
    expect(badgeLogo('p669-62')).toBe('/logos/p669-62.svg')
  })

  it('у всех олимпиад ВсОШ один логотип', () => {
    expect(badgeLogo('vsosh-fizika')).toBe('/logos/vsosh.png')
    expect(badgeLogo('vsosh-himiya')).toBe('/logos/vsosh.png')
  })

  it('без логотипа возвращает null', () => {
    expect(badgeLogo('kfu')).toBeNull()
    expect(badgeLogo('other-tyk')).toBeNull()
  })
})
