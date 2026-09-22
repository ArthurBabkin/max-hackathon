import type { ErrorCode } from '@contract'

/**
 * Ошибка API в едином виде.
 *
 * По ТЗ §10 сервер отдаёт `{"error": {"code", "message"}}`. Скелет apps/api
 * пока отвечает строкой (`{"error": "not_implemented"}`), поэтому разбор
 * принимает обе формы — это три строки, зато фронт не ломается, пока бэкенд
 * приводит ответ к ТЗ.
 */
export class ApiError extends Error {
  readonly code: ErrorCode | string
  readonly status: number

  constructor(status: number, code: ErrorCode | string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }

  /** Истёк JWT — клиенту нужно заново позвать POST /session. */
  get isUnauthorized(): boolean {
    return this.status === 401
  }

  /** Ручки ещё нет на сервере. */
  get isNotImplemented(): boolean {
    return this.status === 501
  }

  /** Сети нет вообще — экран H4, а не сообщение сервера. */
  get isOffline(): boolean {
    return this.status === 0
  }
}

const FALLBACK_MESSAGES: Record<number, string> = {
  0: 'Нет соединения с интернетом',
  401: 'Сессия истекла',
  403: 'Действие недоступно',
  404: 'Не найдено',
  429: 'Слишком много запросов, попробуйте чуть позже',
  500: 'Что-то сломалось на сервере',
  501: 'Эта часть сервиса ещё не готова',
}

export function toApiError(status: number, payload: unknown): ApiError {
  const error = (payload as { error?: unknown } | null)?.error

  if (error && typeof error === 'object') {
    const { code, message } = error as { code?: string; message?: string }
    return new ApiError(status, code ?? 'INTERNAL', message ?? fallback(status))
  }

  // Форма скелета: error строкой.
  if (typeof error === 'string') return new ApiError(status, error, fallback(status))

  return new ApiError(status, 'INTERNAL', fallback(status))
}

function fallback(status: number): string {
  return FALLBACK_MESSAGES[status] ?? 'Не удалось выполнить запрос'
}
