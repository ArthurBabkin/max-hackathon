import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { TrackerItem } from '@contract'
import { expect, it } from 'vitest'
import { keys } from '@/api/queries'
import { renderApp } from '@/test/render'
import { TrackerScreen } from './Tracker'

const item = (id: string, deadline_at: string | null) =>
  ({
    id,
    olympiad_profile_id: `${id}-inf`,
    olympiad_id: id,
    olympiad_name: `Олимпиада ${id}`,
    subject_name: 'Информатика',
    kind: 'perechen',
    level: 'I',
    short_name: null,
    color: null,
    deadline_at,
    next_stage_title: 'Регистрация',
    registered_at: null,
    registered_by: null,
    added_by: null,
  }) as TrackerItem

function setup(items: TrackerItem[], months: string[]) {
  return renderApp(<TrackerScreen />, {
    route: '/tracker',
    seed: (c) => {
      c.setQueryData(keys.tracker, { items, proposals: [] })
      for (const month of months) c.setQueryData(keys.calendar(month), { month, days: [] })
    },
  })
}

const monthTitle = () => document.querySelector('.calendar-head b')?.textContent

it('календарь открывается на месяце ближайшего срока', async () => {
  // Сроки далеко в будущем: тест не зависит от сегодняшней даты.
  setup([item('far', '2030-05-20T20:59:00Z'), item('near', '2030-03-15T20:59:00Z'), item('old', '2020-01-10T20:59:00Z')], [
    '2030-03',
    '2030-04',
  ])

  await userEvent.click(screen.getByRole('tab', { name: 'Календарь' }))

  expect(monthTitle()).toBe('Март 2030')
})

it('при каждом входе в календарь снова показывает ближайший срок', async () => {
  setup([item('near', '2030-03-15T20:59:00Z')], ['2030-03', '2030-04'])

  await userEvent.click(screen.getByRole('tab', { name: 'Календарь' }))
  await userEvent.click(screen.getByRole('button', { name: 'Следующий месяц' }))
  expect(monthTitle()).toBe('Апрель 2030')

  await userEvent.click(screen.getByRole('tab', { name: 'Список' }))
  await userEvent.click(screen.getByRole('tab', { name: 'Календарь' }))

  expect(monthTitle()).toBe('Март 2030')
})

it('без будущих сроков календарь открывается на текущем месяце', async () => {
  const current = new Intl.DateTimeFormat('en-CA', { timeZone: 'Europe/Moscow' }).format(new Date()).slice(0, 7)
  const [year, month] = current.split('-')
  const title = new Intl.DateTimeFormat('ru-RU', { month: 'long' }).format(new Date(Number(year), Number(month) - 1, 1))
  setup([item('old', '2020-01-10T20:59:00Z'), item('none', null)], [current])

  await userEvent.click(screen.getByRole('tab', { name: 'Календарь' }))

  expect(monthTitle()).toBe(`${title.charAt(0).toUpperCase()}${title.slice(1)} ${year}`)
})
