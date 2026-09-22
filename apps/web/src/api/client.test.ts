import { describe, expect, it } from 'vitest'
import { buildQuery } from './query'

describe('buildQuery', () => {
  it('выбрасывает пустые значения, чтобы в URL не было мусора', () => {
    expect(buildQuery({ q: '', subject: 'inf', city: undefined, page: null })).toBe('?subject=inf')
  })

  it('кодирует кириллицу', () => {
    expect(buildQuery({ city: 'Казань' })).toContain('city=%D0%9A')
  })

  it('без параметров возвращает пустую строку', () => {
    expect(buildQuery({})).toBe('')
    expect(buildQuery(undefined)).toBe('')
  })

  it('числа приводит к строке', () => {
    expect(buildQuery({ grade: 9 })).toBe('?grade=9')
  })
})
