/**
 * Туториал для новичка: восемь шагов по настоящим экранам — главная, список
 * олимпиад и карточка олимпиады, список вузов и карточка вуза, трекер, подбор,
 * помощник.
 *
 * До каждого экрана туториал доходит сам. Между карточками шагов — одно
 * показанное нажатие: подсвечивается вкладка или строка с подписью «Нажимаем
 * «Каталог»», потом нажимается. Так видно, как попасть туда самому, а в
 * карточке шага написан путь: «Каталог → Вузы». Открытый лист закрывается
 * молча — крестик объяснять не нужно. «Назад» переходит сразу, без показа.
 *
 * Элементы помечены атрибутом `data-tour`. Нет цели на экране (пустой трекер)
 * — подсвечивается запасная, нет и её — карточка встаёт по центру.
 * «Пропустить», Esc и последний шаг закрывают туториал и запоминают, что его
 * видели.
 */

import {
  type CSSProperties,
  createContext,
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from 'react'
import { Button } from '@maxhub/max-ui'
import { useNavigate } from 'react-router-dom'
import { useSession } from '@/api/queries'
import { markTutorialSeen } from '@/lib/tutorial'
import type { TextKey } from '@/voice/texts'
import { useVoice } from '@/voice/useVoice'

/** Нажатие по дороге к шагу. */
interface Tap {
  /** Кнопка, которую нажимаем. */
  find: () => HTMLElement | null
  /** Нажимать не нужно: уже там, куда она ведёт. */
  done: () => boolean
  /** Подпись в подсказке «Нажимаем «…»» и в пути шага. */
  label: (el: HTMLElement) => string
  /** Служебное нажатие (закрыть лист): не показывается и в путь не попадает. */
  quiet?: boolean
}

interface Step {
  /** Ключи текстов: `tutorial.<key>.title` и `.text`. */
  key: string
  /** Родителю с учеником — текст про «Предложить» вместо «Добавить». */
  propose?: boolean
  /** Все нажатия от главной до экрана шага; уже сделанные пропускаются. */
  path: Tap[]
  /** Что подсвечиваем: все найденные элементы — одним окном. */
  target: () => HTMLElement[]
  /** Цели нет (пустой трекер, пустой подбор) — что подсветить вместо неё. */
  fallback?: () => HTMLElement[]
}

const $ = (selector: string) => document.querySelector<HTMLElement>(selector)
const $$ = (selector: string) => [...document.querySelectorAll<HTMLElement>(selector)]
const text = (el: Element) => (el.textContent ?? '').trim()
const found = (...els: (HTMLElement | null)[]) => els.filter((el): el is HTMLElement => el !== null)
/** Экран ещё грузится: ленивый раздел или скелетоны вместо данных. */
const loading = () => Boolean($('.screen-loading, [aria-busy="true"]'))

const closeSheet: Tap = {
  find: () => $('[data-tour="sheet-close"]'),
  done: () => !$('[data-tour="sheet-close"]'),
  label: text,
  quiet: true,
}

const tab = (name: string): Tap => ({
  find: () => $(`[data-tour="${name}"]`),
  done: () => $(`[data-tour="${name}"]`)?.getAttribute('aria-current') === 'page',
  // Без счётчика на «Трекере»: в подписи только название вкладки.
  label: (el) => text(el.querySelector('.tab-label') ?? el),
})

const segment = (name: string): Tap => ({
  find: () => $(`[data-tour="${name}"]`),
  done: () => $(`[data-tour="${name}"]`)?.getAttribute('aria-selected') === 'true',
  label: text,
})

const olympiadRow: Tap = {
  // Олимпиада, которую ещё можно добавить или предложить: у той, что уже в
  // трекере, вместо кнопки была бы отметка «В трекере».
  find: () => $('[data-tour="olympiad-row"][data-free]') ?? $('[data-tour="olympiad-row"]'),
  done: () => Boolean($('[data-tour="track-action"]')),
  label: (el) => text(el.querySelector('.row-title') ?? el),
}

const universityRow: Tap = {
  find: () => $('[data-tour="university-row"]'),
  done: () => Boolean($('[data-tour="university-olympiads"]')),
  label: (el) => text(el.querySelector('.row-title') ?? el),
}

const askButton: Tap = {
  find: () => $('[data-tour="ai"]'),
  done: () => Boolean($('[data-tour="ai-compose"]')),
  label: text,
}

const STEPS: Step[] = [
  {
    key: '1',
    path: [closeSheet, tab('home')],
    target: () => found($('[data-tour="goal"]'), $('[data-tour="next-step"]')),
  },
  {
    key: '2',
    path: [closeSheet, tab('catalog'), segment('catalog-olympiads')],
    // Весь список вместе с фильтром по предмету: видно, что олимпиад много,
    // а сейчас показаны по предмету ученика. Пока список грузится, цели нет:
    // иначе окно обвело бы один фильтр, а потом выросло до списка.
    target: () => {
      const groups = $$('[data-tour="olympiad-group"]')
      return groups.length > 0 ? found($('[data-tour="catalog-subjects"]'), ...groups) : []
    },
  },
  {
    key: '3',
    propose: true,
    path: [closeSheet, tab('catalog'), segment('catalog-olympiads'), olympiadRow],
    target: () => found($('[data-tour="track-action"]')),
  },
  {
    key: '4',
    path: [closeSheet, tab('catalog'), segment('catalog-universities')],
    target: () => found($('[data-tour="university-list"]')),
  },
  {
    key: '5',
    path: [closeSheet, tab('catalog'), segment('catalog-universities'), universityRow],
    target: () => found($('[data-tour="university-olympiads"]')),
  },
  {
    key: '6',
    path: [closeSheet, tab('tracker'), segment('tracker-list')],
    target: () => found($('[data-tour="tracker-item"]')),
    fallback: () => found($('[data-tour="tracker-list"]')),
  },
  {
    key: '7',
    path: [closeSheet, tab('match')],
    target: () => found($('[data-tour="match-list"] .oly-card')),
    fallback: () => found($('[data-tour="match"]')),
  },
  {
    key: '8',
    // «Спросить» есть и на подборе, где закончился прошлый шаг.
    path: [closeSheet, tab('match'), askButton],
    target: () => found($('[data-tour="ai-suggest"]'), $('[data-tour="ai-compose"]')),
  },
]

/** Цель шага, а если её нет — запасная. */
function stepTargets(step: Step): HTMLElement[] {
  const main = step.target()
  return main.length > 0 ? main : (step.fallback?.() ?? [])
}

/**
 * Паузы показа нажатий, мс: `point` — подсветка кнопки до нажатия, `press` —
 * само нажатие, `wait` — сколько ждать экран или кнопку, пока они грузятся.
 */
export interface Pace {
  point: number
  press: number
  wait: number
}

export const TutorialPace = createContext<Pace>({ point: 600, press: 250, wait: 4000 })

const sleep = (ms: number) => new Promise<void>((resolve) => window.setTimeout(resolve, ms))

/** Ждёт, пока `find` что-то вернёт. Не дождались или тур отменён — null. */
async function waitFor<T>(find: () => T | null, timeout: number, alive: () => boolean): Promise<T | null> {
  const until = Date.now() + timeout
  for (;;) {
    const result = find()
    if (result !== null || !alive() || Date.now() >= until) return result
    await sleep(40)
  }
}

/** Поле вокруг цели, зазор до карточки и отступ от краёв экрана. */
const PAD = 6
const GAP = 14
const EDGE = 16
/** Толщина обводки окна — столько оставляем до края экрана. */
const RING = 2
/** Столько длится исчезновение, прежде чем туториал уйдёт из дерева. */
const EXIT_MS = 160
/** Перемеры после смены цели: лист выезжает, экран догружается. */
const SETTLE_MS = [120, 300, 600]
/** Цель выше этой доли экрана — список целиком, см. `measure`. */
const TALL = 0.45

interface Layout {
  hole: { x: number; y: number; w: number; h: number; r: number }
  card: { top: number; left: number }
  /** Хвостик карточки: сторона и смещение от её левого края. */
  caret: { side: 'top' | 'bottom'; x: number } | null
}

/** Общий прямоугольник нескольких элементов; нулевые (скрытые) не в счёт. */
function unionRect(els: HTMLElement[]): DOMRect | null {
  const rects = els.map((el) => el.getBoundingClientRect()).filter((r) => r.width > 0 && r.height > 0)
  if (rects.length === 0) return null
  const left = Math.min(...rects.map((r) => r.left))
  const top = Math.min(...rects.map((r) => r.top))
  const right = Math.max(...rects.map((r) => r.right))
  const bottom = Math.max(...rects.map((r) => r.bottom))
  return new DOMRect(left, top, right - left, bottom - top)
}

/** Окно над целью и место карточки: под целью, если влезает, иначе над ней. */
function place(rect: DOMRect, card: HTMLElement, radius: number): { layout: Layout; fits: boolean } {
  const vw = window.innerWidth
  const vh = window.innerHeight
  const cardW = card.offsetWidth
  const cardH = card.offsetHeight

  // Окно не выходит за экран: у вкладок внизу иначе срезалась бы обводка.
  const x = Math.max(rect.left - PAD, RING)
  const y = Math.max(rect.top - PAD, RING)
  const w = Math.min(rect.right + PAD, vw - RING) - x
  const h = Math.min(rect.bottom + PAD, vh - RING) - y
  const hole = { x, y, w, h, r: Math.min(radius + PAD, h / 2) }

  const below = hole.y + hole.h + GAP
  const above = hole.y - GAP - cardH
  const fitsBelow = below + cardH <= vh - EDGE
  const fitsAbove = above >= EDGE
  const side = fitsBelow || !fitsAbove ? 'top' : 'bottom'
  const top = side === 'top' ? Math.min(below, vh - EDGE - cardH) : above

  const center = hole.x + hole.w / 2
  const left = Math.min(Math.max(center - cardW / 2, EDGE), vw - EDGE - cardW)
  const caretX = Math.min(Math.max(center - left, 24), cardW - 24)

  return { layout: { hole, card: { top, left }, caret: { side, x: caretX } }, fits: fitsBelow || fitsAbove }
}

function measure(els: HTMLElement[], card: HTMLElement): Layout {
  let rect = unionRect(els)
  if (!rect) {
    // Без цели «окно» схлопывается в точку в центре: затемнение остаётся
    // сплошным, а к следующей цели окно раскроется оттуда.
    const vw = window.innerWidth
    const vh = window.innerHeight
    return {
      hole: { x: vw / 2, y: vh / 2, w: 0, h: 0, r: 0 },
      card: { top: Math.max(EDGE, (vh - card.offsetHeight) / 2), left: (vw - card.offsetWidth) / 2 },
      caret: null,
    }
  }

  const first = els[0]!
  const radius = parseFloat(getComputedStyle(first).borderTopLeftRadius) || 12
  const vh = window.innerHeight

  // Высокая цель — весь список: он и не должен уместиться. Подсвечиваем
  // видимую часть с начала списка, карточка ложится поверх его низа.
  if (rect.height > vh * TALL) {
    if (rect.top < 0 || rect.top > vh * TALL) {
      first.scrollIntoView?.({ block: 'start' })
      rect = unionRect(els) ?? rect
    }
    const placed = place(rect, card, radius)
    if (placed.fits) return placed.layout
    return { ...placed.layout, card: { ...placed.layout.card, top: vh - EDGE - card.offsetHeight }, caret: null }
  }

  if (rect.top < 0 || rect.bottom > vh) {
    first.scrollIntoView?.({ block: 'center' })
    rect = unionRect(els) ?? rect
  }
  let placed = place(rect, card, radius)
  // Низкий экран: карточке нет места ни над, ни под целью — поднимаем цель
  // к верху ленты, тогда карточка встанет под ней.
  if (!placed.fits) {
    first.scrollIntoView?.({ block: 'start' })
    placed = place(unionRect(els) ?? rect, card, radius)
  }
  return placed.layout
}

/** До первого замера карточка невидима: иначе она мигнула бы в углу. */
function cardStyle(layout: Layout | null): CSSProperties {
  if (!layout) return { visibility: 'hidden' }
  const { card, caret } = layout
  // Карточка вырастает со стороны цели, как всплывающая подсказка.
  const origin = caret ? `${caret.x}px ${caret.side === 'top' ? '0' : '100%'}` : 'center'
  return { top: card.top, left: card.left, transformOrigin: origin }
}

/**
 * Что сейчас на экране: карточка шага, показ нажатия по дороге к нему или
 * мгновенный переход, пока экран не готов.
 */
type View =
  | { mode: 'step'; step: number; path: string[] }
  | { mode: 'tap'; step: number; el: HTMLElement; label: string; pressing: boolean }
  | { mode: 'moving'; step: number }

function prefersReducedMotion(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

export function Tutorial({ onDone }: { onDone: () => void }) {
  const t = useVoice()
  const pace = useContext(TutorialPace)
  const navigate = useNavigate()
  const { data: session } = useSession()
  const [view, setView] = useState<View>({ mode: 'moving', step: 0 })
  const [layout, setLayout] = useState<Layout | null>(null)
  const [closing, setClosing] = useState(false)
  const cardRef = useRef<HTMLDivElement>(null)
  /** Номер текущего прохода: новый проход или закрытие отменяют прежний. */
  const runRef = useRef(0)
  /** Подсказка о нажатии отрисована — можно выдерживать паузу. */
  const shownRef = useRef<(() => void) | null>(null)

  const step = view.step
  const last = step === STEPS.length - 1

  const finish = useCallback(() => {
    runRef.current += 1
    markTutorialSeen()
    setClosing(true)
  }, [])

  const go = useCallback(
    async (to: number, show: boolean) => {
      const run = ++runRef.current
      const alive = () => runRef.current === run
      const { point, press } = prefersReducedMotion() ? { ...pace, press: 0 } : pace
      const path: string[] = []
      setView({ mode: 'moving', step: to })

      for (const tap of STEPS[to]!.path) {
        let label: string | null = null
        if (!tap.done()) {
          const el = await waitFor(tap.find, pace.wait, alive)
          if (!alive()) return
          // Кнопка так и не появилась — дальше пути нет, показываем что есть.
          if (!el) break
          label = tap.label(el)
          if (!tap.done()) {
            if (show && !tap.quiet) {
              await new Promise<void>((resolve) => {
                shownRef.current = resolve
                setView({ mode: 'tap', step: to, el, label: label!, pressing: false })
              })
              await sleep(point)
              if (!alive()) return
              setView({ mode: 'tap', step: to, el, label, pressing: true })
              await sleep(press)
              if (!alive()) return
            }
            el.click()
            await waitFor(() => tap.done() || null, pace.wait, alive)
            if (!alive()) return
          }
        }
        // Уже сделанное нажатие — тоже часть пути: «Каталог» в «Каталог → Вузы».
        if (!tap.quiet) {
          const el = label === null ? tap.find() : null
          if (el) label = tap.label(el)
          if (label) path.push(label)
        }
      }

      const target = STEPS[to]!
      await waitFor(
        () => (target.target().length > 0 || (!loading() && stepTargets(target).length > 0) ? true : null),
        pace.wait,
        alive,
      )
      if (!alive()) return
      setView({ mode: 'step', step: to, path })
    },
    [pace],
  )

  // Первый шаг — с главной; повтор из профиля сначала туда и переходит.
  useEffect(() => {
    void go(0, false)
    return () => {
      runRef.current += 1
    }
  }, [go])

  useEffect(() => {
    if (!closing) return
    const timer = window.setTimeout(onDone, EXIT_MS)
    return () => window.clearTimeout(timer)
  }, [closing, onDone])

  // Замер до отрисовки: карточка сразу появляется на своём месте. Пока идёт
  // мгновенный переход, окно остаётся на прежнем месте.
  useLayoutEffect(() => {
    if (view.mode === 'moving') return
    const update = () => {
      if (!cardRef.current) return
      const els = view.mode === 'tap' ? [view.el] : stepTargets(STEPS[view.step]!)
      setLayout(measure(els, cardRef.current))
    }
    update()
    const timers = SETTLE_MS.map((ms) => window.setTimeout(update, ms))
    window.addEventListener('resize', update)
    window.addEventListener('scroll', update, true)
    return () => {
      timers.forEach((timer) => window.clearTimeout(timer))
      window.removeEventListener('resize', update)
      window.removeEventListener('scroll', update, true)
    }
  }, [view])

  useEffect(() => {
    if (view.mode !== 'tap' || view.pressing) return
    shownRef.current?.()
    shownRef.current = null
  }, [view])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') finish()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [finish])

  const start = () => {
    finish()
    navigate('/')
  }

  const n = step + 1
  const anchored = Boolean(layout?.caret)
  const { key, propose } = STEPS[step]!
  const titleKey = `tutorial.${key}.title` as TextKey
  const textKey = `tutorial.${key}.${propose && session?.permissions.propose ? 'proposeText' : 'text'}` as TextKey

  return (
    <div
      className="tour"
      role="dialog"
      aria-modal="true"
      aria-label={t('tutorial.title')}
      data-anchored={anchored}
      data-mode={view.mode}
      data-closing={closing || undefined}
    >
      {layout ? (
        <span
          className="tour-spot"
          aria-hidden="true"
          data-pressing={(view.mode === 'tap' && view.pressing) || undefined}
          style={{
            width: layout.hole.w,
            height: layout.hole.h,
            borderRadius: layout.hole.r,
            transform: `translate(${layout.hole.x}px, ${layout.hole.y}px)`,
          }}
        />
      ) : null}

      <div ref={cardRef} className="tour-card" style={cardStyle(layout)}>
        {layout?.caret ? (
          <span
            className={`tour-caret tour-caret-${layout.caret.side}`}
            style={{ left: layout.caret.x }}
            aria-hidden="true"
          />
        ) : null}

        {view.mode === 'tap' ? (
          <p className="tour-tap" role="status">
            {t('tutorial.tap', { title: view.label })}
          </p>
        ) : null}

        {view.mode === 'step' ? (
          <>
            <div className="tour-top">
              <span className="tour-dots" role="img" aria-label={t('tutorial.step', { count: n, total: STEPS.length })}>
                {STEPS.map((_, i) => (
                  <i key={i} className={i === step ? 'on' : i < step ? 'done' : undefined} />
                ))}
              </span>
              <span className="tour-count" aria-hidden="true">
                {t('tutorial.count', { count: n, total: STEPS.length })}
              </span>
              <button type="button" className="link tour-skip" onClick={finish}>
                {t('tutorial.skip')}
              </button>
            </div>

            {/* key — чтобы текст нового шага проявлялся заново */}
            <div key={step} className="tour-body" aria-live="polite">
              {view.path.length > 0 ? <p className="tour-path">{view.path.join(' → ')}</p> : null}
              <h2 className="tour-title">{t(titleKey)}</h2>
              <p className="tour-text">{t(textKey)}</p>
            </div>

            <div className="tour-actions">
              {step > 0 ? (
                <Button className="tour-back" variant="secondary" size="medium" onClick={() => void go(step - 1, false)}>
                  {t('tutorial.back')}
                </Button>
              ) : null}
              <Button size="medium" stretched autoFocus onClick={last ? start : () => void go(step + 1, true)}>
                {last ? t('tutorial.start') : t('tutorial.next')}
              </Button>
            </div>
          </>
        ) : null}
      </div>
    </div>
  )
}
