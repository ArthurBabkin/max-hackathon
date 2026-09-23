import { screen, within } from '@testing-library/react'
import type { OlympiadDetail } from '@contract'
import { expect, it, vi } from 'vitest'
import { keys } from '@/api/queries'
import { olympiadDetail } from '@/api/mocks/build'
import { renderApp } from '@/test/render'
import type { SheetStack } from '@/ui/sheets'
import { OlympiadSheet } from './OlympiadSheet'

const sheets = { stack: [], open: vi.fn(), back: vi.fn(), closeAll: vi.fn() } as unknown as SheetStack

function renderSheet(patch: Partial<OlympiadDetail>) {
  const id = 'hse:inf'
  const detail = { ...olympiadDetail(id)!, ...patch }
  renderApp(<OlympiadSheet id={id} sheets={sheets} />, { seed: (c) => c.setQueryData(keys.olympiad(id), detail) })
}

// ТЗ F18: льгота без источника — «данные уточняются» и никакой метки.
// «Демо-даты» относится к этапам, у льгот её быть не может.
it('льготы без источника не помечает ни «Фактом», ни «Демо-датами»', () => {
  renderSheet({ benefits_source: null })
  const block = screen.getByRole('heading', { name: /Льгота в твоих вузах/ }).closest('section')!
  expect(within(block).getByText('данные уточняются')).toBeInTheDocument()
  expect(within(block).queryByText('Демо-даты')).not.toBeInTheDocument()
  expect(within(block).queryByText('Факт')).not.toBeInTheDocument()
})
