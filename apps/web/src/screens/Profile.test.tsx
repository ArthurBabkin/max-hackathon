import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { keys } from '@/api/queries'
import { profile } from '@/api/mocks/build'
import { renderApp } from '@/test/render'

// Сохранение уходит в «живой» API: моки ответили бы сами и тело не поймать.
vi.hoisted(() => vi.stubEnv('VITE_USE_MOCKS', 'off'))
const { ProfileScreen } = await import('./Profile')

// ТЗ F49: в профиле правятся все поля, включая регион и цель.
it('меняет регион и цель и отправляет их в PATCH /profile', async () => {
  const sent: unknown[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((_url: string, init: RequestInit) => {
      if (init.method === 'PATCH') sent.push(JSON.parse(init.body as string))
      return new Promise(() => {})
    }),
  )
  renderApp(<ProfileScreen />, {
    route: '/profile',
    seed: (c) => {
      c.setQueryData(keys.profile, profile())
      c.setQueryData(keys.universities('', 'all'), { items: [] })
      c.setQueryData(keys.directions, {
        items: [
          { id: 'dir-se', name: 'Программная инженерия' },
          { id: 'dir-math', name: 'Математика' },
        ],
      })
    },
  })

  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Регион' }), 'Москва')
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Цель' }), 'Математика')
  await userEvent.click(screen.getByRole('button', { name: /Сохранить/ }))

  expect(sent).toEqual([expect.objectContaining({ region_code: '77', direction_id: 'dir-math' })])
})
