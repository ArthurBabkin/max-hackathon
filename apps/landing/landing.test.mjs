// Проверки лендинга без зависимостей: node --test apps/landing/landing.test.mjs
//
// Лендинг — статика без сборки, поэтому ломается он тихо: текст разошёлся с
// макетом, битая ссылка на картинку, якорь на удалённую секцию, кнопка на
// другого бота. Источник истины — макеты в docs/landing/html: тексты, ссылки
// и подписи картинок сверяются с ними по порядку, отдельно для десктопа и
// телефона.

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { countUp, initCounters, initFaq, initHeader, initInView, initMenu, initMotion, initReveal, initSwap, SWAP_MS } from './main.js'

const here = dirname(fileURLToPath(import.meta.url))
const read = (p) => readFileSync(join(here, p), 'utf8')

const html = read('index.html')
const CSS_FILES = ['tokens/tokens.css', 'styles.css', 'animations.css']
const css = Object.fromEntries(CSS_FILES.map((f) => [f, read(f)]))
const mockup = {
  desktop: read('../../docs/landing/html/desktop-1440.html'),
  mobile: read('../../docs/landing/html/mobile-390.html'),
}

const BOT = 'https://max.ru/t356_hakaton_max_bot'

// --- разбор HTML ---

const VOID = new Set(['area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input', 'link', 'meta', 'source', 'track', 'wbr'])
const SKIP = new Set(['script', 'style', 'svg', 'noscript', 'head', 'template'])
const ENTITIES = { nbsp: '\u00a0', amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", laquo: '«', raquo: '»', mdash: '—', ndash: '–', minus: '−' }

const decode = (s) =>
  s.replace(/&(#x[0-9a-f]+|#\d+|\w+);/gi, (m, e) =>
    e[0] === '#' ? String.fromCodePoint(e[1] === 'x' || e[1] === 'X' ? parseInt(e.slice(2), 16) : Number(e.slice(1))) : (ENTITIES[e] ?? m),
  )

const parseAttrs = (s) =>
  Object.fromEntries([...s.matchAll(/([\w:-]+)(?:\s*=\s*"([^"]*)")?/g)].map(([, k, v]) => [k.toLowerCase(), decode(v ?? '')]))

// Что видно в заданной раскладке: тексты, подписи картинок и ссылки по порядку.
// hide — классы, которые в этой раскладке скрыты (m-only, d-only и т. п.).
function view(source, hide = []) {
  const out = { texts: [], alts: [], links: [], sections: [] }
  const stack = []
  const hidden = () => stack.length > 0 && stack[stack.length - 1].hidden
  for (const [, comment, close, open, attrs, selfClose, text] of source.matchAll(
    /(<!--[\s\S]*?-->|<![^>]*>)|<\/([\w-]+)\s*>|<([\w-]+)((?:\s+[\w:-]+(?:\s*=\s*"[^"]*")?)*)\s*(\/?)>|([^<]+)/g,
  )) {
    if (comment) continue
    if (close) {
      const tag = close.toLowerCase()
      const i = stack.findLastIndex((e) => e.tag === tag)
      if (i >= 0) stack.length = i
      continue
    }
    if (open) {
      const tag = open.toLowerCase()
      const a = parseAttrs(attrs)
      const classes = (a.class ?? '').split(/\s+/)
      const h = hidden() || SKIP.has(tag) || classes.some((c) => hide.includes(c))
      if (!h && tag === 'img') out.alts.push(a.alt)
      if (!h && tag === 'a') out.links.push(a.href)
      if (!h && tag === 'section' && a.id) out.sections.push(a.id)
      if (!VOID.has(tag) && !selfClose) stack.push({ tag, hidden: h })
      continue
    }
    if (hidden()) continue
    // Неразрывный пробел сравниваем как обычный: макеты расходятся в них у
    // одних и тех же кнопок, а на вид это одно и то же.
    const t = decode(text).replace(/[ \t\r\n\u00a0]+/g, ' ').trim()
    if (t) out.texts.push(t)
  }
  return out
}

// Каждый элемент want встречается в got в том же порядке (лишнее в got можно:
// пункты выпадающего меню, подписи для скринридера).
function assertInOrder(got, want, what, same = (a, b) => a === b) {
  let i = 0
  for (const w of want) {
    const j = got.findIndex((g, k) => k >= i && same(g, w))
    assert.ok(j >= 0, `${what}: нет «${w}» после «${got[i - 1] ?? 'начала'}»`)
    i = j + 1
  }
}

// Раскладки страницы: что скрыто на 1440 и на 390.
const LAYOUT = {
  desktop: ['m-only', 'narrow-only'],
  mobile: ['d-only', 'wide-only'],
}

const attrs = (tag, src = html) =>
  [...src.matchAll(new RegExp(`<${tag}\\b((?:\\s+[\\w:-]+(?:\\s*=\\s*"[^"]*")?)*)\\s*/?>`, 'g'))].map(([, a]) => parseAttrs(a))

const isLocal = (u) => u && !/^(https?:|mailto:|tel:|#|data:)/.test(u)

// --- анимации: где стоят классы ---

// Все открывающие теги с атрибутами и местом в разметке.
const tags = [...html.matchAll(/<([\w-]+)((?:\s+[\w:-]+(?:\s*=\s*"[^"]*")?)*)\s*\/?>/g)].map((m) => ({
  tag: m[1],
  at: m.index,
  ...parseAttrs(m[2]),
}))
const has = (t, c) => (t.class ?? '').split(/\s+/).includes(c)
const within = (from, to) => tags.filter((t) => t.at >= html.indexOf(from) && t.at < html.indexOf(to))
const withClass = (c, list = tags) => list.filter((t) => has(t, c))

test('анимации: первый экран появляется лесенкой, телефоны выезжают, объекты парят', () => {
  const hero = within('<section id="hero"', '<section id="problem"')
  for (const c of ['pill', 'hero__title', 'hero__lead', 'hero__cta', 'stage']) {
    const els = withClass(c, hero)
    assert.ok(els.length > 0, `нет .${c}`)
    for (const e of els) assert.ok(has(e, 'tr-hero'), `.${c} без tr-hero`)
  }
  // Лесенка: номер растёт сверху вниз, у двух вариантов одного элемента — общий.
  const order = withClass('tr-hero', hero).map((e) => Number(/--i:\s*([\d.]+)/.exec(e.style ?? '')?.[1]))
  assert.deepEqual(order, [...order].sort((a, b) => a - b), `--i не по порядку: ${order}`)
  assert.equal(order[0], 0)

  for (const c of ['w-chat', 'w-home', 'n-chat']) assert.ok(has(withClass(c, hero)[0], 'tr-phone'), `.${c} без tr-phone`)
  for (const c of ['w-medal', 'w-soon', 'w-cap', 'w-cal', 'w-perk', 'n-medal', 'n-perk', 'n-soon']) {
    assert.ok(has(withClass(c, hero)[0], 'tr-float'), `.${c} без tr-float`)
  }
  // Колокольчик в «Через 3 дня» звенит — и на десктопе, и на телефоне.
  for (const plate of ['w-soon', 'n-soon']) {
    const start = withClass(plate, hero)[0].at
    const bell = hero.find((t) => t.at > start && t.tag === 'svg')
    assert.ok(has(bell, 'tr-ring'), `колокольчик в .${plate} без tr-ring`)
  }
})

test('анимации: разделы ниже первого экрана проявляются при прокрутке, шапка, подвал и первый экран — сразу', () => {
  const sections = [...html.matchAll(/<section id="([\w-]+)"/g)].map(([, id]) => id).filter((id) => id !== 'hero')
  assert.ok(sections.length >= 8)
  sections.forEach((id, i) => {
    const next = sections[i + 1] ? `<section id="${sections[i + 1]}"` : '</main>'
    const inside = within(`<section id="${id}"`, next)
    assert.ok(withClass('tr-reveal', inside).length + withClass('tr-stagger', inside).length > 0, `#${id} без появления`)
  })
  for (const c of ['stats', 'steps', 'duo', 'roles', 'versus']) assert.ok(has(withClass(c)[0], 'tr-stagger'), `.${c} без tr-stagger`)
  for (const [from, to] of [['<header', '</header>'], ['<section id="hero"', '<section id="problem"'], ['<footer', '</footer>']]) {
    const els = within(from, to)
    assert.equal(withClass('tr-reveal', els).length + withClass('tr-stagger', els).length, 0, `${from} должен быть виден сразу`)
  }
})

test('анимации: у кнопок подъём при наведении, ответы на вопросы проявляются', () => {
  for (const a of tags.filter((t) => t.tag === 'a' && has(t, 'btn'))) assert.ok(has(a, 'tr-btn'), `кнопка ${a.href} без tr-btn`)
  const answers = withClass('faq-answer')
  assert.equal(answers.length, 5)
  for (const a of answers) assert.ok(has(a, 'tr-faq-answer'), `#${a.id} без tr-faq-answer`)
})

test('анимации подключены после основных стилей', () => {
  const sheets = attrs('link').filter((l) => l.rel === 'stylesheet').map((l) => l.href)
  assert.deepEqual(sheets, ['tokens/tokens.css', 'styles.css', 'animations.css'])
})

// Анимация может прятать элемент только сама: базовый стиль с opacity: 0
// оставил бы текст невидимым, если скрипт не загрузился или анимация не пошла.
test('анимации не прячут контент без скрипта', () => {
  const rules = css['animations.css']
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/@keyframes[^{]*\{(?:[^{}]*\{[^{}]*\})*[^{}]*\}/g, '')
    .replace(/@supports[^{]*\{((?:[^{}]*\{[^{}]*\})*)[^{}]*\}/g, '')
  const hiding = [...rules.matchAll(/([^{}]+)\{([^{}]*)\}/g)].filter(([, , body]) => /opacity:\s*0(?![.\d])/.test(body))
  assert.ok(hiding.length > 0)
  for (const [, selectors] of hiding) {
    for (const sel of selectors.split(',').map((x) => x.trim())) {
      assert.ok(sel.startsWith('.anim ') || sel === '.tr-io-hide', `«${sel}» прячет контент без скрипта`)
    }
  }
})

test('при reduced motion анимаций нет', () => {
  const rm = css['animations.css'].match(/@media \(prefers-reduced-motion: reduce\)\s*\{([\s\S]*?)\n\}/)
  assert.ok(rm, 'нет блока prefers-reduced-motion')
  for (const c of ['.tr-hero', '.tr-phone', '.tr-float', '.tr-ring', '.tr-reveal', '.tr-faq-answer']) {
    assert.ok(rm[1].includes(c), `${c} не выключен`)
  }
  assert.match(rm[1], /animation:\s*none/)
})

// --- тексты и порядок разделов из макетов ---

for (const layout of ['desktop', 'mobile']) {
  const want = view(mockup[layout])
  const got = view(html, LAYOUT[layout])

  test(`${layout}: разделы идут в порядке макета`, () => {
    assert.ok(want.sections.length >= 8)
    assert.deepEqual(
      got.sections.filter((id) => want.sections.includes(id)),
      want.sections,
    )
  })

  test(`${layout}: все тексты макета на странице, в том же порядке`, () => {
    assert.ok(want.texts.length > 100)
    assertInOrder(got.texts, want.texts, 'текст')
  })

  test(`${layout}: ссылки макета на странице, в том же порядке`, () => {
    assertInOrder(got.links, want.links, 'ссылка')
  })

  // Подпись на странице может быть полнее, чем в мобильном макете: одна
  // картинка обслуживает обе раскладки, а подпись не видна глазами.
  test(`${layout}: картинки макета с теми же подписями, в том же порядке`, () => {
    assertInOrder(got.alts, want.alts, 'подпись картинки', (g, w) => g?.startsWith(w))
  })
}

// --- мета ---

test('страница на русском, светлая, с заголовком, описанием и превью', () => {
  assert.match(html, /<html lang="ru" data-theme="light"/)
  assert.match(html, /<meta name="viewport" content="width=device-width, initial-scale=1[^"]*"/)
  assert.match(html, /<title>Траектория — олимпиады, которые ведут в твой вуз<\/title>/)
  assert.match(
    html,
    /<meta name="description" content="Бот и мини-приложение в MAX: подборка олимпиад под цель, льготы в твоих вузах и напоминания о сроках для всей семьи">/,
  )
  assert.match(html, /<meta name="color-scheme" content="light">/)
  assert.match(html, /<meta name="theme-color" content="#6B2BFF">/)
  assert.match(html, /<meta property="og:image" content="https:\/\/traektoriaedu\.ru\/assets\/logo\/traektoriya-og\.png">/)
  assert.match(html, /<meta property="og:image:width" content="1200">/)
  assert.match(html, /<meta property="og:image:height" content="630">/)
  assert.match(html, /<link rel="icon" type="image\/png" href="assets\/logo\/traektoriya-app-icon\.png">/)
  assert.match(html, /<link rel="apple-touch-icon" href="assets\/logo\/traektoriya-app-icon\.png">/)
})

test('один H1, у каждого раздела свой H2', () => {
  assert.equal(attrs('h1').length, 1)
  for (const id of ['problem', 'start', 'features', 'roles', 'compare', 'trust', 'faq', 'cta']) {
    const start = html.indexOf(`<section id="${id}"`)
    assert.ok(start >= 0, `нет раздела #${id}`)
    const body = html.slice(start, html.indexOf('</section>', start))
    assert.match(body, /<h2[\s>]/, `у #${id} нет H2`)
  }
})

test('заголовки без эмодзи', () => {
  for (const [, h] of html.matchAll(/<h[1-3]\b[^>]*>([\s\S]*?)<\/h[1-3]>/g)) {
    assert.doesNotMatch(h, /\p{Extended_Pictographic}/u, h)
  }
})

// --- ассеты ---

const refs = () => [
  ...[...html.matchAll(/\s(?:src|href|srcset|content)="([^"]+)"/g)].flatMap(([, v]) =>
    v.split(',').map((s) => s.trim().split(/\s+/)[0].replace(/^https:\/\/traektoriaedu\.ru\//, '')),
  ),
  ...CSS_FILES.flatMap((f) =>
    [...css[f].matchAll(/url\(['"]?([^'")]+)['"]?\)/g)].map(([, v]) => relative(here, resolve(here, dirname(f), v))),
  ),
].filter((u) => isLocal(u) && /\.\w{2,5}$/.test(u))

test('все локальные файлы, на которые ссылается страница, существуют', () => {
  assert.ok(refs().length > 0, 'ссылок на ассеты не найдено')
  for (const ref of refs()) assert.ok(existsSync(join(here, ref)), `нет файла ${ref}`)
})

// Всё из папки уезжает в бакет: неиспользуемый файл — мусор на проде.
test('в папке нет файлов, на которые никто не ссылается', () => {
  const used = new Set(refs())
  const walk = (d) => readdirSync(join(here, d)).flatMap((f) => (statSync(join(here, d, f)).isDirectory() ? walk(join(d, f)) : [join(d, f)]))
  const own = new Set(['index.html', 'landing.test.mjs', 'README.md'])
  for (const f of walk('.').filter((f) => !own.has(f))) assert.ok(used.has(f), `${f} нигде не используется`)
})

test('у картинок есть alt и размеры, ниже первого экрана — ленивая загрузка', () => {
  const heroEnd = html.indexOf('<section id="problem"')
  const imgs = [...html.matchAll(/<img\b[^>]*>/g)]
  assert.ok(imgs.length > 0)
  for (const m of imgs) {
    const img = parseAttrs(m[0].slice(4))
    assert.ok('alt' in img, `нет alt у ${img.src}`)
    assert.ok(img.width && img.height, `нет width/height у ${img.src}`)
    if (img.srcset) assert.ok(img.sizes, `у ${img.src} есть srcset, но нет sizes`)
    if (m.index > heroEnd) assert.equal(img.loading, 'lazy', `${img.src} ниже первого экрана грузится сразу`)
  }
})

test('скриншоты продукта — WebP в двух плотностях', () => {
  const shots = attrs('img').filter((i) => i.src?.startsWith('assets/screens/'))
  assert.ok(shots.length >= 10)
  for (const s of shots) {
    assert.match(s.src, /\.webp$/)
    assert.match(s.srcset, /-292\.webp 292w, .+-584\.webp 584w/)
  }
})

test('декоративные SVG скрыты от скринридера', () => {
  for (const svg of attrs('svg')) assert.equal(svg['aria-hidden'], 'true')
})

test('кнопки ведут в нашего бота, и только в него', () => {
  const max = attrs('a').filter((a) => a.href?.includes('max.ru'))
  assert.ok(max.length >= view(mockup.desktop).links.filter((l) => l.includes('max.ru')).length)
  for (const a of max) assert.equal(a.href, BOT)
})

test('каждый якорь ведёт на существующий элемент', () => {
  const ids = new Set([...html.matchAll(/\sid="([^"]+)"/g)].map(([, id]) => id))
  for (const { href } of attrs('a').filter((a) => a.href?.startsWith('#') && a.href.length > 1)) {
    assert.ok(ids.has(href.slice(1)), `якорь ${href} никуда не ведёт`)
  }
})

test('ни скриптов, ни стилей с чужих доменов', () => {
  for (const s of attrs('script')) assert.ok(!s.src || isLocal(s.src), `внешний скрипт ${s.src}`)
  for (const l of attrs('link').filter((l) => l.rel === 'stylesheet')) assert.ok(isLocal(l.href), `внешний стиль ${l.href}`)
  for (const f of CSS_FILES) assert.doesNotMatch(css[f], /@import/)
})

test('без градиентов: в бренде их нет', () => {
  assert.doesNotMatch(html + css['styles.css'], /gradient\(/)
})

// --- политика конфиденциальности ---

// Бот и мини-приложение открывают политику по полному адресу: переименуешь
// страницу — их ссылки сломаются тихо.
test('политика конфиденциальности: отдельная страница, бот и приложение ведут на неё', () => {
  const page = read('privacy.html')
  assert.match(page, /<html lang="ru" data-theme="light"/)
  assert.match(page, /<title>Политика конфиденциальности — Траектория<\/title>/)
  assert.equal([...page.matchAll(/<h1[\s>]/g)].length, 1)
  const url = attrs('link', page).find((l) => l.rel === 'canonical')?.href
  assert.equal(url, 'https://traektoriaedu.ru/privacy.html')
  for (const f of ['../bot/internal/bot/onboarding.go', '../web/src/screens/Profile.tsx']) {
    assert.ok(read(f).includes(url), `${f} не ведёт на ${url}`)
  }
  for (const s of attrs('script', page)) assert.ok(!s.src || isLocal(s.src), `внешний скрипт ${s.src}`)
  for (const l of attrs('link', page).filter((l) => l.rel === 'stylesheet')) assert.ok(isLocal(l.href), `внешний стиль ${l.href}`)
  for (const [, v] of page.matchAll(/\s(?:src|href)="([^"]+)"/g)) {
    if (isLocal(v) && /\.\w{2,5}$/.test(v)) assert.ok(existsSync(join(here, v)), `нет файла ${v}`)
  }
})

// --- вопросы ---

test('вопросы: кнопки связаны с ответами, открыт первый', () => {
  const toggles = attrs('button').filter((b) => 'data-faq-toggle' in b)
  assert.equal(toggles.length, 5)
  toggles.forEach((b, i) => {
    assert.equal(b['aria-expanded'], String(i === 0))
    const answer = html.match(new RegExp(`<div\\b[^>]*\\sid="${b['aria-controls']}"[^>]*>`))
    assert.ok(answer, `нет ответа #${b['aria-controls']}`)
    assert.equal(/\shidden[\s>]/.test(answer[0]), i !== 0)
  })
})

// --- движение и шапка ---

test('при reduced motion экраны не сменяются и прокрутка не плавная', () => {
  const rm = css['styles.css'].match(/@media \(prefers-reduced-motion: reduce\)\s*\{([\s\S]*?)\n\}/)
  assert.ok(rm, 'нет блока prefers-reduced-motion')
  assert.match(rm[1], /\.swap-a[\s\S]*transition:\s*none/)
  assert.match(css['styles.css'], /@media \(prefers-reduced-motion: no-preference\)\s*\{\s*html\s*\{\s*scroll-behavior:\s*smooth/)
})

test('якорь не прячется под липкой шапкой', () => {
  assert.match(css['styles.css'], /position:\s*sticky/)
  assert.match(css['styles.css'], /scroll-margin-top:/)
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

// Светлые токены: первый блок :root в tokens.css и дополнительные в styles.css.
const firstRoot = (src) => src.slice(src.indexOf(':root {'), src.indexOf('}', src.indexOf(':root {')))
const palette = Object.fromEntries(
  [firstRoot(css['tokens/tokens.css']), firstRoot(css['styles.css'])].flatMap((b) =>
    [...b.matchAll(/--([\w-]+):\s*(#[0-9a-fA-F]{6})\b/g)].map(([, k, v]) => [k, v]),
  ),
)

const PAIRS = [
  ['текст на фоне', 'color-ink', 'color-surface'],
  ['вторичный текст на фоне', 'color-ink-muted', 'color-surface'],
  ['вторичный текст на карточке', 'color-ink-muted', 'color-surface-raised'],
  ['вторичный текст на серой карточке', 'color-ink-muted', 'color-surface-tint'],
  ['бренд на лиловой плашке', 'color-brand', 'color-brand-soft'],
  ['текст на лиловой панели', 'color-ink-on-lilac', 'color-brand-soft'],
  ['текст кнопки', 'color-on-brand', 'color-brand'],
  ['вторичный текст на фиолетовом', 'color-on-brand-muted', 'color-brand'],
  ['текст на тёмном', 'color-on-night', 'color-ink'],
  ['вторичный текст на тёмном', 'color-on-night-muted', 'color-ink'],
  ['плашка тёмной панели', 'color-night-accent', 'color-night-badge'],
  ['бирюзовая карточка', 'color-cyan-ink', 'color-cyan-soft'],
  ['розовая карточка', 'color-pink-ink', 'color-pink-soft'],
  ['жёлтая карточка', 'color-amber-ink', 'color-amber-soft'],
  ['зелёная плашка', 'color-green-ink', 'color-green-soft'],
  ['текст на бирюзовой плашке', 'color-ink', 'color-cyan'],
]

test('контраст текста не ниже 4.5:1', () => {
  for (const [what, fg, bg] of PAIRS) {
    assert.ok(palette[fg] && palette[bg], `нет токена --${fg} или --${bg}`)
    const r = ratio(palette[fg], palette[bg])
    assert.ok(r >= 4.5, `${what}: ${r.toFixed(2)}:1 (--${fg} на --${bg})`)
  }
})

// --- поведение: main.js на игрушечном DOM ---

class ClassList {
  #s = new Set()
  add(c) { this.#s.add(c) }
  remove(c) { this.#s.delete(c) }
  contains(c) { return this.#s.has(c) }
  toggle(c, force) {
    const on = force ?? !this.#s.has(c)
    on ? this.#s.add(c) : this.#s.delete(c)
    return on
  }
}

class El {
  constructor(tag, attrs = {}, ...children) {
    this.tag = tag
    this.attrs = { ...attrs }
    this.hidden = 'hidden' in attrs
    this.classList = new ClassList()
    this.style = {}
    this.children = []
    this.parent = null
    this.textContent = ''
    this.listeners = {}
    for (const c of children) typeof c === 'string' ? (this.textContent = c) : this.append(c)
  }
  append(c) { c.parent = this; this.children.push(c) }
  get lastElementChild() { return this.children.at(-1) ?? null }
  get parentElement() { return this.parent }
  getAttribute(k) { return this.attrs[k] ?? null }
  setAttribute(k, v) { this.attrs[k] = String(v) }
  hasAttribute(k) { return k in this.attrs }
  toggleAttribute(k, force) {
    const on = force ?? !(k in this.attrs)
    on ? (this.attrs[k] = '') : delete this.attrs[k]
    return on
  }
  addEventListener(type, fn) { (this.listeners[type] ??= []).push(fn) }
  dispatch(type, init = {}) {
    const ev = { type, target: this, preventDefault() {}, ...init }
    for (let n = this; n; n = n.parent) for (const fn of n.listeners[type] ?? []) fn(ev)
  }
  click() { this.dispatch('click') }
  contains(n) { for (; n; n = n.parent) if (n === this) return true; return false }
  closest(tag) { for (let n = this; n; n = n.parent) if (n.tag === tag) return n; return null }
  focus() { focused = this }
}
let focused = null

function faqDom() {
  const doc = new El('document')
  const byId = {}
  doc.getElementById = (id) => byId[id]
  const items = [0, 1, 2].map((i) => {
    const btn = new El('button', { 'aria-expanded': String(i === 0), 'aria-controls': `faq-${i}` }, new El('span', {}, 'Вопрос'), new El('span', {}, i === 0 ? '−' : '+'))
    const answer = new El('div', i === 0 ? { id: `faq-${i}` } : { id: `faq-${i}`, hidden: '' })
    byId[`faq-${i}`] = answer
    doc.append(btn)
    doc.append(answer)
    return { btn, answer }
  })
  initFaq(items.map((i) => i.btn), doc)
  const state = () => items.map(({ btn, answer }) => [btn.getAttribute('aria-expanded'), answer.hidden, btn.lastElementChild.textContent])
  return { items, state }
}

test('вопросы: открытие одного закрывает остальные', () => {
  const { items, state } = faqDom()
  items[2].btn.click()
  assert.deepEqual(state(), [
    ['false', true, '+'],
    ['false', true, '+'],
    ['true', false, '−'],
  ])
})

test('вопросы: повторный клик закрывает открытый', () => {
  const { items, state } = faqDom()
  items[0].btn.click()
  assert.deepEqual(state().map(([e, h]) => [e, h]), [
    ['false', true],
    ['false', true],
    ['false', true],
  ])
})

const fakeWindow = ({ reduced = false } = {}) => {
  const w = new El('window')
  w.scrollY = 0
  w.timers = []
  w.matchMedia = (q) => ({ matches: q === '(prefers-reduced-motion: reduce)' && reduced })
  w.setInterval = (fn, ms) => w.timers.push({ fn, ms })
  return w
}

test('экраны в панелях сменяются каждые 3,5 с', () => {
  const body = new El('body')
  const w = fakeWindow()
  initSwap(body, w)
  assert.equal(SWAP_MS, 3500)
  assert.deepEqual(w.timers.map((t) => t.ms), [3500])
  w.timers[0].fn()
  assert.ok(body.classList.contains('alt'))
  w.timers[0].fn()
  assert.ok(!body.classList.contains('alt'))
})

test('при reduced motion экраны стоят на первом', () => {
  const body = new El('body')
  const w = fakeWindow({ reduced: true })
  initSwap(body, w)
  assert.equal(w.timers.length, 0)
  assert.ok(!body.classList.contains('alt'))
})

function menuDom() {
  const doc = new El('document')
  const button = new El('button', { 'aria-expanded': 'false' })
  const link = new El('a', { href: '#faq' }, 'Вопросы')
  const menu = new El('nav', { hidden: '' }, link)
  const outside = new El('main')
  doc.append(button)
  doc.append(menu)
  doc.append(outside)
  initMenu(button, menu, doc)
  const open = () => button.getAttribute('aria-expanded') === 'true' && !menu.hidden
  return { doc, button, menu, link, outside, open }
}

test('меню: бургер открывает и закрывает', () => {
  const m = menuDom()
  m.button.click()
  assert.ok(m.open())
  m.button.click()
  assert.ok(!m.open() && m.menu.hidden)
})

test('меню: закрывается после выбора пункта, по Escape и по клику мимо', () => {
  const m = menuDom()
  m.button.click()
  m.link.click()
  assert.ok(!m.open(), 'пункт меню')

  m.button.click()
  focused = null
  m.doc.dispatch('keydown', { key: 'Escape' })
  assert.ok(!m.open(), 'Escape')
  assert.equal(focused, m.button, 'после Escape фокус возвращается на бургер')

  m.button.click()
  m.outside.click()
  assert.ok(!m.open(), 'клик мимо')
})

test('шапка получает разделитель, когда страницу прокрутили', () => {
  const header = new El('header')
  const w = fakeWindow()
  initHeader(header, w)
  assert.ok(!header.hasAttribute('data-scrolled'))
  w.scrollY = 120
  w.dispatch('scroll')
  assert.ok(header.hasAttribute('data-scrolled'))
  w.scrollY = 0
  w.dispatch('scroll')
  assert.ok(!header.hasAttribute('data-scrolled'))
})

// --- анимации: поведение ---

// IntersectionObserver на игрушечном DOM: show(el) — элемент вошёл в экран.
function withObserver(w) {
  w.observers = []
  w.IntersectionObserver = class {
    constructor(cb) {
      this.cb = cb
      this.els = new Set()
      w.observers.push(this)
    }
    observe(el) { this.els.add(el) }
    unobserve(el) { this.els.delete(el) }
  }
  w.show = (el) => w.observers.filter((o) => o.els.has(el)).forEach((o) => o.cb([{ target: el, isIntersecting: true }]))
  w.watched = (el) => w.observers.some((o) => o.els.has(el))
  return w
}

// Кадры анимации по требованию: frame(ms) — прошло столько от начала.
function withFrames(w) {
  let now = 0
  let queue = []
  w.performance = { now: () => now }
  w.requestAnimationFrame = (fn) => queue.push(fn)
  w.frame = (ms) => {
    now = ms
    const run = queue
    queue = []
    run.forEach((fn) => fn(now))
  }
  return w
}

test('движение включается классом anim, только когда оно есть', () => {
  const root = new El('html')
  assert.ok(initMotion(root, withObserver(fakeWindow())))
  assert.ok(root.classList.contains('anim'))

  const reduced = new El('html')
  assert.ok(!initMotion(reduced, withObserver(fakeWindow({ reduced: true }))))
  assert.ok(!reduced.classList.contains('anim'))

  // Без IntersectionObserver блоки не узнали бы, что их показали, — лучше без движения.
  const old = new El('html')
  assert.ok(!initMotion(old, fakeWindow()))
  assert.ok(!old.classList.contains('anim'))
})

test('вход в экран отмечает блок один раз', () => {
  const w = withObserver(fakeWindow())
  const block = new El('div')
  const seen = []
  initInView([block], w, (el) => seen.push(el))
  assert.deepEqual(seen, [])
  w.show(block)
  assert.deepEqual(seen, [block])
  assert.ok(!w.watched(block), 'после первого входа блок больше не отслеживается')
})

function revealDom() {
  const grid = new El('div')
  grid.classList.add('tr-stagger')
  const cards = [0, 1, 2, 3, 4].map(() => new El('article'))
  cards.forEach((c) => grid.append(c))
  const title = new El('h2')
  return { title, cards, items: [title, ...cards] }
}

// Без animation-timeline: view() (Firefox, Safari до 26) появление ведёт скрипт.
test('появление при прокрутке без view(): прячет, проявляет при входе в экран, карточки — лесенкой', () => {
  const w = withObserver(fakeWindow())
  w.CSS = { supports: () => false }
  const { title, cards, items } = revealDom()
  initReveal(items, w)
  for (const el of items) assert.ok(el.classList.contains('tr-io-hide') && !el.classList.contains('tr-io-in'))
  assert.deepEqual(cards.map((c) => c.style.transitionDelay), ['0ms', '60ms', '120ms', '180ms', '180ms'])
  assert.equal(title.style.transitionDelay, undefined)
  w.show(cards[1])
  assert.ok(cards[1].classList.contains('tr-io-in'))
  assert.ok(!cards[0].classList.contains('tr-io-in'))
  assert.ok(!w.watched(cards[1]))
})

test('появление при прокрутке: с view(), при reduced motion и без IntersectionObserver скрипт ничего не прячет', () => {
  const cases = {
    'есть view()': Object.assign(withObserver(fakeWindow()), { CSS: { supports: () => true } }),
    'reduced motion': Object.assign(withObserver(fakeWindow({ reduced: true })), { CSS: { supports: () => false } }),
    'нет IntersectionObserver': Object.assign(fakeWindow(), { CSS: { supports: () => false } }),
  }
  for (const [what, w] of Object.entries(cases)) {
    const { items } = revealDom()
    initReveal(items, w)
    assert.ok(items.every((el) => !el.classList.contains('tr-io-hide')), what)
  }
})

test('цифра досчитывает до своего значения и заканчивает ровно на нём', () => {
  const w = withFrames(fakeWindow())
  const num = new El('div', {}, '26%')
  countUp(num, w)
  w.frame(0)
  assert.equal(num.textContent, '0%')
  w.frame(600)
  const mid = Number.parseInt(num.textContent)
  assert.ok(mid > 0 && mid < 26, `на середине ${num.textContent}`)
  w.frame(5000)
  assert.equal(num.textContent, '26%')
})

// «50→35»: МФТИ сократил список — число идёт вниз от старого значения.
test('цифра с началом отсчёта считает от него', () => {
  const w = withFrames(fakeWindow())
  const num = new El('div', { 'data-count-from': '50' }, '50→35')
  countUp(num, w)
  w.frame(0)
  assert.equal(num.textContent, '50→50')
  w.frame(5000)
  assert.equal(num.textContent, '50→35')
})

// Без скрипта в карточке сразу итоговая цифра; со скриптом она стоит на
// начале отсчёта и досчитывает, когда карточку показали, — без скачка 83 → 0.
test('цифры в карточках: до показа — начало отсчёта, при показе досчитывают', () => {
  const w = withFrames(withObserver(fakeWindow()))
  const nums = [new El('div', {}, '83'), new El('div', { 'data-count-from': '50' }, '50→35')]
  initCounters(nums, w)
  assert.deepEqual(nums.map((n) => n.textContent), ['0', '50→50'])
  w.show(nums[0])
  w.frame(0)
  w.frame(5000)
  assert.deepEqual(nums.map((n) => n.textContent), ['83', '50→50'])
})
