import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { getWebApp } from '@/bridge'

/**
 * Куда вести по `start_param` — разделу, который бот кладёт в кнопку
 * «Открыть приложение» (open_app) или в ссылку `?startapp=`:
 * `home`, `match`, `tracker`, `family`, `faq`, `o_<olympiad_profile_id>` — карточка.
 * Параметр приходит от пользователя, поэтому всё незнакомое — главная.
 */

export interface StartRoute {
  path: string
  /** Карточка олимпиады поверх экрана — в стеке листов `?sheet=`. */
  sheet?: string
}

const TABS: Record<string, string> = { match: '/match', tracker: '/tracker', family: '/family', faq: '/faq' }
// Алфавит payload MAX: [A-Za-z0-9_-], до 512 символов (ТЗ §11.2).
const SAFE = /^[A-Za-z0-9_-]{1,512}$/

export function startRoute(param: string | null | undefined): StartRoute | null {
  if (!param || !SAFE.test(param)) return null
  const tab = TABS[param]
  if (tab) return { path: tab }
  if (param.startsWith('o_') && param.length > 2) return { path: '/', sheet: `oly:${param.slice(2)}` }
  return null
}

/**
 * Переход по start_param — один раз, когда сессия открыта. Главная остаётся
 * в истории под разделом: «Назад» ведёт туда, а не закрывает мини-приложение.
 */
export function useStartRoute(ready: boolean): void {
  const navigate = useNavigate()
  const done = useRef(false)
  useEffect(() => {
    if (done.current || !ready) return
    done.current = true
    const route = startRoute(getWebApp().initDataUnsafe.start_param)
    if (!route) return
    navigate({ pathname: route.path, search: route.sheet ? `?sheet=${route.sheet}` : '' })
  }, [ready, navigate])
}
