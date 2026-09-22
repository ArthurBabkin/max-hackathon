/** Заглушка стека листов — карточки появятся в следующем блоке. */
import type { SheetStack } from '@/ui/sheets'

export function SheetHost({ sheets }: { sheets: SheetStack }) {
  return sheets.top ? null : null
}
