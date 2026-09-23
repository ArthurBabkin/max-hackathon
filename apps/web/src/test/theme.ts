/** Тема оформления в тестах: в jsdom нет `matchMedia`. */

import { vi } from 'vitest'

/** Системная тема телефона — то, что отдаёт `prefers-color-scheme`. */
export function stubSystemTheme(dark: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: dark && query.includes('dark'),
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  }))
}

/** Тема, которую видят наши токены (tokens.css). */
export const htmlTheme = () => document.documentElement.dataset.theme
