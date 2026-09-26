/**
 * Строка олимпиады со сроком. Одна и та же на главной, в трекере и в
 * календаре — отличается только подписью под названием.
 */

import type { TrackerItem } from '@contract'
import { formatDay } from '@/lib/deadline'
import { useVoice } from '@/voice/useVoice'
import { Pill, Tile } from './primitives'

export interface TrackerRowProps {
  item: TrackerItem
  onOpen: (olympiadProfileId: string) => void
  /** Показывать, кто поставил отметку, вместо срока. */
  showAuthor?: boolean
}

export function TrackerRow({ item, onOpen, showAuthor = true }: TrackerRowProps) {
  const t = useVoice()
  const registered = Boolean(item.registered_at)

  const subtitle = () => {
    if (registered && showAuthor && item.registered_by) {
      return t('tracker.markedBy', { name: item.registered_by.name })
    }
    const date = formatDay(item.deadline_at)
    if (!date) return item.next_stage_title ?? ''
    if (item.kind === 'vsosh') {
      return t('tracker.stageOn', { stage: item.next_stage_title ?? t('tracker.stageFallback'), date })
    }
    return t('tracker.deadlineUntil', { date })
  }

  return (
    <button type="button" className="row" onClick={() => onOpen(item.olympiad_profile_id)}>
      <Tile id={item.olympiad_id} name={item.olympiad_name} shortName={item.short_name} color={item.color} />
      <span className="row-main">
        <span className="row-title">{item.olympiad_name}</span>
        <span className="row-subtitle">{subtitle()}</span>
      </span>
      <Pill deadlineAt={item.deadline_at} registered={registered} doneLabel={t('pill.done')} />
    </button>
  )
}
