/**
 * Моки разработки.
 *
 * Отвечают на ручки контракта до того, как их напишет бэкенд, и меняют
 * состояние по-настоящему: добавление в трекер, отметка регистрации,
 * предложения и приглашения работают, иначе сценарий нечем проверить.
 *
 * Модуль подключается только динамическим импортом под `import.meta.env.DEV`
 * из api/client.ts — в прод-бандле его нет, прод ходит в живой API.
 *
 * Отладочные переключатели в адресной строке:
 *   ?as=parent      смотреть глазами родителя
 *   ?mock=error     ответы падают — экран H4
 *   ?mock=empty     подбор и каталог пусты — экран H3
 */

import { ApiError } from '../errors'
import { regions as REGIONS } from '@regions'
import { DIRECTIONS, OLYMPIADS, SOURCES, UNIVERSITIES, inDays } from './fixtures'
import {
  type DemoAiChat,
  dropOrphanPrograms,
  findProfile,
  hasKid,
  nextId,
  primaryProfile,
  profileKey,
  programById,
  role,
  state,
  universityById,
  viewer,
} from './state'
import * as build from './build'
import { daysLeft, isSoon, moscowDay } from '@/lib/deadline'

const LATENCY_MS = 200

const flag = (name: string) =>
  typeof location === 'undefined' ? null : new URLSearchParams(location.search).get(name)

const wantsError = () => flag('mock') === 'error'
const wantsEmpty = () => flag('mock') === 'empty'

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

type Params = Record<string, string>
type Query = URLSearchParams
type Handler = (ctx: { params: Params; query: Query; body: unknown }) => unknown

interface Route {
  method: string
  pattern: RegExp
  keys: string[]
  handler: Handler
}

const routes: Route[] = []

/** `/tracker/:id/registered` → регулярка с именованной группой. */
function route(method: string, path: string, handler: Handler): void {
  const keys: string[] = []
  const pattern = new RegExp(
    `^${path.replace(/:([a-zA-Z_]+)/g, (_, key: string) => {
      keys.push(key)
      return '([^/]+)'
    })}$`,
  )
  routes.push({ method, pattern, keys, handler })
}

const forbidden = (message: string) => new ApiError(403, 'FORBIDDEN', message)
const notFound = () => new ApiError(404, 'NOT_FOUND', 'Не найдено')

// --- Сессия и главная --------------------------------------------------------

route('POST', '/session', () => ({
  token: 'dev-mock-token',
  expires_at: new Date(Date.now() + 3_600_000).toISOString(),
  session: build.session(),
}))

route('GET', '/home', () => build.home())

// --- Подбор ------------------------------------------------------------------

route('GET', '/recommendations', ({ query }) => {
  const filter = query.get('filter') ?? 'all'

  const cards = OLYMPIADS.map((o) => {
    const primary = primaryProfile(o.id)
    return primary ? build.olympiadCard(profileKey(o.id, primary.subject_code)) : null
  }).filter((c): c is NonNullable<typeof c> => c !== null)

  const mine = cards.filter((c) => state.subjects.includes(c.subject_code))

  const matchesFilter = (c: (typeof cards)[number]) => {
    if (filter === 'level1') return c.level === 'I'
    if (filter === 'soon') return isSoon(daysLeft(c.deadline_at))
    if (filter === 'online') return c.is_online
    return true
  }

  const items = wantsEmpty()
    ? []
    : mine
        .filter((c) => c.kind !== 'other')
        .filter(matchesFilter)
        // Сортировка по близости срока — как требует ТЗ §6.1 п. 5.
        .sort((a, b) => (a.deadline_at ?? '').localeCompare(b.deadline_at ?? ''))

  return {
    items,
    outside: wantsEmpty() ? [] : mine.filter((c) => c.kind === 'other'),
    note: 'Сначала ближайшие сроки и точное совпадение профиля',
  }
})

// --- Каталог -----------------------------------------------------------------

const matches = (haystack: string, needle: string) =>
  haystack.toLowerCase().includes(needle.toLowerCase())

route('GET', '/olympiads', ({ query }) => {
  const q = query.get('q')?.trim() ?? ''
  const subject = query.get('subject')
  const city = query.get('city')

  const items = (wantsEmpty() ? [] : OLYMPIADS)
    .filter((o) => !q || matches(o.name, q) || matches(o.organizer, q))
    .filter((o) => !subject || o.profiles.some((p) => p.subject_code === subject))
    .filter((o) => !city || o.final_city === city)
    .map(build.olympiadListItem)
    .filter((i): i is NonNullable<typeof i> => i !== null)

  return { items }
})

route('GET', '/olympiads/:id', ({ params }) => {
  const detail = build.olympiadDetail(params.id!)
  if (!detail) throw notFound()
  return detail
})

route('GET', '/universities', ({ query }) => {
  const q = query.get('q')?.trim() ?? ''
  const city = query.get('city')
  const direction = query.get('direction')

  const items = (wantsEmpty() ? [] : UNIVERSITIES)
    .filter((u) => !q || matches(u.name, q) || matches(u.short_name, q))
    .filter((u) => !city || u.city === city)
    .filter((u) => !direction || (u.programs ?? []).some((p) => p.direction_id === direction))
    .map(build.universityListItem)

  return { items }
})

route('GET', '/directions', () => ({ items: DIRECTIONS }))

route('GET', '/universities/:id', ({ params, query }) => {
  const u = universityById(params.id!)
  if (!u) throw notFound()
  const program = query.get('program')
  if (program && !u.programs?.some((p) => p.id === program)) throw notFound()
  return build.universityDetail(u, program)
})

// --- Трекер ------------------------------------------------------------------

route('GET', '/tracker', () => ({
  items: state.tracker.map(build.trackerItem).filter(Boolean),
  proposals: state.proposals
    .filter((p) => p.status === 'pending')
    .map(build.proposal)
    .filter(Boolean),
}))

route('POST', '/tracker', ({ body }) => {
  const profileId = (body as { olympiad_profile_id?: string })?.olympiad_profile_id
  if (!profileId || !findProfile(profileId)) throw notFound()

  // Матрица ТЗ §3.1: родитель добавляет сам только пока ученика в траектории нет.
  if (role() === 'parent' && hasKid()) {
    throw forbidden('Пока ученик в траектории, олимпиаду можно только предложить')
  }

  const existing = state.tracker.find((i) => i.profileId === profileId)
  if (existing) return build.trackerItem(existing)

  const item = {
    id: nextId('tr'),
    profileId,
    added_by: state.viewerId,
    created_at: new Date().toISOString(),
    registered_at: null,
    registered_by: null,
  }
  state.tracker.push(item)
  // Принятое предложение закрывается вместе с добавлением.
  for (const p of state.proposals) {
    if (p.profileId === profileId && p.status === 'pending') {
      p.status = 'accepted'
      p.resolved_at = new Date().toISOString()
    }
  }
  return build.trackerItem(item)
})

route('DELETE', '/tracker/:id', ({ params }) => {
  if (role() === 'parent' && hasKid()) throw forbidden('Убрать олимпиаду может ученик')
  const index = state.tracker.findIndex((i) => i.id === params.id)
  if (index < 0) throw notFound()
  state.tracker.splice(index, 1)
  return undefined
})

route('PUT', '/tracker/:id/registered', ({ params }) => {
  const item = state.tracker.find((i) => i.id === params.id)
  if (!item) throw notFound()
  item.registered_at = new Date().toISOString()
  item.registered_by = state.viewerId
  return build.trackerItem(item)
})

route('DELETE', '/tracker/:id/registered', ({ params }) => {
  const item = state.tracker.find((i) => i.id === params.id)
  if (!item) throw notFound()
  item.registered_at = null
  item.registered_by = null
  return build.trackerItem(item)
})

route('GET', '/calendar', ({ query }) => {
  const month = query.get('month') ?? new Date().toISOString().slice(0, 7)
  const byDate = new Map<string, ReturnType<typeof build.trackerItem>[]>()

  for (const item of state.tracker) {
    const built = build.trackerItem(item)
    if (!built?.deadline_at) continue
    const date = built.deadline_at.slice(0, 10)
    if (!date.startsWith(month)) continue
    const list = byDate.get(date) ?? []
    list.push(built)
    byDate.set(date, list)
  }

  return {
    month,
    days: [...byDate.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([date, items]) => ({ date, items })),
  }
})

/**
 * Выгрузка календаря. Живой API отдаёт путь к файлу со своим пропуском, а
 * здесь API нет — файл со сроками трекера собирается в браузере, чтобы
 * кнопку можно было проверить руками.
 */
route('GET', '/calendar/link', () => {
  const file = new Blob([mockIcs()], { type: 'text/calendar' })
  return { url: URL.createObjectURL(file), expires_at: new Date(Date.now() + 10 * 60_000).toISOString() }
})

function mockIcs(): string {
  const text = (s: string) => s.replace(/[\\;,]/g, '\\$&')
  const stamp = new Date().toISOString().replace(/[-:]/g, '').replace(/\.\d+/, '')
  const lines = [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//Traektoria//Mock//RU',
    'X-WR-CALNAME:Траектория: сроки олимпиад',
  ]
  for (const item of state.tracker.map(build.trackerItem)) {
    if (!item?.deadline_at) continue
    const day = moscowDay(new Date(item.deadline_at)).replaceAll('-', '')
    lines.push(
      'BEGIN:VEVENT',
      `UID:${item.id}@traektoria`,
      `DTSTAMP:${stamp}`,
      `DTSTART;VALUE=DATE:${day}`,
      `SUMMARY:${text(`${item.olympiad_name}: ${item.next_stage_title ?? ''}`)}`,
      'END:VEVENT',
    )
  }
  lines.push('END:VCALENDAR')
  return `${lines.join('\r\n')}\r\n`
}

// --- Предложения -------------------------------------------------------------

route('POST', '/proposals', ({ body }) => {
  const profileId = (body as { olympiad_profile_id?: string })?.olympiad_profile_id
  if (!profileId || !findProfile(profileId)) throw notFound()
  if (role() !== 'parent' || !hasKid()) throw forbidden('Предлагать олимпиады может родитель')

  const existing = state.proposals.find((p) => p.profileId === profileId && p.status === 'pending')
  if (existing) return build.proposal(existing)

  const proposal = {
    id: nextId('pr'),
    profileId,
    proposed_by: state.viewerId,
    status: 'pending' as const,
    created_at: new Date().toISOString(),
    resolved_at: null,
  }
  state.proposals.push(proposal)
  return build.proposal(proposal)
})

function resolveProposal(id: string, accepted: boolean) {
  if (role() !== 'kid') throw forbidden('Отвечать на предложение может ученик')
  const proposal = state.proposals.find((p) => p.id === id)
  if (!proposal) throw notFound()
  if (proposal.status !== 'pending') {
    throw new ApiError(409, 'CONFLICT', 'Предложение уже закрыто')
  }

  proposal.status = accepted ? 'accepted' : 'declined'
  proposal.resolved_at = new Date().toISOString()

  if (!accepted) return build.proposal(proposal)

  const item = {
    id: nextId('tr'),
    profileId: proposal.profileId,
    added_by: proposal.proposed_by,
    created_at: new Date().toISOString(),
    registered_at: null,
    registered_by: null,
  }
  state.tracker.push(item)
  return { proposal: build.proposal(proposal), tracker_item: build.trackerItem(item) }
}

route('POST', '/proposals/:id/accept', ({ params }) => resolveProposal(params.id!, true))
route('POST', '/proposals/:id/decline', ({ params }) => resolveProposal(params.id!, false))

// --- Семья -------------------------------------------------------------------

route('GET', '/family', () => build.family())

route('POST', '/family/invites', ({ body }) => {
  const inviteRole = (body as { role?: 'kid' | 'parent' })?.role ?? 'parent'
  if (inviteRole === 'kid' && hasKid()) {
    throw new ApiError(409, 'CONFLICT', 'Ученик в траектории уже есть')
  }
  const invite = {
    id: nextId('inv'),
    token: Math.random().toString(36).slice(2, 10) + Math.random().toString(36).slice(2, 8),
    role: inviteRole,
    created_at: new Date().toISOString(),
  }
  state.invites.push(invite)
  return build.family().invites.at(-1)
})

route('DELETE', '/family/members/:id', ({ params }) => {
  if (!viewer().is_creator) throw forbidden('Удалить участника может только создатель')
  const index = state.members.findIndex((m) => m.id === params.id)
  if (index < 0) throw notFound()
  if (state.members[index]!.is_creator) throw forbidden('Создателя удалить нельзя')
  state.members.splice(index, 1)
  return undefined
})

route('POST', '/family/leave', () => {
  if (viewer().is_creator) throw forbidden('Создатель не может выйти, только удалить траекторию')
  return undefined
})

// --- Профиль -----------------------------------------------------------------

route('GET', '/profile', () => build.profile())

route('PATCH', '/profile', ({ body }) => {
  const patch = (body ?? {}) as Record<string, unknown>
  if (typeof patch.student_name === 'string' && patch.student_name.trim()) {
    state.student_name = patch.student_name.trim()
  }
  if (typeof patch.grade === 'number') state.grade = patch.grade as 8 | 9 | 10 | 11
  if (typeof patch.region_code === 'string') {
    state.region_code = patch.region_code
    // "" — регион не указан.
    state.region_name = REGIONS.find((r) => r.code === patch.region_code)?.name ?? ''
  }
  if (Array.isArray(patch.subject_codes) && patch.subject_codes.length > 0) {
    state.subjects = patch.subject_codes as string[]
  }
  if (Array.isArray(patch.direction_ids)) {
    const ids = patch.direction_ids as string[]
    state.directions = ids.flatMap((id) => DIRECTIONS.filter((d) => d.id === id))
    state.goal_status = state.directions.length > 0 ? 'known' : 'exploring'
  }
  if (Array.isArray(patch.places)) {
    const places = patch.places as { region_code: string; city?: string | null }[]
    state.places = places.map((p) => ({ region_code: p.region_code, city: p.city ?? null }))
  } else if (typeof patch.target_region_code === 'string') {
    state.places = patch.target_region_code ? [{ region_code: patch.target_region_code, city: null }] : []
  }
  if (patch.experience === 'none' || patch.experience === 'school' || patch.experience === 'region') {
    state.experience = patch.experience
  }
  if (Array.isArray(patch.university_ids)) {
    state.universities = patch.university_ids as string[]
    dropOrphanPrograms()
  }
  return build.profile()
})

route('PUT', '/profile/universities', ({ body }) => {
  const ids = (body as { university_ids?: string[] })?.university_ids
  if (!Array.isArray(ids)) {
    throw new ApiError(400, 'BAD_REQUEST', 'Не передан список вузов')
  }
  state.universities = ids
  dropOrphanPrograms()
  return build.profile()
})

route('PUT', '/profile/programs', ({ body }) => {
  const ids = (body as { program_ids?: string[] })?.program_ids
  if (!Array.isArray(ids)) {
    throw new ApiError(400, 'BAD_REQUEST', 'Не передан список направлений')
  }
  const found = ids.map(programById)
  if (found.some((x) => x === null)) {
    throw new ApiError(400, 'BAD_REQUEST', 'Неизвестный предмет, направление или вуз.')
  }
  state.programs = [...new Set(ids)]
  // Направление без своего вуза в списке не бывает.
  for (const x of found) {
    if (x && !state.universities.includes(x.university.id)) state.universities.push(x.university.id)
  }
  return build.profile()
})

// --- Помощник ----------------------------------------------------------------

const chatDay = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long', timeZone: 'Europe/Moscow' })

/** Свой чат участника; чужой для него не существует — 404, как на сервере. */
function ownChat(id: string | undefined) {
  const chat = state.aiChats.find((c) => c.id === id && c.memberId === state.viewerId)
  if (!chat) throw notFound()
  return chat
}

function readQuestion(body: unknown): string {
  const text = String((body as { text?: string })?.text ?? '').trim()
  if (!text) throw new ApiError(400, 'BAD_REQUEST', 'Напишите вопрос.')
  return text
}

/** Пара вопрос–ответ в чат; чат с новой репликой встаёт в начало списка. */
function exchange(chat: DemoAiChat, question: string) {
  const now = new Date().toISOString()
  const asked = {
    id: nextId('ai'),
    chatId: chat.id,
    role: 'user' as const,
    text: question,
    card_refs: [],
    sources: [],
    refused: false,
    created_at: now,
  }
  const answer = { ...composeAnswer(question), id: nextId('ai'), chatId: chat.id, created_at: now }
  state.ai.push(asked, answer)
  chat.last_message_at = now
  state.aiChats = [chat, ...state.aiChats.filter((c) => c !== chat)]
  return { chat: stripChat(chat), question: strip(asked), answer: strip(answer) }
}

route('GET', '/ai/chats', () => ({
  items: state.aiChats.filter((c) => c.memberId === state.viewerId).map(stripChat),
}))

route('POST', '/ai/chats', ({ body }) => {
  const question = readQuestion(body)
  const now = new Date().toISOString()
  const chat: DemoAiChat = {
    id: nextId('chat'),
    memberId: state.viewerId,
    title: `Чат ${chatDay.format(new Date())}`,
    created_at: now,
    last_message_at: now,
  }
  state.aiChats.unshift(chat)
  return exchange(chat, question)
})

route('PATCH', '/ai/chats/:id', ({ params, body }) => {
  const chat = ownChat(params.id)
  const title = String((body as { title?: string })?.title ?? '').trim()
  if (!title) throw new ApiError(400, 'BAD_REQUEST', 'Напишите название чата.')
  if ([...title].length > 60) throw new ApiError(400, 'BAD_REQUEST', 'Название длиннее 60 символов — сократите его.')
  chat.title = title
  return stripChat(chat)
})

route('GET', '/ai/chats/:id/messages', ({ params }) => {
  const chat = ownChat(params.id)
  return { items: state.ai.filter((m) => m.chatId === chat.id).map(strip) }
})

route('POST', '/ai/chats/:id/messages', ({ params, body }) => {
  const chat = ownChat(params.id)
  return exchange(chat, readQuestion(body))
})

function strip(m: (typeof state.ai)[number]) {
  const { chatId: _chatId, ...rest } = m
  return rest
}

function stripChat(c: DemoAiChat) {
  const { memberId: _memberId, ...rest } = c
  return rest
}

/**
 * Ответ помощника по нашей базе. Логика простая, зато честная: нашли карточку —
 * ответили со ссылкой, не нашли — прямо сказали «данных нет» (ТЗ F36).
 */
function composeAnswer(question: string) {
  const q = question.toLowerCase()
  const mentioned = OLYMPIADS.find((o) => q.includes(o.name.toLowerCase()))
  const university = UNIVERSITIES.find(
    (u) => q.includes(u.nick.toLowerCase()) || q.includes(u.short_name.toLowerCase()),
  )

  if (mentioned) {
    const primary = primaryProfile(mentioned.id)!
    const rows = state.universities
      .map(universityById)
      .filter((u) => u && u.benefits[mentioned.id])
      .map((u) => `${u!.nick} — ${u!.benefits[mentioned.id] === 'bvi' ? 'БВИ' : '100 баллов'}`)

    // Вуз в вопросе есть, а записи о льготе по этой олимпиаде в базе нет.
    if (university && !university.benefits[mentioned.id]) {
      return {
        role: 'assistant' as const,
        // «правил вуза «ИТМО»» — приложение в именительном, чтобы не склонять
        // названия вузов: склонять их пришлось бы по-разному и в каждом падеже.
        text: `Данных нет. В нашей базе нет правил приёма вуза «${university.nick}» по олимпиаде «${mentioned.name}», поэтому гадать не буду. ${role() === 'kid' ? 'Проверь' : 'Проверьте'} правила приёма на сайте вуза.`,
        card_refs: [],
        sources: [SOURCES.rules],
        refused: true,
      }
    }

    return {
      role: 'assistant' as const,
      text: rows.length
        ? `По профилю «${build.subjectName(primary.subject_code)}»: ${rows.join(', ')}. Льготу подтверждают ЕГЭ от 75 баллов.`
        : `Льгот по «${mentioned.name}» в ${role() === 'kid' ? 'твоих' : 'ваших'} вузах нет.`,
      card_refs: [
        {
          type: 'olympiad' as const,
          id: profileKey(mentioned.id, primary.subject_code),
          title: mentioned.name,
        },
      ],
      sources: [SOURCES.rules],
      refused: false,
    }
  }

  if (q.includes('бви') || q.includes('100 балл')) {
    return {
      role: 'assistant' as const,
      text: 'БВИ — зачисление без вступительных испытаний. 100 баллов — диплом засчитывается как 100 баллов ЕГЭ по профильному предмету. И то и другое по олимпиадам перечня нужно подтвердить ЕГЭ от 75 баллов.',
      card_refs: [],
      sources: [SOURCES.order821],
      refused: false,
    }
  }

  return {
    role: 'assistant' as const,
    text: `Данных нет. В нашей базе нет ответа на этот вопрос, поэтому гадать не буду. ${role() === 'kid' ? 'Проверь' : 'Проверьте'} первоисточник.`,
    card_refs: [],
    sources: [SOURCES.perechen],
    refused: true,
  }
}

// --- Диспетчер ---------------------------------------------------------------

/**
 * Возвращает `undefined`, если мока на ручку нет, — тогда клиент идёт
 * в настоящий API. Так недостающие ручки видно сразу, а не через тишину.
 */
export async function handleMock<T>(
  method: string,
  fullPath: string,
  body?: unknown,
): Promise<T | undefined> {
  const [path = '', search = ''] = fullPath.split('?')
  const match = routes.find((r) => r.method === method && r.pattern.test(path))
  if (!match) return undefined

  await sleep(LATENCY_MS)

  if (wantsError()) throw new ApiError(0, 'INTERNAL', 'Нет соединения с интернетом')

  const found = match.pattern.exec(path)!
  const params: Params = {}
  match.keys.forEach((key, i) => {
    params[key] = decodeURIComponent(found[i + 1] ?? '')
  })

  return match.handler({ params, query: new URLSearchParams(search), body }) as T
}

export { inDays }
