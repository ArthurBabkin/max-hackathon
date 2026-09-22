/** Сборка query-строки. Вынесено из client.ts отдельно, чтобы покрыть тестом. */
export function buildQuery(
  query?: Record<string, string | number | undefined | null>,
): string {
  if (!query) return ''
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== null && value !== '') params.set(key, String(value))
  }
  const qs = params.toString()
  return qs ? `?${qs}` : ''
}
