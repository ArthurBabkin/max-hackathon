import { act, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { TrackerItem, TrackerStage } from '@contract'
import { afterEach, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import { ApiError } from '@/api/errors'
import { keys } from '@/api/queries'
import { getWebApp } from '@/bridge'
import { makeSession, renderApp } from '@/test/render'
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
    status: 'open',
    outcome: null,
    stages: [],
    action: null,
  }) as TrackerItem

const stage = (id: string, kind: TrackerStage['kind'], over: Partial<TrackerStage> = {}): TrackerStage => ({
  id,
  kind,
  title: { registration: 'Регистрация', qualifying: 'Отборочный этап', final: 'Заключительный этап' }[kind as string] ?? kind,
  subtitle: null,
  starts_at: null,
  ends_at: null,
  deadline_at: null,
  state: 'future',
  registered: false,
  result: null,
  can_register: false,
  results: kind === 'registration' ? [] : kind === 'final' ? ['winner', 'prizer', 'participant'] : ['passed', 'failed'],
  results_allowed: [],
  asking: false,
  ...over,
})

/** Высшая проба: регистрация, отборочный, финал — в нужном состоянии. */
const hse = (over: Partial<TrackerItem> = {}, stages: Partial<Record<'reg' | 'qual' | 'fin', Partial<TrackerStage>>> = {}) =>
  ({
    ...item('hse', '2030-03-15T20:59:00Z'),
    olympiad_name: 'Высшая проба',
    stages: [
      stage('hse-reg', 'registration', { state: 'current', can_register: true, deadline_at: '2030-03-15T20:59:00Z', ...stages.reg }),
      stage('hse-qual', 'qualifying', { starts_at: '2030-04-01T06:00:00Z', ...stages.qual }),
      stage('hse-fin', 'final', { subtitle: 'февраль, очно', ...stages.fin }),
    ],
    action: { type: 'register', stage_id: 'hse-reg' },
    ...over,
  }) as TrackerItem

/** Отборочный позади, итог не отмечен — карточка спрашивает. */
const hseAsking = () =>
  hse(
    { status: 'active', registered_at: '2030-01-01T00:00:00Z', action: { type: 'result', stage_id: 'hse-qual' }, deadline_at: null },
    {
      reg: { state: 'past', registered: true, can_register: false },
      qual: { state: 'past', asking: true, results_allowed: ['passed', 'failed'] },
    },
  )

const inTenMinutes = () => new Date(Date.now() + 10 * 60_000).toISOString()
const expired = { url: '/calendar.ics?token=old', expires_at: new Date(Date.now() - 1000).toISOString() }

function setup(
  items: TrackerItem[],
  months: string[] = [],
  link = { url: '/calendar.ics?token=abc', expires_at: inTenMinutes() },
  session = makeSession(),
) {
  return renderApp(
    <>
      <TrackerScreen />
      <Toaster />
    </>,
    {
      route: '/tracker',
      session,
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

// --- Этапы и отметки (F63) ----------------------------------------------------

const heading = (text: string) => screen.getByText((_, el) => el?.classList.contains('week-title') === true && el.textContent === text)

it('делит трекер на «нужно зарегистрироваться», «участвую» и «завершено»', () => {
  setup([
    hse(),
    { ...hseAsking(), id: 'b' },
    { ...hse({ status: 'finished', outcome: 'prizer', action: null }), id: 'c' },
  ])

  expect(heading('Нужно зарегистрироваться: 1')).toBeInTheDocument()
  expect(heading('Участвую: 1')).toBeInTheDocument()
  expect(heading('Завершено: 1')).toBeInTheDocument()
})

it('полоска этапов: короткие названия, текущий выделен', () => {
  setup([hse()])

  const strip = screen.getByRole('list', { name: 'Этапы' })
  const tiles = within(strip).getAllByRole('listitem')
  expect(tiles.map((t) => t.querySelector('b')?.textContent)).toEqual(['Регистрация', 'Отбор', 'Финал'])
  expect(tiles[0]).toHaveClass('strip-now')
})

it('«Регистрация пройдена» отмечает этап сразу, не дожидаясь сервера', async () => {
  let resolve: (v: unknown) => void = () => {}
  const put = vi.spyOn(api, 'put').mockReturnValue(new Promise((r) => (resolve = r)))
  setup([hse()])

  const check = screen.getByRole('checkbox', { name: 'Регистрация пройдена' })
  await userEvent.click(check)

  expect(put).toHaveBeenCalledWith('/tracker/hse/stages/hse-reg', { registered: true, result: null })
  expect(check).toBeChecked()
  resolve(hse({ status: 'active', registered_at: '2030-01-01T00:00:00Z', action: null }))
})

it('закончившийся этап спрашивает итог кнопками', async () => {
  const put = vi.spyOn(api, 'put').mockResolvedValue(hseAsking())
  setup([hseAsking()])

  expect(screen.getByText('Как прошёл отборочный этап?')).toBeInTheDocument()
  expect(screen.getByRole('listitem', { name: /Отбор/ })).toHaveClass('strip-ask')
  await userEvent.click(screen.getByRole('button', { name: 'Прохожу дальше' }))

  expect(put).toHaveBeenCalledWith('/tracker/hse/stages/hse-qual', { registered: false, result: 'passed' })
})

it('родителю вопрос — об ученике по имени', () => {
  setup([hseAsking()], [], undefined, makeSession({ role: 'parent', is_creator: true }))

  expect(screen.getByText('Как у Артёма прошёл отборочный этап?')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Проходит дальше' })).toBeInTheDocument()
})

it('отметку, которую сервер отклонил, откатывает и объясняет', async () => {
  vi.spyOn(api, 'put').mockRejectedValue(new ApiError(409, 'CONFLICT', 'Отметка противоречит другим отметкам'))
  setup([hse()])

  const check = screen.getByRole('checkbox', { name: 'Регистрация пройдена' })
  await userEvent.click(check)

  expect(await screen.findByRole('alert')).toHaveTextContent('Отметка противоречит другим отметкам')
  expect(check).not.toBeChecked()
})

it('«Все этапы» раскрывает этапы с отметками; итог снимается повторным нажатием', async () => {
  const put = vi.spyOn(api, 'put').mockResolvedValue(hseAsking())
  const passed = hse(
    { status: 'active', registered_at: '2030-01-01T00:00:00Z', action: null },
    {
      reg: { state: 'past', registered: true, can_register: false },
      qual: { state: 'past', result: 'passed', results_allowed: ['passed', 'failed'] },
      fin: { state: 'current' },
    },
  )
  setup([passed])

  await userEvent.click(screen.getByRole('button', { name: 'Все этапы' }))

  expect(screen.getByText('Отборочный этап')).toBeInTheDocument()
  expect(screen.getByText('Заключительный этап')).toBeInTheDocument()
  expect(screen.getByText('февраль, очно · итог — после этапа')).toBeInTheDocument()
  const chosen = screen.getByRole('button', { name: 'Прохожу дальше' })
  expect(chosen).toHaveAttribute('aria-pressed', 'true')
  await userEvent.click(chosen)

  expect(put).toHaveBeenCalledWith('/tracker/hse/stages/hse-qual', { registered: false, result: null })
})

it('этапы впереди итог не предлагают, даже без дат', async () => {
  setup([
    hse(
      { status: 'active', registered_at: '2030-01-01T00:00:00Z', registered_by: { id: 'm', name: 'Артём', role: 'kid' }, action: null },
      {
        reg: { state: 'past', registered: true, can_register: true },
        qual: { state: 'current', results_allowed: ['passed', 'failed'] },
        fin: { state: 'future', results_allowed: ['winner', 'prizer', 'participant'] },
      },
    ),
  ])

  await userEvent.click(screen.getByRole('button', { name: 'Все этапы' }))

  expect(screen.getByRole('button', { name: 'Прохожу дальше' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Диплом призёра' })).not.toBeInTheDocument()
  expect(screen.getByText('февраль, очно · итог — после этапа')).toBeInTheDocument()
  expect(screen.getByText(/регистрация есть · отметил\(а\) Артём/)).toBeInTheDocument()
})

it('«Не прохожу» закрывает олимпиаду: дальше серое, отметку можно снять', async () => {
  setup([
    hse(
      { status: 'finished', outcome: 'failed', registered_at: '2030-01-01T00:00:00Z', action: null },
      {
        reg: { state: 'past', registered: true },
        qual: { state: 'past', result: 'failed', results_allowed: ['passed', 'failed'] },
        fin: { state: 'locked' },
      },
    ),
  ])

  expect(screen.getByText('Итог: отборочный этап не пройден')).toBeInTheDocument()
  expect(screen.getByRole('listitem', { name: /Финал/ })).toHaveClass('strip-off')
  await userEvent.click(screen.getByRole('button', { name: 'Все этапы' }))
  expect(screen.getByText('Отметку можно снять — этапы и напоминания вернутся.')).toBeInTheDocument()
})

it('карточка, ушедшая после отметки в «Завершено», остаётся раскрытой', async () => {
  const failed = hse(
    { status: 'finished', outcome: 'failed', registered_at: '2030-01-01T00:00:00Z', action: null },
    {
      reg: { state: 'past', registered: true },
      qual: { state: 'past', result: 'failed', results_allowed: ['passed', 'failed'] },
      fin: { state: 'locked' },
    },
  )
  vi.spyOn(api, 'put').mockResolvedValue(failed)
  setup([hseAsking(), { ...hse(), id: 'other' }])

  const card = screen.getByText('Как прошёл отборочный этап?').closest('article')!
  await userEvent.click(within(card).getByRole('button', { name: 'Все этапы' }))
  await userEvent.click(screen.getByRole('button', { name: 'Не прохожу' }))

  expect(await screen.findByText('Отметку можно снять — этапы и напоминания вернутся.')).toBeInTheDocument()
})

it('«Убрать из трекера» спрашивает подтверждение прямо в карточке', async () => {
  const remove = vi.spyOn(api, 'delete').mockResolvedValue(undefined)
  setup([hse()])

  await userEvent.click(screen.getByRole('button', { name: 'Все этапы' }))
  await userEvent.click(screen.getByRole('button', { name: 'Убрать из трекера' }))
  expect(screen.getByText('Убрать «Высшая проба» из трекера? Напоминаний по ней больше не будет.')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'Оставить' }))
  expect(remove).not.toHaveBeenCalled()

  await userEvent.click(screen.getByRole('button', { name: 'Убрать из трекера' }))
  await userEvent.click(screen.getByRole('button', { name: 'Убрать' }))
  expect(remove).toHaveBeenCalledWith('/tracker/hse')
})

it('родитель при ученике в траектории убрать олимпиаду не может', async () => {
  setup([hse()], [], undefined, makeSession({ role: 'parent', is_creator: true }))

  await userEvent.click(screen.getByRole('button', { name: 'Все этапы' }))
  expect(screen.queryByRole('button', { name: 'Убрать из трекера' })).not.toBeInTheDocument()
})

it('диплом в «Завершено» ведёт к тому, что он даёт в моих вузах', () => {
  setup([hse({ status: 'finished', outcome: 'prizer', action: null })])

  expect(screen.getByText('Итог: диплом призёра')).toBeInTheDocument()
  expect(screen.getByText('призёр', { selector: '.pill' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Что даёт диплом в твоих вузах/ })).toBeInTheDocument()
})

it('олимпиада без этапа-регистрации — галочка «Участвую»', async () => {
  const put = vi.spyOn(api, 'put').mockResolvedValue(hse())
  setup([hse({ stages: [stage('q', 'qualifying')], action: { type: 'register', stage_id: null } })])

  await userEvent.click(screen.getByRole('checkbox', { name: 'Участвую' }))

  expect(put).toHaveBeenCalledWith('/tracker/hse/registered')
})
