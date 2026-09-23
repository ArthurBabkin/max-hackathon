/**
 * Тема оформления: как в системе, тёмная или светлая (F61).
 *
 * Выбор личный и живёт на устройстве, в localStorage, а не на сервере:
 * у ученика и родителя разные телефоны и разные привычки. Хранилище бывает
 * недоступно (приватный режим, часть встроенных браузеров) — тогда выбор
 * работает до закрытия приложения.
 */

import { useEffect, useSyncExternalStore, type ReactNode } from 'react'
import { MaxUI, useColorScheme, type MaxUIProps } from '@maxhub/max-ui'

export type ThemeChoice = 'system' | 'dark' | 'light'

/** Порядок вариантов в настройке. */
export const THEME_CHOICES: readonly ThemeChoice[] = ['system', 'dark', 'light']

const KEY = 'traektoria.theme'

function readSaved(): ThemeChoice {
  try {
    const value = localStorage.getItem(KEY)
    return THEME_CHOICES.includes(value as ThemeChoice) ? (value as ThemeChoice) : 'system'
  } catch {
    return 'system'
  }
}

let current: ThemeChoice = readSaved()
const listeners = new Set<() => void>()

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function setThemeChoice(next: ThemeChoice): void {
  current = next
  try {
    localStorage.setItem(KEY, next)
  } catch {
    // Хранилища нет — выбор проживёт до закрытия приложения.
  }
  listeners.forEach((listener) => listener())
}

/** Сохранённый выбор — им, например, заполняется форма профиля. */
export function readThemeChoice(): ThemeChoice {
  return current
}

function useThemeChoice(): ThemeChoice {
  return useSyncExternalStore(subscribe, readThemeChoice)
}

/**
 * Ставит сохранённую тему на <html> до первой отрисовки. Иначе при светлой
 * теме в тёмной системе экран на мгновение мигнул бы тёмным.
 */
export function applySavedTheme(): void {
  if (current !== 'system') document.documentElement.dataset.theme = current
}

/** Наши токены (tokens.css) следуют за data-theme, компоненты MAX UI — за провайдером. */
function ThemeSync() {
  const scheme = useColorScheme()
  useEffect(() => {
    document.documentElement.dataset.theme = scheme
  }, [scheme])
  return null
}

/**
 * Провайдер MAX UI с выбранной темой. Без `colorScheme` провайдер сам следит
 * за системной темой, с ним — держит заданную.
 */
export function ThemedMaxUI({ children, ...props }: Omit<MaxUIProps, 'colorScheme'> & { children: ReactNode }) {
  const choice = useThemeChoice()
  return (
    <MaxUI {...props} colorScheme={choice === 'system' ? undefined : choice}>
      <ThemeSync />
      {children}
    </MaxUI>
  )
}
