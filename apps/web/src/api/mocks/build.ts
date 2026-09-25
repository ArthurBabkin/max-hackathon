/** Сборка ответов контракта из демо-состояния. Только для разработки. */

import {
  BENEFIT_LABELS,
  NO_BENEFIT_LABEL,
  type BenefitColumn,
  type BenefitGrant,
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
import { regions as REGIONS } from '@regions'
import { daysLeft, formatDay, plural } from '@/lib/deadline'
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

/** Сколько доп. баллов даёт диплом олимпиады вне перечня в демо-вузе. */
const EXTRA_POINTS = 3

const BVI: BenefitGrant = { kind: 'bvi', label: BENEFIT_LABELS.bvi }
const SCORE100: BenefitGrant = { kind: 'score100', label: BENEFIT_LABELS.score100 }

/** Что получат победитель и призёр — как на сервере (F18). */
function grants(university: DemoUniversity, olympiadId: string): Pick<BenefitRow, 'winner' | 'prizer'> {
  const benefit = university.benefits[olympiadId]
  const rule = university.rules?.[olympiadId]
  if (!benefit) return { winner: null, prizer: null }
  if (benefit === 'extra_points') {
    const extra: BenefitGrant = {
      kind: 'extra_points',
      label: `+${EXTRA_POINTS} ${plural(EXTRA_POINTS, 'балл', 'балла', 'баллов')}`,
    }
    return { winner: extra, prizer: extra }
  }
  const winner = benefit === 'score100' ? SCORE100 : BVI
  if (rule?.prizer) return { winner, prizer: rule.prizer === 'score100' ? SCORE100 : null }
  return { winner, prizer: benefit === 'bvi_winners' ? null : winner }
}

function benefitRow(university: DemoUniversity, olympiadId: string): BenefitRow {
  const benefit = university.benefits[olympiadId] ?? null
  const kind = OLYMPIADS.find((o) => o.id === olympiadId)?.kind
  return {
    university_id: university.id,
    university_name: university.name,
    university_short_name: university.short_name,
    university_nick: university.nick,
    city: university.city,
    color: university.color,
    benefit,
    benefit_label: benefit ? BENEFIT_LABELS[benefit] : NO_BENEFIT_LABEL,
    ...grants(university, olympiadId),
    extra_points: benefit === 'extra_points' ? EXTRA_POINTS : null,
    // У ВсОШ льготу ЕГЭ не подтверждают — порога нет.
    ege_min: benefit && benefit !== 'extra_points' && kind !== 'vsosh' ? DEFAULT_EGE_MIN : null,
    ege_max: university.rules?.[olympiadId]?.egeMax ?? null,
    diploma_grades: benefit ? [9, 10, 11] : null,
    note: null,
    source: benefit ? (SOURCES.rules as Source) : null,
  }
}

const grantRank = (g: BenefitGrant | null) => (g ? { bvi: 0, score100: 1, extra_points: 2 }[g.kind] : 3)

/** Порядок строк, как на сервере: самые выгодные первыми, дальше по алфавиту. */
function sortBenefitRows(rows: BenefitRow[]): BenefitRow[] {
  return rows.sort(
    (a, b) =>
      grantRank(a.winner) - grantRank(b.winner) ||
      (b.extra_points ?? 0) - (a.extra_points ?? 0) ||
      grantRank(a.prizer) - grantRank(b.prizer) ||
      a.university_nick.toLowerCase().localeCompare(b.university_nick.toLowerCase(), 'ru'),
  )
}

/** Столбцы таблицы льгот: порог — только если он в вузах разный. */
function benefitColumns(o: DemoOlympiad, rows: BenefitRow[]): BenefitColumn[] {
  if (o.kind === 'other') return ['extra_points']
  const counted = rows.filter((r) => r.winner)
  const thresholds = new Set(counted.map((r) => `${r.ege_min}–${r.ege_max}`))
  return counted.some((r) => r.ege_max !== null) || thresholds.size > 1
    ? ['winner', 'prizer', 'ege']
    : ['winner', 'prizer']
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
  const benefits = sortBenefitRows(
    state.universities
      .map(universityById)
      .filter((u): u is DemoUniversity => u !== null)
      .map((u) => {
        const notes = u.rules?.[o.id]?.notes
        return notes ? { ...benefitRow(u, o.id), conditions: notes } : benefitRow(u, o.id)
      }),
  )

  return {
    ...card,
    official_url: o.official_url,
    description: null,
    profiles: levels,
    profiles_source: o.kind === 'perechen' ? (SOURCES.perechen as Source) : null,
    benefits,
    benefit_columns: benefitColumns(o, benefits),
    benefits_source: SOURCES.rules as Source,
    conditions: o.conditions,
    stages: stages(o, registered),
    stages_are_demo: o.stages_are_demo,
    why: o.why[role()],
    // Вузы ученика уже в блоке льгот — здесь только остальные (F23).
    benefit_universities: UNIVERSITIES.filter((u) => u.benefits[o.id] && !state.universities.includes(u.id)).map((u) =>
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
    directions: state.directions,
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
    directions: state.directions,
    goal_status: state.goal_status,
    target_region_code: state.places[0]?.region_code ?? null,
    target_region_name: REGIONS.find((r) => r.code === state.places[0]?.region_code)?.name ?? null,
    experience: state.experience,
    home_city: state.home_city,
    places: state.places.map((p) => ({
      region_code: p.region_code,
      region_name: REGIONS.find((r) => r.code === p.region_code)?.name ?? p.region_code,
      city: p.city,
    })),
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
