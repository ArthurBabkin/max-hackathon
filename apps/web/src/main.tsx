import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { HashRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import '@maxhub/max-ui/dist/styles.css'
import './ui/tokens.css'
import './ui/app.css'
import { App } from './App'
import { getWebApp } from './bridge'
import { installCrashBanner } from './ui/crashBanner'
import { ThemedMaxUI, applySavedTheme } from './ui/theme'

// Внутри MAX нет консоли: необработанная ошибка превратилась бы в белый экран,
// который с телефона не диагностируется. Баннер ставим до монтирования React,
// чтобы он поймал и падение при старте.
installCrashBanner()
applySavedTheme()

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Мини-приложение живёт секунды-минуты, данные за это время не устаревают.
      staleTime: 60_000,
      // Мобильная сеть рвётся — один повтор оправдан, дальше показываем H4.
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

/** iOS и Android в MAX UI отличаются отступами и поведением нажатий. */
function platform(): 'ios' | 'android' {
  const raw = getWebApp().platform?.toLowerCase() ?? ''
  if (raw.includes('ios')) return 'ios'
  if (raw.includes('android')) return 'android'
  return /iPhone|iPad|iPod|Mac/.test(navigator.userAgent) ? 'ios' : 'android'
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      {/* Провайдер рисует свой div; без явной высоты он тянется по контенту,
          и тогда .app перестаёт ограничивать себя экраном, а вместо
          внутренней прокрутки страница начинает расти целиком. */}
      <ThemedMaxUI platform={platform()} className="max-root">
        <HashRouter>
          <App />
        </HashRouter>
      </ThemedMaxUI>
    </QueryClientProvider>
  </StrictMode>,
)
