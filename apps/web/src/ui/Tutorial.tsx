/**
 * Туториал для новичка: четыре шага о том, где что лежит. Нижний лист поверх
 * приложения; «Пропустить» и последний шаг закрывают его и запоминают, что
 * пользователь его видел.
 */

import { useEffect, useState } from 'react'
import { Button } from '@maxhub/max-ui'
import { markTutorialSeen } from '@/lib/tutorial'
import type { TextKey } from '@/voice/texts'
import { useVoice } from '@/voice/useVoice'
import { Icon, type IconName } from './Icon'

const STEPS: IconName[] = ['target', 'doc', 'calendar', 'users']

export function Tutorial({ onDone }: { onDone: () => void }) {
  const t = useVoice()
  const [step, setStep] = useState(0)
  const last = step === STEPS.length - 1

  const finish = () => {
    markTutorialSeen()
    onDone()
  }

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') finish()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  const n = step + 1
  return (
    <>
      <div className="sheet-backdrop" aria-hidden="true" />
      <div className="sheet tutorial" role="dialog" aria-modal="true" aria-label={t('tutorial.title')}>
        <span className="sheet-grab" aria-hidden="true" />
        <div className="tutorial-top">
          <span className="tutorial-dots" aria-label={t('tutorial.step', { count: n })}>
            {STEPS.map((_, i) => (
              <i key={i} className={i === step ? 'on' : undefined} />
            ))}
          </span>
          <button type="button" className="link" onClick={finish}>
            {t('tutorial.skip')}
          </button>
        </div>

        {/* key — чтобы шаг заново проигрывал появление */}
        <div key={step} className="tutorial-step">
          <span className="tutorial-icon">
            <Icon name={STEPS[step]!} size={28} />
          </span>
          <h2>{t(`tutorial.${n}.title` as TextKey)}</h2>
          <p>{t(`tutorial.${n}.text` as TextKey)}</p>
        </div>

        <div className="tutorial-actions">
          <Button stretched autoFocus onClick={last ? finish : () => setStep(step + 1)}>
            {last ? t('tutorial.start') : t('tutorial.next')}
          </Button>
        </div>
      </div>
    </>
  )
}
