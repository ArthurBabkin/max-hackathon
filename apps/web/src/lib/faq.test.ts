import { describe, expect, it } from 'vitest'
import raw from '@texts'
import { text } from '@/voice/texts'
import { FAQ, searchFaq } from './faq'

const dict = raw as Record<string, unknown>
const kid = (key: string) => text(key as never, 'kid', { student: 'Артём' })

describe('FAQ', () => {
  it('у каждого вопроса есть текст вопроса и ответа в словаре', () => {
    for (const section of FAQ) {
      expect(dict[section.titleKey], section.titleKey).toBeDefined()
      for (const item of section.items) {
        expect(dict[`faq.${item.id}.q`], item.id).toBeDefined()
        expect(dict[`faq.${item.id}.a`], item.id).toBeDefined()
      }
    }
  })

  it('id вопросов уникальны', () => {
    const ids = FAQ.flatMap((s) => s.items.map((i) => i.id))
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('источники — только https: ответ опирается на первоисточник', () => {
    for (const item of FAQ.flatMap((s) => s.items)) {
      if (item.source) expect(item.source.url, item.id).toMatch(/^https:\/\//)
    }
  })

  it('правила льгот — со ссылкой на закон или Порядок приёма', () => {
    const legal = ['bvi', 'score100', 'confirm', 'duration', 'other']
    for (const id of legal) {
      const item = FAQ.flatMap((s) => s.items).find((i) => i.id === id)
      expect(item?.source, id).toBeDefined()
    }
  })
})

describe('searchFaq', () => {
  it('пустой запрос — все разделы как есть', () => {
    expect(searchFaq('  ', kid)).toEqual(FAQ)
  })

  it('ищет без учёта регистра и по тексту ответа, пустые разделы выбрасывает', () => {
    const found = searchFaq('бви', kid)
    const ids = found.flatMap((s) => s.items.map((i) => i.id))
    expect(ids).toContain('bvi')
    expect(found.every((s) => s.items.length > 0)).toBe(true)
    expect(ids.length).toBeLessThan(FAQ.flatMap((s) => s.items).length)
  })

  it('ничего не нашлось — пустой список', () => {
    expect(searchFaq('квазиэкспонента', kid)).toEqual([])
  })
})
