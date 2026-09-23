/**
 * «Вопросы и ответы»: разделы, порядок и первоисточники. Тексты — в общем
 * словаре (`faq.<id>.q` / `faq.<id>.a`), чтобы ученику и родителю отвечать
 * своим голосом (ТЗ F3). Правила льгот сверены с законом и Порядком приёма,
 * ссылка стоит под ответом.
 */

import type { TextKey } from '@/voice/texts'

export interface FaqSource {
  title: string
  url: string
}

export interface FaqItem {
  id: string
  source?: FaqSource
}

export interface FaqSection {
  id: string
  titleKey: TextKey
  items: FaqItem[]
}

const LAW_71: FaqSource = {
  title: 'Закон «Об образовании», статья 71',
  url: 'https://www.consultant.ru/document/cons_doc_LAW_140174/46a162e9a1bb082c0b7a1643927c9a344c20a2ec/',
}
const ORDER_821: FaqSource = {
  title: 'Порядок приёма в вузы, приказ Минобрнауки № 821',
  url: 'https://www.consultant.ru/document/cons_doc_LAW_491807/',
}
const ORDER_678: FaqSource = {
  title: 'Порядок проведения ВсОШ, приказ Минпросвещения № 678',
  url: 'https://base.garant.ru/400411428/',
}
const RSOSH: FaqSource = { title: 'Российский совет олимпиад школьников', url: 'https://rsr-olymp.ru/' }
const SIRIUS: FaqSource = { title: 'Сириус.Курсы', url: 'https://edu.sirius.online/' }

export const FAQ: FaqSection[] = [
  {
    id: 'olympiads',
    titleKey: 'faq.section.olympiads',
    items: [
      { id: 'what' },
      { id: 'vsosh', source: ORDER_678 },
      { id: 'perechen', source: RSOSH },
      { id: 'format' },
      { id: 'other', source: ORDER_821 },
    ],
  },
  {
    id: 'benefits',
    titleKey: 'faq.section.benefits',
    items: [
      { id: 'bvi', source: LAW_71 },
      { id: 'score100', source: LAW_71 },
      { id: 'confirm', source: ORDER_821 },
      { id: 'duration', source: LAW_71 },
      { id: 'grades' },
      { id: 'check', source: RSOSH },
    ],
  },
  {
    id: 'prepare',
    titleKey: 'faq.section.prepare',
    items: [{ id: 'choose' }, { id: 'howMany' }, { id: 'prepare', source: SIRIUS }, { id: 'calendar', source: ORDER_821 }],
  },
  {
    id: 'app',
    titleKey: 'faq.section.app',
    items: [{ id: 'data' }, { id: 'reminders' }, { id: 'family' }, { id: 'delete' }],
  },
]

export const faqKey = (id: string, part: 'q' | 'a') => `faq.${id}.${part}` as TextKey

/** Вопросы, где запрос встречается в вопросе или ответе; пустые разделы убираются. */
export function searchFaq(query: string, t: (key: TextKey) => string): FaqSection[] {
  const q = query.trim().toLocaleLowerCase('ru')
  if (!q) return FAQ
  return FAQ.map((section) => ({
    ...section,
    items: section.items.filter((item) =>
      `${t(faqKey(item.id, 'q'))} ${t(faqKey(item.id, 'a'))}`.toLocaleLowerCase('ru').includes(q),
    ),
  })).filter((section) => section.items.length > 0)
}
