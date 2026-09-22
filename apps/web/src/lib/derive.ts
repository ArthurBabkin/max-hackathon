/** Производные величины экранов. Без React и без запросов — поэтому под тестом. */

import type { Proposal, Role, TrackerItem } from '@contract'

interface TrackerData {
  items: TrackerItem[]
  proposals: Proposal[]
}

/**
 * Цифра на вкладке «Трекер» (F31): незарегистрированные пункты плюс
 * предложения, ждущие ответа.
 *
 * У родителя предложения не учитываются: ответить на них он не может
 * (ТЗ §3.1), и цифра обещала бы действие, которого у него нет.
 */
export function trackerBadgeCount(tracker: TrackerData | undefined, role: Role): number {
  if (!tracker) return 0
  const unregistered = tracker.items.filter((i) => !i.registered_at).length
  const pending = role === 'kid' ? tracker.proposals.filter((p) => p.status === 'pending').length : 0
  return unregistered + pending
}

/** Две группы трекера из ТЗ E1. */
export function groupTracker(items: TrackerItem[]): { open: TrackerItem[]; done: TrackerItem[] } {
  return {
    open: items.filter((i) => !i.registered_at),
    done: items.filter((i) => i.registered_at),
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
