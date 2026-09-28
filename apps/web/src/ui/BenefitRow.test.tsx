import { screen } from '@testing-library/react'
import type { BenefitRow as BenefitRowData } from '@contract'
import { expect, it, vi } from 'vitest'
import { renderApp } from '@/test/render'
import { BenefitRow, UniversityOlympiadRow } from './BenefitRow'

const itmo: BenefitRowData = {
  university_id: 'itmo',
  university_name: 'Университет ИТМО',
  university_short_name: 'ИТМО',
  university_nick: 'ИТМО',
  city: 'Санкт-Петербург',
  benefit: 'bvi',
  benefit_label: 'БВИ',
  winner: null,
  prizer: null,
  ege_min: 75,
  ege_max: null,
  source: null,
  directions: [],
  other_directions: [],
  unverified: false,
  varies: false,
  directions_count: 18,
  directions_total: 24,
}

// «Где ещё даёт льготу» (F23, F65): на скольких направлениях вуза.
it('показывает, на скольких направлениях вуза льгота', () => {
  renderApp(<BenefitRow data={itmo} />)
  expect(screen.getByText('Санкт-Петербург · на 18 из 24 направлений')).toBeInTheDocument()
})

it('данных по направлениям нет — только город', () => {
  renderApp(<BenefitRow data={{ ...itmo, directions_count: 0, directions_total: 0 }} />)
  expect(screen.getByText('Санкт-Петербург')).toBeInTheDocument()
})

// Предмет для 100 баллов и просьба уточнить сомнительную льготу — под вузом.
it('показывает условия вуза под названием', () => {
  const conditions = ['Уточните в приёмной комиссии — так бывает', '100 баллов засчитают по предмету «Информатика»']
  renderApp(<BenefitRow data={{ ...itmo, benefit: 'score100', benefit_label: '100 баллов', conditions }} />)
  for (const c of conditions) expect(screen.getByText(c)).toBeInTheDocument()
})

// Предмет 100 баллов — подписью под плашкой: целиком в плашке он сжимал
// название вуза до слова в строке.
it('в плашке — вид льготы, предмет — под ней', () => {
  renderApp(<BenefitRow data={{ ...itmo, benefit: 'score100', benefit_label: '100 баллов по физике или химии' }} />)
  expect(screen.getByText('100 баллов')).toHaveClass('benefit-value')
  expect(screen.getByText('по физике или химии')).toHaveClass('benefit-detail')
})

it('в строке олимпиады карточки вуза — так же', () => {
  renderApp(
    <UniversityOlympiadRow
      id="p669-36-fizika"
      olympiadId="p669-36"
      name="Звезда"
      subtitle="Физика, Химия"
      label="100 баллов по физике или химии"
      benefit="score100"
      onOpen={vi.fn()}
    />,
  )
  expect(screen.getByText('100 баллов')).toHaveClass('benefit-value')
  expect(screen.getByText('по физике или химии')).toHaveClass('benefit-detail')
})
