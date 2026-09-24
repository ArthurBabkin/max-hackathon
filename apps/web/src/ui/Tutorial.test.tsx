import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from '@/App'
import { state } from '@/api/mocks/state'
import { markTutorialSeen, shouldShowTutorial, tutorialSeen } from '@/lib/tutorial'
import { renderApp } from '@/test/render'
import { Tutorial, TutorialPace } from './Tutorial'

// Приложение целиком на моках разработки: туториал сам ходит по вкладкам,
// поэтому проверять его надо на настоящих экранах, а не на заглушках.
// Паузы нулевые — показ нажатий проверяется по журналу подсказок.
const FAST = { point: 0, press: 0, wait: 3000 }

let taps: string[] = []
let observer: MutationObserver | null = null

beforeEach(() => {
  Element.prototype.scrollTo = () => {} // в jsdom прокрутки нет, а лист помощника листает ленту
  localStorage.clear()
  state.viewerId = 'mem-artem'
  taps = []
  observer = new MutationObserver(() => {
    const hint = document.querySelector('.tour [role="status"]')?.textContent
    if (hint && taps.at(-1) !== hint) taps.push(hint)
  })
  observer.observe(document.body, { subtree: true, childList: true, characterData: true })
})

afterEach(() => {
  observer?.disconnect()
  localStorage.clear()
  vi.restoreAllMocks()
})

function renderTour() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <TutorialPace.Provider value={FAST}>
          <App />
        </TutorialPace.Provider>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const step = (name: string) => screen.findByRole('heading', { name }, { timeout: 5000 })
const path = () => document.querySelector('.tour-path')?.textContent
const next = () => userEvent.click(screen.getByRole('button', { name: 'Дальше' }))
const back = () => userEvent.click(screen.getByRole('button', { name: 'Назад' }))
const tab = (name: string) => within(screen.getByRole('navigation', { name: 'Разделы приложения' })).getByRole('link', { name })
/** Нажатия, показанные с прошлого вызова. */
const shown = () => taps.splice(0)

describe('обзор приложения', () => {
  it('ведёт по девяти шагам и показывает каждое нажатие по дороге', { timeout: 30_000 }, async () => {
    renderTour()

    await step('Это главная')
    expect(screen.getByRole('dialog', { name: 'Как пользоваться «Траекторией»' })).toBeInTheDocument()
    expect(path()).toBe('Главная')
    expect(screen.getByText('1 из 9')).toBeInTheDocument()
    shown()

    await next()
    await step('Все олимпиады')
    // «Олимпиады» в каталоге открыты сразу — нажимать нечего.
    expect(shown()).toEqual(['Нажимаем «Каталог»'])
    expect(path()).toBe('Каталог → Олимпиады')
    expect(tab('Каталог')).toHaveAttribute('aria-current', 'page')

    await next()
    await step('И вузы')
    expect(shown()).toEqual(['Нажимаем «Вузы»'])
    expect(path()).toBe('Каталог → Вузы')
    expect(screen.getByRole('tab', { name: 'Вузы' })).toHaveAttribute('aria-selected', 'true')

    await next()
    await step('Добавляй в трекер')
    const [olympiads, row] = shown()
    expect(olympiads).toBe('Нажимаем «Олимпиады»')
    expect(row).toMatch(/^Нажимаем «.+»$/)
    const name = row!.slice('Нажимаем «'.length, -1)
    expect(path()).toBe(`Каталог → Олимпиады → ${name}`)
    // Олимпиада выбрана такая, которую ещё можно добавить.
    expect(screen.getByRole('button', { name: 'Добавить в трекер' })).toBeInTheDocument()

    await next()
    await step('Календарь сроков')
    expect(shown()).toEqual(['Нажимаем «Закрыть»', 'Нажимаем «Трекер»', 'Нажимаем «Календарь»'])
    expect(path()).toBe('Трекер → Календарь')
    expect(document.querySelector('.calendar')).toBeInTheDocument()

    await next()
    await step('Отметь регистрацию')
    expect(shown()).toEqual(['Нажимаем «Список»'])
    expect(path()).toBe('Трекер → Список')

    await next()
    await step('Подбор под твою цель')
    expect(shown()).toEqual(['Нажимаем «Подбор»'])
    expect(path()).toBe('Подбор')

    await next()
    await step('ИИ-помощник')
    expect(shown()).toEqual(['Нажимаем «Спросить»'])
    expect(path()).toBe('Подбор → Спросить')
    expect(screen.getByRole('textbox', { name: /Спросите|Спроси|вопрос/i })).toBeInTheDocument()

    await next()
    await step('Позови родителей')
    expect(shown()).toEqual(['Нажимаем «Закрыть»', 'Нажимаем «Семья»'])
    expect(path()).toBe('Семья')
    expect(screen.getByText('9 из 9')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Начать' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Как пользоваться «Траекторией»' })).not.toBeInTheDocument())
    expect(tutorialSeen()).toBe(true)
    // После обзора — на главную, с неё и начинают.
    expect(tab('Главная')).toHaveAttribute('aria-current', 'page')
  })

  it('«Назад» возвращает на прошлый экран сразу, без показа нажатий', { timeout: 20_000 }, async () => {
    renderTour()
    await step('Это главная')
    await next()
    await step('Все олимпиады')
    await next()
    await step('И вузы')
    shown()

    await back()
    await step('Все олимпиады')
    expect(screen.getByRole('tab', { name: 'Олимпиады' })).toHaveAttribute('aria-selected', 'true')
    expect(path()).toBe('Каталог → Олимпиады')
    expect(shown()).toEqual([])
  })

  it('«Назад» из календаря снова открывает карточку олимпиады', { timeout: 20_000 }, async () => {
    renderTour()
    await step('Это главная')
    for (const name of ['Все олимпиады', 'И вузы', 'Добавляй в трекер', 'Календарь сроков']) {
      await next()
      await step(name)
    }

    await back()
    await step('Добавляй в трекер')
    expect(screen.getByRole('button', { name: 'Добавить в трекер' })).toBeInTheDocument()
    expect(path()).toMatch(/^Каталог → Олимпиады → .+/)
  })

  it('родителю — на «вы», а вместо добавления — «Предложить»', { timeout: 20_000 }, async () => {
    state.viewerId = 'mem-olga'
    renderTour()

    await step('Это главная')
    expect(screen.getByText(/Здесь цель Артёма/)).toBeInTheDocument()
    for (const name of ['Все олимпиады', 'И вузы', 'Предлагайте олимпиады']) {
      await next()
      await step(name)
    }
    expect(screen.getByText(/нажмите «Предложить Артёму»/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Предложить Артёму' })).toBeInTheDocument()
  })

  it('«Пропустить» закрывает сразу и запоминает', async () => {
    renderTour()
    await step('Это главная')
    await userEvent.click(screen.getByRole('button', { name: 'Пропустить' }))
    expect(tutorialSeen()).toBe(true)
    await waitFor(() => expect(screen.queryByRole('heading', { name: 'Это главная' })).not.toBeInTheDocument())
  })
})

describe('Tutorial отдельно от приложения', () => {
  const NO_WAIT = { point: 0, press: 0, wait: 0 }

  it('Esc закрывает и запоминает', async () => {
    const onDone = vi.fn()
    renderApp(
      <TutorialPace.Provider value={NO_WAIT}>
        <Tutorial onDone={onDone} />
      </TutorialPace.Provider>,
    )
    await screen.findByRole('heading', { name: 'Это главная' })
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(tutorialSeen()).toBe(true)
    await waitFor(() => expect(onDone).toHaveBeenCalledOnce())
  })

  it('без элемента на экране шаг не ломается: карточка встаёт по центру', async () => {
    renderApp(
      <TutorialPace.Provider value={NO_WAIT}>
        <Tutorial onDone={() => {}} />
      </TutorialPace.Provider>,
    )
    await screen.findByRole('heading', { name: 'Это главная' })
    expect(screen.getByRole('dialog')).toHaveAttribute('data-anchored', 'false')
  })

  it('подсвечивает настоящий элемент интерфейса, о котором шаг', async () => {
    renderApp(
      <TutorialPace.Provider value={NO_WAIT}>
        <section data-tour="goal">Цель</section>
        <Tutorial onDone={() => {}} />
      </TutorialPace.Provider>,
    )
    const goal = document.querySelector<HTMLElement>('[data-tour="goal"]')!
    vi.spyOn(goal, 'getBoundingClientRect').mockReturnValue(new DOMRect(16, 120, 358, 180))
    await screen.findByRole('heading', { name: 'Это главная' })
    fireEvent(window, new Event('resize'))

    expect(screen.getByRole('dialog')).toHaveAttribute('data-anchored', 'true')
  })
})

describe('когда показывать', () => {
  it('новичку, который открыл приложение сам или кнопкой бота «Открыть»', () => {
    expect(shouldShowTutorial(undefined)).toBe(true)
    expect(shouldShowTutorial('')).toBe(true)
    expect(shouldShowTutorial('home')).toBe(true)
  })

  it('не перебивает переход из бота в конкретный раздел', () => {
    expect(shouldShowTutorial('tracker')).toBe(false)
    expect(shouldShowTutorial('family')).toBe(false)
    expect(shouldShowTutorial('o_hse:inf')).toBe(false)
  })

  it('второй раз не показывается', () => {
    markTutorialSeen()
    expect(shouldShowTutorial(undefined)).toBe(false)
    expect(shouldShowTutorial('home')).toBe(false)
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
})
