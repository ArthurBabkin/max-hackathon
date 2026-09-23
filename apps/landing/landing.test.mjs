// Проверки лендинга без зависимостей: node --test apps/landing
//
// Лендинг — статика без сборки, поэтому ломается он тихо: битая ссылка на
// картинку, якорь на удалённую секцию, кнопка на другого бота. Тесты ловят
// именно это, плюс контраст токенов в обеих темах (норма 4.5:1 из ТЗ §12).

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const root = (p) => fileURLToPath(new URL(p, import.meta.url))
const html = readFileSync(root('index.html'), 'utf8')
const css = readFileSync(root('styles.css'), 'utf8')

const BOT = 'https://max.ru/t356_hakaton_max_bot'

const attrs = (tag) =>
  [...html.matchAll(new RegExp(`<${tag}\\b([^>]*)>`, 'g'))].map(([, a]) =>
    Object.fromEntries([...a.matchAll(/([\w:-]+)(?:="([^"]*)")?/g)].map(([, k, v]) => [k, v ?? ''])),
  )

const isLocal = (u) => u && !/^(https?:|mailto:|tel:|#|data:)/.test(u)

test('страница на русском, с заголовком, описанием и viewport', () => {
  assert.match(html, /<html lang="ru"/)
  assert.match(html, /<title>[^<]+<\/title>/)
  assert.match(html, /<meta name="description" content="[^"]{50,}"/)
  assert.match(html, /<meta name="viewport"/)
})

test('все локальные файлы, на которые ссылается страница, существуют', () => {
  const refs = [
    ...[...html.matchAll(/\s(?:src|href|srcset|content)="([^"]+)"/g)].flatMap(([, v]) =>
      v.split(',').map((s) => s.trim().split(/\s+/)[0].replace(/^https:\/\/traektoriaedu\.ru\//, '')),
    ),
    ...[...css.matchAll(/url\(['"]?([^'")]+)['"]?\)/g)].map(([, v]) => v),
  ].filter((u) => isLocal(u) && /\.\w{2,5}$/.test(u))
  assert.ok(refs.length > 0, 'ссылок на ассеты не найдено')
  for (const ref of refs) assert.ok(existsSync(root(ref)), `нет файла ${ref}`)
})

test('каждый якорь ведёт на существующую секцию', () => {
  const ids = new Set([...html.matchAll(/\sid="([^"]+)"/g)].map(([, id]) => id))
  for (const { href } of attrs('a').filter((a) => a.href?.startsWith('#') && a.href.length > 1)) {
    assert.ok(ids.has(href.slice(1)), `якорь ${href} никуда не ведёт`)
  }
})

test('у картинок есть alt и размеры — без alt нет доступности, без размеров прыгает вёрстка', () => {
  const imgs = attrs('img')
  assert.ok(imgs.length > 0)
  for (const img of imgs) {
    assert.ok('alt' in img, `нет alt у ${img.src}`)
    assert.ok(img.width && img.height, `нет width/height у ${img.src}`)
  }
})

test('кнопки ведут в нашего бота, и только в него', () => {
  const max = attrs('a').filter((a) => a.href?.includes('max.ru'))
  assert.ok(max.length >= 2, 'нужен CTA в шапке и внизу страницы')
  for (const a of max) assert.equal(a.href, BOT)
})

test('ни скриптов, ни стилей с чужих доменов', () => {
  for (const s of attrs('script')) assert.ok(!s.src || isLocal(s.src), `внешний скрипт ${s.src}`)
  for (const l of attrs('link').filter((l) => l.rel === 'stylesheet')) {
    assert.ok(isLocal(l.href), `внешний стиль ${l.href}`)
  }
  assert.doesNotMatch(css, /@import/)
})

// --- контраст ---

const channel = (c) => (c / 255 <= 0.03928 ? c / 255 / 12.92 : ((c / 255 + 0.055) / 1.055) ** 2.4)
const lum = (hex) => {
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16))
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b)
}
const ratio = (a, b) => {
  const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p)
  return (x + 0.05) / (y + 0.05)
}

function palette(marker) {
  const start = css.indexOf(marker)
  assert.ok(start >= 0, `не найден блок токенов ${marker}`)
  const block = css.slice(start, css.indexOf('}', start))
  return Object.fromEntries([...block.matchAll(/--([\w-]+):\s*(#[0-9a-fA-F]{6})\b/g)].map(([, k, v]) => [k, v]))
}

const PAIRS = [
  ['текст на фоне', 'text', 'bg'],
  ['текст на карточке', 'text', 'surface'],
  ['приглушённый текст на фоне', 'muted', 'bg'],
  ['приглушённый текст на карточке', 'muted', 'surface'],
  ['акцентный текст на фоне', 'accent-ink', 'bg'],
  ['акцентный текст на карточке', 'accent-ink', 'surface'],
  ['текст кнопки', 'on-accent', 'accent'],
  ['текст на градиенте: светлый край', 'on-brand', 'brand-1'],
  ['текст на градиенте: тёмный край', 'on-brand', 'brand-2'],
]

for (const [theme, marker] of [
  ['светлая', '/* tokens: light */'],
  ['тёмная', '/* tokens: dark */'],
]) {
  test(`контраст токенов не ниже 4.5:1 — ${theme} тема`, () => {
    const light = palette('/* tokens: light */')
    const p = { ...light, ...palette(marker) }
    for (const [what, fg, bg] of PAIRS) {
      assert.ok(p[fg] && p[bg], `нет токена --${fg} или --${bg}`)
      const r = ratio(p[fg], p[bg])
      assert.ok(r >= 4.5, `${what}: ${r.toFixed(2)}:1 (--${fg} на --${bg})`)
    }
  })
}
