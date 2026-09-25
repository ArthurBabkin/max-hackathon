import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { startRoute, useStartRoute } from './startParam'

const bridge = vi.hoisted(() => ({ startParam: undefined as string | undefined }))
vi.mock('@/bridge', () => ({
  getWebApp: () => ({ initDataUnsafe: { start_param: bridge.startParam } }),
}))

describe('startRoute', () => {
  it('вкладки из кнопок бота', () => {
    expect(startRoute('tracker')).toEqual({ path: '/tracker' })
    expect(startRoute('family')).toEqual({ path: '/family' })
    expect(startRoute('match')).toEqual({ path: '/match' })
    expect(startRoute('faq')).toEqual({ path: '/faq' })
    // «Изменить» под профилем в итоге онбординга (SPEC 9).
    expect(startRoute('profile')).toEqual({ path: '/profile' })
  })

  it('карточка олимпиады по профилю — лист поверх главной', () => {
    expect(startRoute('o_p669-8-informatika')).toEqual({ path: '/', sheet: 'oly:p669-8-informatika' })
  })

  it('главная и всё незнакомое — без перехода', () => {
    for (const p of ['home', '', null, undefined, 'o_', 'o_../x', 'tracker?x=1', 'a'.repeat(513)]) {
      expect(startRoute(p)).toBeNull()
    }
  })
})

function Probe({ ready }: { ready: boolean }) {
  useStartRoute(ready)
  const location = useLocation()
  return <output>{location.pathname + location.search}</output>
}

function mount(startParam: string | undefined, ready = true) {
  bridge.startParam = startParam
  return render(
    <MemoryRouter initialEntries={['/']}>
      <Probe ready={ready} />
    </MemoryRouter>,
  )
}

describe('useStartRoute', () => {
  it('открывает раздел из кнопки бота', () => {
    mount('tracker')
    expect(screen.getByRole('status').textContent).toBe('/tracker')
  })

  it('карточку олимпиады — листом', () => {
    mount('o_p669-8-informatika')
    expect(screen.getByRole('status').textContent).toBe('/?sheet=oly:p669-8-informatika')
  })

  it('ждёт сессию и не трогает главную без параметра', () => {
    mount('family', false)
    expect(screen.getByRole('status').textContent).toBe('/')
    mount(undefined).unmount()
  })
})
