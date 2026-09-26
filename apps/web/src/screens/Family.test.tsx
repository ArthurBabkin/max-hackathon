import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Family, Invite, Member } from '@contract'
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

const invite = (over: Partial<Invite>): Invite => ({
  id: 'inv-x',
  token: 'tokenTokenToken',
  url: 'https://max.ru/bot?start=inv_tokenTokenToken',
  role: 'parent',
  can_revoke: true,
  created_at: '2026-09-01T00:00:00Z',
  ...over,
})

function setup(session: ReturnType<typeof makeSession>, members: Member[], invites: Invite[] = []) {
  return renderApp(<FamilyScreen />, {
    session,
    route: '/family',
    seed: (c) => c.setQueryData(keys.family, { members, invites } as unknown as Family),
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

// ТЗ F42: роль приглашённого выбирает тот, кто создаёт ссылку.
it('без ученика можно пригласить и ученика, и второго родителя', async () => {
  const post = vi.spyOn(api, 'post').mockReturnValue(new Promise(() => {}))
  setup(makeSession({ role: 'parent', is_creator: true }), [
    member({ id: 'm-1', name: 'Ольга', is_creator: true, is_me: true, can_remove: false }),
  ])

  expect(screen.getByRole('button', { name: 'Пригласить ученика' })).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'Пригласить родителя' }))
  expect(post).toHaveBeenCalledWith('/family/invites', { role: 'parent' })
})

it('при ученике в траектории — только «Пригласить родителя»', () => {
  setup(makeSession({ role: 'kid', is_creator: true }), [
    member({ id: 'm-1', name: 'Артём', role: 'kid', is_creator: true, is_me: true, can_remove: false }),
  ])

  expect(screen.queryByRole('button', { name: 'Пригласить ученика' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Пригласить родителя' })).toBeInTheDocument()
})

// ТЗ §16: неиспользованную ссылку можно отозвать — автору или создателю.
it('все активные ссылки видны; своя отзывается со второго нажатия', async () => {
  const remove = vi.spyOn(api, 'delete').mockResolvedValue(undefined)
  setup(
    makeSession({ role: 'parent' }),
    [
      member({ id: 'm-1', name: 'Артём', role: 'kid', is_creator: true, can_remove: false }),
      member({ id: 'm-2', name: 'Ольга', is_me: true, can_remove: false }),
    ],
    [invite({ id: 'inv-1', can_revoke: false }), invite({ id: 'inv-2', url: 'https://max.ru/bot?start=inv_second' })],
  )

  expect(screen.getByText('Ждут подключения: 2')).toBeInTheDocument()
  expect(screen.getAllByText('Ссылка для родителя')).toHaveLength(2)
  const revoke = screen.getAllByRole('button', { name: 'Отозвать' })
  expect(revoke).toHaveLength(1)

  await userEvent.click(revoke[0]!)
  expect(remove).not.toHaveBeenCalled()
  await userEvent.click(screen.getByRole('button', { name: 'Точно отозвать?' }))
  expect(remove).toHaveBeenCalledWith('/family/invites/inv-2')
})
