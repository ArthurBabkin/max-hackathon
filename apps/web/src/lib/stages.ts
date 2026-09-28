/**
 * Подписи этапов трекера: короткие названия для полоски этапов и подпись
 * срока в строке олимпиады. Главная, календарь и шапка карточки трекера
 * говорят о сроке то же, что полоска: «Отбор до 4 октября».
 */

import type { TrackerItem, TrackerStage } from '@contract'
import type { TextKey } from '@/voice/texts'
import type { Translate } from '@/voice/useVoice'
import { formatDay } from './deadline'

/**
 * Короткие названия для полоски: «Отбор», «Финал». Полные названия этапов
 * бывают длиной в строку («Предварительный тур, очная предметная
 * олимпиада…»), а в полоске на них три-четыре слова места. Повторы
 * нумеруются: «Отбор 1», «Отбор 2».
 */
export function shortLabels(stages: TrackerStage[], t: Translate): string[] {
  const total = new Map<string, number>()
  for (const s of stages) total.set(s.kind, (total.get(s.kind) ?? 0) + 1)
  const seen = new Map<string, number>()
  return stages.map((s) => {
    const n = (seen.get(s.kind) ?? 0) + 1
    seen.set(s.kind, n)
    const label = t(`tracker.stageShort.${s.kind}` as TextKey)
    return (total.get(s.kind) ?? 0) > 1 ? `${label} ${n}` : label
  })
}

/** Когда этап: срок регистрации или день начала, иначе подпись словами. */
export function stageDate(s: TrackerStage, t: Translate): string {
  if (s.deadline_at) return t('tracker.stageUntil', { date: formatDay(s.deadline_at) ?? '' })
  if (s.starts_at) return formatDay(s.starts_at) ?? ''
  return s.subtitle ?? ''
}

/**
 * Подпись срока пункта: этап, к которому срок относится, и «до» даты. Срок
 * пункта — всегда срок одного из его этапов: в календаре — каждого по
 * очереди, на главной — текущего. У ВсОШ названия этапов короткие и
 * понятные («Муниципальный этап»), у перечневых — короткое из полоски.
 */
export function deadlineLabel(item: TrackerItem, t: Translate): string | null {
  const date = formatDay(item.deadline_at)
  if (!date) return null
  const until = t('tracker.stageUntil', { date })
  const at = Date.parse(item.deadline_at!)
  const i = item.stages.findIndex((s) => s.deadline_at != null && Date.parse(s.deadline_at) === at)
  if (i < 0) return `${item.next_stage_title ?? t('tracker.stageFallback')} ${until}`
  const stage = item.stages[i]!
  return `${item.kind === 'vsosh' ? stage.title : shortLabels(item.stages, t)[i]} ${until}`
}
