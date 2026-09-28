import { screen } from '@testing-library/react'
import type { CalendarMonth, TrackerItem, TrackerStage } from '@contract'
import { expect, it, vi } from 'vitest'
import { renderApp } from '@/test/render'
import { CalendarView } from './Calendar'

const stage = (id: string, kind: TrackerStage['kind'], deadline_at: string): TrackerStage => ({
  id,
  kind,
  title: kind === 'registration' ? 'Регистрация на осенний тур (портал Сириус)' : 'Осенний тур',
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

const STAGES = [stage('reg', 'registration', '2026-10-02T09:00:00Z'), stage('qual', 'qualifying', '2026-10-04T09:00:00Z')]

// Сервер кладёт пункт в каждый день, где у него срок, со сроком этого этапа.
const turlom = (deadline_at: string, next_stage_title: string) =>
  ({
    id: 'turlom',
    olympiad_profile_id: 'p669-82-matematika',
    olympiad_id: 'p669-82',
    olympiad_name: 'Турнир имени М.В. Ломоносова',
    subject_name: 'Математика',
    kind: 'perechen',
    level: 'II',
    short_name: null,
    color: null,
    deadline_at,
    next_stage_title,
    registered_at: null,
    registered_by: null,
    added_by: null,
    status: 'open',
    outcome: null,
    stages: STAGES,
    action: null,
  }) as TrackerItem

// Картинки 4–6 из отчёта: у одной олимпиады два срока, и оба были подписаны
// «Регистрация до», хотя 4 октября — осенний тур. Подпись — как в полоске трекера.
it('в календаре у каждого срока — свой этап', () => {
  const data: CalendarMonth = {
    month: '2026-10',
    days: [
      { date: '2026-10-02', items: [turlom('2026-10-02T09:00:00Z', 'Регистрация на осенний тур (портал Сириус)')] },
      { date: '2026-10-04', items: [turlom('2026-10-04T09:00:00Z', 'Осенний тур')] },
    ],
  }
  renderApp(<CalendarView month="2026-10" data={data} onMonthChange={vi.fn()} onOpen={vi.fn()} />)
  expect(screen.getByText('Регистрация до 2 октября')).toBeInTheDocument()
  expect(screen.getByText('Отбор до 4 октября')).toBeInTheDocument()
  expect(screen.queryByText('Регистрация до 4 октября')).not.toBeInTheDocument()
})
