import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { UniversityDetail } from '@contract'
import { expect, it, vi } from 'vitest'
import { keys } from '@/api/queries'
import { universityDetail } from '@/api/mocks/build'
import { UNIVERSITIES } from '@/api/mocks/fixtures'
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
