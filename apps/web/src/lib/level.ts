/** Подпись уровня олимпиады: «ВсОШ», «II уровень», «вне перечня». */

import type { Translate } from '@/voice/useVoice'

export function levelLabel(kind: string, level: string | null, t: Translate): string {
  if (kind === 'vsosh') return t('level.vsosh')
  if (kind === 'other') return t('catalog.outsidePerechen')
  return level ? t('level.numbered', { level }) : t('level.unknown')
}
