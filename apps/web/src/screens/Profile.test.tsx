import { act, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { keys } from '@/api/queries'
import { profile } from '@/api/mocks/build'
import { renderApp } from '@/test/render'
import { htmlTheme, stubSystemTheme } from '@/test/theme'
import { ThemedMaxUI, setThemeChoice } from '@/ui/theme'

// Сохранение уходит в «живой» API: моки ответили бы сами и тело не поймать.
vi.hoisted(() => vi.stubEnv('VITE_USE_MOCKS', 'off'))
const { ProfileScreen } = await import('./Profile')

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
      c.setQueryData(keys.universities('', 'all', 'all'), { items: [] })
      c.setQueryData(keys.directions, {
        items: [
          { id: 'dir-se', name: 'Программная инженерия' },
          { id: 'dir-math', name: 'Математика' },
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
      c.setQueryData(keys.universities('', 'all', 'all'), { items: data.universities })
      c.setQueryData(keys.directions, { items: [{ id: 'dir-se', name: 'Программная инженерия' }] })
    },
  })

  const goals = within(screen.getByRole('group', { name: 'Направления' }))
  await userEvent.click(goals.getByRole('button', { name: /Программная инженерия/ }))
  const universities = within(screen.getByRole('group', { name: 'Вузы' }))
  for (const u of data.universities) {
    await userEvent.click(universities.getByRole('button', { name: new RegExp(u.short_name) }))
  }
  const places = within(screen.getByRole('group', { name: 'Где хочу учиться' }))
  await userEvent.click(places.getByRole('button', { name: 'Убрать: Республика Татарстан' }))
  expect(places.getByRole('button', { name: /Не важно/ })).toHaveAttribute('aria-pressed', 'true')
  await userEvent.click(screen.getByRole('button', { name: /Сохранить/ }))

  expect(sent).toEqual([expect.objectContaining({ direction_ids: [], university_ids: [], places: [] })])
})

// Сохранённые в каталоге направления в профиле можно только убрать: после PATCH уходит PUT.
it('убирает сохранённое направление через PUT /profile/programs', async () => {
  const sent: { method: string; url: string; body: unknown }[] = []
  const data = profile()
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init: RequestInit) => {
      if (init.method === 'PATCH' || init.method === 'PUT') {
        sent.push({ method: init.method, url, body: JSON.parse(init.body as string) })
      }
      return Promise.resolve(new Response(JSON.stringify(data), { status: 200 }))
    }),
  )
  renderApp(<ProfileScreen />, {
    route: '/profile',
    seed: (c) => {
      c.setQueryData(keys.profile, data)
      c.setQueryData(keys.universities('', 'all', 'all'), { items: data.universities })
      c.setQueryData(keys.directions, { items: [] })
    },
  })

  const programs = within(screen.getByRole('group', { name: 'Сохранённые направления' }))
  const chip = programs.getByRole('button', { name: /УИ: Программная инженерия/ })
  expect(chip).toHaveAttribute('aria-pressed', 'true')
  await userEvent.click(chip)
  await userEvent.click(screen.getByRole('button', { name: /Сохранить/ }))

  await vi.waitFor(() => expect(sent.map((r) => r.method)).toEqual(['PATCH', 'PUT']))
  expect(sent[1]).toEqual({
    method: 'PUT',
    url: expect.stringMatching(/\/profile\/programs$/),
    body: { program_ids: [] },
  })
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
      c.setQueryData(keys.universities('', 'all', 'all'), { items: [] })
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
        c.setQueryData(keys.universities('', 'all', 'all'), { items: [] })
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
