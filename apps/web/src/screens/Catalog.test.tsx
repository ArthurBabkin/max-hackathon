import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { OlympiadListItem } from '@contract'
import { expect, it } from 'vitest'
import { keys } from '@/api/queries'
import { profile, universityListItem } from '@/api/mocks/build'
import { UNIVERSITIES } from '@/api/mocks/fixtures'
import { renderApp } from '@/test/render'
import { CatalogScreen } from './Catalog'

const olympiad = (id: string, level: 'I' | 'II' | null, kind: 'vsosh' | 'perechen' = 'perechen') =>
  ({
    olympiad_id: id,
    name: `Олимпиада ${id}`,
    organizer: 'Вуз',
    kind,
    final_city: null,
    short_name: null,
    color: null,
    primary_profile: { olympiad_profile_id: `${id}-inf`, subject_code: 'inf', subject_name: 'Информатика', level },
    profiles_count: 1,
  }) as unknown as OlympiadListItem

const II = ['b1', 'b2', 'b3', 'b4', 'b5', 'b6', 'b7'].map((id) => olympiad(id, 'II'))

function setup() {
  // Ученик демо-профиля выбрал информатику и математику: каталог сразу про его предмет (D1).
  return renderApp(<CatalogScreen />, {
    route: '/catalog',
    seed: (c) => {
      c.setQueryData(keys.profile, profile())
      c.setQueryData(keys.olympiads('', 'inf', 'all'), {
        items: [olympiad('v', null, 'vsosh'), olympiad('a', 'I'), ...II],
      })
      c.setQueryData(keys.tracker, {
        items: [{ id: 't1', olympiad_id: 'a' }],
        proposals: [],
      })
    },
  })
}

it('по умолчанию — предмет ученика, олимпиады разложены по уровням', () => {
  setup()
  expect(screen.getByRole('button', { name: 'Информатика', pressed: true })).toBeInTheDocument()
  const headings = screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)
  expect(headings).toEqual(['ВсОШ1', 'I уровень1', 'II уровень7'])
})

it('длинная группа свёрнута до пяти, «Показать ещё» раскрывает остальное', async () => {
  setup()
  const group = screen.getByRole('heading', { name: /II уровень/ }).closest('section')!
  expect(within(group).getAllByRole('button', { name: /Олимпиада b/ })).toHaveLength(5)
  await userEvent.click(within(group).getByRole('button', { name: 'Показать ещё 2' }))
  expect(within(group).getAllByRole('button', { name: /Олимпиада b/ })).toHaveLength(7)
})

it('олимпиада из трекера помечена', () => {
  setup()
  expect(within(screen.getByRole('button', { name: /Олимпиада a/ })).getByText('в трекере')).toBeInTheDocument()
})

// Вузы фильтруются по направлению подготовки; направления ученика — первыми.
it('вузы фильтруются по направлению', async () => {
  const byId = (...ids: string[]) => ({
    items: ids.map((id) => universityListItem(UNIVERSITIES.find((u) => u.id === id)!)),
  })
  renderApp(<CatalogScreen />, {
    route: '/catalog',
    seed: (c) => {
      c.setQueryData(keys.profile, profile())
      c.setQueryData(keys.olympiads('', 'inf', 'all'), { items: [] })
      c.setQueryData(keys.tracker, { items: [], proposals: [] })
      c.setQueryData(keys.directions, {
        items: [
          { id: 'dir-math', name: 'Математика' },
          { id: 'dir-se', name: 'Программная инженерия' },
        ],
      })
      c.setQueryData(keys.universities('', 'all', 'all'), byId('inno', 'kfu', 'hse'))
      c.setQueryData(keys.universities('', 'all', 'dir-math'), byId('kfu', 'hse'))
    },
  })

  await userEvent.click(screen.getByRole('tab', { name: 'Вузы' }))
  const chips = screen.getByText('Направление').closest<HTMLElement>('.filter-row')!
  expect(within(chips).getAllByRole('button').map((b) => b.textContent)).toEqual([
    'Все',
    'Программная инженерия',
    'Математика',
  ])
  expect(screen.getByText('Университет Иннополис')).toBeInTheDocument()

  await userEvent.click(within(chips).getByRole('button', { name: 'Математика' }))
  expect(within(chips).getByRole('button', { name: 'Математика' })).toHaveAttribute('aria-pressed', 'true')
  expect(screen.queryByText('Университет Иннополис')).not.toBeInTheDocument()
  expect(screen.getByText('Казанский федеральный университет')).toBeInTheDocument()
})
