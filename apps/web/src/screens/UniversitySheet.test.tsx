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
