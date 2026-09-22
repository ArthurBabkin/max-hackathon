/**
 * Заглушка MAX Bridge для разработки в обычном браузере (ТЗ §11.4).
 *
 * Внутри MAX консоли нет, поэтому вся работа идёт здесь, а мессенджер остаётся
 * средой дымовой проверки. Заглушка не подписывает initData — подписать её
 * может только сервер, у которого есть токен бота; на моках подпись и не нужна,
 * а живой API в dev-режиме такую строку не примет, и это правильно.
 */

import type { MaxWebApp } from './types'

const DEV_USER = { id: 900_000_001, first_name: 'Артём' }

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
