import { describe, expect, it } from 'vitest'
import { ApiError, errorKind, isRetryable, serverMessage, toApiError } from './errors'

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

describe('errorKind', () => {
  it('различает причины по статусу', () => {
    const kinds = [0, 400, 401, 403, 404, 409, 429, 500, 501, 502].map((s) => errorKind(new ApiError(s, 'X', 'm')))
    expect(kinds).toEqual([
      'offline',
      'rejected',
      'unauthorized',
      'forbidden',
      'notFound',
      'rejected',
      'busy',
      'server',
      'notReady',
      'server',
    ])
  })

  it('чужая ошибка — сбой, а не отказ сервера', () => {
    expect(errorKind(new TypeError('x'))).toBe('server')
    expect(errorKind(undefined)).toBe('server')
  })
})

describe('isRetryable', () => {
  // Повторять 404 «анкета не пройдена» или 400 бессмысленно — ответ будет тот же.
  it('повторяет только временные сбои', () => {
    expect([0, 429, 500, 503].map((s) => isRetryable(new ApiError(s, 'X', 'm')))).toEqual([true, true, true, true])
    expect([400, 401, 403, 404, 501].map((s) => isRetryable(new ApiError(s, 'X', 'm')))).toEqual([
      false,
      false,
      false,
      false,
      false,
    ])
  })
})

describe('serverMessage', () => {
  it('отдаёт текст сервера только у ApiError', () => {
    expect(serverMessage(new ApiError(400, 'BAD_REQUEST', 'Имя — от 1 до 40 символов.'))).toBe('Имя — от 1 до 40 символов.')
    expect(serverMessage(new Error('stack'))).toBeNull()
  })
})
