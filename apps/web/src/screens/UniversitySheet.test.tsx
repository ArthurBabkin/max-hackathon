import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { UniversityDetail } from '@contract'
import { expect, it, vi } from 'vitest'
import { keys } from '@/api/queries'
import { profile, universityDetail } from '@/api/mocks/build'
import { UNIVERSITIES } from '@/api/mocks/fixtures'
import { getWebApp } from '@/bridge'
import { renderApp } from '@/test/render'
import type { SheetStack } from '@/ui/sheets'
import { UniversitySheet } from './UniversitySheet'

// Сохранение уходит в «живой» API: моки ответили бы сами и тело не поймать.
vi.hoisted(() => vi.stubEnv('VITE_USE_MOCKS', 'off'))

const sheets = { stack: [], open: vi.fn(), back: vi.fn(), closeAll: vi.fn() } as unknown as SheetStack

function renderSheet(patch: Partial<UniversityDetail>) {
  const base = universityDetail(UNIVERSITIES[0]!)
  renderApp(<UniversitySheet id={base.id} sheets={sheets} />, {
    seed: (c) => c.setQueryData(keys.university(base.id, null), { ...base, ...patch }),
  })
}

it('рассказывает о вузе и ведёт на сайт и правила приёма', async () => {
  const openLink = vi.spyOn(getWebApp(), 'openLink').mockImplementation(() => {})
  renderSheet({
    description: 'Университет в Татарстане, основан в 2012 году.',
    site_url: 'https://innopolis.university/',
    rules_url: 'https://apply.innopolis.university/rules.pdf',
  })

  const about = screen.getByRole('heading', { name: 'О вузе' }).closest('section')!
  expect(within(about).getByText('Университет в Татарстане, основан в 2012 году.')).toBeInTheDocument()
  await userEvent.click(within(about).getByRole('button', { name: /Сайт вуза/ }))
  expect(openLink).toHaveBeenLastCalledWith('https://innopolis.university/')
  await userEvent.click(within(about).getByRole('button', { name: /Правила приёма/ }))
  expect(openLink).toHaveBeenLastCalledWith('https://apply.innopolis.university/rules.pdf')
})

const benefitRow = (olympiad_id: string, subject_name: string, benefit = 'bvi') => ({
  olympiad_profile_id: `${olympiad_id}-${subject_name}`,
  olympiad_id,
  name: `Олимпиада ${olympiad_id}`,
  subject_name,
  level: 'I',
  benefit,
  benefit_label: null,
  short_name: null,
  color: null,
})

it('олимпиады с льготой — строка на олимпиаду, длинный список свёрнут', async () => {
  renderSheet({
    olympiads: [
      benefitRow('hse', 'Информатика'),
      benefitRow('hse', 'Математика'),
      ...['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i'].map((id) => benefitRow(id, 'Физика', 'score100')),
    ] as unknown as UniversityDetail['olympiads'],
  })
  const block = screen.getByRole('heading', { name: /Олимпиады с льготой/ }).closest('section')!
  expect(within(block).getByText('Информатика, Математика')).toBeInTheDocument()
  expect(within(block).getAllByRole('button', { name: /Олимпиада/ })).toHaveLength(8)
  await userEvent.click(within(block).getByRole('button', { name: 'Показать все 10' }))
  expect(within(block).getAllByRole('button', { name: /Олимпиада/ })).toHaveLength(10)
})

it('выбор направления показывает олимпиады с льготой на него, кнопка сохраняет направление', async () => {
  const sent: unknown[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((_url: string, init: RequestInit) => {
      if (init.method === 'PUT') sent.push(JSON.parse(init.body as string))
      return new Promise(() => {})
    }),
  )
  const u = UNIVERSITIES[0]!
  const base = universityDetail(u)
  renderApp(<UniversitySheet id={base.id} sheets={sheets} />, {
    seed: (c) => {
      c.setQueryData(keys.profile, profile())
      c.setQueryData(keys.university(base.id, null), base)
      c.setQueryData(keys.university(base.id, 'inno__ai'), universityDetail(u, 'inno__ai'))
    },
  })

  const programs = screen.getByRole('heading', { name: 'Направления подготовки' }).closest('section')!
  const olympiads = screen.getByRole('heading', { name: /Олимпиады с льготой/ }).closest('section')!
  const rows = () => olympiads.querySelectorAll('.benefit').length
  const before = rows()

  const ai = within(programs).getByRole('button', { name: /^Искусственный интеллект и наука о данных/ })
  await userEvent.click(ai)
  expect(ai).toHaveAttribute('aria-pressed', 'true')
  expect(within(olympiads).getByText('Льготы на направление «Искусственный интеллект и наука о данных»')).toBeVisible()
  expect(rows()).toBeLessThan(before)

  await userEvent.click(within(olympiads).getByRole('button', { name: 'Все направления' }))
  expect(ai).toHaveAttribute('aria-pressed', 'false')

  // Уже сохранённое направление помечено; второе добавляется к нему.
  expect(within(programs).getByRole('button', { name: 'Убрать направление «Программная инженерия» из сохранённых' })).toBeVisible()
  await userEvent.click(
    within(programs).getByRole('button', { name: /Сохранить направление «Искусственный интеллект и наука о данных»/ }),
  )
  expect(sent).toEqual([{ program_ids: ['inno__se', 'inno__ai'] }])
})
