import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, useNavigate } from 'react-router-dom'
import { useSheetStack } from './sheets'

/**
 * Стопка листов — демонстрационный путь ТЗ §14: карточка олимпиады →
 * карточка вуза → «Назад». Стек живёт в адресе, поэтому проверяется вместе
 * с роутером: именно связка с историей и делает системную кнопку «Назад»
 * в MAX предсказуемой.
 */
function Harness() {
  const sheets = useSheetStack()
  const navigate = useNavigate()
  return (
    <div>
      <p data-testid="stack">{sheets.stack.map((s) => `${s.kind}:${s.id}`).join(' > ') || 'пусто'}</p>
      <button type="button" onClick={() => sheets.open({ kind: 'oly', id: 'hse:inf' })}>
        Открыть олимпиаду
      </button>
      <button type="button" onClick={() => sheets.open({ kind: 'vuz', id: 'inno' })}>
        Открыть вуз
      </button>
      <button type="button" onClick={() => sheets.open({ kind: 'ai', id: '' })}>
        Открыть помощника
      </button>
      <button type="button" onClick={() => sheets.replace({ kind: 'ai', id: 'c-2' })}>
        Сменить чат
      </button>
      <button type="button" onClick={sheets.back}>
        Назад
      </button>
      <button type="button" onClick={() => void navigate(-1)}>
        Системная «Назад»
      </button>
      <button type="button" onClick={sheets.closeAll}>
        Закрыть
      </button>
    </div>
  )
}

const stack = () => screen.getByTestId('stack').textContent

describe('стек нижних листов', () => {
  const setup = () =>
    render(
      <MemoryRouter initialEntries={['/match']}>
        <Harness />
      </MemoryRouter>,
    )

  it('олимпиада → вуз → «Назад» возвращает к олимпиаде', async () => {
    setup()
    expect(stack()).toBe('пусто')

    await userEvent.click(screen.getByRole('button', { name: 'Открыть олимпиаду' }))
    expect(stack()).toBe('oly:hse:inf')

    await userEvent.click(screen.getByRole('button', { name: 'Открыть вуз' }))
    expect(stack()).toBe('oly:hse:inf > vuz:inno')

    await userEvent.click(screen.getByRole('button', { name: 'Назад' }))
    expect(stack()).toBe('oly:hse:inf')
  })

  it('«Закрыть» убирает всю стопку, а не один лист', async () => {
    setup()
    await userEvent.click(screen.getByRole('button', { name: 'Открыть олимпиаду' }))
    await userEvent.click(screen.getByRole('button', { name: 'Открыть вуз' }))

    await userEvent.click(screen.getByRole('button', { name: 'Закрыть' }))
    expect(stack()).toBe('пусто')
  })

  it('повторное открытие того же листа не плодит записи в истории', async () => {
    setup()
    await userEvent.click(screen.getByRole('button', { name: 'Открыть олимпиаду' }))
    await userEvent.click(screen.getByRole('button', { name: 'Открыть олимпиаду' }))

    // Иначе «Назад» пришлось бы жать дважды по одному и тому же листу.
    expect(stack()).toBe('oly:hse:inf')
  })

  // Смена чата внутри помощника — не новый лист: системная «Назад» после неё
  // должна закрыть помощника, а не вернуть прошлый чат.
  it('замена верхнего листа не добавляет запись в историю', async () => {
    setup()
    await userEvent.click(screen.getByRole('button', { name: 'Открыть олимпиаду' }))
    await userEvent.click(screen.getByRole('button', { name: 'Открыть помощника' }))

    await userEvent.click(screen.getByRole('button', { name: 'Сменить чат' }))
    expect(stack()).toBe('oly:hse:inf > ai:c-2')

    await userEvent.click(screen.getByRole('button', { name: 'Системная «Назад»' }))
    expect(stack()).toBe('oly:hse:inf')
  })

  it('стек восстанавливается из адреса — карточка открывается по ссылке', () => {
    render(
      <MemoryRouter initialEntries={['/match?sheet=oly:vsosh-inf:inf,vuz:kfu']}>
        <Harness />
      </MemoryRouter>,
    )
    expect(stack()).toBe('oly:vsosh-inf:inf > vuz:kfu')
  })
})
