/**
 * Выбор темы оформления в профиле (F61). Поле формы: тема применяется вместе
 * с остальным профилем по «Сохранить».
 */

import { useId } from 'react'
import { useVoice } from '@/voice/useVoice'
import { THEME_CHOICES, type ThemeChoice } from './theme'

export function ThemeSetting({ value, onChange }: { value: ThemeChoice; onChange: (next: ThemeChoice) => void }) {
  const t = useVoice()
  const label = useId()

  return (
    <div className="field">
      <p className="field-label" id={label}>
        {t('profile.themeLabel')}
      </p>
      {/* Кнопки как у «Класса» выше: одна форма — один вид выбора. */}
      <div className="grades grades-3" role="radiogroup" aria-labelledby={label}>
        {THEME_CHOICES.map((choice) => (
          <button
            key={choice}
            type="button"
            role="radio"
            aria-checked={value === choice}
            className={value === choice ? 'grade grade-text grade-on' : 'grade grade-text'}
            onClick={() => onChange(choice)}
          >
            {t(`theme.${choice}`)}
          </button>
        ))}
      </div>
      <p className="field-note">{t('profile.themeNote')}</p>
    </div>
  )
}
