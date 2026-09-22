/**
 * Видимый баннер вместо белого экрана.
 *
 * Внутри MAX консоли нет, и необработанная ошибка выглядит как пустая
 * страница — на телефоне у проверяющего это неустранимо. Баннер показывает,
 * что именно упало, и даёт перезагрузить мини-приложение.
 *
 * Намеренно на голом DOM и без зависимостей: он должен работать и тогда,
 * когда React не смонтировался.
 */

let shown = false

function render(message: string): void {
  if (shown) return
  shown = true

  const bar = document.createElement('div')
  bar.setAttribute('role', 'alert')
  bar.style.cssText = [
    'position:fixed',
    'inset:auto 0 0 0',
    'z-index:9999',
    'padding:12px 16px calc(12px + env(safe-area-inset-bottom))',
    'background:#B3123F',
    'color:#fff',
    'font:500 13px/1.4 system-ui,sans-serif',
    'display:flex',
    'gap:12px',
    'align-items:flex-start',
  ].join(';')

  const text = document.createElement('div')
  text.style.flex = '1'
  text.textContent = `Что-то сломалось: ${message}`

  const reload = document.createElement('button')
  reload.textContent = 'Перезагрузить'
  reload.style.cssText =
    'flex:0 0 auto;background:#fff;color:#B3123F;border:0;border-radius:8px;padding:8px 12px;font:700 13px system-ui,sans-serif;min-height:36px'
  reload.onclick = () => location.reload()

  bar.append(text, reload)
  document.body.append(bar)
}

export function installCrashBanner(): void {
  window.addEventListener('error', (event) => {
    render(event.message || 'неизвестная ошибка')
  })

  window.addEventListener('unhandledrejection', (event) => {
    const reason = event.reason as { message?: string } | undefined
    render(reason?.message ?? 'запрос не завершился')
  })
}
