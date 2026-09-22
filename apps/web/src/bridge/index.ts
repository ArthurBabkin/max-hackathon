/**
 * Доступ к MAX Bridge.
 *
 * Скрипт моста грузится с st.max.ru тегом <script defer> — то есть со стороннего
 * хоста, который может быть медленным или недоступным. Поэтому `getWebApp()`
 * никогда не ждёт его и не падает: если моста нет, возвращается заглушка,
 * и приложение продолжает работать.
 */

import { createStub } from './stub'
import type { MaxWebApp } from './types'

export type { MaxUser, MaxWebApp } from './types'

let stub: MaxWebApp | null = null

/** Разрешена ли заглушка. Внутри MAX она не нужна, в проде — недопустима. */
function stubAllowed(): boolean {
  return import.meta.env.DEV || import.meta.env.VITE_DEV_FAKE_WEBAPP === 'true'
}

export function getWebApp(): MaxWebApp {
  const real = typeof window !== 'undefined' ? window.WebApp : undefined
  if (real) return real

  if (!stubAllowed()) {
    // В проде это значит, что мини-приложение открыли не из MAX либо скрипт
    // моста не загрузился. Белый экран здесь хуже, чем неполная работа,
    // поэтому отдаём заглушку и пишем в лог — ошибку покажет баннер в App.
    console.error('[bridge] window.WebApp недоступен, работаем без моста')
  }

  stub ??= createStub()
  return stub
}

/** Открыт ли настоящий мост. Пригодится в отладочной панели. */
export function isRealBridge(): boolean {
  return typeof window !== 'undefined' && Boolean(window.WebApp)
}
