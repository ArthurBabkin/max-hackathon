import { screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import { keys } from '@/api/queries'
import { home, olympiadCard, session } from '@/api/mocks/build'
import { renderApp } from '@/test/render'

vi.hoisted(() => vi.stubEnv('VITE_USE_MOCKS', 'off'))
const { MatchScreen } = await import('./Match')

const NO_UNIVERSITIES = 'Вузы не выбраны — льготы показаны по всем вузам с направлением'

function renderMatch(universities: number, region = { region_code: '16', region_name: 'Республика Татарстан' }) {
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
  const h = home()
  renderApp(<MatchScreen />, {
    route: '/match',
    seed: (c) => {
      c.setQueryData(keys.session, session())
      c.setQueryData(keys.home, { ...h, trajectory: { ...h.trajectory, ...region }, universities_count: universities })
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

it('в подзаголовке класс и регион', async () => {
  renderMatch(3)
  expect(await screen.findByText(/, 9 класс, Республика Татарстан$/)).toBeInTheDocument()
})

// «Не важно» в боте: регион не указан — подзаголовок без него и без хвоста «, ».
it('без региона подзаголовок заканчивается классом', async () => {
  renderMatch(3, { region_code: '', region_name: '' })
  expect(await screen.findByText(/, 9 класс$/)).toBeInTheDocument()
})
