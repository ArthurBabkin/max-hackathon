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

// ТЗ F49: в профиле правятся все поля, включая регион, цель и город.
it('меняет регион, направления и город и отправляет их в PATCH /profile', async () => {
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
          { id: 'dir-se', name: 'Программная инженерия' },
          { id: 'dir-math', name: 'Математика' },
        ],
      })
    },
  })

  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Регион' }), 'Москва')
  const goals = within(screen.getByRole('group', { name: 'Направления' }))
  await userEvent.click(goals.getByRole('button', { name: /Математика/ }))
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Где хочу учиться' }), 'Санкт-Петербург')
  await userEvent.click(screen.getByRole('button', { name: /Сохранить/ }))

  expect(sent).toEqual([
    expect.objectContaining({ region_code: '77', direction_ids: ['dir-se', 'dir-math'], target_region_code: '78' }),
  ])
})

// «Пока не решил» и «не важно»: направления и вузы можно снять все (F8, F9).
it('отправляет пустые направления и вузы и город «не важно»', async () => {
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
      c.setQueryData(keys.directions, { items: [{ id: 'dir-se', name: 'Программная инженерия' }] })
    },
  })

  await userEvent.click(screen.getByRole('button', { name: /Программная инженерия/ }))
  for (const u of data.universities) {
    await userEvent.click(screen.getByRole('button', { name: new RegExp(u.short_name) }))
  }
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Где хочу учиться' }), 'Не важно')
  await userEvent.click(screen.getByRole('button', { name: /Сохранить/ }))

  expect(sent).toEqual([expect.objectContaining({ direction_ids: [], university_ids: [], target_region_code: '' })])
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
