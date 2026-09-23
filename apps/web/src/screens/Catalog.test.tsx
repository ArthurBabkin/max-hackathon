import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { OlympiadListItem } from '@contract'
import { expect, it } from 'vitest'
import { keys } from '@/api/queries'
import { profile } from '@/api/mocks/build'
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
