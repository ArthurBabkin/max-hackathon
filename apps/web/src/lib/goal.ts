/**
 * Цель в одну строку — на главной и в подзаголовке «Подбора». Одно-два
 * направления целиком, больше — первое и «ещё N»: иначе десяток названий
 * растягивает карточку на весь экран.
 */

import type { useVoice } from '@/voice/useVoice'
import { plural } from './deadline'

type Voice = ReturnType<typeof useVoice>

export function goalTitle(directions: { name: string }[], t: Voice): string {
  const [first, ...rest] = directions
  if (!first) return t('home.goalEmpty')
  if (rest.length < 2) return directions.map((d) => d.name).join(', ')
  const n = rest.length
  return t('home.goalMore', {
    direction: first.name,
    count: `${n} ${plural(n, 'направление', 'направления', 'направлений')}`,
  })
}
