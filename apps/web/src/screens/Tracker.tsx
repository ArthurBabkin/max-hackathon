/** Трекер и календарь — экраны E1, E2, E3. Функции F28, F31, F32, F44–F46. */

import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@maxhub/max-ui'
import type { Proposal, TrackerItem } from '@contract'
import {
  useCalendar,
  useResolveProposal,
  useSession,
  useToggleRegistered,
  useTracker,
} from '@/api/queries'
import { formatDay, nearestDeadlineMonth } from '@/lib/deadline'
import { groupTracker, sortByDeadline } from '@/lib/derive'
import { Icon } from '@/ui/Icon'
import { CardSkeletons, Hint, Pill, StateBlock, Tile } from '@/ui/primitives'
import { useSheetStack } from '@/ui/sheets'
import { useRole, useVoice } from '@/voice/useVoice'
import { CalendarView } from './Calendar'
import { ErrorState } from '@/ui/ErrorState'


/** Карточка трекера с отметкой «зарегистрирован» — F46. */
function TrackerCard({
  item,
  onOpen,
  onToggle,
  disabled,
}: {
  item: TrackerItem
  onOpen: (id: string) => void
  onToggle: (item: TrackerItem) => void
  disabled: boolean
}) {
  const t = useVoice()
  const registered = Boolean(item.registered_at)
  const date = formatDay(item.deadline_at)

  const subtitle = registered
    ? t('tracker.nextStage', { stage: item.next_stage_title ?? 'следующий этап' })
    : item.kind === 'vsosh' && date
      ? t('tracker.stageOn', { stage: item.next_stage_title ?? 'Этап', date })
      : date
        ? t('tracker.deadlineUntil', { date })
        : (item.next_stage_title ?? '')

  return (
    <article className="tracker-card" data-tour="tracker-item">
      <button type="button" className="tracker-card-top" onClick={() => onOpen(item.olympiad_profile_id)}>
        <Tile id={item.olympiad_id} name={item.olympiad_name} shortName={item.short_name} color={item.color} />
        <span className="row-main">
          <span className="row-title">{item.olympiad_name}</span>
          <span className="row-subtitle">{subtitle}</span>
        </span>
        <Pill deadlineAt={item.deadline_at} registered={registered} doneLabel={t('pill.done')} />
      </button>

      <button
        type="button"
        className="tracker-check"
        role="checkbox"
        aria-checked={registered}
        disabled={disabled}
        onClick={() => onToggle(item)}
      >
        <span className={`checkbox${registered ? ' checkbox-on' : ''}`}>
          {registered ? <Icon name="check" size={12} strokeWidth={3} /> : null}
        </span>
        {t('tracker.registeredCta')}
        <em className="tracker-check-author">
          {registered && item.registered_by ? (
            t('tracker.markedBy', { name: item.registered_by.name })
          ) : (
            <Icon name="bell" size={13} title="Напоминания включены" />
          )}
        </em>
      </button>
    </article>
  )
}

/** Блок предложения от родителя — E1 у ученика, E3 у родителя. */
function ProposalBlock({
  proposal,
  onOpen,
  onResolve,
  canResolve,
  busy,
}: {
  proposal: Proposal
  onOpen: (id: string) => void
  onResolve: (id: string, accept: boolean) => void
  canResolve: boolean
  busy: boolean
}) {
  const t = useVoice()
  const date = formatDay(proposal.deadline_at)

  if (!canResolve) {
    return (
      <article className="tracker-card">
        <button type="button" className="tracker-card-top" onClick={() => onOpen(proposal.olympiad_profile_id)}>
          <Tile
            id={proposal.olympiad_id}
            name={proposal.olympiad_name}
            shortName={proposal.short_name}
            color={proposal.color}
          />
          <span className="row-main">
            <span className="row-title">{proposal.olympiad_name}</span>
            <span className="row-subtitle">{t('tracker.proposalPending')}</span>
          </span>
          <span className="pill pill-ok">
            <Icon name="clock" size={11} />
            {t('tracker.proposalWaiting')}
          </span>
        </button>
      </article>
    )
  }

  return (
    <article className="proposal">
      <p className="proposal-head">
        <Icon name="inbox" size={14} />
        {t('tracker.proposalFrom', { name: proposal.proposed_by.name })}
      </p>
      <button type="button" className="tracker-card-top" onClick={() => onOpen(proposal.olympiad_profile_id)}>
        <Tile
          id={proposal.olympiad_id}
          name={proposal.olympiad_name}
          shortName={proposal.short_name}
          color={proposal.color}
        />
        <span className="row-main">
          <span className="row-title">{proposal.olympiad_name}</span>
          <span className="row-subtitle">{date ? t('tracker.deadlineUntil', { date }) : ''}</span>
        </span>
        <Pill deadlineAt={proposal.deadline_at} doneLabel={t('pill.done')} />
      </button>
      <div className="proposal-actions">
        <Button stretched loading={busy} onClick={() => onResolve(proposal.id, true)}>
          {t('tracker.proposalAccept')}
        </Button>
        <Button stretched variant="secondary" disabled={busy} onClick={() => onResolve(proposal.id, false)}>
          {t('tracker.proposalDecline')}
        </Button>
      </div>
    </article>
  )
}

export function TrackerScreen() {
  const t = useVoice()
  const role = useRole()
  const navigate = useNavigate()
  const sheets = useSheetStack()

  const [view, setView] = useState<'list' | 'calendar'>('list')
  // null — месяц ближайшего срока. Сбрасывается при каждом входе в календарь,
  // чтобы следующий срок был виден сразу, а не через листание.
  const [pickedMonth, setPickedMonth] = useState<string | null>(null)

  const { data: session } = useSession()
  const tracker = useTracker()
  const month = pickedMonth ?? nearestDeadlineMonth(tracker.data?.items ?? [])
  const calendar = useCalendar(month)
  const toggle = useToggleRegistered()
  const resolve = useResolveProposal()

  const openOlympiad = (id: string) => sheets.open({ kind: 'oly', id })
  const canResolve = session?.permissions.resolve_proposals ?? false

  const segment = (
    <div className="segment" role="tablist">
      <button
        type="button"
        role="tab"
        aria-selected={view === 'list'}
        className={view === 'list' ? 'segment-on' : ''}
        data-tour="tracker-list"
        onClick={() => setView('list')}
      >
        {t('tracker.viewList')}
      </button>
      <button
        type="button"
        role="tab"
        aria-selected={view === 'calendar'}
        className={view === 'calendar' ? 'segment-on' : ''}
        data-tour="tracker-calendar"
        onClick={() => {
          setPickedMonth(null)
          setView('calendar')
        }}
      >
        {t('tracker.viewCalendar')}
      </button>
    </div>
  )

  if (tracker.isPending) {
    return (
      <div className="screen">
        {segment}
        <CardSkeletons count={3} />
      </div>
    )
  }

  if (tracker.isError || !tracker.data) {
    return (
      <div className="screen">
        {segment}
        <ErrorState error={tracker.error} onRetry={() => void tracker.refetch()} />
      </div>
    )
  }

  const { items, proposals } = tracker.data

  if (items.length === 0 && proposals.length === 0) {
    return (
      <div className="screen">
        {segment}
        <StateBlock icon="calendar" title={t('tracker.emptyTitle')} text={t('tracker.emptyText')}>
          <Button stretched onClick={() => navigate('/match')}>
            {t('tracker.emptyCta')}
          </Button>
        </StateBlock>
      </div>
    )
  }

  if (view === 'calendar') {
    return (
      <div className="screen">
        {segment}
        <Hint>{t('tracker.remindNote')}</Hint>
        <CalendarView month={month} data={calendar.data} onMonthChange={setPickedMonth} onOpen={openOlympiad} />
      </div>
    )
  }

  const sorted = sortByDeadline(items)
  const { open, done } = groupTracker(sorted)

  return (
    <div className="screen">
      {segment}
      <Hint>{t('tracker.remindNote')}</Hint>

      {proposals.length > 0 ? (
        <>
          {!canResolve ? (
            <p className="week-title">
              {t('tracker.awaitingTitle')}: {proposals.length}
            </p>
          ) : null}
          {proposals.map((proposal) => (
            <ProposalBlock
              key={proposal.id}
              proposal={proposal}
              canResolve={canResolve}
              busy={resolve.isPending}
              onOpen={openOlympiad}
              onResolve={(id, accept) => resolve.mutate({ id, accept })}
            />
          ))}
        </>
      ) : null}

      {open.length > 0 ? (
        <>
          <p className="week-title">
            {t('tracker.groupOpen')}: {open.length}
          </p>
          {open.map((item) => (
            <TrackerCard
              key={item.id}
              item={item}
              disabled={toggle.isPending}
              onOpen={openOlympiad}
              onToggle={(target) => toggle.mutate({ id: target.id, registered: true })}
            />
          ))}
        </>
      ) : null}

      {done.length > 0 ? (
        <>
          <p className="week-title">
            {t('tracker.groupDone')}: {done.length}
          </p>
          {done.map((item) => (
            <TrackerCard
              key={item.id}
              item={item}
              disabled={toggle.isPending}
              onOpen={openOlympiad}
              onToggle={(target) => toggle.mutate({ id: target.id, registered: false })}
            />
          ))}
        </>
      ) : null}

      <Button
        stretched
        variant="secondary"
        className="tracker-add"
        iconBefore={<Icon name="plus" size={16} />}
        onClick={() => navigate(role === 'parent' ? '/match' : '/catalog')}
      >
        {t('tracker.addCta')}
      </Button>
    </div>
  )
}
