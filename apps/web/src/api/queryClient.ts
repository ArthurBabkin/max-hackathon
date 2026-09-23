/**
 * Настройки кеша запросов — общие для приложения и тестов.
 */

import { MutationCache, QueryClient, type QueryClientConfig } from '@tanstack/react-query'
import { showErrorToast } from '@/ui/toast'
import { isRetryable } from './errors'

declare module '@tanstack/react-query' {
  interface Register {
    mutationMeta: {
      /** Ошибку показывает сам экран — всплывающее сообщение не нужно. */
      silentError?: boolean
    }
  }
}

/**
 * Один повтор — только при временном сбое. 404 «анкета не пройдена» или 400
 * повторять незачем: ответ будет тот же, а экран ошибки появится позже.
 */
export function retryOnce(failureCount: number, error: Error): boolean {
  return failureCount < 1 && isRetryable(error)
}

/** Ошибки действий — всплывающим сообщением, если экран не показал их сам. */
export function errorToastCache(): MutationCache {
  return new MutationCache({
    onError: (error, _variables, _context, mutation) => {
      if (!mutation.meta?.silentError) showErrorToast(error)
    },
  })
}

export function createQueryClient(config: QueryClientConfig = {}): QueryClient {
  return new QueryClient({
    mutationCache: errorToastCache(),
    ...config,
    defaultOptions: {
      ...config.defaultOptions,
      queries: {
        // Мини-приложение живёт секунды-минуты, данные за это время не устаревают.
        staleTime: 60_000,
        // Мобильная сеть рвётся — один повтор оправдан, дальше показываем H4.
        retry: retryOnce,
        refetchOnWindowFocus: false,
        ...config.defaultOptions?.queries,
      },
    },
  })
}
