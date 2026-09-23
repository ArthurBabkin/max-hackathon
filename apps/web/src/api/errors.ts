import type { ErrorCode } from '@contract'

/**
 * Ошибка API в едином виде.
 *
 * По ТЗ §10 сервер отдаёт `{"error": {"code", "message"}}`. Любое другое тело
 * (прокси, шлюз, обрыв) сводится к INTERNAL с текстом по статусу.
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

  return new ApiError(status, 'INTERNAL', fallback(status))
}

function fallback(status: number): string {
  return FALLBACK_MESSAGES[status] ?? 'Не удалось выполнить запрос'
}

/**
 * Вид ошибки — от него зависят текст, иконка и есть ли смысл повторять.
 * Сообщения сервера по контракту пишутся по-русски для пользователя, поэтому
 * там, где причина в самом запросе (4xx), показывается текст сервера.
 */
export type ErrorKind =
  | 'offline' // до сервера не достучались
  | 'unauthorized' // вход не подтвердился и после повторного POST /session
  | 'forbidden'
  | 'notFound'
  | 'rejected' // сервер отклонил запрос: 400, 409 и прочие 4xx
  | 'busy' // 429
  | 'notReady' // 501
  | 'server' // 5xx и всё, что не ApiError

export function errorKind(error: unknown): ErrorKind {
  if (!(error instanceof ApiError)) return 'server'
  switch (error.status) {
    case 0:
      return 'offline'
    case 401:
      return 'unauthorized'
    case 403:
      return 'forbidden'
    case 404:
      return 'notFound'
    case 429:
      return 'busy'
    case 501:
      return 'notReady'
  }
  return error.status >= 400 && error.status < 500 ? 'rejected' : 'server'
}

/** Повтор помогает, только если сбой временный: сеть, сервер, лимит запросов. */
export function isRetryable(error: unknown): boolean {
  const kind = errorKind(error)
  return kind === 'offline' || kind === 'server' || kind === 'busy'
}

/** Текст сервера, если он есть: для 4xx он и есть объяснение. */
export function serverMessage(error: unknown): string | null {
  return error instanceof ApiError && error.message ? error.message : null
}
