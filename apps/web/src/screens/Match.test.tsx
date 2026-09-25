import { fireEvent, screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import type { Recommendations } from '@contract'
import { keys } from '@/api/queries'
import { home, olympiadCard, session } from '@/api/mocks/build'
import { makeSession, renderApp } from '@/test/render'

vi.hoisted(() => vi.stubEnv('VITE_USE_MOCKS', 'off'))
const { MatchScreen } = await import('./Match')

const NO_UNIVERSITIES = 'Вузы не выбраны — льготы показаны по всем вузам с направлением'

function recs(over: Partial<Recommendations> = {}): Recommendations {
  return {
    items: [],
    more: [],
    outside: [],
    note: 'Сначала ближайшие сроки',
    tracked_count: 0,
    proposed_count: 0,
    state: 'ok',
    ...over,
  }
}

function renderState(data: Recommendations, role: 'kid' | 'parent' = 'kid') {
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
  renderApp(<MatchScreen />, {
    route: '/match',
    session: makeSession({ role }),
    seed: (c) => {
      c.setQueryData(keys.home, { ...home(), universities_count: 3 })
      c.setQueryData(keys.recommendations('all'), data)
    },
  })
}

function renderMatch(universities: number, region = { region_code: '16', region_name: 'Республика Татарстан' }) {
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
  const h = home()
  renderApp(<MatchScreen />, {
    route: '/match',
    seed: (c) => {
      c.setQueryData(keys.session, session())
      c.setQueryData(keys.home, { ...h, trajectory: { ...h.trajectory, ...region }, universities_count: universities })
      c.setQueryData(keys.recommendations('all'), recs({ items: [olympiadCard('hse:inf')!] }))
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

// C3: добавленное не показывается — строка говорит, сколько его, и ведёт в трекер.
it('сколько уже в трекере — над списком', async () => {
  renderState(recs({ items: [olympiadCard('hse:inf')!], tracked_count: 3 }))
  expect(await screen.findByText('В трекере уже 3 — здесь только новые')).toBeInTheDocument()
})

it('предложенное и ждущее ответа — отдельной строкой', async () => {
  renderState(recs({ items: [olympiadCard('hse:inf')!], proposed_count: 2 }))
  expect(await screen.findByText('Ждут твоего ответа в трекере: 2')).toBeInTheDocument()
})

it('«Показать ещё» раскрывает остальные подходящие', async () => {
  const more = olympiadCard('inno:inf')!
  renderState(recs({ items: [olympiadCard('hse:inf')!], more: [more] }))
  const button = await screen.findByRole('button', { name: 'Показать ещё 1' })
  expect(screen.queryByText(more.name)).toBeNull()
  fireEvent.click(button)
  expect(await screen.findByText(more.name)).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Показать ещё 1' })).toBeNull()
})

// C7: всё подходящее уже в трекере — похвала вместо пустого списка.
it('всё добавлено — ты большой молодец', async () => {
  renderState(recs({ state: 'all_tracked', tracked_count: 12 }))
  expect(await screen.findByText('Ты уже следишь за всеми подходящими олимпиадами')).toBeInTheDocument()
  expect(screen.getByText(/Ты большой молодец!/)).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Открыть трекер' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Посмотреть каталог' })).toBeInTheDocument()
})

it('родителю — на «вы»', async () => {
  renderState(recs({ state: 'all_tracked', tracked_count: 4 }), 'parent')
  expect(await screen.findByText('Вы уже следите за всеми подходящими олимпиадами')).toBeInTheDocument()
  expect(screen.getByText(/Отличная работа!/)).toBeInTheDocument()
})

it('остальное ждёт ответа на предложение', async () => {
  renderState(recs({ state: 'all_proposed', tracked_count: 2, proposed_count: 1 }))
  expect(await screen.findByText('Остальные подходящие ждут твоего ответа')).toBeInTheDocument()
})

it('подходящих нет — предлагает найти вузы', async () => {
  renderState(recs({ state: 'none_suitable' }))
  expect(await screen.findByText('Подходящих олимпиад пока нет')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Найти вузы' })).toBeInTheDocument()
})
