import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { UniversityDetail } from '@contract'
import { expect, it, vi } from 'vitest'
import { keys } from '@/api/queries'
import { universityDetail } from '@/api/mocks/build'
import { UNIVERSITIES } from '@/api/mocks/fixtures'
import { getWebApp } from '@/bridge'
import { renderApp } from '@/test/render'
import type { SheetStack } from '@/ui/sheets'
import { UniversitySheet } from './UniversitySheet'

const sheets = { stack: [], open: vi.fn(), back: vi.fn(), closeAll: vi.fn() } as unknown as SheetStack

function renderSheet(patch: Partial<UniversityDetail>) {
  const base = universityDetail(UNIVERSITIES[0]!)
  renderApp(<UniversitySheet id={base.id} sheets={sheets} />, {
    seed: (c) => c.setQueryData(keys.university(base.id), { ...base, ...patch }),
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

// У НТО в вузе сорок профилей: через запятую они занимали экран. Свёрнутая
// подпись — пара строк, остальное — по кнопке, и так же сворачивается.
it('длинный список профилей олимпиады свёрнут до пары строк', async () => {
  const subjects = Array.from({ length: 40 }, (_, i) => `Профиль ${i + 1}`)
  renderSheet({
    olympiads: [
      ...subjects.map((s) => benefitRow('nto', s, 'score100')),
      ...['Математика', 'Физика', 'Химия', 'Биология'].map((s) => benefitRow('vsosh', s)),
    ] as unknown as UniversityDetail['olympiads'],
  })
  const block = screen.getByRole('heading', { name: /Олимпиады с льготой/ }).closest('section')!
  const collapsed = `${subjects.slice(0, 6).join(', ')} и ещё 34`
  expect(within(block).getByText(collapsed)).toBeInTheDocument()
  // Короткий список — не список: все видны, кнопки нет.
  expect(within(block).getByText('Математика, Физика, Химия, Биология')).toBeInTheDocument()
  expect(within(block).getAllByRole('button', { name: /Все профили/ })).toHaveLength(1)

  await userEvent.click(within(block).getByRole('button', { name: 'Все профили (40)' }))
  expect(within(block).getByText(subjects.join(', '))).toBeInTheDocument()
  await userEvent.click(within(block).getByRole('button', { name: 'Свернуть' }))
  expect(within(block).getByText(collapsed)).toBeInTheDocument()
})

// Профиль НТО бывает длиной в абзац: три таких «коротких» профиля занимали
// семь строк, и казалось, что список не свернулся. Считается длина, а не число.
it('свёрнутая подпись — по длине, а не по числу профилей', () => {
  const subjects = [
    'Автоматизация бизнес-процессов',
    'Автономные транспортные системы',
    'Беспилотный транспорт: аэрокосмические системы, беспилотные авиационные системы, водные робототехнические системы, летающая робототехника',
    'Большие данные и машинное обучение',
  ]
  renderSheet({ olympiads: subjects.map((s) => benefitRow('nto', s)) as unknown as UniversityDetail['olympiads'] })
  const block = screen.getByRole('heading', { name: /Олимпиады с льготой/ }).closest('section')!
  expect(
    within(block).getByText('Автоматизация бизнес-процессов, Автономные транспортные системы и ещё 2'),
  ).toBeInTheDocument()
  expect(within(block).getByRole('button', { name: 'Все профили (4)' })).toBeInTheDocument()
})
