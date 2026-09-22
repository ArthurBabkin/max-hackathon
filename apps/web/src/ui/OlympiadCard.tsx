/**
 * Карточка олимпиады в подборе (экран C3).
 *
 * Кнопка справа зависит от роли: ученик добавляет в трекер, родитель при
 * ученике в траектории предлагает (ТЗ §3.1). Решение принимает
 * `trackerAction` по флагам, которые прислал сервер.
 */

import type { OlympiadCard as OlympiadCardData } from '@contract'
import { Icon } from './Icon'
import { Pill, Tile } from './primitives'
import { useVoice } from '@/voice/useVoice'

export interface OlympiadCardProps {
  card: OlympiadCardData
  action: 'add' | 'propose'
  busy?: boolean
  onOpen: (olympiadProfileId: string) => void
  onTrack: (card: OlympiadCardData) => void
}

function levelChip(card: OlympiadCardData): { label: string; className: string } {
  if (card.kind === 'vsosh') return { label: 'ВсОШ', className: 'level level-vsosh' }
  if (card.kind === 'other') return { label: 'вне перечня', className: 'level level-outside' }
  return { label: card.level ? `${card.level} уровень` : 'уровень уточняется', className: 'level' }
}

export function OlympiadCard({ card, action, busy = false, onOpen, onTrack }: OlympiadCardProps) {
  const t = useVoice()
  const chip = levelChip(card)
  const pending = card.proposal_status === 'pending'

  const trackButton = () => {
    if (card.in_tracker) {
      return (
        <span className="track-button track-button-on" aria-label={t('olympiad.inTracker')}>
          <Icon name="check" size={16} strokeWidth={2.6} />
        </span>
      )
    }
    if (pending) {
      return (
        <span className="track-button" aria-label={t('olympiad.proposalPending')}>
          <Icon name="clock" size={16} strokeWidth={2.2} />
        </span>
      )
    }
    return (
      <button
        type="button"
        className="track-button track-button-action"
        disabled={busy}
        onClick={() => onTrack(card)}
      >
        <Icon
          name={action === 'propose' ? 'send' : 'plus'}
          size={action === 'propose' ? 15 : 16}
          strokeWidth={2.4}
        />
        <span className="sr-only">
          {action === 'propose' ? t('olympiad.proposeCta') : t('olympiad.addCta')}
        </span>
      </button>
    )
  }

  return (
    <article className="oly-card">
      <button type="button" className="oly-card-main" onClick={() => onOpen(card.olympiad_profile_id)}>
        <Tile
          id={card.olympiad_id}
          name={card.name}
          shortName={card.short_name}
          color={card.color}
          size="md"
        />
        <span className="oly-card-body">
          <span className="oly-card-title">{card.name}</span>
          <span className="oly-card-organizer">{card.organizer}</span>
          <span className="oly-card-tags">
            <span className={chip.className}>{chip.label}</span>
            <Pill deadlineAt={card.deadline_at} doneLabel={t('pill.done')} />
          </span>
          <span className="oly-card-benefits">{card.benefits_summary}</span>
          <span className="oly-card-reason">
            <Icon name="spark" size={12} />
            {card.reason}
          </span>
        </span>
      </button>
      {trackButton()}
    </article>
  )
}
