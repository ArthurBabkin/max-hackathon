/**
 * Проверка контраста токенов — ТЗ §12 требует не меньше 4.5:1.
 *
 * Фирменные розовый и зелёный из прототипа на своих светлых подложках нормы
 * не дают, поэтому текстовые оттенки разведены с заливкой (--pk-ink, --gr-ink).
 * Скрипт держит это под контролем: правка палитры «на глаз» сразу уронит
 * `npm run check:contrast`.
 */

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const tokensPath = fileURLToPath(new URL('../src/ui/tokens.css', import.meta.url))
const css = readFileSync(tokensPath, 'utf8')

const channel = (c) => (c / 255 <= 0.03928 ? c / 255 / 12.92 : ((c / 255 + 0.055) / 1.055) ** 2.4)

function luminance(hex) {
  const h = hex.replace('#', '')
  const [r, g, b] = [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16))
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b)
}

function ratio(a, b) {
  const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p)
  return (x + 0.05) / (y + 0.05)
}

function palette(marker) {
  const start = css.indexOf(marker)
  if (start < 0) throw new Error(`не найден блок ${marker}`)
  const block = css.slice(start + marker.length, css.indexOf('}', start))
  return Object.fromEntries([...block.matchAll(/--([a-z0-9-]+):\s*(#[0-9a-fA-F]{6})/g)].map((m) => [m[1], m[2]]))
}

const PAIRS = [
  ['основной текст на карточке', 'tx', 'surface'],
  ['основной текст на фоне', 'tx', 'bg'],
  ['вторичный текст', 'tx-2', 'surface'],
  ['приглушённый текст на карточке', 'mu', 'surface'],
  ['приглушённый текст на фоне', 'mu', 'bg'],
  ['акцент', 'p', 'surface'],
  ['ссылка', 'p2', 'surface'],
  ['метка «Рекомендация»', 'p', 'ps'],
  ['плашка «срок скоро»', 'am-ink', 'ams'],
  ['плашка «срок близко»', 'pk-ink', 'pks'],
  ['плашка «готово»', 'gr-ink', 'grs'],
]

const MIN = 4.5
let failures = 0

for (const [theme, marker] of [
  ['светлая', ':root {'],
  ['тёмная', ":root[data-theme='dark'] {"],
]) {
  const tokens = palette(marker)
  console.log(`\n${theme} тема`)
  for (const [name, fg, bg] of PAIRS) {
    if (!tokens[fg] || !tokens[bg]) {
      console.log(`  ?     нет токена: ${name}`)
      failures += 1
      continue
    }
    const value = ratio(tokens[fg], tokens[bg])
    const ok = value >= MIN
    if (!ok) failures += 1
    console.log(`  ${ok ? 'ok  ' : 'МАЛО'} ${value.toFixed(2)}  ${name}`)
  }
}

if (failures > 0) {
  console.error(`\nПар ниже ${MIN}:1 — ${failures}. ТЗ §12 требует ${MIN}:1.`)
  process.exit(1)
}
console.log(`\nВсе пары проходят ${MIN}:1.`)
