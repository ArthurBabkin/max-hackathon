import { expect, it } from 'vitest'
import { outliers, splitBenefitLabel } from './benefits'

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

// «100 баллов по физике или химии» целиком в плашке сжимала название вуза в
// строке и не влезала в столбец таблицы: в плашке — вид льготы, под ней — предмет.
it('делит подпись 100 баллов на плашку и предмет', () => {
  expect(splitBenefitLabel('score100', '100 баллов по физике или химии')).toEqual({
    main: '100 баллов',
    detail: 'по физике или химии',
  })
})

it('подпись без уточнения не делится', () => {
  expect(splitBenefitLabel('score100', '100 баллов')).toEqual({ main: '100 баллов', detail: null })
  expect(splitBenefitLabel('bvi_winners', 'БВИ победителям')).toEqual({ main: 'БВИ победителям', detail: null })
  expect(splitBenefitLabel('bvi', 'БВИ')).toEqual({ main: 'БВИ', detail: null })
  expect(splitBenefitLabel('extra_points', '+3 балла')).toEqual({ main: '+3 балла', detail: null })
  expect(splitBenefitLabel(null, 'не учитывает')).toEqual({ main: 'не учитывает', detail: null })
})
