import { existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
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

  it('у ВсОШ по каждому предмету свой знак', () => {
    expect(badgeLogo('vsosh-fizika')).toBe('/logos/vsosh-fizika.png')
    expect(badgeLogo('vsosh-himiya')).toBe('/logos/vsosh-himiya.png')
  })

  it('ВсОШ без предметного знака получает общий', () => {
    expect(badgeLogo('vsosh-ekologiya')).toBe('/logos/vsosh.png')
  })

  it('у олимпиад перечня — их собственные знаки, файлы на месте', () => {
    // «Высшая проба», «Ломоносов», ПВГ, Московская, СПбГУ, НТО, «Юниор»,
    // Всесибирская, «Будущие исследователи», Пироговская, РАНХиГС, РГГУ,
    // «Звезда», СПб астрономическая, ВАОИ, Innopolis Open, КФУ, «Бельчонок»,
    // «Курчатов», «Робофест», ОМО, Вернадского, Инженерная, «Изумруд», «Юные таланты»,
    // «Газпром», Санкт-Петербургская, «Турнир городов», Верченко.
    const ids = ['p669-8', 'p669-50', 'p669-52', 'p669-37', 'p669-59', 'p669-5', 'p669-13', 'p669-14',
      'p669-29', 'p669-71', 'p669-58', 'p669-48', 'p669-36', 'p669-74', 'p669-15', 'p669-22', 'p669-34',
      'p669-83', 'p669-43', 'p669-53', 'p669-41', 'p669-4', 'p669-18', 'p669-26', 'p669-35', 'p669-69',
      'p669-75', 'p669-81', 'p669-31']
    for (const id of ids) {
      expect(badgeLogo(id)).toBe(`/logos/${id}.png`)
      expect(existsSync(fileURLToPath(new URL(`../../public/logos/${id}.png`, import.meta.url))), id).toBe(true)
    }
  })

  it('у всех олимпиад КФУ — знак его межрегиональных предметных олимпиад', () => {
    // Межрегиональные предметные олимпиады и «Потомки Менделеева».
    expect(badgeLogo('p669-34')).toBe('/logos/p669-34.png')
    expect(badgeLogo('p669-46')).toBe('/logos/p669-34.png')
  })

  it('без логотипа возвращает null', () => {
    expect(badgeLogo('unknown-uni')).toBeNull()
    expect(badgeLogo('other-tyk')).toBeNull()
  })
})
