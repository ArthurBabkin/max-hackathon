import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { OlympiadDetail } from '@contract'
import { expect, it, vi } from 'vitest'
import { keys } from '@/api/queries'
import { olympiadDetail } from '@/api/mocks/build'
import { spokenText } from '@/test/a11y'
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
  const base = olympiadDetail('hse:inf')!
  renderSheet({ benefits_source: null, benefits: base.benefits.map((row) => ({ ...row, source: null })) })
  const block = screen.getByRole('heading', { name: /Льгота и условия в твоих вузах/ }).closest('section')!
  expect(within(block).getByText('данные уточняются')).toBeInTheDocument()
  expect(within(block).queryByText('Демо-даты')).not.toBeInTheDocument()
  expect(within(block).queryByText('Факт')).not.toBeInTheDocument()
})

// У каждого вуза свои правила приёма: ссылка — на документ своего вуза, а
// «уточняется» — только у вуза без источника, а не у всего блока.
it('источник льготы — у каждого вуза свой, «уточняется» — с названием вуза', () => {
  const first = olympiadDetail('hse:inf')!.benefits[0]!
  const src = (id: string, title: string) => ({
    id,
    kind: 'rules' as const,
    title,
    url: `https://${id}.example/rules.pdf`,
    verified_at: '2026-09-21',
  })
  renderSheet({
    benefits_source: null,
    benefits: [
      { ...first, university_id: 'itmo', university_nick: 'ИТМО', source: src('itmo', 'ИТМО: особые права, 2026') },
      { ...first, university_id: 'hse', university_nick: 'ВШЭ', source: src('hse', 'ВШЭ: особые права, 2026') },
      { ...first, university_id: 'kazan-gmu', university_nick: 'КГМУ', source: null },
    ],
  })
  const block = screen.getByRole('heading', { name: /Льгота и условия в твоих вузах/ }).closest('section')!
  expect(within(block).getByRole('link', { name: /ИТМО: особые права, 2026/ })).toHaveAttribute(
    'href',
    'https://itmo.example/rules.pdf',
  )
  expect(within(block).getByRole('link', { name: /ВШЭ: особые права, 2026/ })).toHaveAttribute(
    'href',
    'https://hse.example/rules.pdf',
  )
  expect(within(block).getByText('КГМУ: данные уточняются')).toBeInTheDocument()
})

// Олимпиаду не учитывает ни один вуз ученика — уточнять нечего: строки
// таблицы уже это говорят, «данные уточняются» под ними вводит в заблуждение.
it('без учитывающих вузов не пишет «данные уточняются»', () => {
  const first = olympiadDetail('hse:inf')!.benefits[0]!
  renderSheet({
    benefits_source: null,
    benefits: [{ ...first, winner: null, prizer: null, source: null }],
  })
  const block = screen.getByRole('heading', { name: /Льгота и условия в твоих вузах/ }).closest('section')!
  expect(within(block).queryByText(/данные уточняются/)).not.toBeInTheDocument()
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

// F18 и F19 в одном блоке: вузы — таблицей, своё у вуза — под его строкой,
// общее для всех — после таблицы под подписью «Во всех вузах».
it('льготы — таблицей, общие условия — после неё под «Во всех вузах»', async () => {
  const { default: userEvent } = await import('@testing-library/user-event')
  const base = olympiadDetail('hse:inf')!
  const first = base.benefits[0]!
  renderSheet({
    conditions: ['Нужен диплом победителя или призёра', 'БВИ можно использовать только в одном вузе'],
    benefits: [{ ...first, conditions: ['Засчитывает только диплом 11 класса'] }, ...base.benefits.slice(1)],
  })

  expect(screen.queryByRole('heading', { name: 'Условия' })).not.toBeInTheDocument()
  const block = screen.getByRole('heading', { name: /Льгота и условия в твоих вузах/ }).closest('section')!
  const table = within(block).getByRole('table')
  // Под вузом — направления (F65), порядок сверяется по кнопкам вузов.
  expect(within(table).getAllByRole('rowheader').map((h) => spokenText(within(h).getByRole('button')))).toEqual(
    base.benefits.filter((row) => row.winner || row.prizer).map((row) => row.university_nick),
  )
  expect(within(table).getByText('Засчитывает только диплом 11 класса')).toBeInTheDocument()

  const everywhere = within(block).getByText('Во всех вузах')
  const general = within(block).getByRole('list')
  expect(within(general).getAllByRole('listitem').map((li) => li.textContent)).toEqual([
    'Нужен диплом победителя или призёра',
    'БВИ можно использовать только в одном вузе',
  ])
  expect(table.compareDocumentPosition(everywhere) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(everywhere.compareDocumentPosition(general) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

  await userEvent.click(within(table).getByRole('button', { name: first.university_nick }))
  expect(sheets.open).toHaveBeenCalledWith({ kind: 'vuz', id: first.university_id })
})

// Вузов в таблице нет — «во всех» сказать не о ком, условия идут без подписи.
it('ни один вуз олимпиаду не учитывает — без таблицы и подписи «Во всех вузах»', () => {
  const base = olympiadDetail('hse:inf')!
  renderSheet({
    benefits: base.benefits.map((row) => ({
      ...row,
      benefit: null,
      benefit_label: null,
      winner: null,
      prizer: null,
      directions_count: 0,
    })),
    conditions: ['Вузы из базы льгот по этому профилю не дают'],
  })
  const block = screen.getByRole('heading', { name: /Льгота и условия в твоих вузах/ }).closest('section')!
  expect(within(block).queryByRole('table')).not.toBeInTheDocument()
  expect(within(block).queryByText('Во всех вузах')).not.toBeInTheDocument()
  expect(within(block).getByText('Вузы из базы льгот по этому профилю не дают')).toBeInTheDocument()
  expect(within(block).getByText(/^Не учитыва/)).toBeInTheDocument()
})

// Кнопку трекера приходилось искать, пролистав карточку до конца. Она
// закреплена внизу листа, вне прокрутки: видна сразу, какой бы длинной ни
// была карточка, — вместе с подписью о напоминаниях.
it('кнопка трекера закреплена внизу листа, вне прокрутки', () => {
  renderSheet({ in_tracker: false, proposal_status: null })
  const button = screen.getByRole('button', { name: 'Добавить в трекер' })
  expect(button.closest('[role="dialog"]')).not.toBeNull()
  expect(button.closest('.sheet-body')).toBeNull()
  expect(screen.getByText('После добавления напоминания получат все участники').closest('.sheet-body')).toBeNull()
})

it('олимпиада уже в трекере — статус там же, внизу листа', () => {
  renderSheet({ in_tracker: true })
  const status = screen.getByRole('button', { name: 'В трекере' })
  expect(status).toBeDisabled()
  expect(status.closest('.sheet-body')).toBeNull()
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

// Регистрация закрылась, а отборочный ещё впереди: пилюля срока — уже про
// отборочный, без пометки казалось бы, что вступить ещё можно.
it('закрытая регистрация — пометкой под шапкой', () => {
  renderSheet({ registration_closed: true })
  expect(screen.getByText('Регистрация на этот сезон закрыта')).toBeInTheDocument()
  expect(screen.getByText(/Если регистрация уже есть — добавь олимпиаду в трекер/)).toBeInTheDocument()
})

it('открытая регистрация — без пометки', () => {
  renderSheet({ registration_closed: false })
  expect(screen.queryByText('Регистрация на этот сезон закрыта')).toBeNull()
})

// У НТО 21 профиль — плашки уровней занимали больше экрана. Видно шесть,
// профиль ученика среди них, даже если в списке он последний.
it('уровни по профилям: длинный список свёрнут, мой профиль виден', async () => {
  const profiles = Array.from({ length: 21 }, (_, i) => ({
    olympiad_profile_id: `nto-${i + 1}`,
    subject_code: `p${i + 1}`,
    subject_name: `Профиль ${i + 1}`,
    level: 'II' as const,
    is_mine: i === 20,
  }))
  renderSheet({ kind: 'perechen', profiles })
  const block = screen.getByRole('heading', { name: /Уровень по профилям/ }).closest('section')!
  const chips = () => within(block).getAllByText(/^Профиль \d+: II$/).map((c) => c.textContent)
  expect(chips()).toEqual(['Профиль 21: II', ...[1, 2, 3, 4, 5].map((n) => `Профиль ${n}: II`)])

  await userEvent.click(within(block).getByRole('button', { name: 'Все профили (21)' }))
  expect(chips()).toHaveLength(21)
  await userEvent.click(within(block).getByRole('button', { name: 'Свернуть' }))
  expect(chips()).toHaveLength(6)
})

it('уровни по профилям: короткий список — без кнопки', () => {
  renderSheet({ kind: 'perechen' })
  const block = screen.getByRole('heading', { name: /Уровень по профилям/ }).closest('section')!
  expect(within(block).queryByRole('button', { name: /Все профили/ })).not.toBeInTheDocument()
})
