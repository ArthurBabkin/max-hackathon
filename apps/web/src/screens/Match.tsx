/** Подбор (экран C3): 3–5 олимпиад под цель, фильтры, блок «Вне перечня». */

import { useState } from 'react'
import { Button } from '@maxhub/max-ui'
import { useNavigate } from 'react-router-dom'
import { MATCH_FILTERS, type MatchFilter, type OlympiadCard as CardData } from '@contract'
import { useAddToTracker, useHome, usePropose, useRecommendations, useSession } from '@/api/queries'
import { trackerAction } from '@/lib/permissions'
import { Icon } from '@/ui/Icon'
import { OlympiadCard } from '@/ui/OlympiadCard'
import { CardSkeletons, Chip, Section, StateBlock } from '@/ui/primitives'
import { useSheetStack } from '@/ui/sheets'
import { useVoice } from '@/voice/useVoice'
import type { TextKey } from '@/voice/texts'
import { ErrorState } from '@/ui/ErrorState'

const FILTER_LABELS: Record<MatchFilter, TextKey> = {
  all: 'match.filterAll',
  level1: 'match.filterLevel1',
  soon: 'match.filterSoon',
  online: 'match.filterOnline',
}

export function MatchScreen() {
  const t = useVoice()
  const navigate = useNavigate()
  const sheets = useSheetStack()
  const [filter, setFilter] = useState<MatchFilter>('all')

  const { data: session } = useSession()
  const { data: home } = useHome()
  const recommendations = useRecommendations(filter)
  const add = useAddToTracker()
  const propose = usePropose()

  const action = session ? trackerAction(session) : 'add'
  const busy = add.isPending || propose.isPending

  const onTrack = (card: CardData) => {
    if (action === 'propose') propose.mutate(card.olympiad_profile_id)
    else add.mutate(card.olympiad_profile_id)
  }

  const openOlympiad = (id: string) => sheets.open({ kind: 'oly', id })

  const trajectory = home?.trajectory ?? session?.trajectory

  const header = (
    <header className="match-head">
      <h1 className="match-title">{t('match.title')}</h1>
      {trajectory ? (
        <p className="match-subtitle">
          {t('match.subtitle', {
            direction: trajectory.directions.map((d) => d.name).join(', ') || t('home.goalEmpty'),
            grade: trajectory.grade,
            region: trajectory.region_name,
          })}
        </p>
      ) : null}
    </header>
  )

  const chips = (
    <div className="chips" role="group" aria-label="Фильтры подбора">
      {MATCH_FILTERS.map((key) => (
        <Chip key={key} active={filter === key} onClick={() => setFilter(key)}>
          {t(FILTER_LABELS[key])}
        </Chip>
      ))}
    </div>
  )

  if (recommendations.isPending) {
    return (
      <div className="screen">
        {header}
        {chips}
        <p className="info-line">
          <Icon name="spark" size={14} />
          {t('match.loading')}
        </p>
        <CardSkeletons count={3} />
      </div>
    )
  }

  if (recommendations.isError || !recommendations.data) {
    return (
      <div className="screen">
        {header}
        <ErrorState error={recommendations.error} title={t('state.matchErrorTitle')} onRetry={() => void recommendations.refetch()} />
      </div>
    )
  }

  const { items, outside, note } = recommendations.data

  if (items.length === 0) {
    return (
      <div className="screen">
        {header}
        {chips}
        <StateBlock icon="search" title={t('match.emptyTitle')} text={t('match.emptyText')}>
          <Button stretched onClick={() => setFilter('all')}>
            {t('match.emptyReset')}
          </Button>
          <Button stretched variant="secondary" onClick={() => navigate('/profile')}>
            {t('match.emptyProfile')}
          </Button>
        </StateBlock>
      </div>
    )
  }

  return (
    <div className="screen">
      {header}
      {chips}

      <p className="info-line">
        <span className="tag tag-recommendation">{t('match.recommendationTag')}</span>
        {note}
      </p>

      <div className="match-list" data-tour="match-list">
        {items.map((card) => (
          <OlympiadCard
            key={card.olympiad_profile_id}
            card={card}
            action={action}
            busy={busy}
            onOpen={openOlympiad}
            onTrack={onTrack}
          />
        ))}
      </div>

      {outside.length > 0 ? (
        <>
          <Section title={t('match.outsideTitle')} />
          <p className="info-line info-line-plain">{t('match.outsideNote')}</p>
          {outside.map((card) => (
            <OlympiadCard
              key={card.olympiad_profile_id}
              card={card}
              action={action}
              busy={busy}
              onOpen={openOlympiad}
              onTrack={onTrack}
            />
          ))}
        </>
      ) : null}
    </div>
  )
}
