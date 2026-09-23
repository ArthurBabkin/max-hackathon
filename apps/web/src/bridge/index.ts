/**
 * Доступ к MAX Bridge.
 *
 * Скрипт моста грузится с st.max.ru тегом <script defer>, то есть со стороннего
 * хоста, который может быть медленным или недоступным. Поэтому `getWebApp()`
 * никогда его не ждёт и не падает.
 *
 * Тонкость, которая видна только в браузере: вне MAX скрипт всё равно
 * загружается и создаёт `window.WebApp`, но транспорта у него нет —
 * `initData` и `platform` приходят пустыми, а вызовы BackButton уходят в
 * предупреждение «транспорт недоступен». Проверять «мост есть» недостаточно,
 * проверяем, что он рабочий.
 */

import { createStub } from './stub'
import type { MaxWebApp } from './types'

export type { MaxUser, MaxWebApp } from './types'

let stub: MaxWebApp | null = null

/** Мост есть и у него есть подписанные данные запуска — значит мы внутри MAX. */
export function isWorkingBridge(candidate: MaxWebApp | undefined): candidate is MaxWebApp {
  return Boolean(candidate && candidate.initData)
}

/** Разрешена ли заглушка. Внутри MAX она не нужна, в проде — недопустима. */
function stubAllowed(): boolean {
  return import.meta.env.DEV || import.meta.env.VITE_DEV_FAKE_WEBAPP === 'true'
}

export function getWebApp(): MaxWebApp {
  const real = typeof window !== 'undefined' ? window.WebApp : undefined

  if (isWorkingBridge(real)) return real

  if (!stubAllowed()) {
    // В проде это значит, что мини-приложение открыли не из MAX либо скрипт
    // моста не загрузился. Подменять данные запуска здесь нельзя: сервер
    // всё равно отвергнет неподписанный initData, и лучше, чтобы причина
    // была видна в ошибке, а не спрятана за заглушкой.
    console.error('[bridge] рабочий window.WebApp недоступен')
    if (real) return real
  }

  stub ??= createStub()
  return stub
}

/** Работаем ли мы внутри настоящего MAX. Пригодится в отладочной панели. */
export function isRealBridge(): boolean {
  return isWorkingBridge(typeof window !== 'undefined' ? window.WebApp : undefined)
}

/**
 * Отправить ссылку в чат MAX. false — поделиться не вышло: метода нет в
 * клиенте или MAX отказал. Промис никогда не отклоняется — отказ моста не
 * должен долетать до пользователя баннером «Что-то сломалось».
 */
export async function shareLink(link: string): Promise<boolean> {
  const bridge = getWebApp()
  if (!bridge.shareMaxContent) return false
  try {
    await bridge.shareMaxContent({ link })
    return true
  } catch (error) {
    console.warn('[bridge] shareMaxContent отказал', error)
    return false
  }
}
