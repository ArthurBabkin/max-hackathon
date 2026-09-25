import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { beforeEach, expect, it } from 'vitest'
import { keys, useSetUniversityDirections } from './queries'
import { state } from './mocks/state'
import { makeSession } from '@/test/render'

beforeEach(() => {
  state.universities = ['inno', 'kfu', 'hse']
  state.chosen = {}
})

// Каталог «Ведут в мои вузы» считается по направлениям в вузах: выбрали
// направление — список пересчитывается.
it('выбор направлений в вузе обновляет каталог олимпиад', async () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } })
  client.setQueryData(keys.session, makeSession())
  client.setQueryData(keys.olympiads('', 'inf', true), { items: [] })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
  const { result } = renderHook(() => useSetUniversityDirections('hse'), { wrapper })

  act(() => result.current.mutate(['dir-se']))

  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  expect(client.getQueryState(keys.olympiads('', 'inf', true))?.isInvalidated).toBe(true)
})
