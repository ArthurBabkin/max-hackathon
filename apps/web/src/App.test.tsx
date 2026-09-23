import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { expect, it, vi } from 'vitest'
import { makeSession } from './test/render'

// Моки токен не проверяют — тест ходит в «живой» API: fetch отвечает 401
// на любой запрос без токена, как Go-сервис. vi.hoisted — чтобы client.ts
// прочитал переменную уже выключенной.
vi.hoisted(() => vi.stubEnv('VITE_USE_MOCKS', 'off'))
const { App } = await import('./App')

function reply(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

it('не ходит за данными, пока не открыта сессия', async () => {
  const calls: string[] = []
  let openSession: () => void = () => {}
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init: RequestInit) => {
      const path = url.replace('/api/v1', '')
      const authorized = 'Authorization' in (init.headers as Record<string, string>)
      calls.push(`${init.method} ${path}${authorized || path === '/session' ? '' : ' 401'}`)
      if (path === '/session')
        return new Promise((resolve) => (openSession = () => resolve(reply(200, { token: 't', session: makeSession() }))))
      if (!authorized) return Promise.resolve(reply(401, { error: { code: 'UNAUTHORIZED', message: 'нет' } }))
      if (path === '/tracker') return Promise.resolve(reply(200, { items: [], proposals: [] }))
      return new Promise(() => {})
    }),
  )

  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  )

  await waitFor(() => expect(calls).toContain('POST /session'))
  openSession()
  await waitFor(() => expect(calls).toContain('GET /tracker'))
  expect(calls.filter((c) => c.endsWith('401'))).toEqual([])
  expect(calls.filter((c) => c === 'POST /session')).toHaveLength(1)
})
