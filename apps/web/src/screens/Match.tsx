/**
 * Подбор (экраны C3, C7): подходящие олимпиады, которых ещё нет в трекере,
 * фильтры, «Показать ещё» и блок «Вне перечня». Когда добавлено всё — C7.
 */

import { useRef, useState } from 'react'
import { Button } from '@maxhub/max-ui'
import { useNavigate } from 'react-router-dom'
import { MATCH_FILTERS, type MatchFilter, type OlympiadCard as CardData } from '@contract'
import { useAddToTracker, useHome, usePropose, useRecommendations, useSession } from '@/api/queries'
import { goalTitle } from '@/lib/goal'
import { trackerAction } from '@/lib/permissions'
import { Icon } from '@/ui/Icon'
import { OlympiadCard } from '@/ui/OlympiadCard'
import { CardSkeletons, Chip, Section, StateBlock } from '@/ui/primitives'
import { useSheetStack } from '@/ui/sheets'
import { useVoice } from '@/voice/useVoice'
import type { TextKey } from '@/voice/texts'
import { ErrorState } from '@/ui/ErrorState'
import { showInfoToast } from '@/ui/toast'

const FILTER_LABELS: Record<MatchFilter, TextKey> = {
  all: 'match.filterAll',
  level1: 'match.filterLevel1',
  soon: 'match.filterSoon',
  online: 'match.filterOnline',
}

// Длительность ухода карточки — должна совпадать с transition в app.css
// (.match-card-leaving).
const CARD_LEAVE_MS = 260

export function MatchScreen() {
  const t = useVoice()
  const navigate = useNavigate()
  const sheets = useSheetStack()
  const [filter, setFilter] = useState<MatchFilter>('all')
  const [showMore, setShowMore] = useState(false)

  const { data: session } = useSession()
  const { data: home } = useHome()
  const recommendations = useRecommendations(filter)
  const add = useAddToTracker()
  const propose = usePropose()

  const action = session ? trackerAction(session) : 'add'
  const busy = add.isPending || propose.isPending

  // Карточка уходит из подбора с анимацией, а не мгновенно: кнопка сразу
  // показывает результат (F26 доработка), а сама карточка — ещё
  // CARD_LEAVE_MS, пока сервер её уже убрал из ответа.
  const [leaving, setLeaving] = useState<Set<string>>(new Set())
  const leavingCards = useRef<Map<string, CardData>>(new Map())

  const onTrack = (card: CardData) => {
    const id = card.olympiad_profile_id
    leavingCards.current.set(id, card)
    setLeaving((prev) => new Set(prev).add(id))
    setTimeout(() => {
      leavingCards.current.delete(id)
      setLeaving((prev) => {
        const next = new Set(prev)
        next.delete(id)
        return next
      })
    }, CARD_LEAVE_MS)
    if (action === 'propose')
      propose.mutate(id, { onSuccess: () => showInfoToast(t('toast.proposalSent')) })
    else add.mutate(id, { onSuccess: () => showInfoToast(t('toast.trackAdded')) })
  }

  // withLeaving — держит карточку в списке, пока идёт анимация ухода, даже
  // если сервер её уже не прислал среди items/more/outside.
  const withLeaving = (list: CardData[]): CardData[] => {
    const ids = new Set(list.map((c) => c.olympiad_profile_id))
    const extra = [...leavingCards.current.values()].filter((c) => !ids.has(c.olympiad_profile_id))
    return [...list, ...extra]
  }

  const openOlympiad = (id: string) => sheets.open({ kind: 'oly', id })

  const trajectory = home?.trajectory ?? session?.trajectory

  const header = (
    <header className="match-head">
      <h1 className="match-title">{t('match.title')}</h1>
      {trajectory ? (
        <p className="match-subtitle">
          {t(trajectory.region_name ? 'match.subtitle' : 'match.subtitleNoRegion', {
            direction: goalTitle(trajectory.directions, t),
            grade: trajectory.grade,
            region: trajectory.region_name,
          })}
        </p>
      ) : null}
    </header>
  )

  const chips = (
    <div className="chips" role="group" aria-label={t('match.filtersLabel')}>
      {MATCH_FILTERS.map((key) => (
        <Chip
          key={key}
          active={filter === key}
          onClick={() => {
            setFilter(key)
            setShowMore(false)
          }}
        >
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

  const { items, more, outside, note, tracked_count, proposed_count, state } = recommendations.data
  const filtered = filter !== 'all'

  const resetFilter = (
    <Button stretched variant="secondary" onClick={() => setFilter('all')}>
      {t('match.emptyReset')}
    </Button>
  )

  // Что спрятано: добавленное и ждущее ответа на предложение (C3).
  const hidden =
    tracked_count > 0 || proposed_count > 0 ? (
      <div className="match-hidden">
        {tracked_count > 0 ? (
          <button type="button" className="match-hidden-row" onClick={() => navigate('/tracker')}>
            <Icon name="check" size={13} strokeWidth={2.6} />
            <span>{t('match.hiddenTracked', { count: tracked_count })}</span>
            <Icon name="chevron" size={14} />
          </button>
        ) : null}
        {proposed_count > 0 ? (
          <button type="button" className="match-hidden-row" onClick={() => navigate('/tracker')}>
            <Icon name="clock" size={13} />
            <span>{t('match.hiddenProposed', { count: proposed_count })}</span>
            <Icon name="chevron" size={14} />
          </button>
        ) : null}
      </div>
    ) : null

  if (items.length === 0 && leaving.size === 0) {
    let block
    if (state === 'all_tracked') {
      block = (
        <StateBlock
          icon="award"
          tone="ok"
          title={t(filtered ? 'match.allTrackedFilterTitle' : 'match.allTrackedTitle')}
          text={filtered ? undefined : t('match.allTrackedText')}
        >
          <Button stretched onClick={() => navigate('/tracker')}>
            {t('match.openTracker')}
          </Button>
          {filtered ? (
            resetFilter
          ) : (
            <Button stretched variant="secondary" onClick={() => navigate('/catalog')}>
              {t('match.openCatalog')}
            </Button>
          )}
        </StateBlock>
      )
    } else if (state === 'all_proposed') {
      block = (
        <StateBlock icon="clock" title={t('match.allProposedTitle')} text={t('match.allProposedText')}>
          <Button stretched onClick={() => navigate('/tracker')}>
            {t('match.openTracker')}
          </Button>
          {filtered ? resetFilter : null}
        </StateBlock>
      )
    } else if (filtered) {
      block = (
        <StateBlock icon="search" title={t('match.emptyTitle')} text={t('match.emptyText')}>
          <Button stretched onClick={() => setFilter('all')}>
            {t('match.emptyReset')}
          </Button>
          <Button stretched variant="secondary" onClick={() => navigate('/profile')}>
            {t('match.emptyProfile')}
          </Button>
        </StateBlock>
      )
    } else {
      block = (
        <StateBlock icon="search" title={t('match.noneTitle')} text={t('match.noneText')}>
          <Button stretched onClick={() => navigate('/catalog?segment=universities')}>
            {t('match.findUniversities')}
          </Button>
          <Button stretched variant="secondary" onClick={() => navigate('/profile')}>
            {t('match.emptyProfile')}
          </Button>
        </StateBlock>
      )
    }
    return (
      <div className="screen">
        {header}
        {chips}
        {block}
      </div>
    )
  }

  const shown = showMore ? [...items, ...more] : items

  return (
    <div className="screen">
      {header}
      {chips}

      <p className="info-line">
        <span className="tag tag-recommendation">{t('match.recommendationTag')}</span>
        {note}
      </p>
      {/* Без вузов льготы считаются по всем вузам с направлением (SPEC 2.3). */}
      {home?.universities_count === 0 ? (
        <p className="info-line info-line-plain">{t('match.noUniversities')}</p>
      ) : null}
      {hidden}

      <div className="match-list" data-tour="match-list">
        {withLeaving(shown).map((card) => {
          const isLeaving = leaving.has(card.olympiad_profile_id)
          return (
            <div
              key={card.olympiad_profile_id}
              className={`match-card-wrap${isLeaving ? ' match-card-leaving' : ''}`}
            >
              <OlympiadCard
                card={card}
                action={action}
                busy={busy}
                justAdded={isLeaving}
                onOpen={openOlympiad}
                onTrack={onTrack}
              />
            </div>
          )
        })}
      </div>
      {!showMore && more.length > 0 ? (
        <button type="button" className="match-more" onClick={() => setShowMore(true)}>
          {t('match.showMore', { count: more.length })}
          <Icon name="down" size={14} />
        </button>
      ) : null}

      {outside.length > 0 ? (
        <>
          <Section title={t('match.outsideTitle')} />
          <p className="info-line info-line-plain">{t('match.outsideNote')}</p>
          {withLeaving(outside).map((card) => {
            const isLeaving = leaving.has(card.olympiad_profile_id)
            return (
              <div
                key={card.olympiad_profile_id}
                className={`match-card-wrap${isLeaving ? ' match-card-leaving' : ''}`}
              >
                <OlympiadCard
                  card={card}
                  action={action}
                  busy={busy}
                  justAdded={isLeaving}
                  onOpen={openOlympiad}
                  onTrack={onTrack}
                />
              </div>
            )
          })}
        </>
      ) : null}
    </div>
  )
}
