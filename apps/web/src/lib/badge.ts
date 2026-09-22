/**
 * Оформление плитки олимпиады или вуза.
 *
 * `short_name` и `color` в контракте могут быть пустыми: таких колонок в схеме
 * БД нет, и заполнять их сервер не обязан. Тогда аббревиатура собирается из
 * названия, а цвет берётся из фиксированной палитры по хешу идентификатора —
 * так одна и та же олимпиада всегда одного цвета, и это не зависит от сервера.
 */

/** Палитра из прототипа: цвета плиток олимпиад и вузов. */
const PALETTE = ['#6B2BFF', '#1A6DFF', '#E92E78', '#0C8F62', '#D48806', '#7A7E90']

/** Предлоги и союзы, которые в аббревиатуре только мешают. */
const STOP_WORDS = new Set(['по', 'и', 'в', 'на', 'для', 'the', 'of', 'a'])

export function badgeShortName(name: string, provided: string | null | undefined): string {
  if (provided) return provided

  const words = name
    .trim()
    .split(/[\s—–-]+/)
    .filter((w) => w.length > 0 && !STOP_WORDS.has(w.toLowerCase()))

  if (words.length === 0) return '?'

  if (words.length === 1) {
    const word = words[0]!
    return word.slice(0, 1).toUpperCase() + word.slice(1, 2).toLowerCase()
  }

  return words
    .slice(0, 3)
    .map((w) => w[0]!.toUpperCase())
    .join('')
}

export function badgeColor(id: string, provided: string | null | undefined): string {
  if (provided) return provided

  // FNV-1a: короткая, стабильная и без зависимостей. Криптостойкость здесь
  // не нужна — нужно только, чтобы цвет не прыгал между запусками.
  let hash = 0x811c9dc5
  for (let i = 0; i < id.length; i += 1) {
    hash ^= id.charCodeAt(i)
    hash = Math.imul(hash, 0x01000193) >>> 0
  }
  return PALETTE[hash % PALETTE.length]!
}
