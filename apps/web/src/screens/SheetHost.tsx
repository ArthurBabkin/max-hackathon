/**
 * Отрисовка верхнего листа стопки.
 *
 * Показывается только верхний: листы лежат друг на друге, и держать в DOM
 * всю стопку значило бы держать и её запросы. Стек при этом сохраняется —
 * «Назад» возвращает к предыдущему листу с его состоянием.
 */

import type { SheetStack } from '@/ui/sheets'
import { AiSheet } from './AiSheet'
import { OlympiadSheet } from './OlympiadSheet'
import { UniversitySheet } from './UniversitySheet'

export function SheetHost({ sheets }: { sheets: SheetStack }) {
  const top = sheets.top
  if (!top) return null

  if (top.kind === 'ai') return <AiSheet chatId={top.id} sheets={sheets} />
  if (top.kind === 'vuz') return <UniversitySheet id={top.id} sheets={sheets} />
  return <OlympiadSheet id={top.id} sheets={sheets} />
}
