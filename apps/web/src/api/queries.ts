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
  AiMessage,
  CalendarMonth,
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
  TrackerItem,
  UniversityDetail,
  UniversityListItem,
} from '@contract'
import { api, request, setReauth, setToken } from './client'
import { getWebApp } from '@/bridge'

export const keys = {
  session: ['session'] as const,
  home: ['home'] as const,
  recommendations: (filter: MatchFilter) => ['recommendations', filter] as const,
  olympiads: (q: string, subject: string, city: string) => ['olympiads', q, subject, city] as const,
  olympiad: (id: string) => ['olympiad', id] as const,
  universities: (q: string, city: string) => ['universities', q, city] as const,
  university: (id: string) => ['university', id] as const,
  tracker: ['tracker'] as const,
  calendar: (month: string) => ['calendar', month] as const,
  family: ['family'] as const,
  profile: ['profile'] as const,
  ai: ['ai'] as const,
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
    retry: 1,
  })
}

// --- Чтение ------------------------------------------------------------------

export const useHome = () => useQuery({ queryKey: keys.home, queryFn: () => api.get<Home>('/home') })

export const useRecommendations = (filter: MatchFilter) =>
  useQuery({
    queryKey: keys.recommendations(filter),
    queryFn: () => api.get<Recommendations>('/recommendations', { filter }),
  })

export const useOlympiads = (q: string, subject: string, city: string) =>
  useQuery({
    queryKey: keys.olympiads(q, subject, city),
    queryFn: () =>
      api.get<{ items: OlympiadListItem[] }>('/olympiads', {
        q,
        subject: subject === 'all' ? undefined : subject,
        city: city === 'all' ? undefined : city,
      }),
  })

export const useOlympiad = (id: string | null) =>
  useQuery({
    queryKey: keys.olympiad(id ?? ''),
    queryFn: () => api.get<OlympiadDetail>(`/olympiads/${encodeURIComponent(id!)}`),
    enabled: id !== null,
  })

export const useUniversities = (q: string, city: string) =>
  useQuery({
    queryKey: keys.universities(q, city),
    queryFn: () =>
      api.get<{ items: UniversityListItem[] }>('/universities', {
        q,
        city: city === 'all' ? undefined : city,
      }),
  })

export const useUniversity = (id: string | null) =>
  useQuery({
    queryKey: keys.university(id ?? ''),
    queryFn: () => api.get<UniversityDetail>(`/universities/${encodeURIComponent(id!)}`),
    enabled: id !== null,
  })

export const useTracker = () =>
  useQuery({
    queryKey: keys.tracker,
    queryFn: () => api.get<{ items: TrackerItem[]; proposals: Proposal[] }>('/tracker'),
  })

export const useCalendar = (month: string) =>
  useQuery({
    queryKey: keys.calendar(month),
    queryFn: () => api.get<CalendarMonth>('/calendar', { month }),
  })

export const useFamily = () =>
  useQuery({ queryKey: keys.family, queryFn: () => api.get<Family>('/family') })

export const useProfile = () =>
  useQuery({ queryKey: keys.profile, queryFn: () => api.get<Profile>('/profile') })

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

export function useToggleRegistered() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, registered }: { id: string; registered: boolean }) =>
      registered
        ? api.put<TrackerItem>(`/tracker/${encodeURIComponent(id)}/registered`)
        : api.delete<TrackerItem>(`/tracker/${encodeURIComponent(id)}/registered`),

    // Оптимистично: это кнопка, которую жмут на демонстрации, и ждать ответа
    // ради галочки незачем. При ошибке состояние откатывается.
    onMutate: async ({ id, registered }) => {
      await qc.cancelQueries({ queryKey: keys.tracker })
      const previous = qc.getQueryData(keys.tracker)
      qc.setQueryData<{ items: TrackerItem[]; proposals: Proposal[] }>(keys.tracker, (old) =>
        old
          ? {
              ...old,
              items: old.items.map((item) =>
                item.id === id
                  ? { ...item, registered_at: registered ? new Date().toISOString() : null }
                  : item,
              ),
            }
          : old,
      )
      return { previous }
    },
    onError: (_error, _vars, context) => {
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

// --- Помощник ----------------------------------------------------------------

export const useAiMessages = () =>
  useQuery({
    queryKey: keys.ai,
    queryFn: () => api.get<{ items: AiMessage[] }>('/ai/messages'),
  })

export function useAskAi() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (text: string) =>
      api.post<{ question: AiMessage; answer: AiMessage }>('/ai/messages', { text }),
    onSuccess: ({ question, answer }) => {
      // Дописываем пару в кеш вместо перезапроса: история чата только растёт,
      // и лишний круг к серверу ничего не уточнит.
      qc.setQueryData<{ items: AiMessage[] }>(keys.ai, (old) => ({
        items: [...(old?.items ?? []), question, answer],
      }))
    },
  })
}
