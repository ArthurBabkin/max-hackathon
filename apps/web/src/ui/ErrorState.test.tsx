import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { ApiError } from '@/api/errors'
import { makeSession, renderApp } from '@/test/render'
import { ErrorState } from './ErrorState'

const retryButton = () => screen.queryByRole('button', { name: /Повторить загрузку/ })

// ТЗ §4.11: «что случилось» — по настоящей причине, а не всегда «нет интернета».
it('без сети — «нет соединения» и повтор', async () => {
  const retry = vi.fn()
  renderApp(<ErrorState error={new ApiError(0, 'INTERNAL', 'x')} title="Подбор не загрузился" onRetry={retry} />)

  expect(screen.getByText('Подбор не загрузился')).toBeInTheDocument()
  expect(screen.getByText(/Нет соединения с интернетом/)).toBeInTheDocument()
  await userEvent.click(retryButton()!)
  expect(retry).toHaveBeenCalledOnce()
})

it('сбой сервера — про сервер, а не про интернет', () => {
  renderApp(<ErrorState error={new ApiError(500, 'INTERNAL', 'Что-то пошло не так.')} onRetry={() => {}} />)

  expect(screen.getByText('Не загрузилось')).toBeInTheDocument()
  expect(screen.getByText(/Сервер ответил ошибкой/)).toBeInTheDocument()
  expect(screen.queryByText(/интернет/)).not.toBeInTheDocument()
  expect(retryButton()).toBeInTheDocument()
})

it('не найдено — свой заголовок и без бесполезного повтора', () => {
  renderApp(<ErrorState error={new ApiError(404, 'NOT_FOUND', 'Не найдено.')} title="Карточка" onRetry={() => {}} />)

  expect(screen.getByText('Не нашли')).toBeInTheDocument()
  expect(retryButton()).not.toBeInTheDocument()
})

it('отказ сервера — его собственное объяснение', () => {
  renderApp(<ErrorState error={new ApiError(403, 'FORBIDDEN', 'Править профиль нельзя.')} onRetry={() => {}} />)

  expect(screen.getByText('Недоступно')).toBeInTheDocument()
  expect(screen.getByText('Править профиль нельзя.')).toBeInTheDocument()
  expect(retryButton()).not.toBeInTheDocument()
})

it('родителю — на «вы»', () => {
  renderApp(<ErrorState error={new ApiError(429, 'X', 'x')} onRetry={() => {}} />, {
    session: makeSession({ role: 'parent' }),
  })
  expect(screen.getByText(/Подождите минуту/)).toBeInTheDocument()
})
