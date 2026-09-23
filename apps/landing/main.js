// Прогрессивное улучшение: без этого файла страница целиком читается и работает,
// просто без плашки у шапки, проявления блоков и вкладок (панели идут списком).

// Класс ставит сам скрипт: если он не загрузится, скрытые до появления блоки
// не останутся невидимыми навсегда.
document.documentElement.classList.add('js')

// Шапка становится «пилюлей», как только страницу чуть прокрутили.
const header = document.querySelector('[data-top]')
const sentinel = Object.assign(document.createElement('div'), { ariaHidden: 'true' })
sentinel.style.cssText = 'position:absolute;top:24px;left:0;width:1px;height:1px'
document.body.prepend(sentinel)
new IntersectionObserver(([e]) => header.toggleAttribute('data-scrolled', !e.isIntersecting)).observe(sentinel)

// Блоки проявляются один раз, когда доезжают до экрана.
const reveal = new IntersectionObserver(
  (entries) => {
    for (const e of entries) {
      if (!e.isIntersecting) continue
      e.target.classList.add('is-in')
      reveal.unobserve(e.target)
    }
  },
  { rootMargin: '0px 0px -8% 0px', threshold: 0.12 },
)
document.querySelectorAll('.reveal').forEach((el) => reveal.observe(el))

// Вкладки по паттерну WAI-ARIA: стрелки, Home и End, фокус ходит по кругу.
for (const root of document.querySelectorAll('[data-tabs]')) {
  const list = root.querySelector('[role=tablist]')
  const tabs = [...list.querySelectorAll('[role=tab]')]
  const panels = tabs.map((t) => document.getElementById(t.getAttribute('aria-controls')))

  const select = (i, focus) => {
    tabs.forEach((t, j) => {
      const on = i === j
      t.setAttribute('aria-selected', String(on))
      t.tabIndex = on ? 0 : -1
      panels[j].hidden = !on
      panels[j].classList.toggle('is-shown', on && focus !== undefined)
    })
    if (focus) {
      tabs[i].focus()
      tabs[i].scrollIntoView({ block: 'nearest', inline: 'nearest' })
    }
  }

  list.hidden = false
  select(0)
  tabs.forEach((t, i) => t.addEventListener('click', () => select(i, false)))
  list.addEventListener('keydown', (e) => {
    const i = tabs.indexOf(document.activeElement)
    const next = { ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: tabs.length - 1 }[e.key]
    if (next === undefined) return
    e.preventDefault()
    select((next + tabs.length) % tabs.length, true)
  })
}
