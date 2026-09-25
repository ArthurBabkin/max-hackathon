import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Family, Member } from '@contract'
import { afterEach, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import { keys } from '@/api/queries'
import { makeSession, renderApp } from '@/test/render'
import { FamilyScreen } from './Family'

const member = (over: Partial<Member>): Member =>
  ({
    id: 'm-x',
    name: 'Игорь',
    role: 'parent',
    is_creator: false,
    is_me: false,
    can_remove: true,
    color: null,
    joined_at: '2026-09-01T00:00:00Z',
    ...over,
  }) as Member

function setup(session: ReturnType<typeof makeSession>, members: Member[]) {
  return renderApp(<FamilyScreen />, {
    session,
    route: '/family',
    seed: (c) => c.setQueryData(keys.family, { members, invites: [] } as unknown as Family),
  })
}

afterEach(() => vi.restoreAllMocks())

it('создатель удаляет участника только со второго нажатия', async () => {
  const remove = vi.spyOn(api, 'delete').mockResolvedValue(undefined)
  setup(makeSession({ role: 'parent', is_creator: true }), [
    member({ id: 'm-1', name: 'Ольга', is_creator: true, is_me: true, can_remove: false }),
    member({ id: 'm-2', name: 'Игорь' }),
  ])

  await userEvent.click(screen.getByRole('button', { name: 'Удалить' }))
  expect(remove).not.toHaveBeenCalled()

  await userEvent.click(screen.getByRole('button', { name: 'Точно удалить?' }))
  expect(remove).toHaveBeenCalledWith('/family/members/m-2')
})

it('приглашённый выходит из траектории только со второго нажатия', async () => {
  const post = vi.spyOn(api, 'post').mockResolvedValue(undefined)
  setup(makeSession({ role: 'kid' }), [
    member({ id: 'm-1', name: 'Ольга', is_creator: true, can_remove: false }),
    member({ id: 'm-2', name: 'Артём', role: 'kid', is_me: true, can_remove: false }),
  ])

  await userEvent.click(screen.getByRole('button', { name: 'Выйти из траектории' }))
  expect(post).not.toHaveBeenCalled()

  await userEvent.click(screen.getByRole('button', { name: 'Точно выйти?' }))
  expect(post).toHaveBeenCalledWith('/family/leave')
})
