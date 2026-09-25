import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { handleMock } from './index'
import { state } from './state'
import { ApiError } from '../errors'
import type {
  AiChat,
  AiExchange,
  AiMessage,
  CalendarLink,
  CatalogUniversity,
  Home,
  OlympiadDetail,
  OlympiadListItem,
  Profile,
  Tracker,
  TrackerItem,
  UniversityDetail,
} from '@contract'

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
  state.aiChats = []
  state.ai = []
  state.universities = ['inno', 'kfu', 'hse']
  state.directions = [{ id: 'dir-se', name: 'Программная инженерия' }]
  state.chosen = {}
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

describe('этапы в трекере (F63)', () => {
  const put = (id: string, stage: string, body: unknown) =>
    handleMock('PUT', `/tracker/${id}/stages/${stage}`, body) as Promise<TrackerItem>
  const stage = (item: TrackerItem, id: string) => item.stages.find((s) => s.id === id)!
  // ВсОШ: школьный этап уже прошёл, отметка о нём ждёт итога.
  const vsosh = () =>
    state.tracker.push({
      id: 'tr-2',
      profileId: 'vsosh-inf:inf',
      added_by: 'mem-artem',
      created_at: '',
      registered_at: '2026-09-01T00:00:00Z',
      registered_by: 'mem-artem',
    })

  it('до регистрации — «нужно зарегистрироваться», итогов у регистрации нет', async () => {
    const { items } = (await handleMock('GET', '/tracker')) as Tracker
    const hse = items[0]!
    expect(hse.status).toBe('open')
    expect(hse.action).toEqual({ type: 'register', stage_id: 'hse-registration-0' })
    expect(hse.stages.map((s) => s.results)).toEqual([[], ['passed', 'failed'], ['winner', 'prizer', 'participant']])
    expect(stage(hse, 'hse-qualifying-1').results_allowed).toEqual([])
  })

  it('регистрация на этап — участвует, отборочный впереди', async () => {
    const item = await put('tr-1', 'hse-registration-0', { registered: true, result: null })
    expect(item.status).toBe('active')
    expect(item.registered_at).not.toBeNull()
    expect(stage(item, 'hse-registration-0').registered).toBe(true)
    expect(stage(item, 'hse-qualifying-1').state).toBe('current')
  })

  it('итог этапа, который ещё не начался, — 400', async () => {
    await expect(put('tr-1', 'hse-qualifying-1', { registered: false, result: 'passed' })).rejects.toMatchObject({
      status: 400,
    })
  })

  it('прошедший этап спрашивает итог; «не прошёл» закрывает олимпиаду, снятие возвращает', async () => {
    vsosh()
    const { items } = (await handleMock('GET', '/tracker')) as Tracker
    const asking = items.find((i) => i.id === 'tr-2')!
    expect(asking.action).toEqual({ type: 'result', stage_id: 'vsosh-inf-school-0' })
    expect(stage(asking, 'vsosh-inf-school-0').asking).toBe(true)

    const failed = await put('tr-2', 'vsosh-inf-school-0', { registered: true, result: 'failed' })
    expect(failed.status).toBe('finished')
    expect(failed.outcome).toBe('failed')
    expect(failed.action).toBeNull()
    expect(stage(failed, 'vsosh-inf-municipal-1').state).toBe('locked')
    // Этап после закрывающего итога не отметить: он серый.
    await expect(put('tr-2', 'vsosh-inf-municipal-1', { registered: true, result: null })).rejects.toMatchObject({
      status: 409,
    })
    await expect(handleMock('DELETE', '/tracker/tr-2/registered')).rejects.toMatchObject({ status: 409 })

    const back = await put('tr-2', 'vsosh-inf-school-0', { registered: true, result: null })
    expect(back.status).toBe('active')
    expect(back.outcome).toBeNull()
  })

  it('этап чужой олимпиады — 404', async () => {
    await expect(put('tr-1', 'inno-registration-0', { registered: true, result: null })).rejects.toMatchObject({
      status: 404,
    })
  })
})

describe('выгрузка календаря', () => {
  afterEach(() => vi.restoreAllMocks())

  // В разработке API нет: ссылка ведёт на файл, собранный прямо в браузере,
  // чтобы кнопку можно было проверить руками.
  it('ссылка открывает файл календаря со сроками трекера', async () => {
    const createObjectURL = vi.spyOn(URL, 'createObjectURL')

    const link = (await handleMock('GET', '/calendar/link')) as CalendarLink

    expect(link.url).toMatch(/^blob:/)
    expect(Date.parse(link.expires_at)).toBeGreaterThan(Date.now())
    const file = createObjectURL.mock.calls[0]![0] as Blob
    expect(file.type).toBe('text/calendar')
    const text = await file.text()
    expect(text).toMatch(/^BEGIN:VCALENDAR\r\n/)
    expect(text).toMatch(/\r\nSUMMARY:Высшая проба: .+\r\n/)
    expect(text).toMatch(/\r\nEND:VCALENDAR\r\n$/)
  })
})

describe('карточка олимпиады', () => {
  it('«Где ещё даёт льготу» не повторяет вузы ученика из блока льгот', async () => {
    const detail = (await handleMock('GET', '/olympiads/hse:inf')) as OlympiadDetail

    const mine = detail.benefits.map((row) => row.university_id)
    const elsewhere = detail.benefit_universities.map((row) => row.university_id)
    expect(mine).toEqual(expect.arrayContaining(state.universities))
    expect(elsewhere.length).toBeGreaterThan(0)
    expect(elsewhere.filter((id) => mine.includes(id))).toEqual([])
  })

  // Таблица льгот, как на сервере: самые выгодные первыми, порог — столбцом,
  // если он в вузах разный.
  it('строки льгот — для таблицы: что получат победитель и призёр, порог, порядок', async () => {
    const detail = (await handleMock('GET', '/olympiads/hse:inf')) as OlympiadDetail

    expect(detail.benefit_columns).toEqual(['winner', 'prizer', 'ege'])
    expect(
      detail.benefits.map(
        (r) => `${r.university_nick}: ${r.winner?.label} / ${r.prizer?.label} / ${r.ege_min}–${r.ege_max}`,
      ),
    ).toEqual([
      'Иннополис: БВИ / БВИ / 75–null',
      'ВШЭ: 100 баллов / 100 баллов / 75–90',
      'КФУ: 100 баллов / 100 баллов / 75–null',
    ])
    // Своё у вуза — только то, что льгота ВШЭ на ПИ не на всех программах.
    expect(detail.benefits.map((r) => r.conditions)).toEqual([
      undefined,
      ['Зависит от программы: на части программ направления льготы нет'],
      undefined,
    ])
  })

  // Льгота в моих вузах — на мои направления (F65): ВШЭ целиком даёт БВИ,
  // а на Программную инженерию — 100 баллов, и это зависит от программы.
  it('льгота в моём вузе — на направления цели, выбор в вузе важнее цели', async () => {
    const row = async () =>
      ((await handleMock('GET', '/olympiads/hse:inf')) as OlympiadDetail).benefits.find((r) => r.university_id === 'hse')!

    expect(await row()).toMatchObject({ benefit: 'score100', directions: ['Программная инженерия'], varies: true })

    const profile = (await handleMock('PUT', '/profile/universities/hse/directions', {
      direction_ids: ['dir-se', 'dir-ami'],
    })) as Profile
    expect(profile.directions.map((d) => d.id)).toEqual(['dir-se', 'dir-ami'])
    expect(profile.universities.find((u) => u.id === 'hse')).toMatchObject({
      target_basis: 'chosen',
      chosen_directions: [
        { id: 'dir-se', name: 'Программная инженерия' },
        { id: 'dir-ami', name: 'Прикладная математика и информатика' },
      ],
    })
    expect(await row()).toMatchObject({
      benefit: 'bvi',
      directions: ['Прикладная математика и информатика'],
      other_directions: [{ benefit: 'score100', benefit_label: '100 баллов', directions: ['Программная инженерия'] }],
    })

    const uni = (await handleMock('GET', '/universities/hse')) as UniversityDetail
    expect(uni.offered_directions.slice(0, 2).map((d) => [d.id, d.is_mine])).toEqual([
      ['dir-ami', true],
      ['dir-se', true],
    ])
    // Технокубок на ПИ не учитывается, на ПМИ — 100 баллов; всего 4 из 8 направлений.
    expect(uni.olympiads.find((o) => o.olympiad_id === 'tk')).toMatchObject({
      my_benefit: 'score100',
      my_directions: ['Прикладная математика и информатика'],
      directions_count: 4,
      directions_total: 8,
    })

    await expect(
      handleMock('PUT', '/profile/universities/inno/directions', { direction_ids: ['dir-bio'] }),
    ).rejects.toMatchObject({ status: 400 })

    // Сняли ПМИ с цели в настройках — ушло и из выбора в ВШЭ.
    const after = (await handleMock('PATCH', '/profile', { direction_ids: ['dir-se'] })) as Profile
    expect(after.universities.find((u) => u.id === 'hse')?.chosen_directions).toEqual([
      { id: 'dir-se', name: 'Программная инженерия' },
    ])
  })

  // «Ведут в мои вузы и на мои направления» (F66): только олимпиады с
  // сильной льготой в моих вузах на мои направления, льгота — в строке.
  it('каталог «мои» — олимпиады с льготой на мои направления', async () => {
    const list = async (query: string) =>
      ((await handleMock('GET', `/olympiads${query}`)) as { items: OlympiadListItem[] }).items

    expect((await list('')).every((o) => o.my_benefits.length === 0)).toBe(true)
    expect((await list('?mine=true')).find((o) => o.olympiad_id === 'hse')?.my_benefits).toEqual([
      { benefit: 'bvi', benefit_label: 'БВИ', universities: ['Иннополис'], partial_universities: [] },
      // На Программную инженерию в ВШЭ — не на все программы.
      { benefit: 'score100', benefit_label: '100 баллов', universities: ['КФУ', 'ВШЭ'], partial_universities: ['ВШЭ'] },
    ])

    // Только ВШЭ: Технокубок даёт там льготу, но не на Программную инженерию.
    state.universities = ['hse']
    const hseOnly = await list('?mine=true')
    expect(hseOnly.map((o) => o.olympiad_id)).toContain('hse')
    expect(hseOnly.map((o) => o.olympiad_id)).not.toContain('tk')

    state.universities = []
    expect(await list('?mine=true')).toEqual([])
  })

  // Каталог вузов по направлению (F67): с укрупнёнными группами, сначала
  // где больше олимпиад, непроверенные льготы — в конце.
  it('каталог вузов по направлению', async () => {
    const list = async (query: string) =>
      ((await handleMock('GET', `/universities${query}`)) as { items: CatalogUniversity[] }).items

    const se = await list('?direction=dir-se')
    expect(se.map((u) => u.id)).not.toContain('mipt')
    expect(se.find((u) => u.id === 'inno')?.direction_match).toMatchObject({ direction_ids: ['dir-it'], status: 'offered' })
    const counts = se.map((u) => u.direction_match!.olympiads_count)
    expect(counts).toEqual([...counts].sort((a, b) => b - a))

    const is = await list('?direction=dir-is')
    expect(is.at(-1)).toMatchObject({ id: 'kfu', direction_match: { status: 'to_check', olympiads_count: 0 } })

    expect((await list('')).every((u) => u.direction_match === undefined)).toBe(true)
    await expect(handleMock('GET', '/universities?direction=нет')).rejects.toMatchObject({ status: 400 })
  })

  it('БВИ только победителю — призёр получает 100 баллов, а не БВИ', async () => {
    const detail = (await handleMock('GET', '/olympiads/tk:inf')) as OlympiadDetail
    const mipt = detail.benefit_universities.find((r) => r.university_id === 'mipt')

    expect(mipt?.winner).toEqual({ kind: 'bvi', label: 'БВИ' })
    expect(mipt?.prizer).toEqual({ kind: 'score100', label: '100 баллов' })
  })

  it('вне перечня — один столбец доп. баллов', async () => {
    const detail = (await handleMock('GET', '/olympiads/tyk:inf')) as OlympiadDetail
    const kfu = detail.benefits.find((r) => r.university_id === 'kfu')

    expect(detail.benefit_columns).toEqual(['extra_points'])
    expect(kfu?.winner).toEqual({ kind: 'extra_points', label: '+3 балла' })
    expect(detail.benefits.at(-1)?.winner).toBeNull()
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

describe('профиль', () => {
  afterEach(() => handleMock('PATCH', '/profile', { region_code: '16' }))

  it('пустой регион — «не указан», без прежнего названия', async () => {
    const p = (await handleMock('PATCH', '/profile', { region_code: '' })) as Profile
    expect([p.region_code, p.region_name]).toEqual(['', ''])
  })
})

describe('помощник', () => {
  const chats = async () => ((await handleMock('GET', '/ai/chats')) as { items: AiChat[] }).items

  it('первый вопрос создаёт чат по дню, следующие вопросы идут в него (F58, F59)', async () => {
    const first = (await handleMock('POST', '/ai/chats', { text: ' Что такое БВИ? ' })) as AiExchange
    expect(first.chat.title).toMatch(/^Чат \d{1,2} [а-я]+$/)
    expect(first.question.text).toBe('Что такое БВИ?')

    const next = (await handleMock('POST', `/ai/chats/${first.chat.id}/messages`, { text: 'А 100 баллов?' })) as AiExchange
    expect(next.chat.id).toBe(first.chat.id)

    const log = (await handleMock('GET', `/ai/chats/${first.chat.id}/messages`)) as { items: AiMessage[] }
    expect(log.items.map((m) => m.text).filter((_, i) => i % 2 === 0)).toEqual(['Что такое БВИ?', 'А 100 баллов?'])
    expect(await chats()).toHaveLength(1)
  })

  it('чат с новой репликой — первым в списке', async () => {
    const a = (await handleMock('POST', '/ai/chats', { text: 'Что такое БВИ?' })) as AiExchange
    const b = (await handleMock('POST', '/ai/chats', { text: 'А 100 баллов?' })) as AiExchange
    expect((await chats()).map((c) => c.id)).toEqual([b.chat.id, a.chat.id])

    await handleMock('POST', `/ai/chats/${a.chat.id}/messages`, { text: 'Ещё раз про БВИ' })
    expect((await chats()).map((c) => c.id)).toEqual([a.chat.id, b.chat.id])
  })

  it('чаты личные: родителю чата ученика не видно (F37)', async () => {
    const { chat } = (await handleMock('POST', '/ai/chats', { text: 'Что такое БВИ?' })) as AiExchange
    asParent()
    expect(await chats()).toEqual([])
    await expect(handleMock('GET', `/ai/chats/${chat.id}/messages`)).rejects.toMatchObject({ status: 404 })
    await expect(handleMock('POST', `/ai/chats/${chat.id}/messages`, { text: 'А мне?' })).rejects.toMatchObject({ status: 404 })
    await expect(handleMock('PATCH', `/ai/chats/${chat.id}`, { title: 'Моё' })).rejects.toMatchObject({ status: 404 })
  })

  it('переименование обрезает пробелы, пустое название — 400', async () => {
    const { chat } = (await handleMock('POST', '/ai/chats', { text: 'Что такое БВИ?' })) as AiExchange
    const renamed = (await handleMock('PATCH', `/ai/chats/${chat.id}`, { title: '  Льготы  ' })) as AiChat
    expect(renamed.title).toBe('Льготы')
    expect((await chats())[0]!.title).toBe('Льготы')
    await expect(handleMock('PATCH', `/ai/chats/${chat.id}`, { title: '   ' })).rejects.toMatchObject({ status: 400 })
  })
})
