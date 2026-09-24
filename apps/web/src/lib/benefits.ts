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
