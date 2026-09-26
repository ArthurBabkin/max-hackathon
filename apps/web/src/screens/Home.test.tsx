import { screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import { keys } from '@/api/queries'
import { home } from '@/api/mocks/build'
import { renderApp } from '@/test/render'

vi.hoisted(() => vi.stubEnv('VITE_USE_MOCKS', 'off'))
const { HomeScreen } = await import('./Home')

const dir = (name: string, i: number) => ({ id: `dir-${i}`, name })

function renderHome(names: string[]) {
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
  const h = home()
  renderApp(<HomeScreen />, {
    seed: (c) => c.setQueryData(keys.home, { ...h, trajectory: { ...h.trajectory, directions: names.map(dir) } }),
  })
}

// Цель из многих направлений не растягивает карточку на весь экран.
it('цель из трёх и больше направлений — первое и «ещё N»', () => {
  renderHome(['Программная инженерия', 'Прикладная математика и информатика', 'Экономика', 'Менеджмент'])
  expect(screen.getByRole('heading', { name: 'Программная инженерия и ещё 3 направления' })).toBeInTheDocument()
})

it('два направления — оба целиком', () => {
  renderHome(['Программная инженерия', 'Экономика'])
  expect(screen.getByRole('heading', { name: 'Программная инженерия, Экономика' })).toBeInTheDocument()
})

// Слово после «из N» склоняется по N, а не по числителю (#77).
it('«1 из 3 регистраций» и «1 из 1 регистрации»', () => {
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
  const h = home()
  const { unmount } = renderApp(<HomeScreen />, {
    seed: (c) => c.setQueryData(keys.home, { ...h, tracker_count: 3, registered_count: 1 }),
  })
  expect(screen.getByText('1 из 3').parentElement).toHaveTextContent('1 из 3 регистраций')
  unmount()

  renderApp(<HomeScreen />, {
    seed: (c) => c.setQueryData(keys.home, { ...h, tracker_count: 1, registered_count: 1 }),
  })
  expect(screen.getByText('1 из 1').parentElement).toHaveTextContent('1 из 1 регистрации')
})

it('следующий шаг — название олимпиады без падежа: «Регистрация: «…»»', () => {
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
  const h = home()
  const next_step = { ...h.next_step!, stage_title: 'Регистрация', stage_kind: 'registration' as const, olympiad_name: 'Высшая проба' }
  renderApp(<HomeScreen />, { seed: (c) => c.setQueryData(keys.home, { ...h, next_step }) })
  expect(screen.getByText('Регистрация: «Высшая проба»')).toBeInTheDocument()
})
