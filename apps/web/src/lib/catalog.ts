/**
 * Раскладка длинных списков каталога: олимпиады — по уровню, строки
 * «Олимпиады с льготой» в карточке вуза — по олимпиаде, а не по профилю.
 */

import { BENEFIT_LABELS, type BenefitKind, type Level } from '@contract'

export type LevelGroup = 'vsosh' | 'I' | 'II' | 'III' | 'other' | 'unknown'

const ORDER: LevelGroup[] = ['vsosh', 'I', 'II', 'III', 'other', 'unknown']

interface Groupable {
  kind: string
  primary_profile: { level: Level }
}

function levelGroup(item: Groupable): LevelGroup {
  if (item.kind === 'vsosh') return 'vsosh'
  if (item.kind === 'other') return 'other'
  return item.primary_profile.level ?? 'unknown'
}

/** Группы в порядке ценности льготы; пустые не возвращаются. */
export function groupByLevel<T extends Groupable>(items: T[]): { key: LevelGroup; items: T[] }[] {
  return ORDER.map((key) => ({ key, items: items.filter((item) => levelGroup(item) === key) })).filter(
    (group) => group.items.length > 0,
  )
}

const STRENGTH: Record<BenefitKind, number> = { bvi: 3, bvi_winners: 2, score100: 1, extra_points: 0 }

interface ProfileRow {
  olympiad_profile_id: string
  olympiad_id?: string
  name: string
  subject_name?: string
  benefit: BenefitKind
  /** Подпись льготы от сервера: «100 баллов по информатике». */
  benefit_label?: string | null
}

export interface OlympiadRow<T> {
  olympiad_id: string
  /** Первая строка олимпиады — её же и открываем: сервер ставит профили ученика вперёд. */
  first: T
  subjects: string
  benefit: BenefitKind
  /** Подпись лучшей льготы; у её профилей разные — общая: «100 баллов». */
  label: string
  open_profile_id: string
  /** Все профили олимпиады в порядке появления. */
  rows: T[]
}

/** Строка на олимпиаду: предметы её профилей и лучшая из льгот. Порядок — по первому появлению. */
export function byOlympiad<T extends ProfileRow>(rows: T[]): OlympiadRow<T>[] {
  const out = new Map<string, OlympiadRow<T>>()
  for (const r of rows) {
    const id = r.olympiad_id ?? r.olympiad_profile_id
    const seen = out.get(id)
    if (!seen) {
      out.set(id, {
        olympiad_id: id,
        first: r,
        subjects: r.subject_name ?? '',
        benefit: r.benefit,
        label: r.benefit_label ?? BENEFIT_LABELS[r.benefit],
        open_profile_id: r.olympiad_profile_id,
        rows: [r],
      })
      continue
    }
    seen.rows.push(r)
    if (r.subject_name && !seen.subjects.split(', ').includes(r.subject_name)) {
      seen.subjects = seen.subjects ? `${seen.subjects}, ${r.subject_name}` : r.subject_name
    }
    const label = r.benefit_label ?? BENEFIT_LABELS[r.benefit]
    if (STRENGTH[r.benefit] > STRENGTH[seen.benefit]) {
      seen.benefit = r.benefit
      seen.label = label
    } else if (r.benefit === seen.benefit && label !== seen.label) {
      seen.label = BENEFIT_LABELS[r.benefit]
    }
  }
  return [...out.values()]
}

