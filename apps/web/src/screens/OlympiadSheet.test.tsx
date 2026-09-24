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
  const block = screen.getByRole('heading', { name: /Льгота и условия в твоих вузах/ }).closest('section')!
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

// F18 и F19 в одном блоке: сначала общие условия, потом вузы, и под каждым —
// что в нём не так, как у всех.
it('условия — в блоке льгот: общие сверху, свои — под вузом', () => {
  const base = olympiadDetail('hse:inf')!
  const first = base.benefits[0]!
  const second = base.benefits[1]!
  renderSheet({
    conditions: ['Нужен диплом победителя или призёра', 'БВИ можно использовать только в одном вузе'],
    benefits: [
      { ...first, conditions: ['Призёру — 100 баллов вместо БВИ', 'Порог ЕГЭ от 75 до 80 — зависит от программы'] },
      second,
    ],
  })

  expect(screen.queryByRole('heading', { name: 'Условия' })).not.toBeInTheDocument()
  const block = screen.getByRole('heading', { name: /Льгота и условия в твоих вузах/ }).closest('section')!
  const general = within(block).getByRole('list')
  expect(within(general).getAllByRole('listitem').map((li) => li.textContent)).toEqual([
    'Нужен диплом победителя или призёра',
    'БВИ можно использовать только в одном вузе',
  ])

  const rows = within(block).getAllByRole('button')
  const firstRow = rows[0]!
  const secondRow = rows[1]!
  expect(firstRow).toHaveTextContent(first.university_name)
  expect(within(firstRow).getByText('Призёру — 100 баллов вместо БВИ')).toBeInTheDocument()
  expect(within(firstRow).getByText('Порог ЕГЭ от 75 до 80 — зависит от программы')).toBeInTheDocument()
  expect(secondRow).toHaveTextContent(second.university_name)
  expect(within(secondRow).queryByText(/Призёру|Порог/)).not.toBeInTheDocument()
  // Общий список — над вузами.
  expect(general.compareDocumentPosition(firstRow) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
})

const blockOrder = () => screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)

// Даты — сразу под уровнями: «когда» важно не меньше, чем «насколько сильная».
// «Где ещё даёт льготу» — в конце, после своих вузов.
it('этапы и даты — сразу под уровнем по профилям', () => {
  renderSheet({ description: null, official_url: null })
  expect(blockOrder()).toEqual([
    expect.stringMatching(/^Уровень по профилям/),
    expect.stringMatching(/^Этапы и даты/),
    expect.stringMatching(/^Льгота и условия в твоих вузах/),
    expect.stringMatching(/^Почему подходит/),
    'Где ещё даёт льготу',
  ])
})

it('вне перечня этапы и даты — сразу под «Не входит в перечень»', () => {
  renderSheet({ kind: 'other', description: null, official_url: null })
  expect(blockOrder().slice(0, 2)).toEqual(['Не входит в перечень', expect.stringMatching(/^Этапы и даты/)])
})
