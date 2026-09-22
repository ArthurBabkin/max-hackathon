import { describe, expect, it } from 'vitest'
import { isWorkingBridge } from './index'
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
