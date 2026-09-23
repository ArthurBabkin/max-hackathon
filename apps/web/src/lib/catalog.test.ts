import { describe, expect, it } from 'vitest'
import { byOlympiad, groupByLevel } from './catalog'

const item = (olympiad_id: string, kind: 'vsosh' | 'perechen' | 'other', level: 'I' | 'II' | 'III' | null) => ({
  olympiad_id,
  kind,
  primary_profile: { level },
})

describe('groupByLevel', () => {
  it('ВсОШ → I → II → III → вне перечня, порядок внутри группы сохраняется', () => {
    const groups = groupByLevel([
      item('b', 'perechen', 'II'),
      item('v', 'vsosh', null),
      item('o', 'other', null),
      item('a', 'perechen', 'I'),
      item('c', 'perechen', 'II'),
    ])
    expect(groups.map((g) => [g.key, g.items.map((i) => i.olympiad_id)])).toEqual([
      ['vsosh', ['v']],
      ['I', ['a']],
      ['II', ['b', 'c']],
      ['other', ['o']],
    ])
  })

  it('профиль перечня без уровня — в конце, отдельной группой', () => {
    expect(groupByLevel([item('x', 'perechen', null), item('a', 'perechen', 'III')]).map((g) => g.key)).toEqual([
      'III',
      'unknown',
    ])
  })
})

const row = (olympiad_id: string, subject_name: string, benefit: 'bvi' | 'score100' | 'bvi_winners' | 'extra_points') => ({
  olympiad_profile_id: `${olympiad_id}-${subject_name}`,
  olympiad_id,
  name: olympiad_id.toUpperCase(),
  subject_name,
  benefit,
})

describe('byOlympiad', () => {
  it('строка на олимпиаду: предметы через запятую, лучшая льгота, первый профиль открывается', () => {
    const out = byOlympiad([
      row('hse', 'Информатика', 'score100'),
      row('msu', 'Физика', 'bvi'),
      row('hse', 'Математика', 'bvi'),
    ])
    expect(out).toEqual([
      expect.objectContaining({
        olympiad_id: 'hse',
        subjects: 'Информатика, Математика',
        benefit: 'bvi',
        open_profile_id: 'hse-Информатика',
      }),
      expect.objectContaining({ olympiad_id: 'msu', subjects: 'Физика', benefit: 'bvi' }),
    ])
  })

  it('БВИ победителям уступает БВИ, но сильнее 100 баллов', () => {
    expect(byOlympiad([row('a', 'X', 'score100'), row('a', 'Y', 'bvi_winners')])[0]!.benefit).toBe('bvi_winners')
  })
})
