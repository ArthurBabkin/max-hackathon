/**
 * Нижний лист. Карточки олимпиады и вуза открываются стопкой, поэтому у листа
 * два способа закрыться: «Назад» снимает верхний, крестик закрывает всю стопку
 * (ТЗ §7.1).
 */

import { useEffect, type ReactNode } from 'react'
import { IconButton } from '@maxhub/max-ui'
import { Icon } from './Icon'

export interface SheetProps {
  label: string
  /** Есть ли под этим листом другой — тогда показываем «Назад». */
  canGoBack: boolean
  onBack: () => void
  onClose: () => void
  header?: ReactNode
  children: ReactNode
  /** Лист помощника занимает почти весь экран и скроллится внутри. */
  tall?: boolean
}

export function Sheet({ label, canGoBack, onBack, onClose, header, children, tall = false }: SheetProps) {
  // Пока лист открыт, фон под ним не должен прокручиваться: иначе на телефоне
  // экран уезжает под листом и возвращается уже не туда.
  useEffect(() => {
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previous
    }
  }, [])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <>
      <div className="sheet-backdrop" onClick={onClose} aria-hidden="true" />
      <div className={`sheet${tall ? ' sheet-tall' : ''}`} role="dialog" aria-modal="true" aria-label={label}>
        <span className="sheet-grab" aria-hidden="true" />
        {canGoBack ? (
          <button type="button" className="sheet-back" onClick={onBack}>
            <Icon name="back" size={16} />
            Назад
          </button>
        ) : null}
        <div className="sheet-head">
          {header}
          <IconButton variant="secondary" size="small" className="sheet-close" data-tour="sheet-close" onClick={onClose}>
            <Icon name="close" size={15} title="Закрыть" />
          </IconButton>
        </div>
        {tall ? children : <div className="sheet-body">{children}</div>}
      </div>
    </>
  )
}
