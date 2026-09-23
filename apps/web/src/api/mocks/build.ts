/** Сборка ответов контракта из демо-состояния. Только для разработки. */

import {
  BENEFIT_LABELS,
  NO_BENEFIT_LABEL,
  type BenefitRow,
  type Family,
  type Home,
  type Invite,
  type Member,
  type MemberBrief,
  type NextStep,
  type OlympiadCard,
  type OlympiadDetail,
  type OlympiadListItem,
  type Profile,
  type ProfileLevel,
  type Proposal,
  type Session,
  type Source,
  type Stage,
  type TrackerItem,
  type TrajectorySummary,
  type UniversityDetail,
  type UniversityListItem,
} from '@contract'
import { daysLeft, formatDay } from '@/lib/deadline'
import { dative, genitive } from '@/lib/declension'
import { derivePermissions } from '@/lib/permissions'
import { text } from '@/voice/texts'
import {
  DEFAULT_EGE_MIN,
  OLYMPIADS,
  SOURCES,
  SUBJECTS,
  UNIVERSITIES,
  type DemoOlympiad,
  type DemoUniversity,
  inDays,
} from './fixtures'
import {
  findProfile,
  hasKid,
  memberById,
  primaryProfile,
  profileKey,
  role,
  state,
  universityById,
  viewer,
  type DemoProposal,
  type DemoTrackerItem,
} from './state'

const subjectName = (code: string) => SUBJECTS[code] ?? code

/** Подстановки, которые голос ждёт от вызывающего кода. */
function voiceVars(extra: Record<string, string | number> = {}) {
  return {
    student: state.student_name,
    student_gen: genitive(state.student_name),
    student_dat: dative(state.student_name),
    me: viewer().name,
    ...extra,
  }
}

const t = (key: Parameters<typeof text>[0], extra?: Record<string, string | number>) =>
  text(key, role(), voiceVars(extra))

const badge = (o: DemoOlympiad) => ({ short_name: o.short_name, color: o.color })

// --- Льготы ------------------------------------------------------------------

function benefitRow(university: DemoUniversity, olympiadId: string): BenefitRow {
  const benefit = university.benefits[olympiadId] ?? null
  return {
    university_id: university.id,
    university_name: university.name,
    university_short_name: university.short_name,
    city: university.city,
    color: university.color,
    benefit,
    benefit_label: benefit ? BENEFIT_LABELS[benefit] : NO_BENEFIT_LABEL,
    extra_points: benefit === 'extra_points' ? 10 : null,
    ege_min: benefit && benefit !== 'extra_points' ? DEFAULT_EGE_MIN : null,
    diploma_grades: benefit ? [9, 10, 11] : null,
    note: null,
    source: benefit ? (SOURCES.rules as Source) : null,
  }
}

/** Строка «Иннополис, ВШЭ: БВИ» под карточкой в подборе (F13). */
function benefitsSummary(olympiadId: string): string {
  const rows = state.universities
    .map(universityById)
    .filter((u): u is DemoUniversity => u !== null)
    .map((u) => ({ nick: u.nick, benefit: u.benefits[olympiadId] }))
    .filter((r) => r.benefit)

  if (rows.length === 0) return t('match.noBenefits')

  const bvi = rows.filter((r) => r.benefit === 'bvi' || r.benefit === 'bvi_winners')
  if (bvi.length > 0) return `${bvi.map((r) => r.nick).join(', ')}: ${BENEFIT_LABELS.bvi}`

  const first = rows[0]!
  return `${first.nick}: ${BENEFIT_LABELS[first.benefit!]}`
}

// --- Этапы -------------------------------------------------------------------

function stages(o: DemoOlympiad, registered: boolean): Stage[] {
  // Текущий этап — ближайший из ещё не прошедших. Если регистрация уже
  // отмечена, она считается пройденной, и выделяется следующий этап.
  const upcoming = o.stages.findIndex((s, i) => {
    if (registered && i === 0) return false
    return s.offset === null || s.offset >= 0
  })

  return o.stages.map((s, i) => {
    const at = s.offset === null ? null : inDays(s.offset)
    const isRegistration = s.kind === 'registration' || s.kind === 'school'
    return {
      id: `${o.id}-${s.kind}-${i}`,
      kind: s.kind,
      title: s.title,
      // «до» — единственная подпись, которой нужна конкретная дата.
      // Остальные описательные («октябрь — ноябрь, онлайн»), и приписывать
      // к ним ещё и число значило бы сказать одно и то же дважды.
      subtitle: s.subtitle === 'до' && at ? `до ${formatDay(at)}` : s.subtitle,
      starts_at: isRegistration ? null : at,
      ends_at: null,
      deadline_at: isRegistration ? at : null,
      is_online: s.is_online,
      state: i === upcoming ? 'current' : i < upcoming || upcoming === -1 ? 'past' : 'future',
    }
  })
}

const deadlineOf = (o: DemoOlympiad) => inDays(o.deadlineIn)

/**
 * Какой этап впереди. После отметки о регистрации регистрация считается
 * пройденной, и дальше идёт отборочный — иначе карточка так и писала бы
 * «Дальше: Регистрация» уже после того, как на неё зарегистрировались.
 */
const nextStageTitle = (o: DemoOlympiad, registered = false) =>
  (registered ? o.stages[1]?.title : o.stages[0]?.title) ?? null

// --- Олимпиады ---------------------------------------------------------------

const inTracker = (profileId: string) => state.tracker.some((i) => i.profileId === profileId)
const pendingProposal = (profileId: string) =>
  state.proposals.some((p) => p.profileId === profileId && p.status === 'pending')

export function olympiadCard(profileId: string): OlympiadCard | null {
  const found = findProfile(profileId)
  if (!found) return null
  const { olympiad: o, profile } = found

  return {
    ...badge(o),
    olympiad_profile_id: profileId,
    olympiad_id: o.id,
    name: o.name,
    organizer: o.organizer,
    kind: o.kind,
    level: profile.level,
    subject_code: profile.subject_code,
    subject_name: subjectName(profile.subject_code),
    format: o.format,
    is_online: o.is_online,
    final_city: o.final_city,
    deadline_at: deadlineOf(o),
    next_stage_title: nextStageTitle(o),
    benefits_summary: benefitsSummary(o.id),
    reason: o.reason,
    in_tracker: inTracker(profileId),
    proposal_status: pendingProposal(profileId) ? 'pending' : null,
  }
}

export function olympiadDetail(profileId: string): OlympiadDetail | null {
  const card = olympiadCard(profileId)
  const found = findProfile(profileId)
  if (!card || !found) return null
  const { olympiad: o, profile } = found

  const levels: ProfileLevel[] = o.profiles.map((p) => ({
    olympiad_profile_id: profileKey(o.id, p.subject_code),
    subject_code: p.subject_code,
    subject_name: subjectName(p.subject_code),
    level: p.level,
    is_mine: p.subject_code === profile.subject_code,
  }))

  const registered = state.tracker.some((i) => i.profileId === profileId && i.registered_at)

  return {
    ...card,
    official_url: o.official_url,
    description: null,
    profiles: levels,
    profiles_source: o.kind === 'perechen' ? (SOURCES.perechen as Source) : null,
    benefits: state.universities
      .map(universityById)
      .filter((u): u is DemoUniversity => u !== null)
      .map((u) => benefitRow(u, o.id)),
    benefits_source: SOURCES.rules as Source,
    conditions: o.conditions,
    stages: stages(o, registered),
    stages_are_demo: o.stages_are_demo,
    why: o.why[role()],
    benefit_universities: UNIVERSITIES.filter((u) => u.benefits[o.id]).map((u) =>
      benefitRow(u, o.id),
    ),
  }
}

export function olympiadListItem(o: DemoOlympiad): OlympiadListItem | null {
  const primary = primaryProfile(o.id)
  if (!primary) return null
  return {
    ...badge(o),
    olympiad_id: o.id,
    name: o.name,
    organizer: o.organizer,
    kind: o.kind,
    final_city: o.final_city,
    primary_profile: {
      olympiad_profile_id: profileKey(o.id, primary.subject_code),
      subject_code: primary.subject_code,
      subject_name: subjectName(primary.subject_code),
      level: primary.level,
    },
    profiles_count: o.profiles.length,
  }
}

// --- Вузы --------------------------------------------------------------------

const benefitOlympiadsCount = (u: DemoUniversity) =>
  Object.values(u.benefits).filter((b) => b === 'bvi' || b === 'score100' || b === 'bvi_winners')
    .length

export function universityListItem(u: DemoUniversity): UniversityListItem {
  return {
    id: u.id,
    short_name: u.short_name,
    name: u.name,
    city: u.city,
    color: u.color,
    benefit_olympiads_count: benefitOlympiadsCount(u),
    is_mine: state.universities.includes(u.id),
  }
}

export function universityDetail(u: DemoUniversity): UniversityDetail {
  return {
    ...universityListItem(u),
    directions: u.directions,
    ege_note: u.ege_note,
    rules_url: u.rules_url,
    description: null,
    site_url: null,
    rules_verified_at: u.rules_verified_at,
    olympiads: Object.entries(u.benefits).flatMap(([olympiadId, benefit]) => {
      const o = OLYMPIADS.find((x) => x.id === olympiadId)
      const primary = primaryProfile(olympiadId)
      if (!o || !primary) return []
      return [
        {
          ...badge(o),
          olympiad_profile_id: profileKey(o.id, primary.subject_code),
          olympiad_id: o.id,
          name: o.name,
          subject_name: subjectName(primary.subject_code),
          level: primary.level,
          benefit,
          benefit_label: BENEFIT_LABELS[benefit],
        },
      ]
    }),
  }
}

// --- Трекер и предложения ----------------------------------------------------

const brief = (id: string | null): MemberBrief | null => {
  const m = id ? memberById(id) : null
  return m ? { id: m.id, name: m.name, role: m.role } : null
}

export function trackerItem(item: DemoTrackerItem): TrackerItem | null {
  const found = findProfile(item.profileId)
  if (!found) return null
  const { olympiad: o, profile } = found
  return {
    ...badge(o),
    id: item.id,
    olympiad_profile_id: item.profileId,
    olympiad_id: o.id,
    olympiad_name: o.name,
    subject_name: subjectName(profile.subject_code),
    kind: o.kind,
    level: profile.level,
    deadline_at: deadlineOf(o),
    next_stage_title: nextStageTitle(o, Boolean(item.registered_at)),
    registered_at: item.registered_at,
    registered_by: brief(item.registered_by),
    added_by: brief(item.added_by),
  }
}

export function proposal(p: DemoProposal): Proposal | null {
  const found = findProfile(p.profileId)
  if (!found) return null
  const { olympiad: o } = found
  return {
    ...badge(o),
    id: p.id,
    olympiad_profile_id: p.profileId,
    olympiad_id: o.id,
    olympiad_name: o.name,
    deadline_at: deadlineOf(o),
    status: p.status,
    proposed_by: brief(p.proposed_by)!,
    created_at: p.created_at,
    resolved_at: p.resolved_at,
  }
}

// --- Сессия, главная, профиль, семья ----------------------------------------

export function trajectorySummary(): TrajectorySummary {
  return {
    id: state.trajectoryId,
    student_name: state.student_name,
    grade: state.grade,
    region_code: state.region_code,
    region_name: state.region_name,
    direction_id: state.direction_id,
    direction_name: state.direction_name,
    goal_status: state.goal_status,
    has_kid: hasKid(),
    members_count: state.members.length,
  }
}

export function session(): Session {
  const me = viewer()
  return {
    user: { id: `usr-${me.id}`, max_user_id: 900_000_001, first_name: me.name },
    member: { id: me.id, role: me.role, is_creator: me.is_creator, reminder_offsets: [30, 7, 3, 1] },
    trajectory: trajectorySummary(),
    permissions: derivePermissions({
      role: me.role,
      is_creator: me.is_creator,
      has_kid: hasKid(),
    }),
    start_param: null,
  }
}

export function home(): Home {
  const items = state.tracker
    .map(trackerItem)
    .filter((i): i is TrackerItem => i !== null)
    .sort((a, b) => (a.deadline_at ?? '').localeCompare(b.deadline_at ?? ''))

  const open = items.filter((i) => !i.registered_at)
  const first = open[0]
  const next: NextStep | null = first
    ? {
        tracker_item_id: first.id,
        olympiad_profile_id: first.olympiad_profile_id,
        olympiad_name: first.olympiad_name,
        stage_title: first.next_stage_title ?? 'Регистрация',
        stage_kind: first.kind === 'vsosh' ? 'school' : 'registration',
        deadline_at: first.deadline_at,
      }
    : null

  return {
    trajectory: trajectorySummary(),
    tracker_count: items.length,
    registered_count: items.length - open.length,
    universities_count: state.universities.length,
    pending_proposals_count: state.proposals.filter((p) => p.status === 'pending').length,
    next_step: next,
    upcoming: items.slice(0, 3),
  }
}

export function profile(): Profile {
  return {
    student_name: state.student_name,
    grade: state.grade,
    region_code: state.region_code,
    region_name: state.region_name,
    subjects: state.subjects.map((code) => ({ code, name: subjectName(code) })),
    direction_id: state.direction_id,
    direction_name: state.direction_name,
    goal_status: state.goal_status,
    universities: state.universities
      .map(universityById)
      .filter((u): u is DemoUniversity => u !== null)
      .map(universityListItem),
    other_member_names: state.members.filter((m) => m.id !== state.viewerId).map((m) => m.name),
  }
}

export function family(): Family {
  const me = viewer()
  const members: Member[] = state.members.map((m) => ({
    id: m.id,
    name: m.name,
    role: m.role,
    is_creator: m.is_creator,
    is_me: m.id === state.viewerId,
    can_remove: me.is_creator && !m.is_creator && m.id !== state.viewerId,
    color: m.color,
    joined_at: m.joined_at,
  }))

  const invites: Invite[] = state.invites.map((i) => ({
    id: i.id,
    token: i.token,
    url: `https://max.ru/t356_hakaton_max_bot?start=inv_${i.token}`,
    role: i.role,
    created_at: i.created_at,
  }))

  return { trajectory: trajectorySummary(), members, invites }
}

export { daysLeft, t, subjectName }
