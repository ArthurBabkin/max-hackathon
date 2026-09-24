/**
 * Контракт API мини-приложения «Траектория».
 *
 * Источник истины — openapi.yaml. Файл types.gen.ts генерируется из него
 * командой `npm run contract` в apps/web и правится только через yaml.
 * Здесь — короткие человеческие имена поверх `components["schemas"][…]`,
 * чтобы в коде писалось `OlympiadCard`, а не трёхэтажный индексный доступ.
 */

import type { components, paths } from './types.gen'

type S = components['schemas']

// --- Справочные перечисления -------------------------------------------------

export type Role = S['Role']
export type OlympiadKind = S['OlympiadKind']
export type Level = S['Level']
export type BenefitKind = S['BenefitKind']
export type StageKind = S['StageKind']
export type ErrorCode = S['Error']['error']['code']

// --- Сессия и права ----------------------------------------------------------

export type ApiError = S['Error']
export type Session = S['Session']
export type SessionResponse = S['SessionResponse']
export type Permissions = S['Permissions']
export type TrajectorySummary = S['TrajectorySummary']

// --- Экраны ------------------------------------------------------------------

export type Home = S['Home']
export type NextStep = S['NextStep']
export type Recommendations = S['Recommendations']
export type OlympiadCard = S['OlympiadCard']
export type OlympiadDetail = S['OlympiadDetail']
export type OlympiadListItem = S['OlympiadListItem']
export type ProfileLevel = S['ProfileLevel']
export type BenefitRow = S['BenefitRow']
export type Stage = S['Stage']
export type Source = S['Source']
export type Direction = S['Direction']
export type UniversityListItem = S['UniversityListItem']
export type UniversityDetail = S['UniversityDetail']
export type Tracker = S['Tracker']
export type TrackerItem = S['TrackerItem']
export type Proposal = S['Proposal']
export type CalendarLink = S['CalendarLink']
export type CalendarMonth = S['CalendarMonth']
export type Family = S['Family']
export type Member = S['Member']
export type MemberBrief = S['MemberBrief']
export type Invite = S['Invite']
export type Profile = S['Profile']
export type ProfilePatch = S['ProfilePatch']
export type AiChat = S['AiChat']
export type AiExchange = S['AiExchange']
export type AiMessage = S['AiMessage']
export type AiCardRef = S['AiCardRef']

// --- Значения, которые нужны в рантайме --------------------------------------

/** Фильтры подбора, чипы экрана C3. Значение = query-параметр `filter`. */
export const MATCH_FILTERS = ['all', 'level1', 'soon', 'online'] as const
export type MatchFilter = (typeof MATCH_FILTERS)[number]

/** Пороги напоминаний в днях (ТЗ §6.3). */
export const REMINDER_OFFSETS = [30, 7, 3, 1] as const

/** Классы, с которыми работает продукт (ТЗ §1.3). */
export const GRADES = [8, 9, 10, 11] as const
export type Grade = (typeof GRADES)[number]

/**
 * Подписи льгот. Сервер присылает `benefit_label` готовым, но клиенту нужен
 * тот же словарь для моков и для `benefit: null` («не учитывает»).
 */
export const BENEFIT_LABELS: Record<BenefitKind, string> = {
  bvi: 'БВИ',
  score100: '100 баллов',
  bvi_winners: 'БВИ победителям',
  extra_points: 'доп. баллы',
}

export const NO_BENEFIT_LABEL = 'не учитывает'

export type { components, paths }
