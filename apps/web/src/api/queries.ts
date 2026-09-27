/**
 * Все запросы к API в одном месте.
 *
 * TanStack Query взят не ради кеша, а ради инвалидации: почти каждая мутация
 * меняет сразу несколько экранов — добавление в трекер трогает бейдж вкладки,
 * «следующий шаг» на главной и кнопку в подборе. Разводить это руками дороже,
 * чем библиотека.
 */

import {
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type UseQueryResult,
} from '@tanstack/react-query'
import type {
  AiChat,
  AiExchange,
  AiMessage,
  CalendarLink,
  CalendarMonth,
  CatalogUniversity,
  DirectionOption,
  Family,
  Home,
  Invite,
  MatchFilter,
  OlympiadDetail,
  OlympiadListItem,
  Profile,
  ProfilePatch,
  Proposal,
  Recommendations,
  Role,
  Session,
  SessionResponse,
  StageResult,
  TrackerItem,
  UniversityDetail,
} from '@contract'
import { api, request, setReauth, setToken } from './client'
import { getWebApp } from '@/bridge'
import { retryOnce } from './queryClient'

export const keys = {
  session: ['session'] as const,
  home: ['home'] as const,
  recommendations: (filter: MatchFilter) => ['recommendations', filter] as const,
  olympiads: (q: string, subject: string, mine: boolean, sort: string = 'name') =>
    ['olympiads', q, subject, mine, sort] as const,
  olympiad: (id: string) => ['olympiad', id] as const,
  universities: (q: string, city: string, direction = '') => ['universities', q, city, direction] as const,
  university: (id: string) => ['university', id] as const,
  tracker: ['tracker'] as const,
  calendar: (month: string) => ['calendar', month] as const,
  calendarLink: ['calendar-link'] as const,
  family: ['family'] as const,
  profile: ['profile'] as const,
  directions: ['directions'] as const,
  aiChats: ['ai', 'chats'] as const,
  aiChat: (id: string) => ['ai', 'chat', id] as const,
  health: ['health'] as const,
}

/** Экраны, на которых виден состав трекера. */
function invalidateTracker(qc: QueryClient): void {
  void qc.invalidateQueries({ queryKey: keys.tracker })
  void qc.invalidateQueries({ queryKey: keys.home })
  void qc.invalidateQueries({ queryKey: ['recommendations'] })
  void qc.invalidateQueries({ queryKey: ['olympiad'] })
  void qc.invalidateQueries({ queryKey: ['calendar'] })
}

/** Экраны, зависящие от профиля ученика: цель, предметы, список вузов. */
function invalidateProfile(qc: QueryClient): void {
  void qc.invalidateQueries({ queryKey: keys.profile })
  void qc.invalidateQueries({ queryKey: keys.session })
  void qc.invalidateQueries({ queryKey: keys.home })
  void qc.invalidateQueries({ queryKey: ['recommendations'] })
  // Каталог «Ведут в мои вузы» и предмет по умолчанию — от профиля.
  void qc.invalidateQueries({ queryKey: ['olympiads'] })
  void qc.invalidateQueries({ queryKey: ['olympiad'] })
  void qc.invalidateQueries({ queryKey: ['universities'] })
  void qc.invalidateQueries({ queryKey: ['university'] })
}

// --- Сессия ------------------------------------------------------------------

async function openSession(): Promise<Session> {
  const webApp = getWebApp()
  const response = await request<SessionResponse>('POST', '/session', {
    anonymous: true,
    body: {
      init_data: webApp.initData,
      start_param: webApp.initDataUnsafe.start_param ?? null,
    },
  })
  setToken(response.token)
  return response.session
}

// Истёкший или отозванный токен клиент меняет сам и повторяет запрос.
setReauth(async () => {
  await openSession()
})

export function useSession(): UseQueryResult<Session> {
  return useQuery({
    queryKey: keys.session,
    queryFn: openSession,
    // JWT живёт час, дёргать /session на каждом монтировании незачем.
    staleTime: 50 * 60 * 1000,
    retry: retryOnce,
    // После ошибки вход повторяет только кнопка на экране ошибки. Иначе
    // каждый новый подписчик — тот же экран ошибки через useVoice — заново
    // запускал бы POST /session, и приложение крутилось бы в цикле.
    retryOnMount: false,
  })
}

// --- Чтение ------------------------------------------------------------------

export const useHome = () => useQuery({ queryKey: keys.home, queryFn: () => api.get<Home>('/home') })

export const useRecommendations = (filter: MatchFilter) =>
  useQuery({
    queryKey: keys.recommendations(filter),
    queryFn: () => api.get<Recommendations>('/recommendations', { filter }),
  })

export const useOlympiads = (q: string, subject: string, mine: boolean, sort: string = 'name') =>
  useQuery({
    queryKey: keys.olympiads(q, subject, mine, sort),
    queryFn: () =>
      api.get<{ items: OlympiadListItem[] }>('/olympiads', {
        q,
        subject: subject === 'all' ? undefined : subject,
        mine: mine ? 'true' : undefined,
        sort: sort === 'name' ? undefined : sort,
      }),
  })

export const useOlympiad = (id: string | null) =>
  useQuery({
    queryKey: keys.olympiad(id ?? ''),
    queryFn: () => api.get<OlympiadDetail>(`/olympiads/${encodeURIComponent(id!)}`),
    enabled: id !== null,
  })

export const useUniversities = (q: string, city: string, direction = '') =>
  useQuery({
    queryKey: keys.universities(q, city, direction),
    queryFn: () =>
      api.get<{ items: CatalogUniversity[] }>('/universities', {
        q,
        city: city === 'all' ? undefined : city,
        direction: direction || undefined,
      }),
  })

export const useUniversity = (id: string | null) =>
  useQuery({
    queryKey: keys.university(id ?? ''),
    queryFn: () => api.get<UniversityDetail>(`/universities/${encodeURIComponent(id!)}`),
    enabled: id !== null,
  })

// enabled — для оболочки: бейдж вкладки нельзя запрашивать раньше сессии,
// иначе запрос без токена получит 401 и откроет вторую сессию.
export const useTracker = (enabled = true) =>
  useQuery({
    queryKey: keys.tracker,
    queryFn: () => api.get<{ items: TrackerItem[]; proposals: Proposal[] }>('/tracker'),
    enabled,
  })

export const useCalendar = (month: string) =>
  useQuery({
    queryKey: keys.calendar(month),
    queryFn: () => api.get<CalendarMonth>('/calendar', { month }),
  })

/**
 * Пропуск к файлу календаря (F32). Берётся заранее, пока открыт календарь:
 * MAX открывает внешнюю ссылку только прямо в обработчике нажатия, ждать
 * ответа сервера там нельзя. Пропуск живёт 10 минут — берём новый через пять.
 */
export const useCalendarLink = (enabled: boolean) =>
  useQuery({
    queryKey: keys.calendarLink,
    queryFn: () => api.get<CalendarLink>('/calendar/link'),
    enabled,
    staleTime: 5 * 60_000,
  })

export const useFamily = () =>
  useQuery({ queryKey: keys.family, queryFn: () => api.get<Family>('/family') })

export const useProfile = () =>
  useQuery({ queryKey: keys.profile, queryFn: () => api.get<Profile>('/profile') })

/** Справочник целей для правки профиля (F49); меняется только с выкладкой. */
export const useDirections = () =>
  useQuery({
    queryKey: keys.directions,
    queryFn: () => api.get<{ items: DirectionOption[] }>('/directions'),
    staleTime: Infinity,
  })

/** Коммит, из которого собран сервер. Меняется только с выкладкой. */
export const useServerVersion = () =>
  useQuery({
    queryKey: keys.health,
    queryFn: () => api.get<{ version?: string }>('/health'),
    select: (health) => health.version ?? '—',
    staleTime: Infinity,
    retry: false,
  })

// --- Трекер ------------------------------------------------------------------

export function useAddToTracker() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (olympiadProfileId: string) =>
      api.post<TrackerItem>('/tracker', { olympiad_profile_id: olympiadProfileId }),
    onSuccess: () => invalidateTracker(qc),
  })
}

export function useRemoveFromTracker() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (trackerItemId: string) =>
      api.delete<void>(`/tracker/${encodeURIComponent(trackerItemId)}`),
    onSuccess: () => invalidateTracker(qc),
  })
}

/** Отметка этапа: регистрация или итог. Без этапа — «участвую» (F46). */
export interface StageMark {
  item: TrackerItem
  stageId: string | null
  registered: boolean
  result: StageResult | null
}

type TrackerCache = { items: TrackerItem[]; proposals: Proposal[] }

/** Пункт с отметкой, какой она станет, — до ответа сервера. */
function withMark(item: TrackerItem, { stageId, registered, result }: StageMark): TrackerItem {
  if (stageId === null) return { ...item, registered_at: registered ? new Date().toISOString() : null }
  return {
    ...item,
    stages: item.stages.map((s) => (s.id === stageId ? { ...s, registered, result } : s)),
  }
}

/**
 * Отметить этап (F63). Как и галочка регистрации — оптимистично: отметку
 * видно сразу, а ответ сервера приносит всё, что из неё следует (статус,
 * серые этапы, следующее действие). Отказ (409 — отметка противоречит
 * другим) откатывает карточку, текст сервера показывает всплывашка.
 */
export function useSetStageMark() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ item, stageId, registered, result }: StageMark) => {
      const id = encodeURIComponent(item.id)
      if (stageId === null) {
        return registered
          ? api.put<TrackerItem>(`/tracker/${id}/registered`)
          : api.delete<TrackerItem>(`/tracker/${id}/registered`)
      }
      return api.put<TrackerItem>(`/tracker/${id}/stages/${encodeURIComponent(stageId)}`, { registered, result })
    },
    onMutate: async (mark) => {
      await qc.cancelQueries({ queryKey: keys.tracker })
      const previous = qc.getQueryData<TrackerCache>(keys.tracker)
      qc.setQueryData<TrackerCache>(keys.tracker, (old) =>
        old ? { ...old, items: old.items.map((i) => (i.id === mark.item.id ? withMark(i, mark) : i)) } : old,
      )
      return { previous }
    },
    onSuccess: (updated) => {
      qc.setQueryData<TrackerCache>(keys.tracker, (old) =>
        old ? { ...old, items: old.items.map((i) => (i.id === updated.id ? updated : i)) } : old,
      )
    },
    onError: (_error, _mark, context) => {
      if (context?.previous) qc.setQueryData(keys.tracker, context.previous)
    },
    onSettled: () => invalidateTracker(qc),
  })
}

// --- Предложения -------------------------------------------------------------

export function usePropose() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (olympiadProfileId: string) =>
      api.post<Proposal>('/proposals', { olympiad_profile_id: olympiadProfileId }),
    onSuccess: () => invalidateTracker(qc),
  })
}

export function useResolveProposal() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, accept }: { id: string; accept: boolean }) =>
      api.post(`/proposals/${encodeURIComponent(id)}/${accept ? 'accept' : 'decline'}`),
    onSuccess: () => invalidateTracker(qc),
  })
}

// --- Семья -------------------------------------------------------------------

export function useCreateInvite() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (role: Role) => api.post<Invite>('/family/invites', { role }),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.family }),
  })
}

/** Отозвать неиспользованную ссылку-приглашение (ТЗ §16). */
export function useRevokeInvite() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (inviteId: string) => api.delete<void>(`/family/invites/${encodeURIComponent(inviteId)}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.family }),
  })
}

export function useRemoveMember() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (memberId: string) =>
      api.delete<void>(`/family/members/${encodeURIComponent(memberId)}`),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: keys.family })
      void qc.invalidateQueries({ queryKey: keys.session })
    },
  })
}

export function useLeaveTrajectory() {
  return useMutation({ mutationFn: () => api.post<void>('/family/leave') })
}

// --- Профиль -----------------------------------------------------------------

export function usePatchProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (patch: ProfilePatch) => api.patch<Profile>('/profile', patch),
    onSuccess: () => invalidateProfile(qc),
  })
}

export function useSetUniversities() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (universityIds: string[]) =>
      api.put<Profile>('/profile/universities', { university_ids: universityIds }),
    onSuccess: () => invalidateProfile(qc),
  })
}

/**
 * Направления в вузе (F65): вуз становится моим, новые направления — в цель.
 * Галочка ставится сразу; ошибка откатывает её.
 */
export function useSetUniversityDirections(universityId: string) {
  const qc = useQueryClient()
  const key = keys.university(universityId)
  return useMutation({
    mutationFn: (directionIds: string[]) =>
      api.put<Profile>(`/profile/universities/${encodeURIComponent(universityId)}/directions`, {
        direction_ids: directionIds,
      }),
    onMutate: async (directionIds) => {
      await qc.cancelQueries({ queryKey: key })
      const before = qc.getQueryData<UniversityDetail>(key)
      if (before) {
        qc.setQueryData<UniversityDetail>(key, {
          ...before,
          is_mine: true,
          offered_directions: before.offered_directions.map((d) => ({ ...d, is_mine: directionIds.includes(d.id) })),
        })
      }
      return { before }
    },
    onError: (_error, _ids, context) => {
      if (context?.before) qc.setQueryData(key, context.before)
    },
    onSuccess: (profile) => qc.setQueryData(keys.profile, profile),
    onSettled: () => invalidateProfile(qc),
  })
}

// --- Помощник ----------------------------------------------------------------

type Items<T> = { items: T[] }

export const useAiChats = () =>
  useQuery({
    queryKey: keys.aiChats,
    queryFn: () => api.get<Items<AiChat>>('/ai/chats'),
  })

/** Реплики чата. `null` — чата ещё нет (новый, до первого вопроса). */
export const useAiChatMessages = (id: string | null) =>
  useQuery({
    queryKey: keys.aiChat(id ?? ''),
    queryFn: () => api.get<Items<AiMessage>>(`/ai/chats/${encodeURIComponent(id!)}/messages`),
    enabled: id !== null,
    // Чужой или исчезнувший чат не появится от повтора.
    retry: false,
  })

/** Свежий чат — первым; остальные в прежнем порядке. */
function putChatFirst(qc: QueryClient, chat: AiChat): void {
  const list = qc.getQueryData<Items<AiChat>>(keys.aiChats)
  if (list) qc.setQueryData(keys.aiChats, { items: [chat, ...list.items.filter((c) => c.id !== chat.id)] })
  else void qc.invalidateQueries({ queryKey: keys.aiChats })
}

/** Вопрос помощнику. `chatId: null` — первый вопрос нового чата, он же создаёт чат. */
export function useAskAi() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ chatId, text }: { chatId: string | null; text: string }) =>
      chatId === null
        ? api.post<AiExchange>('/ai/chats', { text })
        : api.post<AiExchange>(`/ai/chats/${encodeURIComponent(chatId)}/messages`, { text }),
    // Неудачный вопрос показывается в самом чате, с кнопкой повтора.
    meta: { silentError: true },
    onSuccess: ({ chat, question, answer }, { chatId }) => {
      // Дописываем пару в кеш вместо перезапроса: история чата только растёт,
      // и лишний круг к серверу ничего не уточнит. У нового чата истории нет.
      const log = qc.getQueryData<Items<AiMessage>>(keys.aiChat(chat.id))
      if (log || chatId === null) {
        qc.setQueryData(keys.aiChat(chat.id), { items: [...(log?.items ?? []), question, answer] })
      } else {
        void qc.invalidateQueries({ queryKey: keys.aiChat(chat.id) })
      }
      putChatFirst(qc, chat)
    },
  })
}

export function useRenameAiChat() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, title }: { id: string; title: string }) =>
      api.patch<AiChat>(`/ai/chats/${encodeURIComponent(id)}`, { title }),
    // Ошибка названия показывается под полем ввода.
    meta: { silentError: true },
    onSuccess: (chat) => {
      // Переименование не новая реплика: место чата в списке не меняется.
      qc.setQueryData<Items<AiChat>>(keys.aiChats, (old) =>
        old ? { items: old.items.map((c) => (c.id === chat.id ? chat : c)) } : old,
      )
    },
  })
}
