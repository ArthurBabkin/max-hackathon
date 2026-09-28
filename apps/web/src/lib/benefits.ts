import { BENEFIT_LABELS, type BenefitKind } from '@contract'

// Какие значения столбца таблицы льгот «не как в большинстве вузов» (F19):
// большинство — строго больше половины. Его нет — не отмечаем ничего, иначе
// жёлтым залило бы полтаблицы и отметка перестала бы что-то значить.
export function outliers(values: string[]): Set<number> {
  const counts = new Map<string, number>()
  for (const v of values) counts.set(v, (counts.get(v) ?? 0) + 1)
  const majority = [...counts].find(([, n]) => n * 2 > values.length)?.[0]
  const out = new Set<number>()
  if (majority === undefined) return out
  values.forEach((v, i) => {
    if (v !== majority) out.add(i)
  })
  return out
}

/**
 * Подпись льготы — плашка и уточнение: «100 баллов по информатике» — это
 * плашка «100 баллов» и «по информатике» под ней. Уточнение — всё, что сервер
 * дописал к названию вида льготы (предмет 100 баллов).
 */
export function splitBenefitLabel(
  kind: BenefitKind | null | undefined,
  label: string,
): { main: string; detail: string | null } {
  const main = kind ? BENEFIT_LABELS[kind] : undefined
  if (main && label.startsWith(`${main} `)) return { main, detail: label.slice(main.length + 1) }
  return { main: label, detail: null }
}
