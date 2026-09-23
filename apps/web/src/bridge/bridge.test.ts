// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { isWorkingBridge, shareLink } from './index'
import type { MaxWebApp } from './types'

/**
 * Вне MAX скрипт моста всё равно грузится и создаёт window.WebApp, но
 * транспорта у него нет: initData приходит пустым. Проверка «объект есть»
 * на это не ловится, поэтому проверяется именно работоспособность.
 */
describe('isWorkingBridge', () => {
  const bridge = (over: Partial<MaxWebApp>) => ({ initData: '', ...over }) as MaxWebApp

  it('моста нет вовсе', () => {
    expect(isWorkingBridge(undefined)).toBe(false)
  })

  it('мост есть, но initData пустой — мы не внутри MAX', () => {
    expect(isWorkingBridge(bridge({ initData: '' }))).toBe(false)
    expect(isWorkingBridge(bridge({ initData: null as unknown as string }))).toBe(false)
  })

  it('мост с подписанными данными запуска — настоящий', () => {
    expect(isWorkingBridge(bridge({ initData: 'user=%7B%7D&hash=abc' }))).toBe(true)
  })
})

/**
 * Настоящий мост отклоняет shareMaxContent объектом { error: { code } } —
 * так в исходнике st.max.ru/js/max-web-app.js. Необработанный отказ давал
 * баннер «Что-то сломалось: запрос не завершился» на экране «Семья».
 */
describe('shareLink', () => {
  const inMax = (shareMaxContent?: MaxWebApp['shareMaxContent']) => {
    window.WebApp = { initData: 'user=%7B%7D&hash=abc', shareMaxContent } as MaxWebApp
  }

  afterEach(() => {
    delete window.WebApp
    vi.restoreAllMocks()
  })

  it('отдаёт ссылку в поле link — поле url MAX не знает', async () => {
    const share = vi.fn().mockResolvedValue({ status: 'shared' })
    inMax(share)
    expect(await shareLink('https://max.ru/bot?start=inv_x')).toBe(true)
    expect(share).toHaveBeenCalledWith({ link: 'https://max.ru/bot?start=inv_x' })
  })

  it('отказ моста не отклоняет промис, а возвращает false', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    inMax(() => Promise.reject({ error: { code: 'client.web_app_max_share.request_timeout' } }))
    await expect(shareLink('https://max.ru/bot?start=inv_x')).resolves.toBe(false)
  })

  it('метода нет в клиенте — false, ссылку никто не открывает', async () => {
    inMax(undefined)
    expect(await shareLink('https://max.ru/bot?start=inv_x')).toBe(false)
  })
})
