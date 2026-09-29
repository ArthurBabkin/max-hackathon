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

/** Города фильтра каталога вузов — из самих вузов: где вузов больше, раньше, при равенстве — по алфавиту. */
export function citiesOf(universities: { city: string | null }[]): string[] {
  const count = new Map<string, number>()
  for (const u of universities) if (u.city) count.set(u.city, (count.get(u.city) ?? 0) + 1)
  return [...count.keys()].sort((a, b) => count.get(b)! - count.get(a)! || a.localeCompare(b, 'ru'))
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
  /** Предметы профилей без повторов, в порядке появления. */
  subjects: string[]
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
        subjects: r.subject_name ? [r.subject_name] : [],
        benefit: r.benefit,
        label: r.benefit_label ?? BENEFIT_LABELS[r.benefit],
        open_profile_id: r.olympiad_profile_id,
        rows: [r],
      })
      continue
    }
    seen.rows.push(r)
    if (r.subject_name && !seen.subjects.includes(r.subject_name)) seen.subjects.push(r.subject_name)
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


/** Сколько знаков спрятанного хвоста не стоит кнопки: меньше строки. */
const REST_MIN = 30

/**
 * Пункты для свёрнутого списка: пока подпись через запятую не длиннее
 * бюджета, и хотя бы один. Считается длина, а не число: у НТО профиль бывает
 * длиной в абзац, и три «первых» занимали семь строк. Если спрятать осталось
 * меньше строки, список целиком: «и ещё 1» с кнопкой места не экономит.
 */
export function previewByLength(items: string[], budget: number): string[] {
  const out: string[] = []
  let length = 0
  for (const item of items) {
    if (out.length > 0 && length + 2 + item.length > budget) break
    length += (out.length > 0 ? 2 : 0) + item.length
    out.push(item)
  }
  const rest = items.slice(out.length).reduce((sum, item) => sum + 2 + item.length, 0)
  return rest > REST_MIN ? out : items
}
