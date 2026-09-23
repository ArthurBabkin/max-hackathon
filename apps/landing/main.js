// Поведение лендинга. Без скрипта страница читается целиком: первый ответ в
// вопросах открыт, остальные раскрывает <noscript>-стиль; экраны в панелях
// стоят на первом; меню-бургер прячется.

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

if (typeof document !== 'undefined') {
  initFaq([...document.querySelectorAll('[data-faq-toggle]')], document)
  initSwap(document.body, window)
  initMenu(document.querySelector('.burger'), document.getElementById('menu'), document)
  initHeader(document.querySelector('[data-top]'), window)
}
