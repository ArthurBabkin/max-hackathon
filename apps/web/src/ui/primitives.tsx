/**
 * Мелкие элементы, которых нет в MAX UI: плашки сроков, метки источника,
 * плитки, чипы, скелетоны, пустые и ошибочные состояния, нижний лист.
 *
 * MAX UI закрывает кнопки, поля, переключатели и аватары — их берём оттуда.
 * Всё остальное в прототипе своё, и повторять его чужими компонентами дороже,
 * чем описать.
 */

import { useRef, useState, type ReactNode } from 'react'
import { flushSync } from 'react-dom'
import type { BenefitKind } from '@contract'
import { badgeColor, badgeLogo, badgeShortName } from '@/lib/badge'
import { splitBenefitLabel } from '@/lib/benefits'
import { daysLabel, daysLeft, deadlineTone } from '@/lib/deadline'
import { Icon, type IconName } from './Icon'
import { useVoice } from '@/voice/useVoice'

// --- Плашка срока ------------------------------------------------------------

export interface PillProps {
  deadlineAt: string | null | undefined
  registered?: boolean
  doneLabel?: string
}

/** Цвет и подпись срока. Пороги — ТЗ §7.3, считает lib/deadline. */
export function Pill({ deadlineAt, registered = false, doneLabel }: PillProps) {
  const t = useVoice()
  const days = daysLeft(deadlineAt)
  const tone = deadlineTone(days, registered)
  if (!tone) return null

  if (tone === 'done') {
    return (
      <span className="pill pill-done">
        <Icon name="check" size={11} strokeWidth={3} />
        {doneLabel ?? t('pill.done')}
      </span>
    )
  }

  const label = days === null ? '' : days < 0 ? t('pill.overdue') : days === 0 ? t('pill.today') : daysLabel(days)
  return <span className={`pill pill-${tone}`}>{label}</span>
}

// --- Метка источника ---------------------------------------------------------

export type SourceKind = 'fact' | 'recommendation' | 'demo'

/** «Факт», «Рекомендация», «Демо-даты» — ТЗ §7.3. */
export function SourceTag({ kind }: { kind: SourceKind }) {
  const t = useVoice()
  return <span className={`tag tag-${kind}`}>{t(`tag.${kind}`)}</span>
}

// --- Плашка льготы -----------------------------------------------------------

const BENEFIT_TONE: Record<BenefitKind, string> = {
  bvi: 'benefit-bvi',
  bvi_winners: 'benefit-bvi',
  score100: 'benefit-score',
  extra_points: 'benefit-extra',
}

/**
 * Льгота: в плашке — вид, под ней — уточнение («100 баллов» и «по физике или
 * химии»). Целиком в плашке длинная подпись сжимала название вуза в строке
 * до слова и вылезала из узкого столбца таблицы.
 */
export function BenefitValue({ kind, label }: { kind: BenefitKind | null | undefined; label: string }) {
  const { main, detail } = splitBenefitLabel(kind, label)
  return (
    <span className="benefit-grant">
      <span className={`benefit-value ${kind ? BENEFIT_TONE[kind] : 'benefit-none'}`}>{main}</span>
      {detail ? <span className="benefit-detail">{detail}</span> : null}
    </span>
  )
}

// --- Плитка ------------------------------------------------------------------

export interface TileProps {
  id: string
  name: string
  shortName?: string | null
  color?: string | null
  size?: 'sm' | 'md' | 'lg'
  filled?: boolean
}

export function Tile({ id, name, shortName, color, size = 'sm', filled = false }: TileProps) {
  const hue = badgeColor(id, color)
  const label = badgeShortName(name, shortName)
  const logo = badgeLogo(id)
  const [logoFailed, setLogoFailed] = useState(false)
  const showLogo = logo && !logoFailed
  return (
    <span
      className={`tile tile-${size}${filled ? ' tile-filled' : ''}${showLogo ? ' tile-logo' : ''}`}
      style={{ '--tile-hue': hue, '--tile-chars': label.length } as React.CSSProperties}
      aria-hidden="true"
    >
      {showLogo ? <img src={logo} alt="" loading="lazy" onError={() => setLogoFailed(true)} /> : label}
    </span>
  )
}

// --- Чип ---------------------------------------------------------------------

export interface ChipProps {
  active?: boolean
  onClick?: () => void
  /** Подпись для скринридера, если текст чипа её не передаёт («Казань ✕»). */
  'aria-label'?: string
  children: ReactNode
}

export function Chip({ active = false, onClick, 'aria-label': label, children }: ChipProps) {
  return (
    <button
      type="button"
      className={`chip${active ? ' chip-on' : ''}`}
      aria-pressed={active}
      aria-label={label}
      onClick={onClick}
    >
      {children}
    </button>
  )
}

// --- Раскрыть список ---------------------------------------------------------

/**
 * «Все профили (40)» и «Свернуть» под длинным списком. Свернули — кнопка
 * остаётся на экране: без этого после длинного списка страница уезжала бы
 * вниз, в Safari на iOS 15 прокрутка за содержимым не следит.
 */
export function ListToggle({ expanded, count, onToggle }: { expanded: boolean; count: number; onToggle: () => void }) {
  const t = useVoice()
  const ref = useRef<HTMLButtonElement>(null)
  return (
    <button
      ref={ref}
      type="button"
      className="list-toggle"
      aria-expanded={expanded}
      onClick={() => {
        if (!expanded) return onToggle()
        // Сначала список сворачивается, потом кнопка возвращается на экран.
        flushSync(onToggle)
        ref.current?.scrollIntoView?.({ block: 'nearest' })
      }}
    >
      {expanded ? t('olympiad.profilesCollapse') : t('olympiad.allProfiles', { count })}
      <Icon name="chevron" size={13} className={expanded ? 'list-toggle-up' : 'list-toggle-down'} />
    </button>
  )
}

// --- Скелетон ----------------------------------------------------------------

export function Skeleton({ width, height = 12, radius = 9 }: { width?: string | number; height?: number; radius?: number }) {
  return (
    <span
      className="skeleton"
      style={{ width: width ?? '100%', height, borderRadius: radius }}
      aria-hidden="true"
    />
  )
}

/** Заглушка списка карточек, пока идёт загрузка — экран H2. */
export function CardSkeletons({ count = 3 }: { count?: number }) {
  return (
    <div aria-busy="true" aria-live="polite">
      {Array.from({ length: count }, (_, i) => (
        <div className="card-skeleton" key={i}>
          <Skeleton width={42} height={42} radius={13} />
          <span className="card-skeleton-lines">
            <Skeleton width="68%" height={12} />
            <Skeleton width="48%" height={10} />
            <Skeleton width="82%" height={18} />
          </span>
        </div>
      ))}
    </div>
  )
}

// --- Пустые и ошибочные состояния --------------------------------------------

export interface StateBlockProps {
  icon: IconName
  tone?: 'neutral' | 'error' | 'ok'
  title: string
  text?: string
  children?: ReactNode
}

/** Общая рамка экранов H3 и H4: что случилось и что делать дальше. */
export function StateBlock({ icon, tone = 'neutral', title, text, children }: StateBlockProps) {
  return (
    <div className="state" role="status">
      <span className={`state-icon state-icon-${tone}`}>
        <Icon name={icon} size={30} />
      </span>
      <p className="state-title">{title}</p>
      {text ? <p className="state-text">{text}</p> : null}
      {children ? <div className="state-actions">{children}</div> : null}
    </div>
  )
}

// --- Заголовок раздела -------------------------------------------------------

export function Section({ title, action }: { title: string; action?: ReactNode }) {
  return (
    <div className="section-head">
      <h2 className="section-title">{title}</h2>
      {action}
    </div>
  )
}

// --- Подсказка и сноска ------------------------------------------------------

export function Hint({ icon = 'bell', children }: { icon?: IconName; children: ReactNode }) {
  return (
    <p className="hint">
      <Icon name={icon} size={15} />
      <span>{children}</span>
    </p>
  )
}

export function Note({ icon = 'shield', children }: { icon?: IconName; children: ReactNode }) {
  return (
    <p className="note">
      <Icon name={icon} size={13} />
      <span>{children}</span>
    </p>
  )
}

/** Строка источника под блоком факта: «Перечень олимпиад, приказ № 669». */
export function SourceLine({ title, url }: { title: string; url?: string | null }) {
  const content = (
    <>
      <Icon name="doc" size={13} />
      <span>{title}</span>
    </>
  )
  if (!url) return <p className="source-line">{content}</p>
  return (
    <a className="source-line" href={url} target="_blank" rel="noopener noreferrer">
      {content}
    </a>
  )
}
