import { describe, expect, it } from 'vitest'
import { parseSheetStack } from './sheets'

describe('parseSheetStack', () => {
  it('разбирает стек из нескольких листов', () => {
    expect(parseSheetStack('oly:hse:inf,vuz:inno')).toEqual([
      { kind: 'oly', id: 'hse:inf' },
      { kind: 'vuz', id: 'inno' },
    ])
  })

  it('не путается в двоеточиях внутри идентификатора профиля', () => {
    // vsosh-inf:inf — идентификатор профиля сам содержит двоеточие.
    expect(parseSheetStack('oly:vsosh-inf:inf')).toEqual([{ kind: 'oly', id: 'vsosh-inf:inf' }])
  })

  it('лист помощника идёт без идентификатора', () => {
    expect(parseSheetStack('ai')).toEqual([{ kind: 'ai', id: '' }])
  })

  it('выбрасывает неизвестные типы, а не падает', () => {
    expect(parseSheetStack('oly:hse:inf,мусор:1')).toEqual([{ kind: 'oly', id: 'hse:inf' }])
  })

  it('пустой параметр — пустой стек', () => {
    expect(parseSheetStack(null)).toEqual([])
    expect(parseSheetStack('')).toEqual([])
  })
})
