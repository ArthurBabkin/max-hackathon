import { expect, it } from 'vitest'
import { outliers } from './benefits'

// Жёлтым в таблице льгот — только вузы не как большинство (F19).
it('отмечает значения не как у большинства', () => {
  expect(outliers(['БВИ', 'БВИ', '100 баллов', 'БВИ'])).toEqual(new Set([2]))
})

it('большинства нет — не отмечает ничего', () => {
  expect(outliers(['+3 балла', '+2 балла'])).toEqual(new Set())
  expect(outliers(['БВИ', 'БВИ', 'нет', 'нет'])).toEqual(new Set())
  expect(outliers([])).toEqual(new Set())
})

it('все одинаковые — отмечать нечего', () => {
  expect(outliers(['от 75', 'от 75', 'от 75'])).toEqual(new Set())
})
