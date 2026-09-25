import { screen } from '@testing-library/react'
import type { BenefitRow as BenefitRowData } from '@contract'
import { expect, it } from 'vitest'
import { renderApp } from '@/test/render'
import { BenefitRow } from './BenefitRow'

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
