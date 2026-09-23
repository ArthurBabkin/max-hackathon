import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { markTutorialSeen, tutorialSeen } from '@/lib/tutorial'
import { makeSession, renderApp } from '@/test/render'
import { Tutorial } from './Tutorial'

afterEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

it('ведёт по шагам и на последнем закрывается с отметкой «видел»', async () => {
  const onDone = vi.fn()
  renderApp(<Tutorial onDone={onDone} />)

  expect(screen.getByRole('dialog', { name: 'Как пользоваться «Траекторией»' })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: 'Подбор под твою цель' })).toBeInTheDocument()

  await userEvent.click(screen.getByRole('button', { name: 'Дальше' }))
  expect(screen.getByRole('heading', { name: 'Карточка олимпиады' })).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'Дальше' }))
  await userEvent.click(screen.getByRole('button', { name: 'Дальше' }))
  await userEvent.click(screen.getByRole('button', { name: 'Начать' }))

  expect(onDone).toHaveBeenCalledOnce()
  expect(tutorialSeen()).toBe(true)
})

it('«Пропустить» закрывает сразу и тоже запоминает', async () => {
  const onDone = vi.fn()
  renderApp(<Tutorial onDone={onDone} />)
  await userEvent.click(screen.getByRole('button', { name: 'Пропустить' }))
  expect(onDone).toHaveBeenCalledOnce()
  expect(tutorialSeen()).toBe(true)
})

it('родителю — на «вы»', () => {
  renderApp(<Tutorial onDone={() => {}} />, { session: makeSession({ role: 'parent' }) })
  expect(screen.getByText(/цель Артёма/)).toBeInTheDocument()
})

it('без localStorage (приватный режим, вебвью) не падает: считаем, что не видел', () => {
  vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
    throw new Error('SecurityError')
  })
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
    throw new Error('SecurityError')
  })
  expect(tutorialSeen()).toBe(false)
  expect(() => markTutorialSeen()).not.toThrow()
})
