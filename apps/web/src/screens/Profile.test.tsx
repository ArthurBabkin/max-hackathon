import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useLocation } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { QueryClient } from '@tanstack/react-query'
import { keys } from '@/api/queries'
import { profile, universityListItem } from '@/api/mocks/build'
import { DIRECTIONS, UNIVERSITIES } from '@/api/mocks/fixtures'
import { state } from '@/api/mocks/state'
import { renderApp } from '@/test/render'
import { htmlTheme, stubSystemTheme } from '@/test/theme'
import { ThemedMaxUI, setThemeChoice } from '@/ui/theme'

// Сохранение уходит в «живой» API: моки ответили бы сами и тело не поймать.
vi.hoisted(() => vi.stubEnv('VITE_USE_MOCKS', 'off'))
const { ProfileScreen, rebase } = await import('./Profile')

// ТЗ F49: в профиле правятся все поля, включая регион, цель, места и опыт.
it('меняет регион, направления, места и опыт и отправляет их в PATCH /profile', async () => {
  const sent: unknown[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((_url: string, init: RequestInit) => {
      if (init.method === 'PATCH') sent.push(JSON.parse(init.body as string))
      return new Promise(() => {})
    }),
  )
  renderApp(<ProfileScreen />, {
    route: '/profile',
    seed: (c) => {
      c.setQueryData(keys.profile, profile())
      c.setQueryData(keys.universities('', 'all'), { items: [] })
      c.setQueryData(keys.directions, {
        items: [
          { id: 'dir-se', name: 'Программная инженерия', code: '09.03.04', groups: ['ИТ'], popular: true },
          { id: 'dir-math', name: 'Математика', code: '01.03.01', groups: [], popular: true },
        ],
      })
    },
  })

  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Регион' }), 'Москва')
  const goals = within(screen.getByRole('group', { name: 'Направления' }))
  await userEvent.click(goals.getByRole('button', { name: /Математика/ }))
  const addPlace = screen.getByRole('combobox', { name: '+ Добавить регион' })
  await userEvent.selectOptions(addPlace, 'Санкт-Петербург')
  // Повтор не дублирует место, список сбрасывается к подсказке.
  await userEvent.selectOptions(addPlace, 'Санкт-Петербург')
  expect(addPlace).toHaveValue('')
  const experience = within(screen.getByRole('group', { name: 'Опыт в олимпиадах' }))
  expect(experience.getByRole('button', { name: /Школьный/ })).toHaveAttribute('aria-pressed', 'true')
  await userEvent.click(experience.getByRole('button', { name: /Региональный/ }))
  await userEvent.click(screen.getByRole('button', { name: /Сохранить/ }))

  expect(sent).toEqual([
    expect.objectContaining({
      region_code: '77',
      direction_ids: ['dir-se', 'dir-math'],
      places: [
        { region_code: '16', city: null },
        { region_code: '78', city: null },
      ],
      experience: 'region',
    }),
  ])
  expect(sent[0]).not.toHaveProperty('target_region_code')
})

// «Пока не решил» и «не важно»: направления, вузы и места можно снять все (F8, F9).
it('отправляет пустые направления, вузы и места «не важно»', async () => {
  const sent: unknown[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((_url: string, init: RequestInit) => {
      if (init.method === 'PATCH') sent.push(JSON.parse(init.body as string))
      return new Promise(() => {})
    }),
  )
  const data = profile()
  renderApp(<ProfileScreen />, {
    route: '/profile',
    seed: (c) => {
      c.setQueryData(keys.profile, data)
      c.setQueryData(keys.universities('', 'all'), { items: data.universities })
      c.setQueryData(keys.directions, {
        items: [{ id: 'dir-se', name: 'Программная инженерия', code: '09.03.04', groups: ['ИТ'], popular: true }],
      })
    },
  })

  await userEvent.click(
    within(screen.getByRole('group', { name: 'Направления' })).getByRole('button', { name: /Программная инженерия/ }),
  )
  const unis = within(screen.getByRole('group', { name: 'Вузы' }))
  for (const u of data.universities) {
    await userEvent.click(unis.getByRole('button', { name: new RegExp(u.short_name) }))
  }
  const places = within(screen.getByRole('group', { name: 'Где хочу учиться' }))
  await userEvent.click(places.getByRole('button', { name: 'Убрать: Республика Татарстан' }))
  expect(places.getByRole('button', { name: /Не важно/ })).toHaveAttribute('aria-pressed', 'true')
  await userEvent.click(screen.getByRole('button', { name: /Сохранить/ }))

  expect(sent).toEqual([expect.objectContaining({ direction_ids: [], university_ids: [], places: [] })])
})

// «Не важно» в боте: регион не указан — так и видно в профиле и так и сохраняется.
it('показывает и сохраняет регион «Не указан»', async () => {
  const sent: unknown[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((_url: string, init: RequestInit) => {
      if (init.method === 'PATCH') sent.push(JSON.parse(init.body as string))
      return new Promise(() => {})
    }),
  )
  renderApp(<ProfileScreen />, {
    route: '/profile',
    seed: (c) => {
      c.setQueryData(keys.profile, { ...profile(), region_code: '', region_name: '' })
      c.setQueryData(keys.universities('', 'all'), { items: [] })
      c.setQueryData(keys.directions, { items: [] })
    },
  })

  const region = screen.getByRole('combobox', { name: 'Регион' })
  expect(within(region).getByRole('option', { name: 'Не указан' })).toHaveProperty('selected', true)
  await userEvent.click(screen.getByRole('button', { name: /Сохранить/ }))
  expect(sent).toEqual([expect.objectContaining({ region_code: '' })])
})

/** Профиль в провайдере темы: пустые справочники, сохранение не отвечает. */
function renderProfile() {
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
  renderApp(
    <ThemedMaxUI platform="ios">
      <ProfileScreen />
    </ThemedMaxUI>,
    {
      route: '/profile',
      seed: (c) => {
        c.setQueryData(keys.profile, profile())
        c.setQueryData(keys.universities('', 'all'), { items: [] })
        c.setQueryData(keys.directions, { items: [] })
      },
    },
  )
}

const themeOption = (name: string) =>
  within(screen.getByRole('radiogroup', { name: 'Тема оформления' })).getByRole('radio', { name })
const saveButton = () => screen.getByRole('button', { name: /Сохранить/ })

describe('тема оформления (F61)', () => {
  afterEach(() => {
    act(() => setThemeChoice('system'))
    localStorage.clear()
    delete document.documentElement.dataset.theme
    vi.unstubAllGlobals()
  })

  it('отмечена та, что сохранена на устройстве', () => {
    stubSystemTheme(false)
    act(() => setThemeChoice('light'))
    renderProfile()

    const group = screen.getByRole('radiogroup', { name: 'Тема оформления' })
    expect(within(group).getAllByRole('radio').map((r) => r.textContent)).toEqual(['Как в системе', 'Тёмная', 'Светлая'])
    expect(themeOption('Светлая')).toBeChecked()
  })

  // Тема — поле формы: пользователь ждёт её рядом с остальными, а не под кнопкой.
  it('стоит последним полем перед «Сохранить»', () => {
    stubSystemTheme(false)
    renderProfile()

    const group = screen.getByRole('radiogroup', { name: 'Тема оформления' })
    const universities = screen.getByText('Вузы')
    expect(universities.compareDocumentPosition(group) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(group.compareDocumentPosition(saveButton()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('применяется и запоминается только кнопкой «Сохранить»', async () => {
    stubSystemTheme(false)
    renderProfile()
    expect(themeOption('Как в системе')).toBeChecked()

    await userEvent.click(themeOption('Тёмная'))
    expect(themeOption('Тёмная')).toBeChecked()
    expect(themeOption('Как в системе')).not.toBeChecked()
    expect(htmlTheme()).toBe('light')
    expect(localStorage.getItem('traektoria.theme')).toBeNull()

    await userEvent.click(saveButton())
    expect(htmlTheme()).toBe('dark')
    expect(localStorage.getItem('traektoria.theme')).toBe('dark')
  })
})

// --- Направления: основные чипами, остальные — выбором с поиском (F65) -------

describe('направления цели и в вузах (F65)', () => {
  const sent: unknown[] = []
  beforeEach(() => {
    sent.length = 0
    state.directions = [{ id: 'dir-se', name: 'Программная инженерия' }]
    state.chosen = { hse: ['dir-se', 'dir-ami'] }
    vi.stubGlobal(
      'fetch',
      vi.fn((_url: string, init: RequestInit) => {
        if (init.method === 'PATCH') sent.push(JSON.parse(init.body as string))
        return new Promise(() => {})
      }),
    )
  })
  afterEach(() => vi.unstubAllGlobals())

  function renderWithDirections(universities: typeof UNIVERSITIES = []) {
    let client!: QueryClient
    renderApp(
      <>
        <ProfileScreen />
        <Search />
      </>,
      {
        route: '/profile',
        seed: (c) => {
          client = c
          c.setQueryData(keys.profile, profile())
          c.setQueryData(keys.universities('', 'all'), { items: universities.map(universityListItem) })
          c.setQueryData(keys.directions, { items: DIRECTIONS })
        },
      },
    )
    return () => client
  }

  /** Адрес: открытый лист живёт в параметрах поиска. */
  function Search() {
    return <output data-testid="search">{useLocation().search}</output>
  }

  it('чипами — только основные направления, остальные — в выборе с поиском', async () => {
    renderWithDirections()
    const goals = within(screen.getByRole('group', { name: 'Направления' }))
    const chips = goals.getAllByRole('button').map((b) => b.textContent)
    expect(chips).toContain('✓ Программная инженерия')
    expect(chips).not.toContain('Прикладная информатика')
    expect(chips.at(-1)).toBe('Ещё направления…')

    await userEvent.click(goals.getByRole('button', { name: 'Ещё направления…' }))
    const picker = within(screen.getByRole('dialog', { name: 'Направления' }))
    // Цель — сверху, дальше по группам, у каждого — код.
    expect(picker.getByRole('heading', { name: 'Твоя цель' })).toBeInTheDocument()
    expect(picker.getByRole('checkbox', { name: /Программная инженерия/ })).toHaveAttribute('aria-checked', 'true')
    expect(picker.getByRole('heading', { name: 'ИТ' })).toBeInTheDocument()

    await userEvent.type(picker.getByRole('textbox', { name: 'Название или код' }), '09.03.03')
    expect(picker.getAllByRole('checkbox')).toHaveLength(1)
    await userEvent.click(picker.getByRole('checkbox', { name: /Прикладная информатика 09.03.03/ }))
    await userEvent.click(picker.getByRole('button', { name: 'Готово' }))

    expect(screen.queryByRole('dialog', { name: 'Направления' })).not.toBeInTheDocument()
    // Выбранное не из основных — тоже чипом, иначе сохранение молча его выбросит.
    expect(goals.getByRole('button', { name: '✓ Прикладная информатика' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /Сохранить/ }))
    expect(sent).toEqual([expect.objectContaining({ direction_ids: ['dir-se', 'dir-pi'] })])
  })

  it('поиск без результатов — так и сказано', async () => {
    renderWithDirections()
    await userEvent.click(screen.getByRole('button', { name: 'Ещё направления…' }))
    const picker = within(screen.getByRole('dialog', { name: 'Направления' }))
    await userEvent.type(picker.getByRole('textbox', { name: 'Название или код' }), 'астрофизика')
    expect(picker.queryAllByRole('checkbox')).toHaveLength(0)
    expect(picker.getByText(/Ничего не нашлось/)).toBeInTheDocument()
  })

  it('у каждого моего вуза — на какие направления смотрим льготы', () => {
    renderWithDirections()
    const list = within(screen.getByRole('list', { name: 'Направления в вузах' }))
    expect(list.getByRole('button', { name: /ВШЭ/ })).toHaveTextContent(
      'Программная инженерия, Прикладная математика и информатика',
    )
    expect(list.getByRole('button', { name: /^Иннополис/ })).toHaveTextContent(
      'по цели: Информатика и вычислительная техника',
    )
  })

  // Отметил вуз чипом — строка сразу, до «Сохранить»: направления выбирают
  // в карточке вуза, и выбор там сам добавляет вуз в мои.
  it('только что отмеченный вуз — сразу строкой, она открывает карточку вуза', async () => {
    renderWithDirections(UNIVERSITIES)
    const chips = within(screen.getByRole('group', { name: 'Вузы' }))
    await userEvent.click(chips.getByRole('button', { name: 'МФТИ' }))

    const list = within(screen.getByRole('list', { name: 'Направления в вузах' }))
    const row = list.getByRole('button', { name: /^МФТИ/ })
    expect(row).toHaveTextContent('выбери направления')
    await userEvent.click(row)
    expect(screen.getByTestId('search')).toHaveTextContent('mipt')

    // Снятый чип убирает строку сразу, и сохранённого вуза тоже.
    await userEvent.click(chips.getByRole('button', { name: '✓ МФТИ' }))
    await userEvent.click(chips.getByRole('button', { name: '✓ ВШЭ' }))
    expect(list.queryByRole('button', { name: /^МФТИ/ })).not.toBeInTheDocument()
    expect(list.queryByRole('button', { name: /^ВШЭ/ })).not.toBeInTheDocument()
  })

  // Направление выбрали в карточке вуза поверх профиля: профиль обновился,
  // а несохранённые правки формы остались.
  it('обновление профиля из карточки вуза не стирает несохранённое', async () => {
    const client = renderWithDirections()
    const name = screen.getByRole('textbox', { name: 'Имя ученика' })
    await userEvent.clear(name)
    await userEvent.type(name, 'Тёма')

    state.chosen = { hse: ['dir-se', 'dir-ami', 'dir-is'] }
    state.directions = [...state.directions, { id: 'dir-is', name: 'Информационная безопасность' }]
    act(() => {
      client().setQueryData(keys.profile, profile())
    })

    const goals = within(screen.getByRole('group', { name: 'Направления' }))
    await waitFor(() =>
      expect(goals.getByRole('button', { name: '✓ Информационная безопасность' })).toBeInTheDocument(),
    )
    expect(name).toHaveValue('Тёма')
  })
})

// Новая версия профиля поверх несохранённой формы: элементы списков —
// значения, а не ссылки. Места приходят новыми объектами при каждом ответе.
describe('rebase', () => {
  const kazan = { region_code: '16', city: 'Казань' }
  const moscow = { region_code: '77', city: null }

  it('убранное в форме место не возвращается, если на сервере его не трогали', () => {
    const base = [kazan, moscow]
    const server = [{ ...kazan }, { ...moscow }]
    expect(rebase([kazan], base, server)).toEqual([kazan])
  })

  it('добавленное на сервере добавляется, убранное на сервере — убирается', () => {
    const spb = { region_code: '78', city: null }
    expect(rebase([kazan, moscow], [kazan, moscow], [{ ...kazan }, { ...moscow }, spb])).toEqual([kazan, moscow, spb])
    expect(rebase([kazan, moscow, spb], [kazan, moscow], [{ ...kazan }])).toEqual([kazan, spb])
  })
})
