import { beforeEach, describe, expect, it } from 'vitest'
import { handleMock } from './index'
import { state } from './state'
import { ApiError } from '../errors'
import type { Home, Tracker, TrackerItem } from '@contract'

const asKid = () => {
  state.viewerId = 'mem-artem'
}
const asParent = () => {
  state.viewerId = 'mem-olga'
}

beforeEach(() => {
  asKid()
  state.tracker = [
    { id: 'tr-1', profileId: 'hse:inf', added_by: 'mem-olga', created_at: '', registered_at: null, registered_by: null },
  ]
  state.proposals = [
    { id: 'pr-1', profileId: 'tk:inf', proposed_by: 'mem-olga', status: 'pending', created_at: '', resolved_at: null },
  ]
  state.members = [
    { id: 'mem-olga', name: 'Ольга', role: 'parent', is_creator: true, color: '#E92E78', joined_at: '' },
    { id: 'mem-artem', name: 'Артём', role: 'kid', is_creator: false, color: '#FF8A3D', joined_at: '' },
  ]
})

describe('диспетчер маршрутов', () => {
  it('на неизвестную ручку не отвечает — запрос уходит в настоящий API', async () => {
    await expect(handleMock('GET', '/чего-нет')).resolves.toBeUndefined()
  })

  it('различает /tracker/:id и /tracker/:id/registered', async () => {
    const marked = (await handleMock('PUT', '/tracker/tr-1/registered')) as TrackerItem
    expect(marked.registered_at).not.toBeNull()
    expect(state.tracker).toHaveLength(1)

    await handleMock('DELETE', '/tracker/tr-1')
    expect(state.tracker).toHaveLength(0)
  })

  it('разбирает query-параметры', async () => {
    const empty = (await handleMock('GET', '/olympiads?q=такого-нет')) as { items: unknown[] }
    expect(empty.items).toEqual([])

    const found = (await handleMock('GET', '/olympiads?q=проба')) as { items: unknown[] }
    expect(found.items).toHaveLength(1)
  })
})

describe('матрица прав ТЗ §3.1 на сервере мока', () => {
  it('ученик добавляет в трекер сам', async () => {
    const item = (await handleMock('POST', '/tracker', {
      olympiad_profile_id: 'lomo:inf',
    })) as TrackerItem
    expect(item.olympiad_profile_id).toBe('lomo:inf')
  })

  it('родитель при ученике в траектории получает 403', async () => {
    asParent()
    await expect(
      handleMock('POST', '/tracker', { olympiad_profile_id: 'lomo:inf' }),
    ).rejects.toMatchObject({ status: 403, code: 'FORBIDDEN' })
  })

  it('родитель без ученика в траектории добавляет сам', async () => {
    asParent()
    state.members = state.members.filter((m) => m.role !== 'kid')
    const item = (await handleMock('POST', '/tracker', {
      olympiad_profile_id: 'lomo:inf',
    })) as TrackerItem
    expect(item.olympiad_profile_id).toBe('lomo:inf')
  })

  it('отвечать на предложение может только ученик', async () => {
    asParent()
    await expect(handleMock('POST', '/proposals/pr-1/accept')).rejects.toBeInstanceOf(ApiError)
  })
})

describe('трекер и предложения', () => {
  it('повторное добавление не создаёт дубль (F28)', async () => {
    await handleMock('POST', '/tracker', { olympiad_profile_id: 'hse:inf' })
    expect(state.tracker.filter((i) => i.profileId === 'hse:inf')).toHaveLength(1)
  })

  it('принятое предложение попадает в трекер и закрывается', async () => {
    await handleMock('POST', '/proposals/pr-1/accept')
    expect(state.tracker.some((i) => i.profileId === 'tk:inf')).toBe(true)
    expect(state.proposals[0]!.status).toBe('accepted')
  })

  it('повторный ответ на закрытое предложение — 409', async () => {
    await handleMock('POST', '/proposals/pr-1/accept')
    await expect(handleMock('POST', '/proposals/pr-1/decline')).rejects.toMatchObject({
      status: 409,
    })
  })

  it('отклонённое предложение в трекер не попадает', async () => {
    await handleMock('POST', '/proposals/pr-1/decline')
    expect(state.tracker.some((i) => i.profileId === 'tk:inf')).toBe(false)
    expect(state.proposals[0]!.status).toBe('declined')
  })

  it('предложения отдаются только пока ждут ответа', async () => {
    const before = (await handleMock('GET', '/tracker')) as Tracker
    expect(before.proposals).toHaveLength(1)

    await handleMock('POST', '/proposals/pr-1/decline')
    const after = (await handleMock('GET', '/tracker')) as Tracker
    expect(after.proposals).toHaveLength(0)
  })
})

describe('главная', () => {
  it('следующий шаг — ближайший неотмеченный пункт (ТЗ §6.6)', async () => {
    const home = (await handleMock('GET', '/home')) as Home
    expect(home.next_step?.olympiad_profile_id).toBe('hse:inf')
  })

  it('когда всё отмечено, следующего шага нет — экран пишет «Всё по плану»', async () => {
    await handleMock('PUT', '/tracker/tr-1/registered')
    const home = (await handleMock('GET', '/home')) as Home
    expect(home.next_step).toBeNull()
    expect(home.registered_count).toBe(1)
  })
})
