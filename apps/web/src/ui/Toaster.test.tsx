import { act, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useMutation } from '@tanstack/react-query'
import { afterEach, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/errors'
import { renderApp } from '@/test/render'
import { Toaster } from './Toaster'
import { dismissToast, showInfoToast } from './toast'

afterEach(() => {
  act(() => dismissToast())
  vi.useRealTimers()
})

/** Кнопка, чьё действие падает с ошибкой error. */
function Failing({ error, silent = false }: { error: unknown; silent?: boolean }) {
  const m = useMutation({ mutationFn: () => Promise.reject(error), meta: { silentError: silent } })
  return (
    <>
      <button type="button" onClick={() => m.mutate()}>
        Сохранить
      </button>
      <Toaster />
    </>
  )
}

// Ошибка действия раньше терялась: кнопка просто переставала крутиться.
it('показывает объяснение сервера, если действие отклонено', async () => {
  renderApp(<Failing error={new ApiError(400, 'BAD_REQUEST', 'Имя — от 1 до 40 символов.')} />)
  await userEvent.click(screen.getByRole('button', { name: 'Сохранить' }))

  expect(await screen.findByRole('alert')).toHaveTextContent('Имя — от 1 до 40 символов.')
})

it('без сети — «не сохранились», и сообщение можно закрыть', async () => {
  renderApp(<Failing error={new ApiError(0, 'INTERNAL', 'x')} />)
  await userEvent.click(screen.getByRole('button', { name: 'Сохранить' }))

  expect(await screen.findByRole('alert')).toHaveTextContent('Нет соединения, изменения не сохранились')
  await userEvent.click(screen.getByRole('button', { name: 'Закрыть' }))
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

it('уходит само через несколько секунд', async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  renderApp(<Failing error={new ApiError(500, 'INTERNAL', 'x')} />)
  await userEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('сервер ответил ошибкой')

  act(() => vi.advanceTimersByTime(5000))
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

it('молчит, если ошибку показывает сам экран', async () => {
  renderApp(<Failing error={new ApiError(500, 'INTERNAL', 'x')} silent />)
  await userEvent.click(screen.getByRole('button', { name: 'Сохранить' }))

  await act(() => Promise.resolve())
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

// Добавленная из «Подбора» карточка исчезает — без подтверждения это похоже на сбой.
it('подтверждение действия — статусом, не тревогой', async () => {
  renderApp(<Toaster />)
  act(() => showInfoToast('Добавлено в трекер'))
  expect(await screen.findByRole('status')).toHaveTextContent('Добавлено в трекер')
  expect(screen.queryByRole('alert')).toBeNull()
})
