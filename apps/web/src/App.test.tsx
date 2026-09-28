import { QueryClient, QueryClientProvider, focusManager } from '@tanstack/react-query'
import { act, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { createQueryClient } from './api/queryClient'
import { getWebApp } from './bridge'
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

// MAX открывает мини-приложение с данными запуска в hash:
// `#WebAppData=…&WebAppPlatform=ios`. Хеш-роутер принимает это за путь, и
// главная открывалась без кнопки «Спросить» — до первого перехода по вкладкам.
it('при запуске из MAX главная открывается с кнопкой «Спросить»', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) =>
      url.endsWith('/session')
        ? Promise.resolve(reply(200, { token: 't', session: makeSession() }))
        : new Promise(() => {}),
    ),
  )

  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={['/WebAppData=user%3D%7B%7D%26hash%3Dabc&WebAppPlatform=ios&WebAppVersion=25.9.0']}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  )

  expect(await screen.findByRole('button', { name: 'Спросить' })).toBeInTheDocument()
})

// jsdom считает страницу скрытой, и TanStack Query ставит повторы на паузу
// до возвращения фокуса. В MAX окно видно — так же и здесь.
afterEach(() => focusManager.setFocused(undefined))

function renderWith(client: QueryClient) {
  focusManager.setFocused(true)
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// Траектории ещё нет — анкета в боте не пройдена. Это не «нет интернета»,
// и повторять запрос сразу бессмысленно: ответ будет тот же.
it('без траектории просит закончить анкету и не повторяет вход сам', async () => {
  const calls: string[] = []
  let status = 404
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      calls.push(url)
      if (url.endsWith('/session'))
        return Promise.resolve(
          status === 404
            ? reply(404, { error: { code: 'NOT_FOUND', message: 'Сначала пройдите онбординг в чате бота.' } })
            : reply(200, { token: 't', session: makeSession() }),
        )
      return new Promise(() => {})
    }),
  )
  renderWith(createQueryClient({ defaultOptions: { queries: { retryDelay: 0 } } }))

  expect(await screen.findByText('Анкета ещё не пройдена')).toBeInTheDocument()
  expect(screen.queryByText(/интернет/)).not.toBeInTheDocument()
  expect(calls.filter((c) => c.endsWith('/session'))).toHaveLength(1)

  // Анкету закончили — «Проверить снова» открывает приложение.
  status = 200
  await userEvent.click(screen.getByRole('button', { name: /Проверить снова/ }))
  expect(await screen.findByRole('button', { name: 'Спросить' })).toBeInTheDocument()
})

// Анкета — в чате бота: кнопка ведёт туда, а вернулись в приложение — вход
// проверяется сам, без «Проверить снова».
it('без траектории ведёт в чат бота и проверяет вход по возвращении', async () => {
  let status = 404
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) =>
      url.endsWith('/session')
        ? Promise.resolve(
            status === 404
              ? reply(404, { error: { code: 'NOT_FOUND', message: 'Сначала пройдите онбординг в чате бота.' } })
              : reply(200, { token: 't', session: makeSession() }),
          )
        : new Promise(() => {}),
    ),
  )
  const open = vi.spyOn(getWebApp(), 'openMaxLink').mockImplementation(() => {})
  renderWith(createQueryClient({ defaultOptions: { queries: { retryDelay: 0 } } }))

  await userEvent.click(await screen.findByRole('button', { name: /Пройти анкету в чате бота/ }))
  expect(open).toHaveBeenCalledWith('https://max.ru/t356_hakaton_max_bot')

  // Ушли в чат, прошли анкету, вернулись.
  status = 200
  act(() => focusManager.setFocused(false))
  act(() => focusManager.setFocused(true))
  expect(await screen.findByRole('button', { name: 'Спросить' })).toBeInTheDocument()
})

it('сбой сервера при входе — не «нет интернета», с повтором', async () => {
  const calls: string[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      calls.push(url)
      return Promise.resolve(reply(500, { error: { code: 'INTERNAL', message: 'Что-то пошло не так. Попробуйте ещё раз.' } }))
    }),
  )
  renderWith(createQueryClient({ defaultOptions: { queries: { retryDelay: 0 } } }))

  expect(await screen.findByText(/Сервер ответил ошибкой/)).toBeInTheDocument()
  expect(screen.queryByText(/интернет/)).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Повторить загрузку/ })).toBeInTheDocument()
  // Временный сбой повторяется один раз сам — и не больше.
  await new Promise((resolve) => setTimeout(resolve, 50))
  expect(calls).toHaveLength(2)
})

it('без сети при входе — «нет соединения»', async () => {
  vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new TypeError('Failed to fetch'))))
  renderWith(createQueryClient({ defaultOptions: { queries: { retryDelay: 0 } } }))

  expect(await screen.findByText(/Нет соединения с интернетом/)).toBeInTheDocument()
})
