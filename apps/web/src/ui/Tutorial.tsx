/**
 * Туториал для новичка: пять шагов, каждый подсвечивает настоящий элемент
 * интерфейса (вкладку, олимпиаду, кнопку «Спросить»), а карточка с
 * пояснением встаёт рядом с ним. Элементы помечены атрибутом `data-tour`.
 * Если элемента на экране нет, карточка встаёт по центру. «Пропустить»,
 * Esc и последний шаг закрывают туториал и запоминают, что его видели.
 */

import { type CSSProperties, useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Button } from '@maxhub/max-ui'
import { markTutorialSeen } from '@/lib/tutorial'
import type { TextKey } from '@/voice/texts'
import { useVoice } from '@/voice/useVoice'

/** Цели шагов: берётся первая, что нашлась на экране. */
const STEPS: string[][] = [
  ['[data-tour="match"]'],
  // Пустой список сроков у новичка — обычное дело: тогда показываем «Каталог».
  ['[data-tour="upcoming"] > :first-child', '[data-tour="catalog"]'],
  ['[data-tour="tracker"]'],
  ['[data-tour="family"]'],
  ['[data-tour="ai"]'],
]

/** Поле вокруг цели, зазор до карточки и отступ от краёв экрана. */
const PAD = 6
const GAP = 14
const EDGE = 16
/** Толщина обводки окна — столько оставляем до края экрана. */
const RING = 2
/** Столько длится исчезновение, прежде чем туториал уйдёт из дерева. */
const EXIT_MS = 160

interface Layout {
  hole: { x: number; y: number; w: number; h: number; r: number }
  card: { top: number; left: number }
  /** Хвостик карточки: сторона и смещение от её левого края. */
  caret: { side: 'top' | 'bottom'; x: number } | null
}

function findTarget(selectors: string[]): HTMLElement | null {
  for (const selector of selectors) {
    const el = document.querySelector<HTMLElement>(selector)
    const rect = el?.getBoundingClientRect()
    if (el && rect && rect.width > 0 && rect.height > 0) return el
  }
  return null
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

function measure(selectors: string[], card: HTMLElement): Layout {
  const el = findTarget(selectors)
  if (!el) {
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

  const radius = parseFloat(getComputedStyle(el).borderTopLeftRadius) || 12
  let rect = el.getBoundingClientRect()
  if (rect.top < 0 || rect.bottom > window.innerHeight) {
    el.scrollIntoView?.({ block: 'center' })
    rect = el.getBoundingClientRect()
  }
  let placed = place(rect, card, radius)
  // Низкий экран: карточке нет места ни над, ни под целью — поднимаем цель
  // к верху ленты, тогда карточка встанет под ней.
  if (!placed.fits) {
    el.scrollIntoView?.({ block: 'start' })
    placed = place(el.getBoundingClientRect(), card, radius)
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

export function Tutorial({ onDone }: { onDone: () => void }) {
  const t = useVoice()
  const [step, setStep] = useState(0)
  const [layout, setLayout] = useState<Layout | null>(null)
  const [closing, setClosing] = useState(false)
  const cardRef = useRef<HTMLDivElement>(null)
  const nextRef = useRef<HTMLButtonElement>(null)
  const last = step === STEPS.length - 1

  const finish = useCallback(() => {
    markTutorialSeen()
    setClosing(true)
  }, [])

  useEffect(() => {
    if (!closing) return
    const timer = window.setTimeout(onDone, EXIT_MS)
    return () => window.clearTimeout(timer)
  }, [closing, onDone])

  // Замер до отрисовки: карточка сразу появляется на своём месте.
  useLayoutEffect(() => {
    const update = () => {
      if (cardRef.current) setLayout(measure(STEPS[step]!, cardRef.current))
    }
    update()
    window.addEventListener('resize', update)
    return () => window.removeEventListener('resize', update)
  }, [step])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') finish()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [finish])

  const back = () => {
    setStep(step - 1)
    // На первом шаге «Назад» исчезает — фокус не должен пропасть вместе с ним.
    if (step === 1) nextRef.current?.focus()
  }

  const n = step + 1
  const anchored = Boolean(layout?.caret)

  return (
    <div
      className="tour"
      role="dialog"
      aria-modal="true"
      aria-label={t('tutorial.title')}
      data-anchored={anchored}
      data-closing={closing || undefined}
    >
      {layout ? (
        <span
          className="tour-spot"
          aria-hidden="true"
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

        <div className="tour-top">
          <span className="tour-dots" role="img" aria-label={t('tutorial.step', { count: n })}>
            {STEPS.map((_, i) => (
              <i key={i} className={i === step ? 'on' : i < step ? 'done' : undefined} />
            ))}
          </span>
          <button type="button" className="link tour-skip" onClick={finish}>
            {t('tutorial.skip')}
          </button>
        </div>

        {/* key — чтобы текст нового шага проявлялся заново */}
        <div key={step} className="tour-body" aria-live="polite">
          <h2 className="tour-title">{t(`tutorial.${n}.title` as TextKey)}</h2>
          <p className="tour-text">{t(`tutorial.${n}.text` as TextKey)}</p>
        </div>

        <div className="tour-actions">
          {step > 0 ? (
            <Button className="tour-back" variant="secondary" size="medium" onClick={back}>
              {t('tutorial.back')}
            </Button>
          ) : null}
          <Button ref={nextRef} size="medium" stretched autoFocus onClick={last ? finish : () => setStep(step + 1)}>
            {last ? t('tutorial.start') : t('tutorial.next')}
          </Button>
        </div>
      </div>
    </div>
  )
}
