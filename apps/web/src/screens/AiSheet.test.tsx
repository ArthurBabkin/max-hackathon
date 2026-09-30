import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { useLocation } from 'react-router-dom'
import type { AiChat, AiMessage } from '@contract'
import { keys } from '@/api/queries'
import { makeSession, renderApp } from '@/test/render'
import { useSheetStack } from '@/ui/sheets'

// Ответ помощника идёт секунды — держим его «в пути», моки отвечали бы сами.
vi.hoisted(() => vi.stubEnv('VITE_USE_MOCKS', 'off'))
const { SheetHost } = await import('./SheetHost')

beforeEach(() => {
  Element.prototype.scrollTo = () => {} // в jsdom прокрутки нет
  // «сегодня» и «18 сентября» в списке чатов считаются от текущей даты.
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-09-23T12:00:00Z'))
})

afterEach(() => {
  vi.useRealTimers()
})

const chat = (id: string, title: string, day: string): AiChat => ({
  id,
  title,
  created_at: `${day}T09:00:00Z`,
  last_message_at: `${day}T09:05:00Z`,
})

const msg = (id: string, role: AiMessage['role'], text: string, over: Partial<AiMessage> = {}): AiMessage => ({
  id,
  role,
  text,
  card_refs: [],
  sources: [],
  refused: false,
  created_at: '2026-09-21T09:00:00Z',
  ...over,
})

const itmo = chat('c-itmo', 'Льготы в ИТМО', '2026-09-23')
const old = chat('c-old', 'Чат 18 сентября', '2026-09-18')
const itmoLog = [
  msg('m1', 'user', 'Какие льготы даёт «Высшая проба» в ИТМО?'),
  msg('m2', 'assistant', 'БВИ по информатике.'),
]

/** Лист помощника как в приложении: стек в адресе, адрес виден тесту. */
function Host() {
  const sheets = useSheetStack()
  const { search } = useLocation()
  return (
    <>
      <output data-testid="url">{decodeURIComponent(search)}</output>
      <SheetHost sheets={sheets} />
    </>
  )
}

const url = () => screen.getByTestId('url').textContent

type Reply = [status: number, body: unknown]

/** Сервер на время теста: ответы по «МЕТОД путь», на остальное — тишина. */
function server(routes: Record<string, (body: unknown) => Reply> = {}) {
  const sent: { route: string; body: unknown }[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((input: string, init: RequestInit = {}) => {
      const route = `${init.method ?? 'GET'} ${input.replace('/api/v1', '')}`
      const body: unknown = init.body ? JSON.parse(init.body as string) : undefined
      sent.push({ route, body })
      const handler = routes[route]
      if (!handler) return new Promise(() => {})
      const [status, payload] = handler(body)
      return Promise.resolve(
        new Response(JSON.stringify(payload), { status, headers: { 'Content-Type': 'application/json' } }),
      )
    }),
  )
  return sent
}

function open(sheet: string, seed: { chats?: AiChat[]; logs?: Record<string, AiMessage[]> } = {}) {
  return renderApp(<Host />, {
    route: `/?sheet=${sheet}`,
    seed: (c) => {
      if (seed.chats) c.setQueryData(keys.aiChats, { items: seed.chats })
      for (const [id, items] of Object.entries(seed.logs ?? {})) c.setQueryData(keys.aiChat(id), { items })
    },
  })
}

const chatButton = () => screen.getByRole('button', { name: /^Чаты:/ })
const input = () => screen.getByRole('textbox', { name: 'Напиши вопрос' })

/** Строки списка чатов сверху вниз. */
async function chatRows() {
  await userEvent.click(chatButton())
  return within(screen.getByRole('list', { name: 'Чаты' })).getAllByRole('button')
}

it('показывает вопрос сразу, не дожидаясь ответа', async () => {
  server()
  open('ai', { chats: [] })
  expect(chatButton()).toHaveTextContent('Новый чат')

  const question = 'Чем БВИ отличается от 100 баллов?'
  await userEvent.click(screen.getByRole('button', { name: question }))

  expect(screen.getByLabelText('Помощник печатает')).toBeInTheDocument()
  expect(screen.getByText(question, { selector: '.ai-message-user p' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: question })).not.toBeInTheDocument()
})

// Подсказки идут из словаря: родитель видит имя своего ученика, а не демо-имя.
it('подсказки родителю — с именем его ученика', () => {
  server()
  const session = makeSession({ role: 'parent' })
  session.trajectory.student_name = 'Мария'
  renderApp(<Host />, { route: '/?sheet=ai', session, seed: (c) => c.setQueryData(keys.aiChats, { items: [] }) })

  expect(screen.getByRole('button', { name: 'Какие льготы дают олимпиады в вузах Марии?' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /Артёма/ })).not.toBeInTheDocument()
})

it('подсказки ученику — на «ты», без имени', () => {
  server()
  open('ai', { chats: [] })

  expect(screen.getByRole('button', { name: 'Какие льготы дают олимпиады в моих вузах?' })).toBeInTheDocument()
})

// F58: «Спросить» открывает последний чат, а не пустой.
it('открывает последний чат: его название и реплики', () => {
  server()
  open('ai', { chats: [itmo, old], logs: { 'c-itmo': itmoLog } })

  expect(chatButton()).toHaveTextContent('Льготы в ИТМО')
  expect(screen.getByText('БВИ по информатике.')).toBeInTheDocument()
})

// F58: переключение между чатами. Выбранный чат живёт в адресе, поэтому
// «Карточка» → «Назад» возвращает в тот же чат, а не в последний.
it('в списке свежие чаты сверху, выбранный открывается и запоминается в адресе', async () => {
  server()
  open('ai', {
    chats: [itmo, old],
    logs: {
      'c-itmo': itmoLog,
      'c-old': [
        msg('m3', 'user', 'Что даёт «Высшая проба»?'),
        msg('m4', 'assistant', 'БВИ в ВШЭ.', { card_refs: [{ type: 'olympiad', id: 'hse:inf', title: 'Высшая проба' }] }),
      ],
    },
  })

  const rows = await chatRows()
  expect(screen.getByText('Эти чаты видишь только ты')).toBeInTheDocument()
  expect(rows.map((r) => r.textContent)).toEqual([
    expect.stringContaining('Льготы в ИТМО'),
    expect.stringContaining('Чат 18 сентября'),
  ])
  expect(rows[0]).toHaveAttribute('aria-current', 'true')
  expect(rows[0]).toHaveTextContent('сегодня')
  expect(rows[1]).toHaveTextContent('18 сентября')

  await userEvent.click(rows[1]!)
  expect(chatButton()).toHaveTextContent('Чат 18 сентября')
  expect(screen.getByText('БВИ в ВШЭ.')).toBeInTheDocument()
  expect(screen.queryByText('БВИ по информатике.')).not.toBeInTheDocument()
  expect(url()).toBe('?sheet=ai:c-old')

  await userEvent.click(screen.getByRole('button', { name: 'Карточка «Высшая проба»' }))
  expect(url()).toBe('?sheet=ai:c-old,oly:hse:inf')
})

// F58, F59: новый чат пуст, пока в нём нет вопроса; первый вопрос создаёт чат.
it('первый вопрос нового чата создаёт чат и открывает его', async () => {
  const created = chat('c-new', 'Чат 23 сентября', '2026-09-23')
  const sent = server({
    'POST /ai/chats': (body) => [
      201,
      { chat: created, question: msg('q', 'user', (body as { text: string }).text), answer: msg('a', 'assistant', 'БВИ — без экзаменов.') },
    ],
  })
  open('ai', { chats: [itmo], logs: { 'c-itmo': itmoLog } })

  await userEvent.click(screen.getByRole('button', { name: 'Новый чат' }))
  expect(url()).toBe('?sheet=ai:new')
  expect(chatButton()).toHaveTextContent('Новый чат')
  expect(screen.queryByText('БВИ по информатике.')).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Переименовать чат' })).not.toBeInTheDocument()

  await userEvent.type(input(), 'Что такое БВИ?{Enter}')

  expect(await screen.findByText('БВИ — без экзаменов.')).toBeInTheDocument()
  expect(sent).toContainEqual({ route: 'POST /ai/chats', body: { text: 'Что такое БВИ?' } })
  expect(url()).toBe('?sheet=ai:c-new')
  expect(chatButton()).toHaveTextContent('Чат 23 сентября')
  expect((await chatRows()).map((r) => r.textContent)).toEqual([
    expect.stringContaining('Чат 23 сентября'),
    expect.stringContaining('Льготы в ИТМО'),
  ])
})

it('пока новый чат ждёт ответа, можно уйти в другой — лист туда не перескочит', async () => {
  let reply: (r: Response) => void = () => {}
  vi.stubGlobal(
    'fetch',
    vi.fn((input: string, init: RequestInit = {}) =>
      init.method === 'POST' && input.endsWith('/ai/chats')
        ? new Promise<Response>((resolve) => (reply = resolve))
        : new Promise(() => {}),
    ),
  )
  open('ai:new', { chats: [itmo], logs: { 'c-itmo': itmoLog } })

  await userEvent.type(input(), 'Что такое БВИ?{Enter}')
  await userEvent.click((await chatRows())[0]!)
  expect(url()).toBe('?sheet=ai:c-itmo')

  const created = chat('c-new', 'Чат 23 сентября', '2026-09-23')
  reply(
    new Response(
      JSON.stringify({ chat: created, question: msg('q', 'user', 'Что такое БВИ?'), answer: msg('a', 'assistant', 'БВИ — без экзаменов.') }),
      { status: 201, headers: { 'Content-Type': 'application/json' } },
    ),
  )

  // Ответ дошёл: новый чат появился в списке, а открыт по-прежнему выбранный.
  await userEvent.click(chatButton())
  const list = screen.getByRole('list', { name: 'Чаты' })
  await waitFor(() => expect(within(list).getAllByRole('button')).toHaveLength(2))
  expect(url()).toBe('?sheet=ai:c-itmo')
})

// F60: вопрос уходит в открытый чат — сервер покажет модели его реплики.
it('вопрос в открытом чате уходит в этот чат, и чат поднимается в списке', async () => {
  const sent = server({
    'POST /ai/chats/c-old/messages': (body) => [
      200,
      {
        chat: { ...old, last_message_at: '2026-09-23T10:00:00Z' },
        question: msg('q', 'user', (body as { text: string }).text),
        answer: msg('a', 'assistant', 'Ответ в старом чате.'),
      },
    ],
  })
  open('ai:c-old', { chats: [itmo, old], logs: { 'c-old': [] } })

  await userEvent.type(input(), 'А 100 баллов?{Enter}')

  expect(await screen.findByText('Ответ в старом чате.')).toBeInTheDocument()
  expect(sent).toContainEqual({ route: 'POST /ai/chats/c-old/messages', body: { text: 'А 100 баллов?' } })
  expect((await chatRows()).map((r) => r.textContent)).toEqual([
    expect.stringContaining('Чат 18 сентября'),
    expect.stringContaining('Льготы в ИТМО'),
  ])
})

// F59: своё название чата.
it('переименование отправляет название без пробелов по краям и меняет заголовок', async () => {
  const sent = server({
    'PATCH /ai/chats/c-itmo': (body) => [200, { ...itmo, title: (body as { title: string }).title }],
  })
  open('ai:c-itmo', { chats: [itmo], logs: { 'c-itmo': itmoLog } })

  await userEvent.click(screen.getByRole('button', { name: 'Переименовать чат' }))
  const field = screen.getByRole<HTMLInputElement>('textbox', { name: 'Название чата' })
  expect(field).toHaveValue('Льготы в ИТМО')
  // Название выделено целиком: новое набирается поверх, без стирания.
  expect([field.selectionStart, field.selectionEnd]).toEqual([0, 'Льготы в ИТМО'.length])

  await userEvent.clear(field)
  await userEvent.type(field, '  Высшая проба  {Enter}')

  await waitFor(() => expect(chatButton()).toHaveTextContent('Высшая проба'))
  expect(sent).toContainEqual({ route: 'PATCH /ai/chats/c-itmo', body: { title: 'Высшая проба' } })
  expect(screen.queryByRole('textbox', { name: 'Название чата' })).not.toBeInTheDocument()
})

it('отказ в переименовании виден, поле остаётся открытым', async () => {
  server({
    'PATCH /ai/chats/c-itmo': () => [400, { error: { code: 'BAD_REQUEST', message: 'Название длиннее 60 символов — сократите его.' } }],
  })
  open('ai:c-itmo', { chats: [itmo], logs: { 'c-itmo': itmoLog } })

  await userEvent.click(screen.getByRole('button', { name: 'Переименовать чат' }))
  await userEvent.type(screen.getByRole('textbox', { name: 'Название чата' }), ' и МФТИ{Enter}')

  expect(await screen.findByRole('alert')).toHaveTextContent('Название длиннее 60 символов')
  expect(screen.getByRole('textbox', { name: 'Название чата' })).toHaveValue('Льготы в ИТМО и МФТИ')
})

it('Escape отменяет переименование без запроса', async () => {
  const sent = server()
  open('ai:c-itmo', { chats: [itmo], logs: { 'c-itmo': itmoLog } })

  await userEvent.click(screen.getByRole('button', { name: 'Переименовать чат' }))
  await userEvent.type(screen.getByRole('textbox', { name: 'Название чата' }), ' и МФТИ{Escape}')

  expect(screen.queryByRole('textbox', { name: 'Название чата' })).not.toBeInTheDocument()
  expect(chatButton()).toHaveTextContent('Льготы в ИТМО')
  expect(sent.filter((s) => s.route.startsWith('PATCH'))).toEqual([])
})

// Ответ ждут в том чате, куда ушёл вопрос, а не в любом открытом.
it('«печатает…» только в чате, куда ушёл вопрос', async () => {
  server()
  open('ai:c-itmo', { chats: [itmo, old], logs: { 'c-itmo': itmoLog, 'c-old': [] } })

  await userEvent.type(input(), 'А в МФТИ?{Enter}')
  expect(screen.getByLabelText('Помощник печатает')).toBeInTheDocument()

  await userEvent.click((await chatRows())[1]!)
  expect(chatButton()).toHaveTextContent('Чат 18 сентября')
  expect(screen.queryByLabelText('Помощник печатает')).not.toBeInTheDocument()
  expect(screen.queryByText('А в МФТИ?')).not.toBeInTheDocument()
})

// Раньше при 429 вопрос молча пропадал: ни текста в поле, ни объяснения.
it('ошибка отправки возвращает вопрос в поле и показывает сообщение сервера', async () => {
  server({
    'POST /ai/chats/c-itmo/messages': () => [
      429,
      { error: { code: 'RATE_LIMITED', message: 'Слишком много вопросов подряд. Подождите минуту и спросите снова.' } },
    ],
  })
  open('ai:c-itmo', { chats: [itmo], logs: { 'c-itmo': itmoLog } })

  await userEvent.type(input(), 'Что такое БВИ?{Enter}')

  expect(await screen.findByRole('alert')).toHaveTextContent('Слишком много вопросов подряд')
  expect(input()).toHaveValue('Что такое БВИ?')
  expect(screen.queryByText('Что такое БВИ?', { selector: '.ai-message-user p' })).not.toBeInTheDocument()
})

// F37: чужой или несуществующий чат в адресе — открывается свой последний.
it('чат не найден — открывается последний', async () => {
  server({ 'GET /ai/chats/c-gone/messages': () => [404, { error: { code: 'NOT_FOUND', message: 'Чат не найден.' } }] })
  open('ai:c-gone', { chats: [itmo], logs: { 'c-itmo': itmoLog } })

  await waitFor(() => expect(url()).toBe('?sheet=ai'))
  expect(chatButton()).toHaveTextContent('Льготы в ИТМО')
})
