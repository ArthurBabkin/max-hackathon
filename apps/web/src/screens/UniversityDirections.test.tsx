import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { keys } from '@/api/queries'
import { universityDetail } from '@/api/mocks/build'
import { UNIVERSITIES } from '@/api/mocks/fixtures'
import { state } from '@/api/mocks/state'
import { makeSession, renderApp } from '@/test/render'
import type { SheetStack } from '@/ui/sheets'

// Выбор уходит в «живой» API: моки ответили бы сами и тело не поймать.
vi.hoisted(() => vi.stubEnv('VITE_USE_MOCKS', 'off'))
const { UniversitySheet } = await import('./UniversitySheet')

const sheets = { stack: [], open: vi.fn(), back: vi.fn(), closeAll: vi.fn() } as unknown as SheetStack
const uni = (id: string) => universityDetail(UNIVERSITIES.find((u) => u.id === id)!)

let sent: { url: string; body: unknown }[] = []
beforeEach(() => {
  sent = []
  state.directions = [{ id: 'dir-se', name: 'Программная инженерия' }]
  state.chosen = {}
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init: RequestInit) => {
      if (init.method === 'PUT') sent.push({ url, body: JSON.parse(init.body as string) })
      return new Promise(() => {})
    }),
  )
})
afterEach(() => vi.unstubAllGlobals())

function renderSheet(id: string, session = makeSession(), focus?: string) {
  const data = uni(id)
  renderApp(<UniversitySheet id={id} focus={focus} sheets={sheets} />, {
    session,
    seed: (c) => c.setQueryData(keys.university(id), data),
  })
  return screen.getByRole('heading', { name: /^Направления/ }).closest('section')!
}

// D3 (F65): направления вуза — цель первой, дальше где больше олимпиад;
// видно код, число программ и олимпиад с льготой.
it('показывает направления вуза: цель первой, пять сразу, остальные по кнопке', async () => {
  const block = within(renderSheet('hse'))

  const rows = block.getAllByRole('checkbox')
  expect(rows).toHaveLength(5)
  expect(rows[0]).toHaveAccessibleName(/Программная инженерия/)
  expect(rows[0]).toHaveAttribute('aria-checked', 'false')
  expect(rows[0]).toHaveTextContent('09.03.04 · 3 программы')
  expect(rows[0]).toHaveTextContent('3 олимпиады')
  // Льготы сейчас считаются по цели — строка цели помечена.
  expect(rows[0]).toHaveTextContent('по твоей цели')
  expect(rows[1]).not.toHaveTextContent('по твоей цели')
  expect(block.getByText(/Льготы считаем на направления из твоей цели/)).toBeInTheDocument()

  await userEvent.click(block.getByRole('button', { name: 'Все направления (8)' }))
  expect(block.getAllByRole('checkbox')).toHaveLength(8)
})

it('отметка направления сразу сохраняется в вузе', async () => {
  const block = within(renderSheet('hse'))

  await userEvent.click(block.getByRole('checkbox', { name: /Прикладная математика/ }))

  expect(sent).toEqual([
    { url: expect.stringContaining('/profile/universities/hse/directions'), body: { direction_ids: ['dir-ami'] } },
  ])
  expect(block.getByRole('checkbox', { name: /Прикладная математика/ })).toHaveAttribute('aria-checked', 'true')
})

// Отмеченная строка не прыгает наверх, пока карточка открыта: список не
// должен уезжать из-под пальца.
it('порядок направлений не меняется после отметки', async () => {
  const data = uni('hse')
  const names = () => within(section()).getAllByRole('checkbox').map((b) => b.textContent)
  let client!: import('@tanstack/react-query').QueryClient
  renderApp(<UniversitySheet id="hse" sheets={sheets} />, {
    seed: (c) => {
      client = c
      c.setQueryData(keys.university('hse'), data)
    },
  })
  const section = () => screen.getByRole('heading', { name: /^Направления/ }).closest('section')!
  const before = names()

  // Сервер вернул выбранное первым.
  const [first, ...rest] = data.offered_directions
  const math = rest.find((d) => d.id === 'dir-math')!
  act(() => {
    client.setQueryData(keys.university('hse'), {
      ...data,
      offered_directions: [{ ...math, is_mine: true }, first!, ...rest.filter((d) => d !== math)],
    })
  })
  await waitFor(() =>
    expect(within(section()).getByRole('checkbox', { name: /^Математика/ })).toHaveAttribute('aria-checked', 'true'),
  )
  expect(names().map((x) => x?.slice(0, 12))).toEqual(before.map((x) => x?.slice(0, 12)))
})

it('льготы на направлении ещё проверяются — так и написано', () => {
  const block = within(renderSheet('kfu'))
  expect(block.getByRole('checkbox', { name: /Информационная безопасность/ })).toHaveTextContent('льготы уточняются')
})

it('выбранные направления — в счётчике и на кнопке «В моих вузах»', () => {
  state.chosen = { hse: ['dir-se', 'dir-ami'] }
  const block = within(renderSheet('hse'))

  expect(block.getByText('моих: 2 из 8')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'В моих вузах · 2 направления' })).toBeInTheDocument()
})

// Олимпиады: на мои направления — льгота на них, во «Все» — на скольких
// направлениях вуза она есть.
it('олимпиады — «На мои направления» и «Все»', async () => {
  renderSheet('hse')
  const block = within(screen.getByRole('heading', { name: /Олимпиады с льготой/ }).closest('section')!)

  expect(block.getByRole('button', { name: 'На мои направления 3' })).toHaveAttribute('aria-pressed', 'true')
  expect(block.getByRole('button', { name: /Высшая проба/ })).toHaveTextContent('100 баллов')
  expect(block.queryByRole('button', { name: /Технокубок/ })).toBeNull()

  await userEvent.click(block.getByRole('button', { name: 'Все 4' }))
  expect(block.getByRole('button', { name: /Высшая проба/ })).toHaveTextContent('БВИ')
  expect(block.getByRole('button', { name: /Высшая проба/ })).toHaveTextContent('на 7 из 8 направлений')
  expect(block.getByRole('button', { name: /Технокубок/ })).toHaveTextContent('на 4 из 8 направлений')
})

// Цель вузу не подходит и ничего не выбрано — льготы вуза целиком, фильтра нет.
it('без выбора и цели — подсказка отметить направления, олимпиады без фильтра', () => {
  const block = within(renderSheet('mipt', makeSession({ role: 'parent', is_creator: true })))

  expect(block.getByText('Отметьте направления — покажем льготы именно на них.')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /На направления/ })).toBeNull()
})

// Из каталога с фильтром по направлению (F67): это направление — первым,
// даже если оно не из цели и в первые пять не попало бы.
it('открытая на направлении карточка показывает его первым', () => {
  const block = within(renderSheet('hse', makeSession(), 'dir-is'))

  const rows = block.getAllByRole('checkbox')
  expect(rows).toHaveLength(5)
  expect(rows[0]).toHaveAccessibleName(/Информационная безопасность/)
  expect(rows[1]).toHaveAccessibleName(/Программная инженерия/)
})
