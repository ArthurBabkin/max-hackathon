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

/**
 * Логотипы из `public/logos`: значки с официальных сайтов вузов и олимпиад.
 * У олимпиады — только её собственный знак, не логотип вуза-организатора;
 * у кого своего нет или он слишком мелкий, остаётся плитка с аббревиатурой.
 */
const LOGOS: Record<string, string> = {
  msu: 'msu.png',
  spbu: 'spbu.png',
  hse: 'hse.png',
  mipt: 'mipt.png',
  itmo: 'itmo.png',
  kfu: 'kfu.png',
  nsu: 'nsu.svg',
  innopolis: 'innopolis.png',
  sechenov: 'sechenov.png',
  'kazan-gmu': 'kazan-gmu.png',
  'p669-2': 'p669-2.png',
  'p669-6': 'p669-6.png',
  'p669-7': 'p669-7.png',
  'p669-12': 'p669-12.png',
  'p669-23': 'p669-23.png',
  'p669-32': 'p669-32.png',
  'p669-49': 'p669-49.png',
  'p669-51': 'p669-51.png',
  'p669-55': 'p669-55.png',
  'p669-57': 'p669-57.png',
  'p669-62': 'p669-62.svg',
  'p669-64': 'p669-64.png',
  'p669-66': 'p669-66.png',
  'p669-82': 'p669-82.png',
  'vsosh-astronomiya': 'vsosh-astronomiya.png',
  'vsosh-biologiya': 'vsosh-biologiya.png',
  'vsosh-ekonomika': 'vsosh-ekonomika.png',
  'vsosh-fizika': 'vsosh-fizika.png',
  'vsosh-himiya': 'vsosh-himiya.png',
  'vsosh-informatika': 'vsosh-informatika.png',
  'vsosh-matematika': 'vsosh-matematika.png',
  'vsosh-obschestvoznanie': 'vsosh-obschestvoznanie.png',
}

export function badgeLogo(id: string): string | null {
  // У ВсОШ по предмету свой знак, для предмета без него — общий.
  const file = LOGOS[id] ?? (id.startsWith('vsosh-') ? 'vsosh.png' : undefined)
  return file ? `${import.meta.env.BASE_URL}logos/${file}` : null
}
