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
