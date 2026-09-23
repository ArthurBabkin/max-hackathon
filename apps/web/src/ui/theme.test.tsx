import { act, render } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { htmlTheme as html, stubSystemTheme as systemTheme } from '@/test/theme'

/** Модуль темы читает выбор при загрузке — как при новом запуске приложения. */
async function launch() {
  vi.resetModules()
  const theme = await import('./theme')
  render(<theme.ThemedMaxUI platform="ios">{null}</theme.ThemedMaxUI>)
  return theme
}

beforeEach(() => {
  localStorage.clear()
  delete document.documentElement.dataset.theme
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

it('по умолчанию тема как в системе', async () => {
  systemTheme(true)
  const theme = await launch()

  expect(html()).toBe('dark')
  expect(localStorage.getItem('traektoria.theme')).toBeNull()
  expect(theme.readThemeChoice()).toBe('system')
})

it('выбранная тема применяется и запоминается на устройстве', async () => {
  systemTheme(false)
  const theme = await launch()
  expect(html()).toBe('light')

  act(() => theme.setThemeChoice('dark'))
  expect(html()).toBe('dark')
  expect(localStorage.getItem('traektoria.theme')).toBe('dark')

  act(() => theme.setThemeChoice('light'))
  expect(html()).toBe('light')

  systemTheme(true)
  act(() => theme.setThemeChoice('system'))
  expect(html()).toBe('dark')
  expect(localStorage.getItem('traektoria.theme')).toBe('system')
})

it('при новом запуске открывается выбранная тема, а не системная', async () => {
  systemTheme(true)
  localStorage.setItem('traektoria.theme', 'light')
  const theme = await launch()

  expect(html()).toBe('light')
  expect(theme.readThemeChoice()).toBe('light')
})

// Тему надо поставить до первой отрисовки: иначе при светлой теме в
// тёмной системе экран на мгновение мигнул бы тёмным.
it('сохранённую тему можно поставить до первой отрисовки', async () => {
  localStorage.setItem('traektoria.theme', 'dark')
  vi.resetModules()
  const { applySavedTheme } = await import('./theme')

  applySavedTheme()
  expect(html()).toBe('dark')
})

it('мусор в хранилище — как в системе', async () => {
  systemTheme(false)
  localStorage.setItem('traektoria.theme', 'sepia')
  const theme = await launch()

  expect(html()).toBe('light')
  expect(theme.readThemeChoice()).toBe('system')
})

// В приватном режиме и в части встроенных браузеров localStorage бросает.
it('без доступа к хранилищу выбор работает до закрытия приложения', async () => {
  systemTheme(false)
  vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
    throw new Error('SecurityError')
  })
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
    throw new Error('SecurityError')
  })
  const theme = await launch()

  act(() => theme.setThemeChoice('dark'))
  expect(html()).toBe('dark')
})
