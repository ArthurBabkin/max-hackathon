import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
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
  it('девять шагов, между карточками — одно показанное нажатие', { timeout: 30_000 }, async () => {
    renderTour()

    await step('Это главная')
    expect(screen.getByRole('dialog', { name: 'Как пользоваться «Траекторией»' })).toBeInTheDocument()
    expect(path()).toBe('Главная')
    expect(screen.getByText('1 из 9')).toBeInTheDocument()
    shown()

    await next()
    await step('Все олимпиады')
    expect(shown()).toEqual(['Нажимаем «Каталог»'])
    expect(path()).toBe('Каталог → Олимпиады')
    expect(tab('Каталог')).toHaveAttribute('aria-current', 'page')

    // Олимпиада открывается прямо из списка, без возврата к нему потом.
    await next()
    await step('Карточка олимпиады')
    const [row] = shown()
    expect(row).toMatch(/^Нажимаем «.+»$/)
    const olympiad = row!.slice('Нажимаем «'.length, -1)
    expect(path()).toBe(`Каталог → Олимпиады → ${olympiad}`)
    // Олимпиада выбрана такая, которую ещё можно добавить.
    expect(screen.getByRole('button', { name: 'Добавить в трекер' })).toBeInTheDocument()

    // Карточку закрываем молча: показываем только переход в раздел.
    await next()
    await step('Все вузы')
    expect(shown()).toEqual(['Нажимаем «Вузы»'])
    expect(path()).toBe('Каталог → Вузы')
    expect(screen.queryByRole('button', { name: 'Добавить в трекер' })).not.toBeInTheDocument()

    await next()
    await step('Карточка вуза')
    const [uni] = shown()
    expect(uni).toMatch(/^Нажимаем «.+»$/)
    expect(path()).toBe(`Каталог → Вузы → ${uni!.slice('Нажимаем «'.length, -1)}`)
    expect(screen.getByRole('heading', { name: /Олимпиады с льготой/ })).toBeInTheDocument()

    await next()
    await step('Трекер')
    expect(shown()).toEqual(['Нажимаем «Трекер»'])
    expect(path()).toBe('Трекер → Список')

    // Календарь — на том же экране: одно нажатие, и видна кнопка выгрузки.
    await next()
    await step('Календарь')
    expect(shown()).toEqual(['Нажимаем «Календарь»'])
    expect(path()).toBe('Трекер → Календарь')
    expect(screen.getByRole('button', { name: 'Выгрузить в календарь' })).toBeInTheDocument()

    await next()
    await step('Подбор под твою цель')
    expect(shown()).toEqual(['Нажимаем «Подбор»'])
    expect(path()).toBe('Подбор')

    await next()
    await step('ИИ-помощник')
    expect(shown()).toEqual(['Нажимаем «Спросить»'])
    expect(path()).toBe('Подбор → Спросить')
    expect(screen.getByText('9 из 9')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Начать' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Как пользоваться «Траекторией»' })).not.toBeInTheDocument())
    expect(tutorialSeen()).toBe(true)
    // После обзора — на главную, с неё и начинают; лист помощника закрыт.
    expect(tab('Главная')).toHaveAttribute('aria-current', 'page')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('«Назад» возвращает на прошлый экран сразу, без показа нажатий', { timeout: 20_000 }, async () => {
    renderTour()
    await step('Это главная')
    for (const name of ['Все олимпиады', 'Карточка олимпиады', 'Все вузы']) {
      await next()
      await step(name)
    }
    shown()

    await back()
    await step('Карточка олимпиады')
    expect(screen.getByRole('button', { name: 'Добавить в трекер' })).toBeInTheDocument()
    expect(path()).toMatch(/^Каталог → Олимпиады → .+/)
    expect(shown()).toEqual([])
  })

  it('родителю — на «вы», а вместо добавления — «Предложить»', { timeout: 20_000 }, async () => {
    state.viewerId = 'mem-olga'
    renderTour()

    await step('Это главная')
    expect(screen.getByText(/^Цель Артёма, прогресс и ближайший срок/)).toBeInTheDocument()
    await next()
    await step('Все олимпиады')
    await next()
    await step('Карточка олимпиады')
    expect(screen.getByText(/Предложите Артёму/)).toBeInTheDocument()
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

  it('фильтр без списка олимпиад — ещё не цель: шаг ждёт список', async () => {
    // Главная и каталог, где список олимпиад ещё грузится.
    function Screens() {
      const [here, setHere] = useState<'home' | 'catalog'>('home')
      return (
        <>
          <button type="button" data-tour="home" aria-current={here === 'home' ? 'page' : undefined} onClick={() => setHere('home')}>
            Главная
          </button>
          <button type="button" data-tour="catalog" aria-current={here === 'catalog' ? 'page' : undefined} onClick={() => setHere('catalog')}>
            Каталог
          </button>
          {here === 'home' ? (
            <section data-tour="goal">Цель</section>
          ) : (
            <>
              <button type="button" role="tab" aria-selected="true" data-tour="catalog-olympiads">
                Олимпиады
              </button>
              <div data-tour="catalog-subjects">Предмет</div>
            </>
          )}
        </>
      )
    }
    renderApp(
      <TutorialPace.Provider value={{ point: 0, press: 0, wait: 3000 }}>
        <Screens />
        <Tutorial onDone={() => {}} />
      </TutorialPace.Provider>,
    )
    await screen.findByRole('heading', { name: 'Это главная' })
    await userEvent.click(screen.getByRole('button', { name: 'Дальше' }))

    await waitFor(() => expect(screen.getByRole('tab', { name: 'Олимпиады' })).toBeInTheDocument())
    await new Promise((resolve) => setTimeout(resolve, 200))
    expect(screen.queryByRole('heading', { name: 'Все олимпиады' })).not.toBeInTheDocument()

    const group = document.createElement('section')
    group.dataset.tour = 'olympiad-group'
    document.body.append(group)
    expect(await screen.findByRole('heading', { name: 'Все олимпиады' })).toBeInTheDocument()
    group.remove()
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

  it('окно над группой элементов не круглое, даже если в группе кнопка-«таблетка»', async () => {
    renderApp(
      <TutorialPace.Provider value={NO_WAIT}>
        <button type="button" data-tour="goal" style={{ borderTopLeftRadius: '999px' }}>
          Выгрузить
        </button>
        <section data-tour="next-step" style={{ borderTopLeftRadius: '16px' }}>
          Месяц
        </section>
        <Tutorial onDone={() => {}} />
      </TutorialPace.Provider>,
    )
    const pill = document.querySelector<HTMLElement>('[data-tour="goal"]')!
    const month = document.querySelector<HTMLElement>('[data-tour="next-step"]')!
    vi.spyOn(pill, 'getBoundingClientRect').mockReturnValue(new DOMRect(16, 120, 358, 44))
    vi.spyOn(month, 'getBoundingClientRect').mockReturnValue(new DOMRect(16, 180, 358, 300))
    await screen.findByRole('heading', { name: 'Это главная' })
    fireEvent(window, new Event('resize'))

    // Скругление — как у месяца (16 + отступ окна), а не половина высоты группы.
    const spot = document.querySelector<HTMLElement>('.tour-spot')!
    expect(parseFloat(spot.style.borderRadius)).toBeLessThan(30)
  })

  describe('высокая цель', () => {
    const height = window.innerHeight
    afterEach(() => {
      window.innerHeight = height
    })

    /** Главная с целью и следующим шагом заданного размера, карточка высотой 210. */
    async function renderHome(screenHeight: number, goal: DOMRect, nextStep: DOMRect) {
      window.innerHeight = screenHeight
      vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockImplementation(function (this: HTMLElement) {
        return this.classList.contains('tour-card') ? 210 : 0
      })
      renderApp(
        <TutorialPace.Provider value={NO_WAIT}>
          <section data-tour="goal">Цель</section>
          <section data-tour="next-step">Следующий шаг</section>
          <Tutorial onDone={() => {}} />
        </TutorialPace.Provider>,
      )
      const first = document.querySelector<HTMLElement>('[data-tour="goal"]')!
      first.scrollIntoView = vi.fn()
      vi.spyOn(first, 'getBoundingClientRect').mockReturnValue(goal)
      vi.spyOn(document.querySelector<HTMLElement>('[data-tour="next-step"]')!, 'getBoundingClientRect').mockReturnValue(nextStep)
      await screen.findByRole('heading', { name: 'Это главная' })
      fireEvent(window, new Event('resize'))
      return first.scrollIntoView as ReturnType<typeof vi.fn>
    }

    // iPhone SE: карточка закрыла бы почти всю цель — поднимаем её к верху ленты.
    it('на низком экране поднимает цель к верху, чтобы карточка её не закрыла', async () => {
      const scroll = await renderHome(568, new DOMRect(16, 123, 288, 180), new DOMRect(16, 311, 288, 140))
      expect(scroll).toHaveBeenCalledWith({ block: 'start' })
    })

    // На большом экране над карточкой видно больше половины списка — ленту не трогаем,
    // иначе уехали бы поиск и переключатель над ним.
    it('на большом экране высокую цель не прокручивает, если её и так видно', async () => {
      const scroll = await renderHome(844, new DOMRect(16, 179, 358, 400), new DOMRect(16, 590, 358, 252))
      expect(scroll).not.toHaveBeenCalled()
    })
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
