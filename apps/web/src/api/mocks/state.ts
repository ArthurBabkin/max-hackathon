/**
 * Изменяемое состояние демо-стенда.
 *
 * Моки не просто отдают фикстуры: добавление в трекер, отметка регистрации,
 * предложения и приглашения реально меняют это состояние. Иначе по интерфейсу
 * нельзя пройти сценарий целиком, а именно его и надо проверять.
 *
 * Роль переключается параметром `?as=parent` в адресе: оба голоса из ТЗ F3
 * должны быть видны при разработке, а не только на живом аккаунте родителя.
 */

import type { Role } from '@contract'
import { OLYMPIADS, UNIVERSITIES } from './fixtures'

export interface DemoMember {
  id: string
  name: string
  role: Role
  is_creator: boolean
  color: string
  joined_at: string
}

export interface DemoTrackerItem {
  id: string
  profileId: string
  added_by: string
  created_at: string
  registered_at: string | null
  registered_by: string | null
}

export interface DemoProposal {
  id: string
  profileId: string
  proposed_by: string
  status: 'pending' | 'accepted' | 'declined'
  created_at: string
  resolved_at: string | null
}

export interface DemoInvite {
  id: string
  token: string
  role: Role
  created_at: string
}

/** Чат с помощником. Свой у каждого участника (F37), пустых не бывает. */
export interface DemoAiChat {
  id: string
  memberId: string
  title: string
  created_at: string
  last_message_at: string
}

export interface DemoAiMessage {
  id: string
  chatId: string
  role: 'user' | 'assistant'
  text: string
  card_refs: { type: 'olympiad' | 'university'; id: string; title: string }[]
  sources: { id: string; kind: 'order' | 'rules' | 'site'; title: string; url: string; verified_at: string | null }[]
  refused: boolean
  created_at: string
}

const MEMBER_PARENT = 'mem-olga'
const MEMBER_KID = 'mem-artem'
const MEMBER_PARENT2 = 'mem-igor'

export interface DemoState {
  viewerId: string
  trajectoryId: string
  student_name: string
  grade: 8 | 9 | 10 | 11
  region_code: string
  region_name: string
  directions: { id: string; name: string }[]
  goal_status: 'known' | 'suggested' | 'exploring'
  /** Где ученик хочет учиться, в порядке выбора; пусто — не важно. */
  places: { region_code: string; city: string | null }[]
  experience: 'none' | 'school' | 'region' | null
  home_city: string | null
  subjects: string[]
  universities: string[]
  members: DemoMember[]
  tracker: DemoTrackerItem[]
  proposals: DemoProposal[]
  invites: DemoInvite[]
  /** Свежие чаты впереди — в том порядке, в каком их отдаёт список. */
  aiChats: DemoAiChat[]
  ai: DemoAiMessage[]
}

function viewerFromUrl(): string {
  if (typeof location === 'undefined') return MEMBER_KID
  return new URLSearchParams(location.search).get('as') === 'parent' ? MEMBER_PARENT : MEMBER_KID
}

const DAY = 86_400_000
const ago = (days: number) => new Date(Date.now() - days * DAY).toISOString()

export const state: DemoState = {
  viewerId: viewerFromUrl(),
  trajectoryId: 'trj-demo',
  student_name: 'Артём',
  grade: 9,
  region_code: '16',
  region_name: 'Республика Татарстан',
  directions: [{ id: 'dir-se', name: 'Программная инженерия' }],
  goal_status: 'known',
  places: [{ region_code: '16', city: null }],
  experience: 'school',
  home_city: 'Казань',
  subjects: ['inf', 'math'],
  universities: ['inno', 'kfu', 'hse'],

  // Траекторию создала Ольга, Артём подключился по ссылке — сценарий из ТЗ §14.
  members: [
    { id: MEMBER_PARENT, name: 'Ольга', role: 'parent', is_creator: true, color: '#E92E78', joined_at: ago(2) },
    { id: MEMBER_KID, name: 'Артём', role: 'kid', is_creator: false, color: '#FF8A3D', joined_at: ago(1) },
    { id: MEMBER_PARENT2, name: 'Игорь', role: 'parent', is_creator: false, color: '#1A6DFF', joined_at: ago(1) },
  ],

  tracker: [
    { id: 'tr-1', profileId: 'hse:inf', added_by: MEMBER_PARENT, created_at: ago(2), registered_at: null, registered_by: null },
    { id: 'tr-2', profileId: 'vsosh-inf:inf', added_by: MEMBER_KID, created_at: ago(1), registered_at: null, registered_by: null },
    { id: 'tr-3', profileId: 'inno:inf', added_by: MEMBER_KID, created_at: ago(1), registered_at: null, registered_by: null },
  ],

  proposals: [
    { id: 'pr-1', profileId: 'tk:inf', proposed_by: MEMBER_PARENT, status: 'pending', created_at: ago(0), resolved_at: null },
  ],

  invites: [],
  aiChats: [],
  ai: [],
}

export const viewer = () => state.members.find((m) => m.id === state.viewerId) ?? state.members[0]!
export const memberById = (id: string) => state.members.find((m) => m.id === id) ?? null
export const hasKid = () => state.members.some((m) => m.role === 'kid')
export const role = (): Role => viewer().role

/** `olympiadId:subjectCode` — в моках этого достаточно вместо id из базы. */
export const profileKey = (olympiadId: string, subject: string) => `${olympiadId}:${subject}`

export function splitProfileId(profileId: string): { olympiadId: string; subject: string } | null {
  const at = profileId.lastIndexOf(':')
  if (at < 0) return null
  return { olympiadId: profileId.slice(0, at), subject: profileId.slice(at + 1) }
}

export function findProfile(profileId: string) {
  const parts = splitProfileId(profileId)
  if (!parts) return null
  const olympiad = OLYMPIADS.find((o) => o.id === parts.olympiadId)
  const profile = olympiad?.profiles.find((p) => p.subject_code === parts.subject)
  if (!olympiad || !profile) return null
  return { olympiad, profile }
}

/** Профиль, который откроется по тапу: совпадающий с предметами ученика либо первый. */
export function primaryProfile(olympiadId: string) {
  const olympiad = OLYMPIADS.find((o) => o.id === olympiadId)
  if (!olympiad) return null
  const mine = olympiad.profiles.find((p) => state.subjects.includes(p.subject_code))
  return mine ?? olympiad.profiles[0] ?? null
}

export const universityById = (id: string) => UNIVERSITIES.find((u) => u.id === id) ?? null

let counter = 0
export const nextId = (prefix: string) => `${prefix}-${++counter}-${Date.now().toString(36)}`
