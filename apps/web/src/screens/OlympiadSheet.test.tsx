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

it('рассказывает об олимпиаде и ведёт на её сайт с самого верха карточки', async () => {
  const { default: userEvent } = await import('@testing-library/user-event')
  const { getWebApp } = await import('@/bridge')
  const openLink = vi.spyOn(getWebApp(), 'openLink').mockImplementation(() => {})
  renderSheet({ description: 'Олимпиада НИУ ВШЭ по двенадцати профилям.', official_url: 'https://olymp.hse.ru/mmo' })

  const about = screen.getByRole('heading', { name: 'Об олимпиаде' }).closest('section')!
  expect(within(about).getByText('Олимпиада НИУ ВШЭ по двенадцати профилям.')).toBeInTheDocument()
  await userEvent.click(within(about).getByRole('button', { name: /Официальный сайт/ }))
  expect(openLink).toHaveBeenCalledWith('https://olymp.hse.ru/mmo')
})

it('без описания и сайта блока «Об олимпиаде» нет', () => {
  renderSheet({ description: null, official_url: null })
  expect(screen.queryByRole('heading', { name: 'Об олимпиаде' })).not.toBeInTheDocument()
})
