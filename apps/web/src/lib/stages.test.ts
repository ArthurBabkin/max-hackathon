import type { TrackerItem, TrackerStage } from '@contract'
import { expect, it } from 'vitest'
import { text } from '@/voice/texts'
import type { Translate } from '@/voice/useVoice'
import { deadlineLabel } from './stages'

const t: Translate = (key, vars) => text(key, 'kid', vars)

const stage = (id: string, kind: TrackerStage['kind'], deadline_at: string | null, title: string = kind): TrackerStage => ({
  id,
  kind,
  title,
  subtitle: null,
  starts_at: null,
  ends_at: null,
  deadline_at,
  state: 'future',
  registered: false,
  result: null,
  can_register: false,
  results: [],
  results_allowed: [],
  asking: false,
})

// Турнир Ломоносова: регистрация до 2 октября, осенний тур до 4-го, финал в марте.
const lomonosov = (deadline_at: string | null, kind: TrackerItem['kind'] = 'perechen') =>
  ({
    kind,
    deadline_at,
    next_stage_title: 'Регистрация на осенний тур (портал Сириус)',
    stages: [
      stage('reg', 'registration', '2026-10-02T09:00:00Z', 'Регистрация на осенний тур (портал Сириус)'),
      stage('qual', 'qualifying', '2026-10-04T09:00:00Z', 'Осенний тур (XLIX Турнир имени М. В. Ломоносова)'),
      stage('fin', 'final', '2027-03-13T09:00:00Z', 'Заключительный этап'),
    ],
  }) as TrackerItem

// В календаре у каждого срока свой этап — подпись та же, что в полоске
// трекера, а не «Регистрация до» на всё подряд.
it('подпись срока — этап, к которому он относится', () => {
  expect(deadlineLabel(lomonosov('2026-10-02T09:00:00Z'), t)).toBe('Регистрация до 2 октября')
  expect(deadlineLabel(lomonosov('2026-10-04T09:00:00Z'), t)).toBe('Отбор до 4 октября')
  expect(deadlineLabel(lomonosov('2027-03-13T09:00:00Z'), t)).toBe('Финал до 13 марта')
})

it('одинаковые этапы нумеруются, как в полоске', () => {
  const item = {
    ...lomonosov('2026-11-02T09:00:00Z'),
    stages: [stage('q1', 'qualifying', '2026-10-02T09:00:00Z'), stage('q2', 'qualifying', '2026-11-02T09:00:00Z')],
  } as TrackerItem
  expect(deadlineLabel(item, t)).toBe('Отбор 2 до 2 ноября')
})

it('у ВсОШ — название этапа целиком', () => {
  const item = {
    kind: 'vsosh',
    deadline_at: '2026-12-25T09:00:00Z',
    next_stage_title: 'Муниципальный этап',
    stages: [stage('mun', 'municipal', '2026-12-25T09:00:00Z', 'Муниципальный этап')],
  } as TrackerItem
  expect(deadlineLabel(item, t)).toBe('Муниципальный этап до 25 декабря')
})

it('этап не нашёлся — его название из пункта, без выдуманной регистрации', () => {
  const item = { ...lomonosov('2026-10-21T09:00:00Z'), next_stage_title: 'Отборочный этап', stages: [] } as TrackerItem
  expect(deadlineLabel(item, t)).toBe('Отборочный этап до 21 октября')
})

it('срока нет — подписи нет', () => {
  expect(deadlineLabel(lomonosov(null), t)).toBeNull()
})
