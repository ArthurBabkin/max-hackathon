/**
 * Экран ошибки загрузки (ТЗ §4.11, экран H4): что случилось, что сохранено
 * и можно ли повторить. Причина берётся из самой ошибки — «нет интернета»
 * пишется только когда до сервера правда не достучались.
 */

import { Button } from '@maxhub/max-ui'
import { errorKind, serverMessage, type ErrorKind } from '@/api/errors'
import { useVoice, type Translate } from '@/voice/useVoice'
import type { TextKey } from '@/voice/texts'
import { Icon, type IconName } from './Icon'
import { StateBlock } from './primitives'

interface ErrorView {
  icon: IconName
  /** Заголовок по причине; нет — остаётся заголовок экрана. */
  title?: TextKey
  text: string
  retry: boolean
}

/** Что показать по виду ошибки. Экспортируется для всплывающих сообщений. */
export function errorView(kind: ErrorKind, error: unknown, t: Translate): ErrorView {
  const fromServer = serverMessage(error)
  switch (kind) {
    case 'offline':
      return { icon: 'wifiOff', text: t('state.errorText'), retry: true }
    case 'busy':
      return { icon: 'clock', text: t('state.busyText'), retry: true }
    case 'unauthorized':
      return { icon: 'shield', title: 'state.unauthorizedTitle', text: t('state.unauthorizedText'), retry: true }
    case 'forbidden':
      return { icon: 'shield', title: 'state.forbiddenTitle', text: fromServer ?? t('state.forbidden'), retry: false }
    case 'notFound':
      return { icon: 'search', title: 'state.notFoundTitle', text: t('state.notFoundText'), retry: false }
    case 'notReady':
      return { icon: 'clock', title: 'state.notReadyTitle', text: t('state.notReadyText'), retry: false }
    case 'rejected':
      return { icon: 'alert', text: fromServer ?? t('state.serverText'), retry: true }
    case 'server':
      return { icon: 'alert', text: t('state.serverText'), retry: true }
  }
}

interface ErrorStateProps {
  error: unknown
  /** Заголовок экрана: «Подбор не загрузился». По умолчанию «Не загрузилось». */
  title?: string
  onRetry?: () => void
}

export function ErrorState({ error, title, onRetry }: ErrorStateProps) {
  const t = useVoice()
  const view = errorView(errorKind(error), error, t)
  return (
    <StateBlock
      icon={view.icon}
      tone="error"
      title={view.title ? t(view.title) : (title ?? t('state.errorTitle'))}
      text={view.text}
    >
      {view.retry && onRetry ? (
        <Button stretched iconBefore={<Icon name="refresh" size={16} />} onClick={onRetry}>
          {t('state.errorRetry')}
        </Button>
      ) : null}
    </StateBlock>
  )
}
