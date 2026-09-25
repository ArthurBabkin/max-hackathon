/** Производные величины экранов. Без React и без запросов — поэтому под тестом. */

import type { Proposal, Role, TrackerItem } from '@contract'

interface TrackerData {
  items: TrackerItem[]
  proposals: Proposal[]
}

/**
 * Цифра на вкладке «Трекер» (F31): пункты «нужно зарегистрироваться» плюс
 * предложения, ждущие ответа. Закрывшаяся без отметки регистрация не
 * считается: сделать с ней сегодня уже нечего.
 *
 * У родителя предложения не учитываются: ответить на них он не может
 * (ТЗ §3.1), и цифра обещала бы действие, которого у него нет.
 */
export function trackerBadgeCount(tracker: TrackerData | undefined, role: Role): number {
  if (!tracker) return 0
  const open = tracker.items.filter((i) => i.status === 'open').length
  const pending = role === 'kid' ? tracker.proposals.filter((p) => p.status === 'pending').length : 0
  return open + pending
}

/**
 * Три группы трекера (E1): нужно зарегистрироваться, участвую, завершено.
 * В «участвую» первыми идут пункты, которые ждут отметки итога, — порядок
 * внутри остаётся прежним.
 */
export function groupTracker(items: TrackerItem[]): {
  open: TrackerItem[]
  active: TrackerItem[]
  finished: TrackerItem[]
} {
  const asks = (i: TrackerItem) => i.action?.type === 'result'
  const active = items.filter((i) => i.status === 'active')
  return {
    open: items.filter((i) => i.status === 'open'),
    active: [...active.filter(asks), ...active.filter((i) => !asks(i))],
    finished: items.filter((i) => i.status === 'finished'),
  }
}

/** Ближайший срок первым; пункты без срока — в конце, а не в начале. */
export function sortByDeadline<T extends { deadline_at: string | null }>(items: T[]): T[] {
  return [...items].sort((a, b) => {
    if (a.deadline_at === b.deadline_at) return 0
    if (!a.deadline_at) return 1
    if (!b.deadline_at) return -1
    return a.deadline_at.localeCompare(b.deadline_at)
  })
}
