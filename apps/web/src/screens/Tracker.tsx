/** Трекер и календарь — экраны E1, E2, E3. Функции F28, F31, F32, F44–F46. */

import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@maxhub/max-ui'
import type { CalendarLink, Proposal, StageResult, TrackerItem, TrackerStage } from '@contract'
import { apiUrl } from '@/api/client'
import {
  useCalendar,
  useCalendarLink,
  useRemoveFromTracker,
  useResolveProposal,
  useSession,
  useSetStageMark,
  useTracker,
} from '@/api/queries'
import { getWebApp } from '@/bridge'
import { formatDay, nearestDeadlineMonth } from '@/lib/deadline'
import { groupTracker, sortByDeadline } from '@/lib/derive'
import { Icon } from '@/ui/Icon'
import { CardSkeletons, Hint, Pill, StateBlock, Tile } from '@/ui/primitives'
import { useSheetStack } from '@/ui/sheets'
import { showErrorToast } from '@/ui/toast'
import type { TextKey } from '@/voice/texts'
import { useRole, useVoice } from '@/voice/useVoice'
import { CalendarView } from './Calendar'
import { ErrorState } from '@/ui/ErrorState'


type Voice = ReturnType<typeof useVoice>
type Mark = (stageId: string | null, registered: boolean, result: StageResult | null) => void

const registrationLike = (kind: string) => kind === 'registration' || kind === 'school'

/**
 * Короткие названия для полоски: «Отбор», «Финал». Полные названия этапов
 * бывают длиной в строку («Предварительный тур, очная предметная
 * олимпиада…»), а в полоске на них три-четыре слова места. Повторы
 * нумеруются: «Отбор 1», «Отбор 2».
 */
function shortLabels(stages: TrackerStage[], t: Voice): string[] {
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
function stageDate(s: TrackerStage): string {
  if (s.deadline_at) return `до ${formatDay(s.deadline_at)}`
  if (s.starts_at) return formatDay(s.starts_at) ?? ''
  return s.subtitle ?? ''
}

/** Класс плитки в полоске: отмечено, ждёт итога, текущий, серый. */
function stripTone(s: TrackerStage): string {
  if (s.state === 'locked') return 'strip-off'
  if (s.result === 'failed') return 'strip-fail'
  if (s.result === 'winner' || s.result === 'prizer') return 'strip-gold'
  if (s.result === 'passed' || (s.result === null && s.registered)) return 'strip-done'
  if (s.asking) return 'strip-ask'
  if (s.state === 'current') return 'strip-now'
  return ''
}

function stripCaption(s: TrackerStage, t: Voice): string {
  if (s.result) return t(`tracker.mark.${s.result}` as TextKey)
  return stageDate(s) || (s.registered ? t('tracker.mark.registered') : '')
}

/** Полоска этапов со сроками — вариант B макета E1. */
function StageStrip({ stages, labels }: { stages: TrackerStage[]; labels: string[] }) {
  const t = useVoice()
  return (
    <ol className={`strip${stages.length > 3 ? ' strip-grid' : ''}`} aria-label="Этапы">
      {stages.map((s, i) => {
        const caption = stripCaption(s, t)
        const done = stripTone(s) === 'strip-done' || stripTone(s) === 'strip-gold'
        return (
          <li key={s.id} className={`strip-item ${stripTone(s)}`} aria-label={`${labels[i]}: ${caption}`}>
            <span className="strip-label">
              {done ? <Icon name="check" size={10} strokeWidth={3.2} /> : null}
              <b>{labels[i]}</b>
            </span>
            <span className="strip-caption">{caption}</span>
          </li>
        )
      })}
    </ol>
  )
}

/** Кнопки итога: выбранный нажат, повторное нажатие снимает отметку. */
function ResultButtons({ stage, onMark, disabled }: { stage: TrackerStage; onMark: Mark; disabled: boolean }) {
  const t = useVoice()
  return (
    <div className="result-buttons">
      {stage.results.map((r) => {
        const chosen = stage.result === r
        return (
          <button
            key={r}
            type="button"
            className={`result-button${r === 'failed' || r === 'participant' ? ' result-button-quiet' : ''}`}
            aria-pressed={chosen}
            disabled={disabled || (!chosen && !stage.results_allowed.includes(r))}
            onClick={() => onMark(stage.id, stage.registered, chosen ? null : r)}
          >
            {t(`tracker.result.${r}` as TextKey)}
          </button>
        )
      })}
    </div>
  )
}

/** Галочка регистрации: первая — «Регистрация пройдена», без этапа — «Участвую». */
function RegisterCheck({
  item,
  stage,
  onMark,
  disabled,
}: {
  item: TrackerItem
  stage: TrackerStage | null
  onMark: Mark
  disabled: boolean
}) {
  const t = useVoice()
  const first = item.stages.find((s) => registrationLike(s.kind))
  const on = stage ? stage.registered : Boolean(item.registered_at)
  const label = !stage
    ? t('tracker.participatingCta')
    : stage.id === first?.id
      ? t('tracker.registeredCta')
      : t('tracker.registeredNextCta')
  return (
    <button
      type="button"
      className="tracker-check"
      role="checkbox"
      aria-checked={on}
      disabled={disabled}
      onClick={() => onMark(stage?.id ?? null, !on, stage?.result ?? null)}
    >
      <span className={`checkbox${on ? ' checkbox-on' : ''}`}>
        {on ? <Icon name="check" size={12} strokeWidth={3} /> : null}
      </span>
      {label}
      {/* Колокольчик — напоминания об этом сроке идут; имени галочки не мешает. */}
      <em className="tracker-check-author">
        <Icon name="bell" size={13} />
      </em>
    </button>
  )
}

/** Строка действия: что отметить сейчас — регистрацию или итог этапа. */
function ActionRow({ item, onMark, disabled }: { item: TrackerItem; onMark: Mark; disabled: boolean }) {
  const t = useVoice()
  const action = item.action
  if (!action) return null
  const stage = item.stages.find((s) => s.id === action.stage_id) ?? null
  if (action.type === 'register') return <RegisterCheck item={item} stage={stage} onMark={onMark} disabled={disabled} />
  if (!stage) return null
  return (
    <div className="tracker-ask">
      <p className="tracker-ask-question">
        {t('tracker.askResult', { stage: t(`tracker.stageKind.${stage.kind}` as TextKey) })}
      </p>
      <ResultButtons stage={stage} onMark={onMark} disabled={disabled} />
    </div>
  )
}

/** Все этапы с отметками — раскрытая карточка (E4). */
function StageList({ item, onMark, disabled }: { item: TrackerItem; onMark: Mark; disabled: boolean }) {
  const t = useVoice()
  const first = item.stages.find((s) => registrationLike(s.kind))
  return (
    <ol className="stage-marks">
      {item.stages.map((s) => {
        const tone = s.state === 'locked' ? 'off' : s.asking ? 'ask' : s.state
        // Итог — у этапа, который идёт или прошёл. Этапы впереди кнопок не
        // показывают, даже если дат нет и сервер отметку бы принял.
        const open = s.state !== 'locked' && s.state !== 'future'
        const notYet = s.results.length > 0 && !s.result && s.state !== 'locked' && (!open || s.results_allowed.length === 0)
        const subtitle = [s.subtitle, notYet ? t('tracker.resultLater') : null].filter(Boolean).join(' · ')
        return (
          <li key={s.id} className={`stage-mark stage-mark-${tone}`}>
            <span className="stage-mark-dot" aria-hidden />
            <div className="stage-mark-main">
              <b>{s.title}</b>
              {subtitle ? <span>{subtitle}</span> : null}
              {registrationLike(s.kind) && s.registered ? (
                <span className="stage-mark-chip">
                  <Icon name="check" size={11} strokeWidth={3} />
                  {t('tracker.mark.registered')}
                  {s.id === first?.id && item.registered_by
                    ? ` · ${t('tracker.markedByInline', { name: item.registered_by.name })}`
                    : null}
                </span>
              ) : null}
              {s.can_register ? (
                <button
                  type="button"
                  className="stage-mark-link"
                  disabled={disabled}
                  onClick={() => onMark(s.id, !s.registered, s.result)}
                >
                  {s.registered ? t('tracker.unmark') : t('tracker.markRegistration')}
                </button>
              ) : null}
              {open && (s.result || s.results_allowed.length > 0) ? (
                <ResultButtons stage={s} onMark={onMark} disabled={disabled} />
              ) : null}
            </div>
          </li>
        )
      })}
    </ol>
  )
}

function Subtitle({ item }: { item: TrackerItem }) {
  const t = useVoice()
  if (item.status === 'finished') {
    const closing = item.stages.find((s) => s.result === 'failed')
    const stage = closing ? t(`tracker.stageKind.${closing.kind}` as TextKey) : ''
    return <>{t(`tracker.outcome.${item.outcome ?? 'unknown'}` as TextKey, { stage })}</>
  }
  const date = formatDay(item.deadline_at)
  if (item.status === 'open') {
    if (item.kind === 'vsosh' && date) return <>{t('tracker.stageOn', { stage: item.next_stage_title ?? 'Этап', date })}</>
    return <>{date ? t('tracker.deadlineUntil', { date }) : (item.next_stage_title ?? '')}</>
  }
  if (!item.next_stage_title) return <>{t('tracker.waitNext')}</>
  return <>{t('tracker.nextStage', { stage: date ? `${item.next_stage_title} ${date}` : item.next_stage_title })}</>
}

function StatusPill({ item }: { item: TrackerItem }) {
  const t = useVoice()
  if (item.status === 'finished') {
    if (item.outcome === 'winner' || item.outcome === 'prizer') {
      return (
        <span className="pill pill-done">
          <Icon name="award" size={11} />
          {t(`tracker.mark.${item.outcome}`)}
        </span>
      )
    }
    return <span className="pill pill-ok">{t('tracker.pill.finished')}</span>
  }
  if (item.action?.type === 'result') return <span className="pill pill-soon">{t('tracker.pill.asking')}</span>
  return <Pill deadlineAt={item.deadline_at} doneLabel={t('pill.done')} />
}

/** Карточка трекера: сроки этапов и отметки — F46, F63. */
function TrackerCard({
  item,
  onOpen,
  canMark,
  canRemove,
}: {
  item: TrackerItem
  onOpen: (id: string) => void
  canMark: boolean
  canRemove: boolean
}) {
  const t = useVoice()
  const [expanded, setExpanded] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const mark = useSetStageMark()
  const remove = useRemoveFromTracker()
  const labels = shortLabels(item.stages, t)
  const onMark: Mark = (stageId, registered, result) => mark.mutate({ item, stageId, registered, result })
  const disabled = !canMark || mark.isPending
  const diploma = item.outcome === 'winner' || item.outcome === 'prizer'

  return (
    <article className="tracker-card" data-tour="tracker-item">
      <button type="button" className="tracker-card-top" onClick={() => onOpen(item.olympiad_profile_id)}>
        <Tile id={item.olympiad_id} name={item.olympiad_name} shortName={item.short_name} color={item.color} />
        <span className="row-main">
          <span className="row-title">{item.olympiad_name}</span>
          <span className="row-subtitle">
            <Subtitle item={item} />
          </span>
        </span>
        <StatusPill item={item} />
      </button>

      {expanded ? (
        <div className="tracker-expanded">
          <StageList item={item} onMark={onMark} disabled={disabled} />
          {item.outcome && item.outcome !== 'missed' && item.outcome !== 'unknown' ? (
            <p className="tracker-note">{t('tracker.undoHint')}</p>
          ) : null}
          {canRemove && !confirming ? (
            <button type="button" className="tracker-remove" onClick={() => setConfirming(true)}>
              <Icon name="trash" size={14} />
              {t('tracker.remove')}
            </button>
          ) : null}
          {canRemove && confirming ? (
            <div className="tracker-confirm" role="group">
              <p>{t('tracker.removeConfirm', { title: item.olympiad_name })}</p>
              <div className="tracker-confirm-actions">
                <Button stretched variant="secondary" disabled={remove.isPending} onClick={() => setConfirming(false)}>
                  {t('tracker.removeNo')}
                </Button>
                <Button stretched loading={remove.isPending} onClick={() => remove.mutate(item.id)}>
                  {t('tracker.removeYes')}
                </Button>
              </div>
            </div>
          ) : null}
        </div>
      ) : (
        <>
          {item.stages.length > 0 ? <StageStrip stages={item.stages} labels={labels} /> : null}
          {canMark ? <ActionRow item={item} onMark={onMark} disabled={disabled} /> : null}
          {diploma ? (
            <button type="button" className="tracker-diploma" onClick={() => onOpen(item.olympiad_profile_id)}>
              <Icon name="award" size={16} />
              {t('tracker.diplomaBenefits')}
              <Icon name="chevron" size={14} />
            </button>
          ) : null}
        </>
      )}

      <button
        type="button"
        className="tracker-more"
        aria-expanded={expanded}
        onClick={() => {
          setExpanded(!expanded)
          setConfirming(false)
        }}
      >
        {expanded ? t('tracker.hideStages') : t('tracker.showStages')}
        <Icon name="chevron" size={14} className={expanded ? 'tracker-more-up' : 'tracker-more-down'} />
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
  const link = useCalendarLink(view === 'calendar')
  const resolve = useResolveProposal()

  const openOlympiad = (id: string) => sheets.open({ kind: 'oly', id })

  /**
   * Файл календаря открывает браузер телефона: iPhone предложит добавить все
   * сроки в «Календарь», Android — импортировать в Google Календарь. Пропуск
   * берётся заранее, иначе MAX не откроет ссылку; устарел — берём новый.
   */
  const exportCalendar = () => {
    const open = (l: CalendarLink) => getWebApp().openLink(apiUrl(l.url))
    if (link.data && Date.parse(link.data.expires_at) - Date.now() > 60_000) return open(link.data)
    link
      .refetch({ throwOnError: true })
      .then((r) => r.data && open(r.data))
      .catch(showErrorToast)
  }
  const canResolve = session?.permissions.resolve_proposals ?? false
  const canMark = session?.permissions.toggle_registered ?? false
  const canRemove = session?.permissions.remove_from_tracker ?? false

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
        <button type="button" className="calendar-export" data-tour="calendar-export" onClick={exportCalendar}>
          <Icon name="calendar" size={18} />
          {t('calendar.export')}
        </button>
        <CalendarView month={month} data={calendar.data} onMonthChange={setPickedMonth} onOpen={openOlympiad} />
      </div>
    )
  }

  const { open, active, finished } = groupTracker(sortByDeadline(items))
  const groups = [
    { key: 'open', title: t('tracker.groupOpen'), items: open },
    { key: 'active', title: t('tracker.groupActive'), items: active },
    { key: 'finished', title: t('tracker.groupFinished'), items: finished },
  ]

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

      {/* Один плоский список: карточка, перешедшая после отметки в другую
          группу, остаётся той же — раскрытой, с подсказкой «можно снять». */}
      {groups.flatMap((group) =>
        group.items.length > 0
          ? [
              <p key={`title-${group.key}`} className="week-title">
                {group.title}: {group.items.length}
              </p>,
              ...group.items.map((item) => (
                <TrackerCard key={item.id} item={item} onOpen={openOlympiad} canMark={canMark} canRemove={canRemove} />
              )),
            ]
          : [],
      )}

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
