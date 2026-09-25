/**
 * Льготы на мои направления на демо-стенде — те же правила, что в
 * packages/core/targets и store.TargetBenefits: выбранные в вузе направления,
 * иначе покрывающие цель, иначе вуз целиком. Только для разработки.
 */

import type { BenefitKind, TargetBasis } from '@contract'
import { DIRECTIONS, type DemoUniversity } from './fixtures'
import { state } from './state'

const isGroup = (code: string) => code.length === 8 && code.endsWith('.00.00')

/** a покрывает b: коды равны или один — укрупнённая группа XX.00.00. */
export function covers(a: string, b: string): boolean {
  if (!a || !b) return false
  if (a === b) return true
  if (isGroup(b)) [a, b] = [b, a]
  return isGroup(a) && b.startsWith(a.slice(0, 3))
}

export const directionById = (id: string) => DIRECTIONS.find((d) => d.id === id)
const codeOf = (id: string) => directionById(id)?.code ?? ''
export const nameOf = (id: string) => directionById(id)?.name ?? id

export interface MockTarget {
  basis: TargetBasis
  ids: string[]
  unverified: boolean
}

const goalCodes = () => state.directions.map((d) => codeOf(d.id))

export const isGoal = (dirId: string) => goalCodes().some((g) => covers(codeOf(dirId), g))

export function targetOf(u: DemoUniversity): MockTarget {
  const chosen = state.chosen[u.id] ?? []
  let picked = u.offered.filter((o) => chosen.includes(o.id))
  let basis: TargetBasis = 'chosen'
  if (picked.length === 0) {
    picked = u.offered.filter((o) => isGoal(o.id))
    basis = 'goal'
  }
  if (picked.length === 0) return { basis: 'university', ids: [], unverified: false }
  return { basis, ids: picked.map((o) => o.id), unverified: picked.every((o) => o.status === 'to_check') }
}

/** Льгота на направлении вуза; null — нет или уточняется. */
export function onDirection(u: DemoUniversity, olympiadId: string, dirId: string): BenefitKind | null {
  const offered = u.offered.find((o) => o.id === dirId)
  if (!offered || offered.status === 'to_check') return null
  const own = u.byDirection?.[olympiadId]
  if (own && dirId in own) return own[dirId] ?? null
  const b = u.benefits[olympiadId]
  return b && b !== 'extra_points' ? b : null
}

const rank: Record<BenefitKind, number> = { bvi: 0, bvi_winners: 1, score100: 2, extra_points: 3 }

export interface MockTargetBenefit {
  basis: TargetBasis
  /** null — на мои направления льготы нет. */
  benefit: BenefitKind | null
  names: string[]
  others: { benefit: BenefitKind; names: string[] }[]
  unverified: boolean
  varies: boolean
}

/** Льгота олимпиады в вузе на мои направления. */
export function targetBenefit(u: DemoUniversity, olympiadId: string): MockTargetBenefit {
  const t = targetOf(u)
  const whole = u.benefits[olympiadId] ?? null
  const base = { basis: t.basis, names: [], others: [], unverified: false, varies: false }
  if (t.basis === 'university' || whole === 'extra_points') return { ...base, benefit: whole }
  if (t.unverified) return { ...base, benefit: whole, names: t.ids.map(nameOf), unverified: true }
  const found = t.ids
    .map((id) => ({ id, benefit: onDirection(u, olympiadId, id) }))
    .filter((x): x is { id: string; benefit: BenefitKind } => x.benefit !== null)
    .sort((a, b) => rank[a.benefit] - rank[b.benefit])
  if (found.length === 0) return { ...base, benefit: null, names: t.ids.map(nameOf) }
  const best = found[0]!.benefit
  const others: MockTargetBenefit['others'] = []
  for (const x of found.filter((f) => f.benefit !== best)) {
    const last = others.at(-1)
    if (last?.benefit === x.benefit) last.names.push(nameOf(x.id))
    else others.push({ benefit: x.benefit, names: [nameOf(x.id)] })
  }
  const bestIds = found.filter((f) => f.benefit === best).map((f) => f.id)
  return {
    ...base,
    benefit: best,
    names: bestIds.map(nameOf),
    others,
    varies: bestIds.some((id) => u.varies?.[olympiadId]?.includes(id) ?? false),
  }
}

/** На скольких направлениях вуза олимпиада даёт льготу. */
export function coverage(u: DemoUniversity, olympiadId: string) {
  return {
    directions_count: u.offered.filter((o) => onDirection(u, olympiadId, o.id) !== null).length,
    directions_total: u.offered.length,
  }
}
