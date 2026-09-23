// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { copyText } from './clipboard'

describe('copyText', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('Clipboard API есть — пишет через него', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    expect(await copyText('ссылка')).toBe(true)
    expect(writeText).toHaveBeenCalledWith('ссылка')
  })

  it('Clipboard API отказал — копирует через выделение', async () => {
    vi.stubGlobal('navigator', { clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } })
    document.execCommand = vi.fn().mockReturnValue(true)
    expect(await copyText('ссылка')).toBe(true)
    expect(document.execCommand).toHaveBeenCalledWith('copy')
    expect(document.querySelector('textarea')).toBeNull()
  })

  it('не вышло никак — false', async () => {
    vi.stubGlobal('navigator', {})
    document.execCommand = vi.fn().mockReturnValue(false)
    expect(await copyText('ссылка')).toBe(false)
  })
})
