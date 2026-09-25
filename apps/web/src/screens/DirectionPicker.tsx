/**
 * Выбор направлений с поиском — экран D4 (F65). Основные направления — чипами
 * в профиле, здесь все: цель сверху, дальше по группам, у каждого — код.
 * Поиск — по части названия или по коду.
 */

import { useState } from 'react'
import { Button, Input } from '@maxhub/max-ui'
import type { DirectionOption } from '@contract'
import { Icon } from '@/ui/Icon'
import { Sheet } from '@/ui/Sheet'
import type { TextKey } from '@/voice/texts'
import { useVoice } from '@/voice/useVoice'

/** Группа направления — первая по этому порядку; без группы — «Другие». */
const GROUPS: { group: string; label: TextKey }[] = [
  { group: 'ИТ', label: 'directions.groupIt' },
  { group: 'Экономика', label: 'directions.groupEcon' },
  { group: 'Физика', label: 'directions.groupPhys' },
  { group: 'Биомед', label: 'directions.groupBio' },
]

const groupOf = (d: DirectionOption) => GROUPS.find((g) => d.groups.includes(g.group))?.group ?? ''

const normalize = (s: string) => s.toLowerCase().replaceAll('ё', 'е').trim()

export function matches(d: DirectionOption, query: string): boolean {
  const q = normalize(query)
  return !q || normalize(d.name).includes(q) || d.code.startsWith(q)
}

export function DirectionPicker({
  directions,
  selected,
  onToggle,
  onClose,
}: {
  directions: DirectionOption[]
  selected: string[]
  onToggle: (id: string) => void
  onClose: () => void
}) {
  const t = useVoice()
  const [query, setQuery] = useState('')
  // Цель — какой была при открытии: снятое направление не прыгает в группу.
  const [goal] = useState(() => selected)

  const row = (d: DirectionOption) => {
    const on = selected.includes(d.id)
    return (
      <button
        key={d.id}
        type="button"
        role="checkbox"
        aria-checked={on}
        className="uni-direction"
        onClick={() => onToggle(d.id)}
      >
        <span className={`checkbox${on ? ' checkbox-on' : ''}`}>
          {on ? <Icon name="check" size={12} strokeWidth={3} /> : null}
        </span>
        <span className="uni-direction-text uni-direction-inline">
          <b>{d.name}</b> <span>{d.code}</span>
        </span>
      </button>
    )
  }

  const found = directions.filter((d) => matches(d, query))
  const rest = directions.filter((d) => !goal.includes(d.id))
  const sections = query
    ? [{ key: 'found', title: null, items: found }]
    : [
        { key: 'goal', title: t('directions.goal'), items: directions.filter((d) => goal.includes(d.id)) },
        ...GROUPS.map((g) => ({ key: g.group, title: t(g.label), items: rest.filter((d) => groupOf(d) === g.group) })),
        { key: 'other', title: t('directions.groupOther'), items: rest.filter((d) => groupOf(d) === '') },
      ]

  return (
    <Sheet
      label={t('directions.pickerTitle')}
      canGoBack={false}
      onBack={onClose}
      onClose={onClose}
      header={
        <div className="sheet-title">
          <h2>{t('directions.pickerTitle')}</h2>
        </div>
      }
    >
      <Input
        value={query}
        placeholder={t('directions.search')}
        aria-label={t('directions.search')}
        iconBefore={<Icon name="search" size={16} />}
        onChange={(event) => setQuery(event.target.value)}
      />
      {query && found.length === 0 ? <p className="block-hint">{t('directions.empty')}</p> : null}
      {sections
        .filter((s) => s.items.length > 0)
        .map((s) => (
          <section key={s.key} className="block">
            {s.title ? <h3 className="block-head">{s.title}</h3> : null}
            {s.items.map(row)}
          </section>
        ))}
      <div className="sheet-actions">
        <Button stretched onClick={onClose}>
          {t('directions.done')}
        </Button>
      </div>
    </Sheet>
  )
}
