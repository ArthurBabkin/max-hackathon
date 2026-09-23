/**
 * Стек нижних листов, живущий в адресе: `?sheet=oly:hse:inf,vuz:inno`.
 *
 * Хранить стек в локальном состоянии нельзя: кнопка «Назад» в MAX — системная,
 * и если она не связана с историей, первое нажатие закроет всё мини-приложение.
 * Каждый push — запись в истории, поэтому «Назад» снимает ровно один лист.
 *
 * Побочная польза: карточка открывается по прямой ссылке — это нужно и для
 * диплинков `startapp=` (ТЗ §11.2), и для кнопок «Карточка «…»» у помощника (F35).
 */

import { useCallback, useMemo } from 'react'
import { useSearchParams } from 'react-router-dom'

export type SheetKind = 'oly' | 'vuz' | 'ai'

export interface SheetEntry {
  kind: SheetKind
  id: string
}

const PARAM = 'sheet'
const KINDS: SheetKind[] = ['oly', 'vuz', 'ai']

/** Идентификаторы профилей содержат двоеточие (`hse:inf`), поэтому режем по первому. */
function parseEntry(raw: string): SheetEntry | null {
  const at = raw.indexOf(':')
  const kind = (at < 0 ? raw : raw.slice(0, at)) as SheetKind
  if (!KINDS.includes(kind)) return null
  return { kind, id: at < 0 ? '' : raw.slice(at + 1) }
}

const formatEntry = (entry: SheetEntry) => (entry.id ? `${entry.kind}:${entry.id}` : entry.kind)

export function parseSheetStack(value: string | null): SheetEntry[] {
  if (!value) return []
  return value.split(',').map(parseEntry).filter((e): e is SheetEntry => e !== null)
}

export interface SheetStack {
  stack: SheetEntry[]
  top: SheetEntry | null
  open: (entry: SheetEntry) => void
  /** Заменить верхний лист без новой записи в истории — смена чата у помощника. */
  replace: (entry: SheetEntry) => void
  /** Снять верхний лист. */
  back: () => void
  /** Закрыть все листы разом — крестик в шапке. */
  closeAll: () => void
}

export function useSheetStack(): SheetStack {
  const [params, setParams] = useSearchParams()
  const raw = params.get(PARAM)

  const stack = useMemo(() => parseSheetStack(raw), [raw])

  const write = useCallback(
    (next: SheetEntry[], replace: boolean) => {
      const updated = new URLSearchParams(params)
      if (next.length === 0) updated.delete(PARAM)
      else updated.set(PARAM, next.map(formatEntry).join(','))
      setParams(updated, { replace })
    },
    [params, setParams],
  )

  const open = useCallback(
    (entry: SheetEntry) => {
      // Повторное открытие того же листа не плодит записи в истории:
      // иначе «Назад» пришлось бы жать дважды подряд по одному и тому же.
      const current = stack.at(-1)
      if (current && current.kind === entry.kind && current.id === entry.id) return
      write([...stack, entry], false)
    },
    [stack, write],
  )

  const replace = useCallback((entry: SheetEntry) => write([...stack.slice(0, -1), entry], true), [stack, write])
  const back = useCallback(() => write(stack.slice(0, -1), false), [stack, write])
  const closeAll = useCallback(() => write([], false), [write])

  return { stack, top: stack.at(-1) ?? null, open, replace, back, closeAll }
}
