import { screen, within } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import { keys } from '@/api/queries'
import { universityDetail } from '@/api/mocks/build'
import { UNIVERSITIES } from '@/api/mocks/fixtures'
import { renderApp } from '@/test/render'
import type { SheetStack } from '@/ui/sheets'
import { SheetHost } from './SheetHost'

// Карточка вуза из каталога с фильтром по направлению (F67): направление —
// в адресе листа, `vuz:hse:dir-is`.
it('лист вуза с направлением открывает карточку на нём', () => {
  const top = { kind: 'vuz' as const, id: 'hse:dir-is' }
  const sheets = { stack: [top], top, open: vi.fn(), back: vi.fn(), closeAll: vi.fn(), replace: vi.fn() } as SheetStack
  renderApp(<SheetHost sheets={sheets} />, {
    seed: (c) => c.setQueryData(keys.university('hse'), universityDetail(UNIVERSITIES.find((u) => u.id === 'hse')!)),
  })

  const block = within(screen.getByRole('heading', { name: /^Направления/ }).closest('section')!)
  expect(block.getAllByRole('checkbox')[0]).toHaveAccessibleName(/Информационная безопасность/)
})
