import { describe, expect, it } from 'vitest'
import { groupTracker, sortByDeadline, trackerBadgeCount } from './derive'
import type { Proposal, TrackerItem } from '@contract'

const item = (id: string, over: Partial<TrackerItem> = {}): TrackerItem =>
  ({
    id,
    olympiad_profile_id: `${id}:inf`,
    olympiad_id: id,
    olympiad_name: id,
    kind: 'perechen',
    level: 'I',
    deadline_at: null,
    next_stage_title: 'Регистрация',
    registered_at: null,
    registered_by: null,
    added_by: null,
    ...over,
  }) as TrackerItem

const proposal = (id: string): Proposal =>
  ({
    id,
    olympiad_profile_id: `${id}:inf`,
    olympiad_id: id,
    olympiad_name: id,
    status: 'pending',
    proposed_by: { id: 'm', name: 'Ольга', role: 'parent' },
    created_at: '',
    resolved_at: null,
    deadline_at: null,
  }) as Proposal

describe('trackerBadgeCount — счётчик на вкладке (F31)', () => {
  const tracker = {
    items: [item('a'), item('b', { registered_at: '2026-09-01T00:00:00Z' })],
    proposals: [proposal('p1')],
  }

  it('ученику: незарегистрированные плюс ждущие ответа предложения', () => {
    expect(trackerBadgeCount(tracker, 'kid')).toBe(2)
  })

  it('родителю предложения не считаются — отвечать на них он не может', () => {
    expect(trackerBadgeCount(tracker, 'parent')).toBe(1)
  })

  it('всё отмечено и предложений нет — счётчика нет', () => {
    expect(
      trackerBadgeCount(
        { items: [item('b', { registered_at: '2026-09-01T00:00:00Z' })], proposals: [] },
        'kid',
      ),
    ).toBe(0)
  })

  it('пустой трекер не ломает счёт', () => {
    expect(trackerBadgeCount(undefined, 'kid')).toBe(0)
  })
})

describe('groupTracker', () => {
  it('делит на «нужно зарегистрироваться» и «зарегистрирован»', () => {
    const { open, done } = groupTracker([
      item('a'),
      item('b', { registered_at: '2026-09-01T00:00:00Z' }),
      item('c'),
    ])
    expect(open.map((i) => i.id)).toEqual(['a', 'c'])
    expect(done.map((i) => i.id)).toEqual(['b'])
  })
})

describe('sortByDeadline', () => {
  it('ближайший срок первым', () => {
    const sorted = sortByDeadline([
      item('поздний', { deadline_at: '2026-10-30T00:00:00Z' }),
      item('ранний', { deadline_at: '2026-09-25T00:00:00Z' }),
    ])
    expect(sorted.map((i) => i.id)).toEqual(['ранний', 'поздний'])
  })

  it('пункты без срока уходят в конец, а не в начало', () => {
    const sorted = sortByDeadline([
      item('без срока', { deadline_at: null }),
      item('со сроком', { deadline_at: '2026-10-30T00:00:00Z' }),
    ])
    expect(sorted.map((i) => i.id)).toEqual(['со сроком', 'без срока'])
  })

  it('не меняет исходный массив', () => {
    const source = [item('b', { deadline_at: '2026-10-01T00:00:00Z' }), item('a', { deadline_at: '2026-09-01T00:00:00Z' })]
    sortByDeadline(source)
    expect(source.map((i) => i.id)).toEqual(['b', 'a'])
  })
})
