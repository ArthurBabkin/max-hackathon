/**
 * HTTP-клиент мини-приложения.
 *
 * Локально запросы идут на `/api/v1` — nginx веб-контейнера проксирует их
 * в Go-сервис, поэтому запрос остаётся same-origin и preflight с
 * Authorization не возникает. В проде адрес подставляется при сборке.
 */

import { toApiError } from './errors'
import { buildQuery } from './query'

const BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api/v1'

/**
 * Моки живут только в разработке. `import.meta.env.DEV` Vite заменяет
 * константой на сборке, поэтому и ветка, и динамический импорт целиком
 * выпадают из прод-бандла: прод ходит только в живой API.
 */
const MOCKS_ENABLED =
  import.meta.env.DEV && (import.meta.env.VITE_USE_MOCKS as string | undefined) !== 'off'

let token: string | null = null

export function setToken(value: string | null): void {
  token = value
}

export function hasToken(): boolean {
  return token !== null
}

/**
 * Как получить новый токен. JWT живёт час, а ещё сервер отвечает 401, если
 * участника удалили из траектории, — контракт обещает, что клиент тогда
 * повторяет POST /session. Функцию регистрирует слой запросов (queries.ts),
 * здесь о сессии ничего не знают.
 */
let reauth: (() => Promise<void>) | null = null
let reauthInFlight: Promise<void> | null = null

export function setReauth(fn: (() => Promise<void>) | null): void {
  reauth = fn
}

/** Один повторный вход на все запросы, получившие 401 одновременно. */
function reauthenticate(): Promise<void> {
  reauthInFlight ??= (reauth ?? (() => Promise.resolve()))().finally(() => {
    reauthInFlight = null
  })
  return reauthInFlight
}

export type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

export interface RequestOptions {
  query?: Record<string, string | number | undefined | null>
  body?: unknown
  /** Запрос без JWT — только POST /session. */
  anonymous?: boolean
  /** Внутреннее: запрос уже повторён после нового входа. */
  retried?: boolean
}

function buildPath(path: string, query: RequestOptions['query']): string {
  return `${path}${buildQuery(query)}`
}

export async function request<T>(
  method: Method,
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const fullPath = buildPath(path, options.query)

  if (MOCKS_ENABLED) {
    const { handleMock } = await import('./mocks')
    const mocked = await handleMock<T>(method, fullPath, options.body)
    if (mocked !== undefined) return mocked
  }

  const headers: Record<string, string> = { Accept: 'application/json' }
  if (options.body !== undefined) headers['Content-Type'] = 'application/json'
  if (!options.anonymous && token) headers.Authorization = `Bearer ${token}`

  let response: Response
  try {
    response = await fetch(`${BASE}${fullPath}`, {
      method,
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
    })
  } catch {
    // fetch бросает только когда до сервера не достучались: нет сети,
    // оборвалось соединение, заблокировал браузер. Статус 0 отличает это
    // от честного ответа сервера с ошибкой.
    throw toApiError(0, null)
  }

  if (response.status === 204) return undefined as T

  if (response.status === 401 && !options.anonymous && !options.retried && reauth) {
    token = null
    await reauthenticate()
    return request<T>(method, path, { ...options, retried: true })
  }

  const payload = await response.json().catch(() => null)
  if (!response.ok) throw toApiError(response.status, payload)
  return payload as T
}

/**
 * Полный адрес ресурса API — для внешнего браузера (`openLink`): он не знает,
 * где живёт API, а в проде BASE — другой домен. Уже полный адрес (Blob URL
 * моков разработки) возвращается как есть.
 */
export function apiUrl(path: string): string {
  if (/^[a-z][a-z\d+.-]*:/i.test(path)) return path
  return new URL(`${BASE}${path}`, location.href).href
}

export const api = {
  get: <T>(path: string, query?: RequestOptions['query']) => request<T>('GET', path, { query }),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, { body }),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, { body }),
  patch: <T>(path: string, body?: unknown) => request<T>('PATCH', path, { body }),
  delete: <T>(path: string) => request<T>('DELETE', path),
}
