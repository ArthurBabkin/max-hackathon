import { describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { OlympiadCard as CardData } from '@contract'
import { OlympiadCard } from './OlympiadCard'
import { trackerAction } from '@/lib/permissions'
import { makeSession, renderApp } from '@/test/render'

const card: CardData = {
  short_name: 'ВП',
  color: '#6B2BFF',
  olympiad_profile_id: 'hse:inf',
  olympiad_id: 'hse',
  name: 'Высшая проба',
  organizer: 'НИУ ВШЭ',
  kind: 'perechen',
  level: 'I',
  subject_code: 'inf',
  subject_name: 'Информатика',
  format: 'Онлайн-отбор',
  is_online: true,
  final_city: 'Москва',
  deadline_at: new Date(Date.now() + 4 * 86_400_000).toISOString(),
  next_stage_title: 'Регистрация',
  benefits_summary: 'Иннополис, ВШЭ: БВИ',
  reason: 'Профиль совпадает с целью',
  in_tracker: false,
  proposal_status: null,
}

/**
 * Пересечение голоса (ТЗ F3) и прав (ТЗ §3.1) — единственное место, где они
 * обязаны сойтись в одной кнопке. Ошибка здесь видна жюри на демонстрации.
 */
describe('OlympiadCard — кнопка зависит от роли', () => {
  it('ученик добавляет в трекер', async () => {
    const session = makeSession({ role: 'kid' })
    renderApp(
      <OlympiadCard card={card} action={trackerAction(session)} onOpen={vi.fn()} onTrack={vi.fn()} />,
      { session },
    )

    expect(await screen.findByRole('button', { name: 'Добавить в трекер' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Предложить/ })).not.toBeInTheDocument()
  })

  it('родитель при ученике в траектории предлагает, и с именем в дательном падеже', async () => {
    const session = makeSession({ role: 'parent', is_creator: true, has_kid: true })
    renderApp(
      <OlympiadCard card={card} action={trackerAction(session)} onOpen={vi.fn()} onTrack={vi.fn()} />,
      { session },
    )

    expect(await screen.findByRole('button', { name: 'Предложить Артёму' })).toBeInTheDocument()
  })

  it('родитель без ученика в траектории добавляет сам (ТЗ §3.2)', async () => {
    const session = makeSession({ role: 'parent', is_creator: true, has_kid: false })
    renderApp(
      <OlympiadCard card={card} action={trackerAction(session)} onOpen={vi.fn()} onTrack={vi.fn()} />,
      { session },
    )

    expect(await screen.findByRole('button', { name: 'Добавить в трекер' })).toBeInTheDocument()
  })

  it('уже в трекере — вместо кнопки отметка, повторно не добавить', async () => {
    const session = makeSession({ role: 'kid' })
    const onTrack = vi.fn()
    renderApp(
      <OlympiadCard
        card={{ ...card, in_tracker: true }}
        action="add"
        onOpen={vi.fn()}
        onTrack={onTrack}
      />,
      { session },
    )

    expect(screen.queryByRole('button', { name: 'Добавить в трекер' })).not.toBeInTheDocument()
    expect(await screen.findByLabelText('В трекере')).toBeInTheDocument()
  })

  it('нажатие на карточку открывает её, а не добавляет в трекер', async () => {
    const onOpen = vi.fn()
    const onTrack = vi.fn()
    renderApp(<OlympiadCard card={card} action="add" onOpen={onOpen} onTrack={onTrack} />)

    await userEvent.click(await screen.findByText('Высшая проба'))

    expect(onOpen).toHaveBeenCalledWith('hse:inf')
    expect(onTrack).not.toHaveBeenCalled()
  })

  it('показывает льготу в вузах ученика и причину рекомендации', async () => {
    renderApp(<OlympiadCard card={card} action="add" onOpen={vi.fn()} onTrack={vi.fn()} />)

    expect(await screen.findByText('Иннополис, ВШЭ: БВИ')).toBeInTheDocument()
    expect(screen.getByText('Профиль совпадает с целью')).toBeInTheDocument()
    expect(screen.getByText('4 дня')).toBeInTheDocument()
  })
})
