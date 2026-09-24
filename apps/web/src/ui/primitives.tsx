/**
 * Мелкие элементы, которых нет в MAX UI: плашки сроков, метки источника,
 * плитки, чипы, скелетоны, пустые и ошибочные состояния, нижний лист.
 *
 * MAX UI закрывает кнопки, поля, переключатели и аватары — их берём оттуда.
 * Всё остальное в прототипе своё, и повторять его чужими компонентами дороже,
 * чем описать.
 */

import type { ReactNode } from 'react'
import { badgeColor, badgeShortName } from '@/lib/badge'
import { daysLabel, daysLeft, deadlineTone } from '@/lib/deadline'
import { Icon, type IconName } from './Icon'

// --- Плашка срока ------------------------------------------------------------

export interface PillProps {
  deadlineAt: string | null | undefined
  registered?: boolean
  doneLabel?: string
}

/** Цвет и подпись срока. Пороги — ТЗ §7.3, считает lib/deadline. */
export function Pill({ deadlineAt, registered = false, doneLabel = 'готово' }: PillProps) {
  const days = daysLeft(deadlineAt)
  const tone = deadlineTone(days, registered)
  if (!tone) return null

  if (tone === 'done') {
    return (
      <span className="pill pill-done">
        <Icon name="check" size={11} strokeWidth={3} />
        {doneLabel}
      </span>
    )
  }

  const label = days === null ? '' : days < 0 ? 'срок прошёл' : days === 0 ? 'сегодня' : daysLabel(days)
  return <span className={`pill pill-${tone}`}>{label}</span>
}

// --- Метка источника ---------------------------------------------------------

export type SourceKind = 'fact' | 'recommendation' | 'demo'

const SOURCE_LABELS: Record<SourceKind, string> = {
  fact: 'Факт',
  recommendation: 'Рекомендация',
  demo: 'Демо-даты',
}

/** «Факт», «Рекомендация», «Демо-даты» — ТЗ §7.3. */
export function SourceTag({ kind }: { kind: SourceKind }) {
  return <span className={`tag tag-${kind}`}>{SOURCE_LABELS[kind]}</span>
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
  return (
    <span
      className={`tile tile-${size}${filled ? ' tile-filled' : ''}${label.length >= 4 ? ' tile-long' : ''}`}
      style={{ '--tile-hue': hue } as React.CSSProperties}
      aria-hidden="true"
    >
      {label}
    </span>
  )
}

// --- Чип ---------------------------------------------------------------------

export interface ChipProps {
  active?: boolean
  onClick?: () => void
  children: ReactNode
}

export function Chip({ active = false, onClick, children }: ChipProps) {
  return (
    <button type="button" className={`chip${active ? ' chip-on' : ''}`} aria-pressed={active} onClick={onClick}>
      {children}
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
