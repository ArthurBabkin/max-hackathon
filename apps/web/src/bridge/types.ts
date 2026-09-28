/**
 * Типы MAX Bridge. Библиотека подключается тегом <script> с st.max.ru и кладёт
 * в страницу глобальный `window.WebApp` — npm-пакета у неё нет, поэтому
 * интерфейс описан здесь вручную по dev.max.ru/docs/webapps/bridge.
 *
 * Описано только то, чем мы пользуемся. Добавлять поля по мере надобности —
 * лишние объявления всё равно нечем проверить.
 */

export interface MaxUser {
  id: number
  first_name: string
  last_name?: string
  username?: string
  photo_url?: string
}

export interface MaxInitDataUnsafe {
  user?: MaxUser
  /** Payload диплинка `?startapp=<payload>` (ТЗ §11.2). */
  start_param?: string
  auth_date?: number
  hash?: string
}

export interface MaxBackButton {
  show(): void
  hide(): void
  onClick(handler: () => void): void
  offClick(handler: () => void): void
}

export interface MaxHapticFeedback {
  impactOccurred(style: 'light' | 'medium' | 'heavy'): void
  notificationOccurred(type: 'error' | 'success' | 'warning'): void
  selectionChanged(): void
}

export interface MaxWebApp {
  /** Подписанная строка запуска. Проверяется только на сервере (ТЗ §11.3). */
  initData: string
  initDataUnsafe: MaxInitDataUnsafe
  platform: string
  version?: string
  openLink(url: string): void
  /**
   * Диплинк `https://max.ru/<…>` — открывается внутри MAX, например чат бота;
   * ссылку другого вида мост откроет во внешнем браузере. Есть не во всех
   * клиентах, поэтому необязательный.
   */
  openMaxLink?(url: string): void
  /**
   * Промис. Отклоняется объектом `{ error: { code } }` без `.message`, поэтому
   * необработанный отказ всплывает безымянной ошибкой. Ссылка — поле `link`,
   * не `url`: с неизвестным полем MAX делиться нечем.
   */
  shareMaxContent?(payload: { text?: string; link?: string }): Promise<unknown>
  BackButton: MaxBackButton
  HapticFeedback?: MaxHapticFeedback
  getViewportSize?(): { width: number; height: number }
  enableClosingConfirmation?(): void
  ready?(): void
}

declare global {
  interface Window {
    WebApp?: MaxWebApp
  }
}
