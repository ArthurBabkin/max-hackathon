import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { markTutorialSeen, tutorialSeen } from '@/lib/tutorial'
import { makeSession, renderApp } from '@/test/render'
import { Tutorial } from './Tutorial'

afterEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

const next = () => userEvent.click(screen.getByRole('button', { name: 'Дальше' }))

it('ведёт по пяти шагам и на последнем закрывается с отметкой «видел»', async () => {
  const onDone = vi.fn()
  renderApp(<Tutorial onDone={onDone} />)

  expect(screen.getByRole('dialog', { name: 'Как пользоваться «Траекторией»' })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: 'Подбор под твою цель' })).toBeInTheDocument()

  await next()
  expect(screen.getByRole('heading', { name: 'Карточка олимпиады' })).toBeInTheDocument()
  await next()
  expect(screen.getByRole('heading', { name: 'Трекер и напоминания' })).toBeInTheDocument()
  await next()
  expect(screen.getByRole('heading', { name: 'Семья' })).toBeInTheDocument()
  await next()
  expect(screen.getByRole('heading', { name: 'Помощник' })).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'Начать' }))

  await waitFor(() => expect(onDone).toHaveBeenCalledOnce())
  expect(tutorialSeen()).toBe(true)
})

it('«Назад» появляется со второго шага и возвращает на предыдущий', async () => {
  renderApp(<Tutorial onDone={() => {}} />)
  expect(screen.queryByRole('button', { name: 'Назад' })).not.toBeInTheDocument()
  expect(screen.getByLabelText('Шаг 1 из 5')).toBeInTheDocument()

  await next()
  expect(screen.getByLabelText('Шаг 2 из 5')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'Назад' }))
  expect(screen.getByRole('heading', { name: 'Подбор под твою цель' })).toBeInTheDocument()
})

it('«Пропустить» закрывает сразу и тоже запоминает', async () => {
  const onDone = vi.fn()
  renderApp(<Tutorial onDone={onDone} />)
  await userEvent.click(screen.getByRole('button', { name: 'Пропустить' }))
  expect(tutorialSeen()).toBe(true)
  await waitFor(() => expect(onDone).toHaveBeenCalledOnce())
})

it('Esc закрывает и запоминает', async () => {
  const onDone = vi.fn()
  renderApp(<Tutorial onDone={onDone} />)
  fireEvent.keyDown(window, { key: 'Escape' })
  expect(tutorialSeen()).toBe(true)
  await waitFor(() => expect(onDone).toHaveBeenCalledOnce())
})

it('подсвечивает настоящий элемент интерфейса, о котором шаг', () => {
  renderApp(
    <>
      <a href="#/match" data-tour="match">
        Подбор
      </a>
      <Tutorial onDone={() => {}} />
    </>,
  )
  const target = document.querySelector<HTMLElement>('[data-tour="match"]')!
  vi.spyOn(target, 'getBoundingClientRect').mockReturnValue(new DOMRect(80, 740, 70, 50))
  fireEvent(window, new Event('resize'))

  expect(screen.getByRole('dialog')).toHaveAttribute('data-anchored', 'true')
})

it('новичку без сроков вместо олимпиады подсвечивает «Каталог»', async () => {
  renderApp(
    <>
      <div data-tour="upcoming" />
      <a href="#/catalog" data-tour="catalog">
        Каталог
      </a>
      <Tutorial onDone={() => {}} />
    </>,
  )
  const catalog = document.querySelector<HTMLElement>('[data-tour="catalog"]')!
  vi.spyOn(catalog, 'getBoundingClientRect').mockReturnValue(new DOMRect(150, 740, 70, 50))

  await next()
  expect(screen.getByRole('dialog')).toHaveAttribute('data-anchored', 'true')
})

it('без элемента на экране шаг не ломается: карточка встаёт по центру', () => {
  renderApp(<Tutorial onDone={() => {}} />)
  expect(screen.getByRole('dialog')).toHaveAttribute('data-anchored', 'false')
  expect(screen.getByRole('heading', { name: 'Подбор под твою цель' })).toBeInTheDocument()
})

it('родителю — на «вы»', async () => {
  renderApp(<Tutorial onDone={() => {}} />, { session: makeSession({ role: 'parent' }) })
  expect(screen.getByText(/олимпиады для Артёма/)).toBeInTheDocument()
  await next()
  expect(screen.getByText(/Нажмите на олимпиаду/)).toBeInTheDocument()
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
