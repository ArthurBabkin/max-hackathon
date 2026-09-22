/**
 * Заглушка MAX Bridge для разработки в обычном браузере (ТЗ §11.4).
 *
 * Внутри MAX консоли нет, поэтому вся работа идёт здесь, а мессенджер остаётся
 * средой дымовой проверки. Заглушка не подписывает initData — подписать её
 * может только сервер, у которого есть токен бота; на моках подпись и не нужна,
 * а живой API в dev-режиме такую строку не примет, и это правильно.
 */

import type { MaxWebApp } from './types'

/**
 * Персонажи демо-траектории (packages/db/migrations-demo): Артём — ученик,
 * Ольга — мама. `?dev_user=parent` в адресе открывает приложение голосом
 * родителя. Id из дев-диапазона 900000000–900000999: живой API принимает их
 * без подписи только вне production.
 */
const DEV_USERS = {
  kid: { id: 900_000_001, first_name: 'Артём' },
  parent: { id: 900_000_002, first_name: 'Ольга' },
}

function devUser() {
  const who = typeof location === 'undefined' ? null : new URLSearchParams(location.search).get('dev_user')
  return who === 'parent' ? DEV_USERS.parent : DEV_USERS.kid
}

const DEV_USER = devUser()

function fakeInitData(): string {
  const params = new URLSearchParams({
    user: JSON.stringify(DEV_USER),
    auth_date: String(Math.floor(Date.now() / 1000)),
    hash: 'dev-stub-unsigned',
  })
  return params.toString()
}

export function createStub(): MaxWebApp {
  const backHandlers = new Set<() => void>()
  let backVisible = false

  return {
    initData: fakeInitData(),
    initDataUnsafe: {
      user: DEV_USER,
      start_param: new URLSearchParams(location.search).get('startapp') ?? undefined,
      auth_date: Math.floor(Date.now() / 1000),
    },
    platform: 'web',
    version: 'stub',

    openLink(url) {
      window.open(url, '_blank', 'noopener,noreferrer')
    },

    shareMaxContent(payload) {
      console.info('[bridge-stub] shareMaxContent', payload)
    },

    BackButton: {
      show() {
        backVisible = true
        // Кнопку «Назад» в браузере рисует сам браузер, здесь достаточно следа
        // в консоли: на телефоне поведение всё равно проверяется отдельно.
        console.debug('[bridge-stub] BackButton.show')
      },
      hide() {
        backVisible = false
        console.debug('[bridge-stub] BackButton.hide')
      },
      onClick(handler) {
        backHandlers.add(handler)
      },
      offClick(handler) {
        backHandlers.delete(handler)
      },
    },

    HapticFeedback: {
      impactOccurred() {},
      notificationOccurred() {},
      selectionChanged() {},
    },

    getViewportSize: () => ({ width: window.innerWidth, height: window.innerHeight }),

    ready() {
      console.debug('[bridge-stub] ready, BackButton виден:', backVisible)
    },
  }
}
