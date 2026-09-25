/**
 * Отметки этапов на демо-стенде — те же правила, что в packages/core/stages
 * (progress.go): итоги по типу этапа, закрывающий итог, 400 и 409. Только для
 * разработки: экран трекера должен проходить сценарий целиком и без сервера.
 */

import type { StageResult, TrackerAction, TrackerItem, TrackerStage } from '@contract'

/** Этап демо-олимпиады с датами — как его отдаёт сборка ответов. */
export interface MockStage {
  id: string
  kind: TrackerStage['kind']
  title: string
  subtitle: string | null
  starts_at: string | null
  ends_at: string | null
  deadline_at: string | null
}

export interface MockMark {
  registered: boolean
  result: StageResult | null
}

/** Первая регистрация — в registered_at пункта, остальное — по этапам. */
export interface MockProgress {
  registered: boolean
  marks: Record<string, MockMark>
}

export class MarkError extends Error {
  constructor(readonly status: 400 | 404 | 409) {
    super(`отметка этапа: ${status}`)
  }
}

const registrationLike = (kind: string) => kind === 'registration' || kind === 'school'
const time = (iso: string | null) => (iso ? Date.parse(iso) : null)
const markOf = (p: MockProgress, s: MockStage): MockMark => p.marks[s.id] ?? { registered: false, result: null }

export const firstRegistration = (st: MockStage[]) => st.findIndex((s) => registrationLike(s.kind))

function registeredOn(st: MockStage[], p: MockProgress, i: number): boolean {
  if (i === firstRegistration(st)) return p.registered
  return registrationLike(st[i]!.kind) && markOf(p, st[i]!).registered
}

function results(st: MockStage[], i: number): StageResult[] {
  if (st[i]!.kind === 'registration') return []
  if (st[i]!.kind === 'final' && !st.slice(i + 1).some((s) => s.kind === 'final')) {
    return ['winner', 'prizer', 'participant']
  }
  return ['passed', 'failed']
}

const closing = (r: StageResult | null) => r !== null && r !== 'passed'
const closedAt = (st: MockStage[], p: MockProgress) => st.findIndex((s) => closing(markOf(p, s).result))
const locked = (st: MockStage[], p: MockProgress, i: number) => {
  const c = closedAt(st, p)
  return c >= 0 && i > c
}
const done = (st: MockStage[], p: MockProgress, i: number) =>
  markOf(p, st[i]!).result !== null || (registrationLike(st[i]!.kind) && registeredOn(st, p, i))

const passed = (s: MockStage, now: number) => {
  const d = time(s.deadline_at)
  return d !== null && d < now
}
const end = (s: MockStage) => time(s.ends_at) ?? time(s.deadline_at) ?? time(s.starts_at)
const started = (s: MockStage, now: number) => {
  const at = time(s.starts_at)
  return at === null || at <= now
}

function current(st: MockStage[], p: MockProgress, now: number): number {
  if (closedAt(st, p) >= 0) return -1
  return st.findIndex((s, i) => !done(st, p, i) && !passed(s, now))
}

function needsResult(st: MockStage[], p: MockProgress, i: number): boolean {
  if (!p.registered || results(st, i).length === 0 || closedAt(st, p) >= 0) return false
  return !st.slice(i).some((s) => markOf(p, s).result !== null)
}

function asking(st: MockStage[], p: MockProgress, now: number): number {
  for (let i = st.length - 1; i >= 0; i--) {
    const e = end(st[i]!)
    if (e !== null && e < now && needsResult(st, p, i)) return i
  }
  return -1
}

function status(st: MockStage[], p: MockProgress, now: number): Pick<TrackerItem, 'status' | 'outcome'> {
  const c = closedAt(st, p)
  if (c >= 0) return { status: 'finished', outcome: markOf(p, st[c]!).result as TrackerItem['outcome'] }
  if (!p.registered) {
    const first = firstRegistration(st)
    const from = first < 0 ? st : st.slice(first)
    return from.length === 0 || !passed(from[0]!, now)
      ? { status: 'open', outcome: null }
      : { status: 'finished', outcome: 'missed' }
  }
  if (current(st, p, now) >= 0) return { status: 'active', outcome: null }
  if (st.length > 0 && markOf(p, st[st.length - 1]!).result === 'passed') return { status: 'active', outcome: null }
  return { status: 'finished', outcome: 'unknown' }
}

/** Ставит отметку; ошибки — как у сервера: 404, 400, 409. */
export function apply(st: MockStage[], p: MockProgress, stageId: string, m: MockMark, now: number): MockProgress {
  const i = st.findIndex((s) => s.id === stageId)
  if (i < 0) throw new MarkError(404)
  if (m.result !== null && (!results(st, i).includes(m.result) || !started(st[i]!, now))) throw new MarkError(400)
  if (locked(st, p, i)) throw new MarkError(409)
  const later = st.slice(i + 1).some((s) => markOf(p, s).registered || markOf(p, s).result !== null)
  if (closing(m.result) && later) throw new MarkError(409)
  const first = firstRegistration(st)
  let registered = registrationLike(st[i]!.kind) && m.registered
  if (registeredOn(st, p, i) && !registered && m.result === null) {
    const anyResult = st.some((s) => s.id !== stageId && markOf(p, s).result !== null)
    if (later || (i === first && anyResult)) throw new MarkError(409)
  }

  const out: MockProgress = { registered: p.registered, marks: { ...p.marks } }
  if (i === first) {
    out.registered = registered || m.result !== null
    registered = false
  }
  if (m.result !== null) out.registered = true
  if (!registered && m.result === null) delete out.marks[stageId]
  else out.marks[stageId] = { registered, result: m.result }
  return out
}

/** Снять первую регистрацию нельзя, пока на ней держатся итоги. */
export const hasResults = (p: MockProgress) => Object.values(p.marks).some((m) => m.result !== null)

/** Поля пункта трекера про этапы — как trackerItemOf на сервере. */
export function progressFields(
  st: MockStage[],
  p: MockProgress,
  now: number,
): Pick<TrackerItem, 'status' | 'outcome' | 'stages' | 'action' | 'deadline_at' | 'next_stage_title'> {
  const cur = current(st, p, now)
  const ask = asking(st, p, now)
  const { status: s, outcome } = status(st, p, now)

  const stages: TrackerStage[] = st.map((x, i) => {
    const reg = registeredOn(st, p, i)
    const result = markOf(p, x).result
    const allowed = (m: MockMark) => {
      try {
        apply(st, p, x.id, m, now)
        return true
      } catch {
        return false
      }
    }
    return {
      ...x,
      state: locked(st, p, i) ? 'locked' : cur === -1 || i < cur ? 'past' : i === cur ? 'current' : 'future',
      registered: reg,
      result,
      can_register: registrationLike(x.kind) && allowed({ registered: !reg, result }),
      results: results(st, i),
      results_allowed: results(st, i).filter((r) => allowed({ registered: reg, result: r })),
      asking: i === ask,
    }
  })

  let action: TrackerAction | null = null
  if (!p.registered && closedAt(st, p) < 0) {
    const first = firstRegistration(st)
    action = { type: 'register', stage_id: first >= 0 ? st[first]!.id : null }
  } else if (ask >= 0) {
    action = { type: 'result', stage_id: st[ask]!.id }
  } else if (s !== 'finished' && cur >= 0 && registrationLike(st[cur]!.kind) && !registeredOn(st, p, cur)) {
    action = { type: 'register', stage_id: st[cur]!.id }
  }

  return {
    status: s,
    outcome,
    stages,
    action,
    deadline_at: cur >= 0 ? st[cur]!.deadline_at : null,
    next_stage_title: cur >= 0 ? st[cur]!.title : null,
  }
}
