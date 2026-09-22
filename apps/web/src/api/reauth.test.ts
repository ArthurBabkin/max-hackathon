import { afterEach, describe, expect, it, vi } from 'vitest'
import { request, setReauth, setToken } from './client'

// Путь, которого нет в моках: запрос уходит в fetch, как в живом API.
const PATH = '/test-reauth'

function reply(status: number, body: unknown = {}) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

afterEach(() => {
  vi.unstubAllGlobals()
  setReauth(null)
  setToken(null)
})

describe('повтор входа при 401', () => {
  it('получает новый токен и повторяет запрос один раз', async () => {
    setToken('старый')
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(reply(401, { error: { code: 'UNAUTHORIZED', message: 'Сессия истекла' } }))
      .mockResolvedValueOnce(reply(200, { ok: true }))
    vi.stubGlobal('fetch', fetchMock)
    const reauth = vi.fn(async () => setToken('новый'))
    setReauth(reauth)

    await expect(request('GET', PATH)).resolves.toEqual({ ok: true })
    expect(reauth).toHaveBeenCalledTimes(1)
    const retried = fetchMock.mock.calls[1]![1] as RequestInit
    expect((retried.headers as Record<string, string>).Authorization).toBe('Bearer новый')
  })

  it('не зацикливается, если и после входа 401', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => reply(401, { error: { code: 'UNAUTHORIZED', message: 'нет' } })))
    const reauth = vi.fn(async () => {})
    setReauth(reauth)
    await expect(request('GET', PATH)).rejects.toMatchObject({ status: 401 })
    expect(reauth).toHaveBeenCalledTimes(1)
  })

  it('одновременные 401 делают один вход', async () => {
    let calls = 0
    vi.stubGlobal('fetch', vi.fn(async () => (++calls <= 2 ? reply(401) : reply(200, { ok: true }))))
    const reauth = vi.fn(() => new Promise<void>((resolve) => setTimeout(resolve, 5)))
    setReauth(reauth)
    await Promise.all([request('GET', PATH), request('GET', PATH)])
    expect(reauth).toHaveBeenCalledTimes(1)
  })

  it('анонимный запрос (сам POST /session) не повторяется', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => reply(401, { error: { code: 'UNAUTHORIZED', message: 'нет' } })))
    const reauth = vi.fn(async () => {})
    setReauth(reauth)
    await expect(request('POST', PATH, { anonymous: true })).rejects.toMatchObject({ status: 401 })
    expect(reauth).not.toHaveBeenCalled()
  })
})
