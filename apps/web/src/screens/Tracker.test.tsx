import { act, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { TrackerItem } from '@contract'
import { afterEach, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import { ApiError } from '@/api/errors'
import { keys } from '@/api/queries'
import { getWebApp } from '@/bridge'
import { renderApp } from '@/test/render'
import { Toaster } from '@/ui/Toaster'
import { dismissToast } from '@/ui/toast'
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

const inTenMinutes = () => new Date(Date.now() + 10 * 60_000).toISOString()
const expired = { url: '/calendar.ics?token=old', expires_at: new Date(Date.now() - 1000).toISOString() }

function setup(items: TrackerItem[], months: string[], link = { url: '/calendar.ics?token=abc', expires_at: inTenMinutes() }) {
  return renderApp(
    <>
      <TrackerScreen />
      <Toaster />
    </>,
    {
      route: '/tracker',
      seed: (c) => {
        c.setQueryData(keys.tracker, { items, proposals: [] })
        for (const month of months) c.setQueryData(keys.calendar(month), { month, days: [] })
        c.setQueryData(keys.calendarLink, link)
      },
    },
  )
}

afterEach(() => {
  act(() => dismissToast())
  vi.restoreAllMocks()
})

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

it('«Выгрузить в календарь» открывает файл со сроками в браузере телефона', async () => {
  const openLink = vi.spyOn(getWebApp(), 'openLink').mockImplementation(() => {})
  setup([item('near', '2030-03-15T20:59:00Z')], ['2030-03'])

  expect(screen.queryByRole('button', { name: 'Выгрузить в календарь' })).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('tab', { name: 'Календарь' }))
  await userEvent.click(screen.getByRole('button', { name: 'Выгрузить в календарь' }))

  // Ссылка — от базового адреса API, целиком: её открывает внешний браузер.
  expect(openLink).toHaveBeenCalledWith(new URL('/api/v1/calendar.ics?token=abc', location.href).toString())
})

it('устаревшую ссылку на календарь сначала обновляет', async () => {
  const openLink = vi.spyOn(getWebApp(), 'openLink').mockImplementation(() => {})
  vi.spyOn(api, 'get').mockResolvedValue({ url: '/calendar.ics?token=new', expires_at: inTenMinutes() })
  setup([item('near', '2030-03-15T20:59:00Z')], ['2030-03'], expired)

  await userEvent.click(screen.getByRole('tab', { name: 'Календарь' }))
  await userEvent.click(screen.getByRole('button', { name: 'Выгрузить в календарь' }))

  await vi.waitFor(() => expect(openLink).toHaveBeenCalledOnce())
  expect(openLink).toHaveBeenCalledWith(new URL('/api/v1/calendar.ics?token=new', location.href).toString())
})

it('ссылку не выдали — объясняет, а не молчит', async () => {
  const openLink = vi.spyOn(getWebApp(), 'openLink').mockImplementation(() => {})
  vi.spyOn(api, 'get').mockRejectedValue(new ApiError(0, 'NETWORK', ''))
  setup([item('near', '2030-03-15T20:59:00Z')], ['2030-03'], expired)

  await userEvent.click(screen.getByRole('tab', { name: 'Календарь' }))
  await userEvent.click(screen.getByRole('button', { name: 'Выгрузить в календарь' }))

  expect(await screen.findByRole('alert')).toBeInTheDocument()
  expect(openLink).not.toHaveBeenCalled()
})
