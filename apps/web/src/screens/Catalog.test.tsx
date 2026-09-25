import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { CatalogUniversity, OlympiadListItem } from '@contract'
import { useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it } from 'vitest'
import { keys } from '@/api/queries'
import { profile } from '@/api/mocks/build'
import { state } from '@/api/mocks/state'
import { makeSession, renderApp } from '@/test/render'
import { CatalogScreen } from './Catalog'

const olympiad = (id: string, level: 'I' | 'II' | null, kind: 'vsosh' | 'perechen' = 'perechen') =>
  ({
    olympiad_id: id,
    name: `Олимпиада ${id}`,
    organizer: 'Вуз',
    kind,
    final_city: null,
    short_name: null,
    color: null,
    primary_profile: { olympiad_profile_id: `${id}-inf`, subject_code: 'inf', subject_name: 'Информатика', level },
    profiles_count: 1,
    my_benefits: [],
  }) as unknown as OlympiadListItem

const II = ['b1', 'b2', 'b3', 'b4', 'b5', 'b6', 'b7'].map((id) => olympiad(id, 'II'))

function setup() {
  // Ученик демо-профиля выбрал информатику и математику: каталог сразу про его предмет (D1).
  return renderApp(<CatalogScreen />, {
    route: '/catalog',
    seed: (c) => {
      c.setQueryData(keys.profile, profile())
      c.setQueryData(keys.olympiads('', 'inf', false), {
        items: [olympiad('v', null, 'vsosh'), olympiad('a', 'I'), ...II],
      })
      c.setQueryData(keys.tracker, {
        items: [{ id: 't1', olympiad_id: 'a' }],
        proposals: [],
      })
    },
  })
}

it('по умолчанию — предмет ученика, олимпиады разложены по уровням', () => {
  setup()
  expect(screen.getByRole('button', { name: 'Информатика', pressed: true })).toBeInTheDocument()
  const headings = screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)
  expect(headings).toEqual(['ВсОШ1', 'I уровень1', 'II уровень7'])
})

it('длинная группа свёрнута до пяти, «Показать ещё» раскрывает остальное', async () => {
  setup()
  const group = screen.getByRole('heading', { name: /II уровень/ }).closest('section')!
  expect(within(group).getAllByRole('button', { name: /Олимпиада b/ })).toHaveLength(5)
  await userEvent.click(within(group).getByRole('button', { name: 'Показать ещё 2' }))
  expect(within(group).getAllByRole('button', { name: /Олимпиада b/ })).toHaveLength(7)
})

it('олимпиада из трекера помечена', () => {
  setup()
  expect(within(screen.getByRole('button', { name: /Олимпиада a/ })).getByText('в трекере')).toBeInTheDocument()
})

// Из «Подбора» без подходящих олимпиад — «Найти вузы» (C3): каталог сразу на вузах.
it('?segment=universities открывает вкладку «Вузы»', () => {
  renderApp(<CatalogScreen />, {
    route: '/catalog?segment=universities',
    seed: (c) => {
      c.setQueryData(keys.profile, profile())
      c.setQueryData(keys.universities('', 'all'), { items: [] })
    },
  })
  expect(screen.getByRole('tab', { name: 'Вузы', selected: true })).toBeInTheDocument()
})

// Строка каталога: вступить в этом сезоне уже нельзя — пометка серым, как «в трекере».
it('закрытую регистрацию помечает в строке', () => {
  renderApp(<CatalogScreen />, {
    route: '/catalog',
    seed: (c) => {
      c.setQueryData(keys.profile, profile())
      c.setQueryData(keys.olympiads('', 'inf', false), {
        items: [{ ...olympiad('z', 'I'), registration_closed: true }, olympiad('y', 'I')],
      })
      c.setQueryData(keys.tracker, { items: [], proposals: [] })
    },
  })
  expect(screen.getAllByText('регистрация закрыта')).toHaveLength(1)
})

// Город финала ничего не говорит о том, куда олимпиада ведёт, — фильтра нет.
it('у олимпиад нет фильтра «Финал»', () => {
  setup()
  expect(screen.queryByText('Финал')).toBeNull()
})

// «Ведут в мои вузы и на мои направления» (F66, R1): переключатель над
// предметами, в строке — льгота в моих вузах.
describe('ведут в мои вузы и на мои направления', () => {
  beforeEach(() => {
    state.universities = ['inno', 'kfu', 'hse']
    state.directions = [{ id: 'dir-se', name: 'Программная инженерия' }]
    state.chosen = {}
  })

  const leads = (id: string, name: string, my_benefits: OlympiadListItem['my_benefits']) =>
    ({ ...olympiad(id, 'I'), name, my_benefits }) as OlympiadListItem

  function renderMine(mine: Record<string, OlympiadListItem[]>, session = makeSession()) {
    renderApp(<CatalogScreen />, {
      route: '/catalog',
      session,
      seed: (c) => {
        c.setQueryData(keys.profile, profile())
        c.setQueryData(keys.tracker, { items: [], proposals: [] })
        c.setQueryData(keys.olympiads('', 'inf', false), { items: [olympiad('a', 'I'), olympiad('b', 'I')] })
        for (const [subject, items] of Object.entries(mine)) c.setQueryData(keys.olympiads('', subject, true), { items })
      },
    })
  }

  it('переключатель — мои вузы и направления; включённый оставляет ведущие туда, с льготой в строке', async () => {
    renderMine({
      inf: [
        leads('hse', 'Высшая проба', [
          { benefit: 'bvi', benefit_label: 'БВИ', universities: ['Иннополис'] },
          { benefit: 'score100', benefit_label: '100 баллов', universities: ['КФУ', 'ВШЭ'] },
        ]),
      ],
    })
    const toggle = screen.getByRole('switch', { name: /Ведут в мои вузы и на мои направления/ })
    expect(toggle).toHaveAttribute('aria-checked', 'false')
    // Цель ПИ: в Иннополисе её покрывает 09.00.00 — два разных направления.
    expect(toggle).toHaveTextContent('Иннополис, КФУ, ВШЭ · 2 направления')
    // Предметы — ниже переключателя.
    expect(toggle.compareDocumentPosition(screen.getByText('Предмет')) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(screen.getByRole('button', { name: /Олимпиада a/ })).toBeInTheDocument()

    await userEvent.click(toggle)

    expect(toggle).toHaveAttribute('aria-checked', 'true')
    expect(screen.queryByRole('button', { name: /Олимпиада a/ })).toBeNull()
    expect(screen.getByRole('button', { name: /Высшая проба/ })).toHaveTextContent('БВИ Иннополис · 100 баллов: КФУ, ВШЭ')
  })

  it('льгота во всех моих вузах — так и написано', async () => {
    state.universities = ['kfu', 'hse']
    renderMine({
      inf: [leads('hse', 'Высшая проба', [{ benefit: 'score100', benefit_label: '100 баллов', universities: ['КФУ', 'ВШЭ'] }])],
    })
    await userEvent.click(screen.getByRole('switch', { name: /Ведут/ }))
    expect(screen.getByRole('button', { name: /Высшая проба/ })).toHaveTextContent('100 баллов во всех твоих вузах')
  })

  it('родителю — о вузах и направлениях ребёнка', () => {
    renderMine({}, makeSession({ role: 'parent', is_creator: true }))
    expect(screen.getByRole('switch', { name: /Ведут в вузы и на направления Артёма/ })).toBeInTheDocument()
  })

  it('без вузов переключатель неактивен и ведёт в выбор вузов', async () => {
    state.universities = []
    renderMine({})
    expect(screen.getByRole('switch', { name: /Ведут/ })).toBeDisabled()

    await userEvent.click(screen.getByRole('button', { name: /Сначала выбери вузы/ }))

    expect(screen.getByRole('tab', { name: 'Вузы', selected: true })).toBeInTheDocument()
  })

  it('по предмету никуда не ведут — предлагает все предметы', async () => {
    renderMine({ chem: [], all: [leads('hse', 'Высшая проба', [{ benefit: 'bvi', benefit_label: 'БВИ', universities: ['ВШЭ'] }])] })
    await userEvent.click(screen.getByRole('switch', { name: /Ведут/ }))
    await userEvent.click(screen.getByRole('button', { name: 'Химия' }))

    expect(screen.getByText(/По этому предмету ни одна олимпиада не даёт БВИ или 100 баллов/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Все предметы' }))

    expect(screen.getByRole('button', { name: 'Все', pressed: true })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Высшая проба/ })).toBeInTheDocument()
  })
})

// Каталог вузов по направлению (F67): «Все», направления цели и «Другое…»;
// в строке — сколько олимпиад дают льготу на это направление.
describe('вузы по направлению', () => {
  beforeEach(() => {
    state.universities = ['inno', 'kfu', 'hse']
    state.directions = [{ id: 'dir-se', name: 'Программная инженерия' }]
  })

  const uni = (id: string, name: string, match?: CatalogUniversity['direction_match']) =>
    ({
      id,
      short_name: id.toUpperCase(),
      nick: id.toUpperCase(),
      name,
      city: 'Казань',
      color: null,
      benefit_olympiads_count: 30,
      is_mine: false,
      ...(match ? { direction_match: match } : {}),
    }) as CatalogUniversity

  function Probe() {
    return <output aria-label="адрес">{decodeURIComponent(useLocation().search)}</output>
  }

  function renderUnis() {
    renderApp(
      <>
        <CatalogScreen />
        <Probe />
      </>,
      {
        route: '/catalog?segment=universities',
        seed: (c) => {
          c.setQueryData(keys.profile, profile())
          c.setQueryData(keys.directions, {
            items: [
              { id: 'dir-se', name: 'Программная инженерия', code: '09.03.04', groups: ['ИТ'], popular: true },
              { id: 'dir-bio', name: 'Биология', code: '06.03.01', groups: ['Биомед'], popular: true },
            ],
          })
          c.setQueryData(keys.universities('', 'all'), { items: [uni('kfu', 'Казанский университет'), uni('mipt', 'МФТИ')] })
          c.setQueryData(keys.universities('', 'all', 'dir-se'), {
            items: [
              uni('kfu', 'Казанский университет', { direction_ids: ['dir-se'], olympiads_count: 24, status: 'offered' }),
              uni('nsu', 'Новосибирский университет', { direction_ids: ['dir-se'], olympiads_count: 0, status: 'to_check' }),
            ],
          })
          c.setQueryData(keys.universities('', 'all', 'dir-bio'), {
            items: [uni('kfu', 'Казанский университет', { direction_ids: ['dir-bio'], olympiads_count: 3, status: 'offered' })],
          })
        },
      },
    )
    return within(screen.getByRole('group', { name: 'Направление' }))
  }

  it('направления цели — чипами, в строке — олимпиады на это направление', async () => {
    const row = renderUnis()
    expect(row.getAllByRole('button').map((b) => b.textContent)).toEqual(['Все', 'Программная инженерия', 'Другое…'])
    expect(row.getByRole('button', { name: 'Все' })).toHaveAttribute('aria-pressed', 'true')

    await userEvent.click(row.getByRole('button', { name: 'Программная инженерия' }))

    expect(screen.queryByRole('button', { name: /МФТИ/ })).toBeNull()
    expect(screen.getByRole('button', { name: /Казанский университет/ })).toHaveTextContent(
      'Казань, 24 олимпиады с льготой на это направление',
    )
    expect(screen.getByRole('button', { name: /Новосибирский университет/ })).toHaveTextContent(
      'Казань, льготы на это направление уточняются',
    )
  })

  it('«Другое…» — выбор с поиском, выбранное встаёт чипом', async () => {
    const row = renderUnis()
    await userEvent.click(row.getByRole('button', { name: 'Другое…' }))
    const dialog = within(screen.getByRole('dialog', { name: 'Направление' }))

    await userEvent.click(dialog.getByRole('radio', { name: /Биология/ }))

    expect(screen.queryByRole('dialog')).toBeNull()
    expect(row.getByRole('button', { name: 'Биология' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /Казанский университет/ })).toHaveTextContent('3 олимпиады')
  })

  it('карточка вуза открывается на этом направлении', async () => {
    const row = renderUnis()
    await userEvent.click(row.getByRole('button', { name: 'Программная инженерия' }))
    await userEvent.click(screen.getByRole('button', { name: /Казанский университет/ }))

    expect(screen.getByRole('status', { name: 'адрес' })).toHaveTextContent('sheet=vuz:kfu:dir-se')
  })
})
