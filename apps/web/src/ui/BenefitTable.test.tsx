import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { BenefitColumn, BenefitGrant, BenefitRow } from '@contract'
import { expect, it, vi } from 'vitest'
import { spokenText } from '@/test/a11y'
import { makeSession, renderApp } from '@/test/render'
import { BenefitTable } from './BenefitTable'

const BVI: BenefitGrant = { kind: 'bvi', label: 'БВИ' }
const SCORE100: BenefitGrant = { kind: 'score100', label: '100 баллов' }

function row(id: string, nick: string, over: Partial<BenefitRow> = {}): BenefitRow {
  return {
    university_id: id,
    university_name: nick,
    university_short_name: nick,
    university_nick: nick,
    city: null,
    benefit: 'bvi',
    benefit_label: 'БВИ',
    winner: BVI,
    prizer: BVI,
    extra_points: null,
    ege_min: 75,
    ege_max: null,
    source: null,
    directions: [],
    other_directions: [],
    unverified: false,
    varies: false,
    directions_count: 0,
    directions_total: 0,
    ...over,
  }
}

const notCounted = (id: string, nick: string) =>
  row(id, nick, { benefit: null, benefit_label: null, winner: null, prizer: null, ege_min: null })

// «Высшая проба» в вузах ученика: у ВШЭ порог зависит от программы, МГУ
// призёру даёт 100 баллов и засчитывает только диплом 11 класса, СПбГУ
// олимпиаду не учитывает.
const ROWS = [
  row('hse', 'ВШЭ', { ege_max: 85 }),
  row('kfu', 'КФУ'),
  row('itmo', 'ИТМО'),
  row('msu', 'МГУ', {
    prizer: SCORE100,
    conditions: ['Засчитывает только диплом 11 класса — диплом за 10 класс не подойдёт'],
  }),
  notCounted('spbu', 'СПбГУ'),
]

function setup(rows: BenefitRow[] = ROWS, columns: BenefitColumn[] = ['winner', 'prizer', 'ege']) {
  const onOpen = vi.fn()
  renderApp(<BenefitTable rows={rows} columns={columns} onOpen={onOpen} />)
  return { onOpen }
}

const headers = () => screen.getAllByRole('columnheader').map((h) => h.textContent)
const universities = () => screen.getAllByRole('rowheader').map(spokenText)
const rowOf = (nick: string) => screen.getByRole('rowheader', { name: nick }).closest('tr')!
const marked = (nick: string) =>
  within(rowOf(nick))
    .queryAllByText('не как в большинстве')
    .map((mark) => mark.closest('td')!.firstChild?.textContent)

it('вузы — строками, столбцы — те, что прислал сервер', () => {
  setup()
  expect(headers()).toEqual(['Вуз', 'Победителю', 'Призёру', 'Порог ЕГЭ'])
  expect(universities()).toEqual(['ВШЭ', 'КФУ', 'ИТМО', 'МГУ'])
  expect(within(rowOf('ВШЭ')).getByText('от 75–85')).toBeInTheDocument()
  expect(within(rowOf('КФУ')).getByText('от 75')).toBeInTheDocument()
})

it('вуз, который олимпиаду не учитывает, — строкой под таблицей', () => {
  setup()
  expect(screen.queryByRole('rowheader', { name: 'СПбГУ' })).not.toBeInTheDocument()
  expect(screen.getByText('Не учитывает эту олимпиаду: СПбГУ')).toBeInTheDocument()
})

it('отмечает только то, что не как в большинстве вузов, и объясняет отметку', () => {
  setup()
  expect(marked('МГУ')).toEqual(['100 баллов'])
  expect(marked('ВШЭ')).toEqual(['от 75–85'])
  expect(marked('КФУ')).toEqual([])
  expect(screen.getByText(/не как в большинстве твоих вузов/)).toBeInTheDocument()
})

// Диапазон — это нижний порог, разный по программам: «от 75–85», а не
// «75–85», иначе читается как «больше 85 нельзя». Пояснение — в той же ячейке.
it('порог-диапазон объясняет себя прямо в ячейке', () => {
  setup()
  expect(within(rowOf('ВШЭ')).getByText('зависит от программы')).toBeInTheDocument()
  expect(within(rowOf('КФУ')).queryByText('зависит от программы')).not.toBeInTheDocument()
  expect(screen.queryByText(/Диапазон/)).not.toBeInTheDocument()
})

it('большинства нет — ничего не отмечено и пояснять нечего', () => {
  setup([row('kfu', 'КФУ'), row('msu', 'МГУ', { prizer: null })], ['winner', 'prizer'])
  expect(marked('МГУ')).toEqual([])
  expect(within(rowOf('МГУ')).getByText('нет')).toBeInTheDocument()
  expect(screen.queryByText(/не как в большинстве твоих вузов/)).not.toBeInTheDocument()
})

it('своё у вуза — плашкой сразу под его строкой', () => {
  setup()
  const note = screen.getByText('Засчитывает только диплом 11 класса — диплом за 10 класс не подойдёт')
  expect(rowOf('МГУ').nextElementSibling).toContainElement(note)
})

it('вуз открывает свою карточку', async () => {
  const { onOpen } = setup()
  await userEvent.click(screen.getByRole('button', { name: 'МГУ' }))
  expect(onOpen).toHaveBeenCalledWith('msu')
})

it('вне перечня — один столбец доп. баллов', () => {
  const extra: BenefitGrant = { kind: 'extra_points', label: '+3 балла' }
  setup(
    [row('kfu', 'КФУ', { winner: extra, prizer: extra }), notCounted('hse', 'ВШЭ'), notCounted('itmo', 'ИТМО')],
    ['extra_points'],
  )
  expect(headers()).toEqual(['Вуз', 'Доп. баллы за диплом'])
  expect(within(rowOf('КФУ')).getByText('+3 балла')).toBeInTheDocument()
  expect(screen.getByText('Не учитывают эту олимпиаду: ВШЭ, ИТМО')).toBeInTheDocument()
})

it('ни один вуз олимпиаду не учитывает — таблицы нет', () => {
  setup([notCounted('spbu', 'СПбГУ')])
  expect(screen.queryByRole('table')).not.toBeInTheDocument()
  expect(screen.getByText('Не учитывает эту олимпиаду: СПбГУ')).toBeInTheDocument()
})

// Вуз засчитывает олимпиаду, но не на мои направления (ВсОШ по биологии в
// КФУ) — это не «не учитывает»: сервер присылает, на скольких направлениях
// вуза льгота есть.
const elsewhere = (id: string, nick: string) => ({
  ...notCounted(id, nick),
  directions: ['Программная инженерия'],
  directions_count: 8,
  directions_total: 40,
})

it('льгота только на другие направления — отдельно от «не учитывает»', () => {
  setup([row('hse', 'ВШЭ'), elsewhere('kfu', 'КФУ'), elsewhere('mipt', 'МФТИ'), notCounted('innopolis', 'Иннополис')])
  expect(screen.getByText('Льгота есть, но не на твои направления: КФУ, МФТИ')).toBeInTheDocument()
  expect(screen.getByText('Не учитывает эту олимпиаду: Иннополис')).toBeInTheDocument()
})

it('родителю — «не на направления» ребёнка по имени', () => {
  renderApp(<BenefitTable rows={[elsewhere('kfu', 'КФУ')]} columns={['winner', 'prizer', 'ege']} onOpen={() => {}} />, {
    session: makeSession({ role: 'parent' }),
  })
  expect(screen.getByText('Льгота есть, но не на направления Артёма: КФУ')).toBeInTheDocument()
})

it('родителю — голос родителя', () => {
  renderApp(<BenefitTable rows={ROWS} columns={['winner', 'prizer', 'ege']} onOpen={() => {}} />, {
    session: makeSession({ role: 'parent' }),
  })
  expect(screen.getByText(/не как в большинстве вузов Артёма/)).toBeInTheDocument()
})

// F65: льгота в моём вузе — на мои направления. Под вузом — на какие; если
// на других моих направлениях льгота слабее — подстрокой со своими столбцами.
it('под вузом — мои направления, другая льгота на других — подстрокой', () => {
  setup(
    [
      row('hse', 'ВШЭ', {
        directions: ['Прикладная математика и информатика'],
        other_directions: [
          {
            benefit: 'score100',
            benefit_label: '100 баллов',
            directions: ['Программная инженерия'],
            winner: SCORE100,
            prizer: SCORE100,
          },
        ],
      }),
      row('kfu', 'КФУ', { directions: ['Программная инженерия'] }),
    ],
    ['winner', 'prizer'],
  )
  expect(screen.getByRole('rowheader', { name: 'ВШЭ Прикладная математика и информатика' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'ВШЭ' })).toBeInTheDocument()
  const sub = screen.getByRole('rowheader', { name: 'Программная инженерия' }).closest('tr')!
  expect(within(sub).getAllByText('100 баллов')).toHaveLength(2)
  expect(screen.getByText('Льгота — на твои направления: отмеченные в карточке вуза или из цели.')).toBeInTheDocument()
})

// Что получит призёр на другом направлении — из ответа сервера, а не по
// виду льготы: «БВИ победителям» бывает и со 100 баллами призёру (ИТМО, ПМИ).
it('на другом направлении призёр — как в правилах вуза', () => {
  setup(
    [
      row('itmo', 'ИТМО', {
        directions: ['Программная инженерия'],
        other_directions: [
          {
            benefit: 'bvi_winners',
            benefit_label: 'БВИ победителям',
            directions: ['Прикладная математика и информатика'],
            winner: BVI,
            prizer: SCORE100,
          },
        ],
      }),
    ],
    ['winner', 'prizer'],
  )
  const sub = screen.getByRole('rowheader', { name: 'Прикладная математика и информатика' }).closest('tr')!
  expect(within(sub).getByText('БВИ')).toBeInTheDocument()
  expect(within(sub).getByText('100 баллов')).toBeInTheDocument()
})

it('льгота вуза целиком — без направлений и без пояснения о них', () => {
  setup()
  expect(screen.queryByText(/на твои направления/)).not.toBeInTheDocument()
})
