import { describe, expect, it } from 'vitest'
import { ApiError, toApiError } from './errors'

describe('toApiError', () => {
  it('разбирает форму из ТЗ §10', () => {
    const e = toApiError(403, { error: { code: 'FORBIDDEN', message: 'Только создатель' } })
    expect(e.code).toBe('FORBIDDEN')
    expect(e.message).toBe('Только создатель')
    expect(e.status).toBe(403)
  })

  it('переживает пустое и мусорное тело', () => {
    expect(toApiError(500, null).code).toBe('INTERNAL')
    expect(toApiError(500, 'ой').code).toBe('INTERNAL')
    expect(toApiError(404, {}).message).toBe('Не найдено')
    expect(toApiError(502, { error: 'bad gateway' }).code).toBe('INTERNAL')
  })

  it('нулевой статус — это отсутствие сети', () => {
    expect(toApiError(0, null).isOffline).toBe(true)
  })

  it('истёкшая сессия распознаётся', () => {
    expect(new ApiError(401, 'UNAUTHORIZED', 'x').isUnauthorized).toBe(true)
  })
})
