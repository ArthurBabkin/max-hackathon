/** Календарь сроков — экран E2, функция F32. */

import { useMemo } from 'react'
import type { CalendarMonth } from '@contract'
import { formatMonthTitle } from '@/lib/deadline'
import { badgeColor } from '@/lib/badge'
import { Icon } from '@/ui/Icon'
import { TrackerRow } from '@/ui/TrackerRow'
import { useVoice } from '@/voice/useVoice'

const WEEKDAYS = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']

/** Сдвиг месяца в формате `YYYY-MM`. */
export function shiftMonth(month: string, delta: number): string {
  const [year, index] = month.split('-').map(Number)
  const date = new Date(year!, index! - 1 + delta, 1)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`
}

/** Сколько пустых клеток перед первым числом: неделя начинается с понедельника. */
export function leadingBlanks(month: string): number {
  const [year, index] = month.split('-').map(Number)
  const weekday = new Date(year!, index! - 1, 1).getDay()
  return (weekday + 6) % 7
}

export function daysInMonth(month: string): number {
  const [year, index] = month.split('-').map(Number)
  return new Date(year!, index!, 0).getDate()
}

export interface CalendarViewProps {
  month: string
  data: CalendarMonth | undefined
  onMonthChange: (month: string) => void
  onOpen: (olympiadProfileId: string) => void
}

export function CalendarView({ month, data, onMonthChange, onOpen }: CalendarViewProps) {
  const t = useVoice()
  const today = new Date().toISOString().slice(0, 10)

  const byDay = useMemo(() => {
    const map = new Map<number, CalendarMonth['days'][number]['items']>()
    for (const day of data?.days ?? []) map.set(Number(day.date.slice(8, 10)), day.items)
    return map
  }, [data])

  const blanks = leadingBlanks(month)
  const total = daysInMonth(month)
  const monthItems = (data?.days ?? []).flatMap((day) => day.items)

  return (
    <>
      <div className="calendar">
        <div className="calendar-head">
          <b>{formatMonthTitle(month)}</b>
          <span>
            <button type="button" onClick={() => onMonthChange(shiftMonth(month, -1))}>
              <Icon name="back" size={16} title="Предыдущий месяц" />
            </button>
            <button type="button" onClick={() => onMonthChange(shiftMonth(month, 1))}>
              <Icon name="chevron" size={16} title="Следующий месяц" />
            </button>
          </span>
        </div>

        <div className="calendar-grid">
          {WEEKDAYS.map((day) => (
            <span key={day} className="calendar-weekday">
              {day}
            </span>
          ))}
          {Array.from({ length: blanks }, (_, i) => (
            <span key={`blank-${i}`} />
          ))}
          {Array.from({ length: total }, (_, i) => {
            const day = i + 1
            const date = `${month}-${String(day).padStart(2, '0')}`
            const items = byDay.get(day)
            const isToday = date === today
            const isPast = date < today

            if (!items || items.length === 0) {
              return (
                <span key={day} className={`calendar-day${isToday ? ' calendar-day-today' : ''}${isPast ? ' calendar-day-past' : ''}`}>
                  {day}
                </span>
              )
            }

            const first = items[0]!
            return (
              <button
                key={day}
                type="button"
                className="calendar-day calendar-day-event"
                style={{ '--day-hue': badgeColor(first.olympiad_id, first.color) } as React.CSSProperties}
                onClick={() => onOpen(first.olympiad_profile_id)}
              >
                {day}
                <i aria-hidden="true" />
                <span className="sr-only">
                  : {items.map((item) => item.olympiad_name).join(', ')}
                </span>
              </button>
            )
          })}
        </div>
      </div>

      <p className="week-title">{t('tracker.calendarMonthTitle', { month: formatMonthTitle(month, 'in') })}</p>
      {monthItems.length > 0 ? (
        <div className="list">
          {/* У одного пункта в месяце может быть несколько сроков — ключ из пункта и срока. */}
          {monthItems.map((item) => (
            <TrackerRow key={`${item.id}-${item.deadline_at}`} item={item} onOpen={onOpen} />
          ))}
        </div>
      ) : (
        <div className="list list-empty">
          <p className="state-text">{t('tracker.calendarEmpty')}</p>
        </div>
      )}
    </>
  )
}
