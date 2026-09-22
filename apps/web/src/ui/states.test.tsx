import { describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { StateBlock } from './primitives'
import { renderApp } from '@/test/render'

/**
 * Экраны H3 и H4 сверяются со скриншотами и оцениваются отдельно: по ТЗ §4.11
 * у каждого состояния должно быть сказано, что случилось и что делать дальше.
 * Состояние без действия — это тупик, а тупиков в основном сценарии быть
 * не должно (критерий «UX/UI» хакатона).
 */
describe('состояния экранов', () => {
  it('пустой результат предлагает сбросить фильтры, а не просто сообщает', async () => {
    const onReset = vi.fn()
    renderApp(
      <StateBlock icon="search" title="Под эти фильтры олимпиад нет" text="Сбрось фильтры или добавь предмет в профиле.">
        <button type="button" onClick={onReset}>
          Сбросить фильтры
        </button>
      </StateBlock>,
    )

    expect(screen.getByText('Под эти фильтры олимпиад нет')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Сбросить фильтры' }))
    expect(onReset).toHaveBeenCalledOnce()
  })

  it('ошибка сети говорит, что данные сохранены, и даёт повторить', async () => {
    const onRetry = vi.fn()
    renderApp(
      <StateBlock
        icon="wifiOff"
        tone="error"
        title="Не загрузилось"
        text="Нет соединения с интернетом. Профиль и трекер сохранены — ничего не потеряно."
      >
        <button type="button" onClick={onRetry}>
          Повторить загрузку
        </button>
      </StateBlock>,
    )

    expect(screen.getByText(/ничего не потеряно/)).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Повторить загрузку' }))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it('состояние объявляется вспомогательным технологиям', () => {
    renderApp(<StateBlock icon="calendar" title="В трекере пока пусто" />)
    expect(screen.getByRole('status')).toBeInTheDocument()
  })
})
