/** Всплывающее сообщение об ошибке действия (см. toast.ts). */

import { useEffect } from 'react'
import { errorKind, serverMessage } from '@/api/errors'
import { useVoice, type Translate } from '@/voice/useVoice'
import { Icon } from './Icon'
import { dismissToast, useErrorToast } from './toast'

/** Сколько висит сообщение: хватает прочитать две строки. */
const TOAST_MS = 5000

/** Текст по причине. Отказ сервера (4xx) объясняет сам сервер. */
export function toastText(error: unknown, t: Translate): string {
  switch (errorKind(error)) {
    case 'offline':
      return t('toast.errorOffline')
    case 'busy':
      return t('state.busyText')
    case 'unauthorized':
      return t('state.unauthorizedText')
    case 'server':
      return t('toast.errorServer')
    default:
      return serverMessage(error) ?? t('toast.errorServer')
  }
}

export function Toaster() {
  const t = useVoice()
  const toast = useErrorToast()

  useEffect(() => {
    if (!toast) return
    const timer = setTimeout(() => dismissToast(toast.id), TOAST_MS)
    return () => clearTimeout(timer)
  }, [toast])

  if (!toast) return null
  if (toast.info) {
    return (
      <div className="toast toast-info" role="status">
        <Icon name="check" size={18} className="toast-icon" />
        <span className="toast-text">{toast.info}</span>
        <button type="button" className="toast-close" aria-label={t('toast.dismiss')} onClick={() => dismissToast(toast.id)}>
          <Icon name="close" size={16} />
        </button>
      </div>
    )
  }
  const offline = errorKind(toast.error) === 'offline'
  return (
    <div className="toast" role="alert">
      <Icon name={offline ? 'wifiOff' : 'alert'} size={18} className="toast-icon" />
      <span className="toast-text">{toastText(toast.error, t)}</span>
      <button type="button" className="toast-close" aria-label={t('toast.dismiss')} onClick={() => dismissToast(toast.id)}>
        <Icon name="close" size={16} />
      </button>
    </div>
  )
}
