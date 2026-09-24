// Поведение лендинга. Без скрипта страница читается целиком: первый ответ в
// вопросах открыт, остальные раскрывает <noscript>-стиль; экраны в панелях
// стоят на первом; меню-бургер прячется; разделы и цифры видны сразу.

export const SWAP_MS = 3500

// Вопросы: одновременно открыт один. Повторный клик закрывает открытый.
export function initFaq(toggles, doc) {
  const items = toggles.map((btn) => ({ btn, answer: doc.getElementById(btn.getAttribute('aria-controls')) }))
  const set = ({ btn, answer }, open) => {
    btn.setAttribute('aria-expanded', String(open))
    btn.lastElementChild.textContent = open ? '−' : '+'
    answer.hidden = !open
  }
  for (const item of items) {
    item.btn.addEventListener('click', () => {
      const open = item.btn.getAttribute('aria-expanded') !== 'true'
      for (const other of items) set(other, false)
      if (open) set(item, true)
    })
  }
}

// Панели «Трекер» и «Помощник»: два экрана сменяют друг друга синхронно.
// При reduced motion стоят на первом.
export function initSwap(target, win) {
  if (win.matchMedia('(prefers-reduced-motion: reduce)').matches) return
  win.setInterval(() => target.classList.toggle('alt'), SWAP_MS)
}

// Меню на телефоне и планшете: закрывается выбором пункта, Escape и кликом мимо.
export function initMenu(button, menu, doc) {
  const isOpen = () => button.getAttribute('aria-expanded') === 'true'
  const set = (open) => {
    button.setAttribute('aria-expanded', String(open))
    menu.hidden = !open
  }
  button.addEventListener('click', () => set(!isOpen()))
  menu.addEventListener('click', (e) => {
    if (e.target.closest('a')) set(false)
  })
  doc.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || !isOpen()) return
    set(false)
    button.focus()
  })
  doc.addEventListener('click', (e) => {
    if (isOpen() && !menu.contains(e.target) && !button.contains(e.target)) set(false)
  })
}

// Липкая шапка получает разделитель, как только страницу прокрутили.
export function initHeader(header, win) {
  const update = () => header.toggleAttribute('data-scrolled', win.scrollY > 0)
  win.addEventListener('scroll', update, { passive: true })
  update()
}

// Движение: класс anim на <html> включает анимации, которые прячут элемент до
// входа в экран. Без него (reduced motion, старый браузер, скрипт не загрузился)
// всё стоит на месте и видно сразу.
export function initMotion(root, win) {
  const on = !win.matchMedia('(prefers-reduced-motion: reduce)').matches && 'IntersectionObserver' in win
  if (on) root.classList.add('anim')
  return on
}

// onIn(el) — один раз, когда элемент впервые показался на экране.
export function initInView(items, win, onIn, threshold = 0.25) {
  const io = new win.IntersectionObserver(
    (entries) => {
      for (const e of entries) {
        if (!e.isIntersecting) continue
        io.unobserve(e.target)
        onIn(e.target)
      }
    },
    { threshold },
  )
  for (const el of items) io.observe(el)
}

// Появление разделов при прокрутке (.tr-reveal, карточки в .tr-stagger). Где
// есть animation-timeline: view(), его ведёт CSS; здесь — запасной вариант.
export function initReveal(items, win) {
  const reduce = win.matchMedia('(prefers-reduced-motion: reduce)').matches
  const css = win.CSS?.supports('animation-timeline: view()')
  if (reduce || css || !('IntersectionObserver' in win)) return
  for (const el of items) {
    const parent = el.parentElement
    if (parent?.classList.contains('tr-stagger')) {
      el.style.transitionDelay = `${Math.min([...parent.children].indexOf(el), 3) * 60}ms`
    }
    el.classList.add('tr-io-hide')
  }
  initInView(items, win, (el) => el.classList.add('tr-io-in'), 0.12)
}

// Цифра в карточке досчитывает до своего значения: последнее число в тексте
// идёт от data-count-from (по умолчанию 0), остальной текст не меняется —
// «50→35» с data-count-from="50" идёт из 50 в 35.
const COUNT_MS = 1200

function counter(el, final = el.textContent) {
  const m = /(\d+)(\D*)$/.exec(final)
  if (!m) return null
  const show = (n) => `${final.slice(0, m.index)}${n}${m[2]}`
  return { to: Number(m[1]), from: Number(el.getAttribute('data-count-from') ?? 0), show }
}

export function countUp(el, win, final = el.textContent) {
  const c = counter(el, final)
  if (!c) return
  let start = null
  const step = (now) => {
    start ??= now
    const t = Math.min(1, (now - start) / COUNT_MS)
    const eased = 1 - (1 - t) ** 3
    el.textContent = c.show(Math.round(c.from + (c.to - c.from) * eased))
    if (t < 1) win.requestAnimationFrame(step)
  }
  win.requestAnimationFrame(step)
}

// Со скриптом цифра стоит на начале отсчёта и досчитывает, когда её показали.
export function initCounters(items, win) {
  const finals = new Map()
  for (const el of items) {
    const c = counter(el)
    if (!c) continue
    finals.set(el, el.textContent)
    el.textContent = c.show(c.from)
  }
  initInView([...finals.keys()], win, (el) => countUp(el, win, finals.get(el)), 0.6)
}

if (typeof document !== 'undefined') {
  const $$ = (sel) => [...document.querySelectorAll(sel)]
  initFaq($$('[data-faq-toggle]'), document)
  initSwap(document.body, window)
  initReveal($$('.tr-reveal, .tr-stagger > *'), window)
  if (initMotion(document.documentElement, window)) {
    initInView($$('.perk, .versus, .unis, .cta'), window, (el) => el.classList.add('is-in'))
    initCounters($$('.stat__num'), window)
  }
  initMenu(document.querySelector('.burger'), document.getElementById('menu'), document)
  initHeader(document.querySelector('[data-top]'), window)
}
