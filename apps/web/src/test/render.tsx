/**
 * Общая обвязка для компонентных тестов: провайдеры, роутер и подменённая
 * сессия. Моки тут не нужны — тесты задают данные напрямую, иначе проверялась
 * бы задержка моков, а не поведение компонента.
 */

import type { ReactElement, ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { render } from '@testing-library/react'
import type { Permissions, Role, Session } from '@contract'
import { keys } from '@/api/queries'
import { derivePermissions } from '@/lib/permissions'

export function makeSession(over: { role?: Role; is_creator?: boolean; has_kid?: boolean } = {}): Session {
  const role = over.role ?? 'kid'
  const is_creator = over.is_creator ?? false
  const has_kid = over.has_kid ?? true

  return {
    user: { id: 'u-1', max_user_id: 1, first_name: role === 'kid' ? 'Артём' : 'Ольга' },
    member: { id: 'm-1', role, is_creator, reminder_offsets: [30, 7, 3, 1] },
    trajectory: {
      id: 'trj-1',
      student_name: 'Артём',
      grade: 9,
      region_code: '16',
      region_name: 'Республика Татарстан',
      directions: [{ id: 'dir-se', name: 'Программная инженерия' }],
      goal_status: 'known',
      has_kid,
      members_count: 2,
    },
    permissions: derivePermissions({ role, is_creator, has_kid }) as Permissions,
    start_param: null,
  }
}

export interface RenderOptions {
  session?: Session
  route?: string
  /** Данные, которые надо положить в кеш до отрисовки. */
  seed?: (client: QueryClient) => void
}

export function renderApp(ui: ReactElement, options: RenderOptions = {}) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })

  client.setQueryData(keys.session, options.session ?? makeSession())
  options.seed?.(client)

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[options.route ?? '/']}>{children}</MemoryRouter>
    </QueryClientProvider>
  )

  return { ...render(ui, { wrapper }), client }
}
