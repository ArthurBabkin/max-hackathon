import { screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import { keys } from '@/api/queries'
import { home, olympiadCard, session } from '@/api/mocks/build'
import { renderApp } from '@/test/render'

vi.hoisted(() => vi.stubEnv('VITE_USE_MOCKS', 'off'))
const { MatchScreen } = await import('./Match')

const NO_UNIVERSITIES = 'Вузы не выбраны — льготы показаны по всем вузам с направлением'

function renderMatch(universities: number) {
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
  renderApp(<MatchScreen />, {
    route: '/match',
    seed: (c) => {
      c.setQueryData(keys.session, session())
      c.setQueryData(keys.home, { ...home(), universities_count: universities })
      c.setQueryData(keys.recommendations('all'), {
        items: [olympiadCard('hse:inf')],
        outside: [],
        note: 'Сначала ближайшие сроки',
      })
    },
  })
}

// SPEC 11: у траектории без вузов — строка над списком подбора.
it('без вузов объясняет, по каким вузам посчитаны льготы', async () => {
  renderMatch(0)
  expect(await screen.findByText(NO_UNIVERSITIES)).toBeInTheDocument()
})

it('с вузами строки нет', async () => {
  renderMatch(3)
  await screen.findByText('Сначала ближайшие сроки')
  expect(screen.queryByText(NO_UNIVERSITIES)).toBeNull()
})
