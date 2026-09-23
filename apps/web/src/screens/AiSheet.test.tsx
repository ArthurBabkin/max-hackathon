import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { keys } from '@/api/queries'
import { renderApp } from '@/test/render'
import type { SheetStack } from '@/ui/sheets'

// Ответ помощника идёт секунды — держим его «в пути», моки отвечали бы сами.
vi.hoisted(() => vi.stubEnv('VITE_USE_MOCKS', 'off'))
const { AiSheet } = await import('./AiSheet')

it('показывает вопрос сразу, не дожидаясь ответа', async () => {
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})))
  Element.prototype.scrollTo = () => {} // в jsdom прокрутки нет
  const sheets = { stack: [], open: vi.fn(), back: vi.fn(), closeAll: vi.fn() } as unknown as SheetStack
  renderApp(<AiSheet sheets={sheets} />, { seed: (c) => c.setQueryData(keys.ai, { items: [] }) })

  const question = 'Чем БВИ отличается от 100 баллов?'
  await userEvent.click(screen.getByRole('button', { name: question }))

  expect(screen.getByLabelText('Помощник печатает')).toBeInTheDocument()
  expect(screen.getByText(question, { selector: '.ai-message-user p' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: question })).not.toBeInTheDocument()
})
