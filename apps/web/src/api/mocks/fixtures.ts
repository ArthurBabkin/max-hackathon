/**
 * Демо-данные для разработки.
 *
 * Это те же олимпиады, вузы и участники, что в прототипе (docs/prototype),
 * приведённые к формам контракта. В прод-бандл файл не попадает: он
 * подтягивается динамическим импортом под `import.meta.env.DEV`.
 *
 * Даты считаются от сегодняшнего дня, а не записаны строками. Захардкоженное
 * «25 сентября» через сутки превратило бы розовую плашку в серую, и разница
 * между «4 дня» и «срок прошёл» вылезла бы прямо на демонстрации.
 */

import type { BenefitKind, Level, OlympiadKind, Role } from '@contract'

/** Полночь через n дней — так дедлайн попадает на конец нужных суток. */
export function inDays(n: number): string {
  const d = new Date()
  d.setDate(d.getDate() + n)
  d.setHours(23, 59, 0, 0)
  return d.toISOString()
}

/** Начало дня через n дней — для этапов, которые начинаются, а не истекают. */
function dayStart(n: number): string {
  const d = new Date()
  d.setDate(d.getDate() + n)
  d.setHours(9, 0, 0, 0)
  return d.toISOString()
}

export const SUBJECTS: Record<string, string> = {
  inf: 'Информатика',
  math: 'Математика',
  phys: 'Физика',
  chem: 'Химия',
  bio: 'Биология',
  soc: 'Обществознание',
  econ: 'Экономика',
  ai: 'Искусственный интеллект',
  infosec: 'Информационная безопасность',
  robo: 'Робототехника',
}

export const SOURCES = {
  perechen: {
    id: 'src-669',
    kind: 'order' as const,
    title: 'Перечень олимпиад, приказ № 669',
    url: 'https://www.rsr-olymp.ru/',
    verified_at: '2026-09-15',
  },
  rules: {
    id: 'src-rules-2026',
    kind: 'rules' as const,
    title: 'Правила приёма вузов на 2026 год',
    url: 'https://minobrnauki.gov.ru/',
    verified_at: '2026-09-15',
  },
  order821: {
    id: 'src-821',
    kind: 'order' as const,
    title: 'Порядок приёма в вузы, приказ № 821',
    url: 'https://minobrnauki.gov.ru/',
    verified_at: '2026-09-15',
  },
}

export interface DemoStage {
  kind: 'registration' | 'qualifying' | 'final' | 'school' | 'municipal' | 'regional'
  title: string
  subtitle: string
  /** Смещение в днях от сегодня; null — дата пока только словами. */
  offset: number | null
  is_online: boolean
}

export interface DemoOlympiad {
  id: string
  name: string
  organizer: string
  kind: OlympiadKind
  short_name: string
  color: string
  official_url: string
  final_city: string | null
  format: string
  is_online: boolean
  /** Смещение ближайшего дедлайна в днях. */
  deadlineIn: number
  profiles: { subject_code: string; level: Level }[]
  stages: DemoStage[]
  conditions: string[]
  reason: string
  why: Record<Role, string>
  /** Даты этапов пока не выверены по первоисточнику. */
  stages_are_demo: boolean
}

export const OLYMPIADS: DemoOlympiad[] = [
  {
    id: 'vsosh-inf',
    name: 'ВсОШ по информатике',
    organizer: 'Всероссийская олимпиада школьников',
    kind: 'vsosh',
    short_name: 'Вс',
    color: '#D48806',
    official_url: 'https://olimpiada.ru/vseros',
    final_city: null,
    format: 'Первый этап в школе',
    is_online: false,
    // Школьный этап уже прошёл — в трекере он ждёт итога, а срок
    // олимпиады — муниципальный этап.
    deadlineIn: 45,
    profiles: [{ subject_code: 'inf', level: null }],
    stages: [
      { kind: 'school', title: 'Школьный этап', subtitle: 'в школе', offset: -4, is_online: false },
      { kind: 'municipal', title: 'Муниципальный этап', subtitle: 'ноябрь — декабрь', offset: 45, is_online: false },
      { kind: 'regional', title: 'Региональный этап', subtitle: 'январь — февраль', offset: null, is_online: false },
      { kind: 'final', title: 'Заключительный этап', subtitle: 'март — апрель', offset: null, is_online: false },
    ],
    conditions: [
      'Нужен диплом победителя или призёра заключительного этапа',
      'Подтверждать льготу ЕГЭ не нужно',
      'БВИ можно использовать только в одном вузе',
    ],
    reason: 'Главная олимпиада страны, первый этап в школе',
    why: {
      kid: 'Школьный этап проходит в твоей школе, а без него не попасть на следующие этапы. Победа на заключительном этапе даёт БВИ без подтверждения ЕГЭ.',
      parent:
        'Школьный этап проходит в школе Артёма, а без него не попасть на следующие этапы. Победа на заключительном этапе даёт БВИ без подтверждения ЕГЭ.',
    },
    stages_are_demo: true,
  },
  {
    id: 'hse',
    name: 'Высшая проба',
    organizer: 'НИУ ВШЭ',
    kind: 'perechen',
    short_name: 'ВП',
    color: '#6B2BFF',
    official_url: 'https://olymp.hse.ru/',
    final_city: 'Москва',
    format: 'Онлайн-отбор',
    is_online: true,
    deadlineIn: 4,
    profiles: [
      { subject_code: 'inf', level: 'I' },
      { subject_code: 'math', level: 'I' },
      { subject_code: 'econ', level: 'I' },
      { subject_code: 'phys', level: 'II' },
    ],
    stages: [
      { kind: 'registration', title: 'Регистрация', subtitle: 'до', offset: 4, is_online: true },
      { kind: 'qualifying', title: 'Отборочный этап', subtitle: 'октябрь — ноябрь, онлайн', offset: 25, is_online: true },
      { kind: 'final', title: 'Заключительный этап', subtitle: 'февраль, очно', offset: null, is_online: false },
    ],
    conditions: [
      'Нужен диплом победителя или призёра',
      'Льготу подтверждает ЕГЭ по предмету «Информатика»',
      'БВИ можно использовать только в одном вузе',
    ],
    reason: 'Профиль совпадает с целью',
    why: {
      kid: 'Профиль «информатика» совпадает с направлением «Программная инженерия». Отборочный этап онлайн, ехать никуда не нужно.',
      parent:
        'Профиль «информатика» совпадает с целью Артёма. Отборочный этап онлайн, ехать никуда не нужно.',
    },
    stages_are_demo: true,
  },
  {
    id: 'inno',
    name: 'Innopolis Open',
    organizer: 'Университет Иннополис',
    kind: 'perechen',
    short_name: 'IO',
    color: '#1A6DFF',
    official_url: 'https://olymp.innopolis.university/',
    final_city: 'Иннополис',
    format: 'Онлайн-отбор',
    is_online: true,
    deadlineIn: 39,
    profiles: [
      { subject_code: 'inf', level: 'II' },
      { subject_code: 'math', level: 'II' },
      { subject_code: 'ai', level: 'III' },
      { subject_code: 'infosec', level: 'III' },
      { subject_code: 'robo', level: 'III' },
    ],
    stages: [
      { kind: 'registration', title: 'Регистрация', subtitle: 'до', offset: 39, is_online: true },
      { kind: 'qualifying', title: 'Отборочные туры', subtitle: 'ноябрь и январь, онлайн', offset: 47, is_online: true },
      { kind: 'final', title: 'Финал', subtitle: 'март, Иннополис', offset: null, is_online: false },
    ],
    conditions: [
      'Нужен диплом победителя или призёра',
      'ЕГЭ по профильному предмету от 75 баллов',
      'БВИ можно использовать только в одном вузе',
    ],
    reason: 'Финал проходит в Татарстане',
    why: {
      kid: 'Финал проходит в Иннополисе, недалеко от дома. Твой профиль: информатика, это II уровень.',
      parent: 'Финал проходит в Иннополисе, недалеко от дома. Профиль Артёма: информатика, это II уровень.',
    },
    stages_are_demo: true,
  },
  {
    id: 'tk',
    name: 'Технокубок',
    organizer: 'VK Education и МФТИ',
    kind: 'perechen',
    short_name: 'ТК',
    color: '#E92E78',
    official_url: 'https://technocup.mail.ru/',
    final_city: 'Москва',
    format: 'Онлайн-отбор',
    is_online: true,
    deadlineIn: 17,
    profiles: [{ subject_code: 'inf', level: 'I' }],
    stages: [
      { kind: 'registration', title: 'Регистрация', subtitle: 'до', offset: 17, is_online: true },
      { kind: 'qualifying', title: 'Отборочные раунды', subtitle: 'октябрь — декабрь, онлайн', offset: 30, is_online: true },
      { kind: 'final', title: 'Финал', subtitle: 'весна, очно', offset: null, is_online: false },
    ],
    conditions: [
      'Нужен диплом победителя или призёра',
      'ЕГЭ по информатике от 75 баллов',
      'БВИ можно использовать только в одном вузе',
    ],
    reason: 'Практика спортивного программирования',
    why: {
      kid: 'Олимпиада по программированию тренирует ровно те навыки, которые нужны на ИТ-направлениях.',
      parent:
        'Олимпиада по программированию тренирует ровно те навыки, которые нужны Артёму на ИТ-направлениях.',
    },
    stages_are_demo: true,
  },
  {
    id: 'lomo',
    name: 'Ломоносов',
    organizer: 'МГУ',
    kind: 'perechen',
    short_name: 'Л',
    color: '#0C8F62',
    official_url: 'https://olymp.msu.ru/',
    final_city: 'Москва',
    format: 'Очно и онлайн',
    is_online: true,
    deadlineIn: 29,
    profiles: [
      { subject_code: 'inf', level: 'I' },
      { subject_code: 'math', level: 'I' },
      { subject_code: 'phys', level: 'I' },
    ],
    stages: [
      { kind: 'registration', title: 'Регистрация', subtitle: 'до', offset: 29, is_online: true },
      { kind: 'qualifying', title: 'Отборочный этап', subtitle: 'ноябрь, онлайн', offset: 45, is_online: true },
      { kind: 'final', title: 'Заключительный этап', subtitle: 'февраль — март, очно', offset: null, is_online: false },
    ],
    conditions: [
      'Нужен диплом победителя или призёра',
      'ЕГЭ по профильному предмету от 75 баллов',
      'БВИ можно использовать только в одном вузе',
    ],
    reason: 'Можно участвовать по двум профилям',
    why: {
      kid: 'Можно пойти сразу по информатике и математике: это два шанса получить диплом за один сезон.',
      parent:
        'Артём может пойти сразу по информатике и математике: это два шанса получить диплом за один сезон.',
    },
    stages_are_demo: true,
  },
  {
    id: 'tyk',
    name: 'Турнир юных программистов Казани',
    organizer: 'Демонстрационный пример',
    kind: 'other',
    short_name: 'ТП',
    color: '#7A7E90',
    official_url: 'https://example.org/',
    final_city: 'Казань',
    format: 'Очно',
    is_online: false,
    deadlineIn: 24,
    profiles: [{ subject_code: 'inf', level: null }],
    stages: [
      { kind: 'registration', title: 'Регистрация', subtitle: 'до', offset: 24, is_online: false },
      { kind: 'final', title: 'Турнир', subtitle: 'ноябрь, Казань', offset: null, is_online: false },
    ],
    conditions: [
      'Льгот при поступлении не даёт',
      'Дополнительные баллы: не больше 10 за все достижения вместе',
    ],
    reason: 'Льгот нет, только дополнительные баллы',
    why: {
      kid: 'Хорошая практика перед перечневыми олимпиадами. В КФУ диплом даст дополнительные баллы.',
      parent:
        'Хорошая практика для Артёма перед перечневыми олимпиадами. В КФУ диплом даст дополнительные баллы.',
    },
    stages_are_demo: true,
  },
]

export interface DemoUniversity {
  id: string
  short_name: string
  /** Как вуз называют в строке «Иннополис, ВШЭ: БВИ» — короче полного имени. */
  nick: string
  name: string
  city: string
  color: string
  /** Направления вуза (F65): id из DIRECTIONS. */
  offered: DemoOffered[]
  ege_note: string
  rules_url: string
  rules_verified_at: string
  /** Льгота по олимпиаде: id олимпиады → вид льготы. */
  benefits: Record<string, BenefitKind>
  /** Чем правила по олимпиаде отличаются от общих (F19): id олимпиады → оговорки. */
  rules?: Record<string, DemoRule>
  /**
   * Льгота на отдельных направлениях, если она не такая, как у вуза целиком:
   * id олимпиады → id направления → льгота (null — на направлении льготы нет).
   * На остальных направлениях (кроме to_check) — льгота вуза.
   */
  byDirection?: Record<string, Record<string, BenefitKind | null>>
  /** Где льгота зависит от программы: id олимпиады → id направлений. */
  varies?: Record<string, string[]>
}

export interface DemoOffered {
  id: string
  /** to_check — льготы на направлении ещё уточняются. */
  status?: 'to_check'
  programs: number
  budget_places: number | null
}

/** Оговорки вуза: что получит призёр, разброс порога, своё — строкой под вузом. */
export interface DemoRule {
  prizer?: 'score100' | 'none'
  egeMax?: number
  notes?: string[]
}

export const UNIVERSITIES: DemoUniversity[] = [
  {
    id: 'inno',
    short_name: 'УИ',
    nick: 'Иннополис',
    name: 'Университет Иннополис',
    city: 'Иннополис',
    color: '#1A6DFF',
    // Иннополис даёт укрупнённую группу 09.00.00 — она покрывает цель 09.03.04.
    offered: [{ id: 'dir-it', programs: 6, budget_places: 250 }],
    ege_note: 'от 75 баллов по профильному предмету и от 60 по двум другим',
    rules_url: 'https://innopolis.university/',
    rules_verified_at: '2026-09-15',
    benefits: { inno: 'bvi', hse: 'bvi', 'vsosh-inf': 'bvi', lomo: 'score100', tk: 'score100' },
    rules: { inno: { notes: ['ЕГЭ по двум другим предметам — от 60'] } },
  },
  {
    id: 'kfu',
    short_name: 'КФУ',
    nick: 'КФУ',
    name: 'Казанский федеральный университет',
    city: 'Казань',
    color: '#0C8F62',
    offered: [
      { id: 'dir-se', programs: 2, budget_places: 75 },
      { id: 'dir-ivt', programs: 3, budget_places: 120 },
      { id: 'dir-pi', programs: 2, budget_places: 50 },
      { id: 'dir-math', programs: 1, budget_places: 60 },
      { id: 'dir-is', status: 'to_check', programs: 1, budget_places: 25 },
    ],
    ege_note: 'от 75 баллов по профильному предмету',
    rules_url: 'https://kpfu.ru/',
    rules_verified_at: '2026-09-15',
    benefits: { 'vsosh-inf': 'bvi', lomo: 'bvi', hse: 'score100', inno: 'score100', tyk: 'extra_points' },
  },
  {
    id: 'hse',
    short_name: 'ВШЭ',
    nick: 'ВШЭ',
    name: 'НИУ ВШЭ',
    city: 'Москва',
    color: '#6B2BFF',
    offered: [
      { id: 'dir-se', programs: 3, budget_places: 90 },
      { id: 'dir-ami', programs: 2, budget_places: 140 },
      { id: 'dir-ivt', programs: 2, budget_places: 80 },
      { id: 'dir-is', programs: 1, budget_places: 40 },
      { id: 'dir-math', programs: 1, budget_places: 70 },
      { id: 'dir-econ', programs: 2, budget_places: 160 },
      { id: 'dir-bi', programs: 2, budget_places: 60 },
      { id: 'dir-mgmt', programs: 3, budget_places: 50 },
    ],
    ege_note: 'от 75 до 80 баллов по профильному предмету',
    rules_url: 'https://www.hse.ru/',
    rules_verified_at: '2026-09-15',
    benefits: { 'vsosh-inf': 'bvi', hse: 'bvi', lomo: 'bvi', tk: 'score100' },
    rules: { hse: { egeMax: 90 } },
    // На Программную инженерию «Высшая проба» даёт 100 баллов, а не БВИ, а
    // Технокубок её не учитывает — демо льготы по направлениям (F65).
    byDirection: {
      hse: { 'dir-se': 'score100', 'dir-mgmt': null },
      tk: { 'dir-se': null, 'dir-econ': null, 'dir-mgmt': null, 'dir-bi': null },
      lomo: { 'dir-econ': 'score100', 'dir-mgmt': null },
    },
    varies: { hse: ['dir-se'] },
  },
  {
    id: 'itmo',
    short_name: 'ИТМО',
    nick: 'ИТМО',
    name: 'Университет ИТМО',
    city: 'Санкт-Петербург',
    color: '#E92E78',
    offered: [
      { id: 'dir-se', programs: 4, budget_places: 200 },
      { id: 'dir-ivt', programs: 5, budget_places: 260 },
      { id: 'dir-is', programs: 2, budget_places: 90 },
      { id: 'dir-infosys', programs: 3, budget_places: 150 },
    ],
    ege_note: 'от 75 баллов по профильному предмету',
    rules_url: 'https://itmo.ru/',
    rules_verified_at: '2026-09-15',
    benefits: { 'vsosh-inf': 'bvi', tk: 'bvi', hse: 'bvi' },
  },
  {
    id: 'mipt',
    short_name: 'МФТИ',
    nick: 'МФТИ',
    name: 'МФТИ',
    city: 'Долгопрудный',
    color: '#D48806',
    offered: [
      { id: 'dir-ami', programs: 4, budget_places: 300 },
      { id: 'dir-ivt', programs: 2, budget_places: 120 },
      { id: 'dir-phys', programs: 3, budget_places: 250 },
    ],
    ege_note: 'от 75 баллов по профильному предмету',
    rules_url: 'https://mipt.ru/',
    rules_verified_at: '2026-09-15',
    benefits: { 'vsosh-inf': 'bvi', tk: 'bvi_winners', lomo: 'bvi_winners' },
    rules: { tk: { prizer: 'score100' }, lomo: { prizer: 'score100' } },
  },
]

/** Порог ЕГЭ для подтверждения льготы, если вуз не указал свой. */
export const DEFAULT_EGE_MIN = 75

export { dayStart }

export interface DemoDirection {
  id: string
  name: string
  code: string
  groups: string[]
  /** Основное — чипом в профиле, как в онбординге бота. */
  popular: boolean
}

/** Направления для цели и вузов; id «dir-se» — у демо-траектории. */
export const DIRECTIONS: DemoDirection[] = [
  { id: 'dir-ami', name: 'Прикладная математика и информатика', code: '01.03.02', groups: ['ИТ'], popular: true },
  { id: 'dir-phys', name: 'Физика', code: '03.03.02', groups: ['Физика'], popular: true },
  { id: 'dir-bio', name: 'Биология', code: '06.03.01', groups: ['Биомед'], popular: true },
  { id: 'dir-ivt', name: 'Информатика и вычислительная техника', code: '09.03.01', groups: ['ИТ'], popular: true },
  { id: 'dir-se', name: 'Программная инженерия', code: '09.03.04', groups: ['ИТ'], popular: true },
  { id: 'dir-is', name: 'Информационная безопасность', code: '10.03.01', groups: ['ИТ'], popular: true },
  { id: 'dir-econ', name: 'Экономика', code: '38.03.01', groups: ['Экономика'], popular: true },
  { id: 'dir-mgmt', name: 'Менеджмент', code: '38.03.02', groups: ['Экономика'], popular: true },
  { id: 'dir-bi', name: 'Бизнес-информатика', code: '38.03.05', groups: ['ИТ', 'Экономика'], popular: true },
  { id: 'dir-math', name: 'Математика', code: '01.03.01', groups: [], popular: false },
  { id: 'dir-it', name: 'Информатика и вычислительная техника', code: '09.00.00', groups: ['ИТ'], popular: false },
  { id: 'dir-infosys', name: 'Информационные системы и технологии', code: '09.03.02', groups: ['ИТ'], popular: false },
  { id: 'dir-pi', name: 'Прикладная информатика', code: '09.03.03', groups: ['ИТ', 'Экономика'], popular: false },
  { id: 'dir-chem', name: 'Химия', code: '04.03.01', groups: [], popular: false },
  { id: 'dir-med', name: 'Лечебное дело', code: '31.05.01', groups: ['Биомед'], popular: true },
]
