/** Каталог — экраны D1 и D2. Функции F24, F25, F27. */

import { useState } from 'react'
import { Button, Input } from '@maxhub/max-ui'
import { useOlympiads, useUniversities } from '@/api/queries'
import { useDebounced } from '@/lib/useDebounced'
import { Icon } from '@/ui/Icon'
import { CardSkeletons, Chip, StateBlock, Tile } from '@/ui/primitives'
import { useSheetStack } from '@/ui/sheets'
import { useVoice } from '@/voice/useVoice'
import { plural } from '@/lib/deadline'

type Segment = 'olympiads' | 'universities'

/**
 * Значения фильтров. В демо-данных это фиксированные списки; когда появятся
 * справочники с сервера, сюда встанут они — разметка не изменится.
 */
const SUBJECTS: { value: string; label: string }[] = [
  { value: 'all', label: 'Все' },
  { value: 'inf', label: 'Информатика' },
  { value: 'math', label: 'Математика' },
  { value: 'phys', label: 'Физика' },
]

const OLYMPIAD_CITIES = ['all', 'Казань', 'Иннополис', 'Москва']
const UNIVERSITY_CITIES = ['all', 'Казань', 'Иннополис', 'Москва', 'Санкт-Петербург', 'Долгопрудный']

function levelLabel(level: string | null, kind: string, outside: string): string {
  if (kind === 'vsosh') return 'ВсОШ'
  if (kind === 'other') return outside
  return level ? `${level} уровень` : 'уровень уточняется'
}

export function CatalogScreen() {
  const t = useVoice()
  const sheets = useSheetStack()

  const [segment, setSegment] = useState<Segment>('olympiads')
  const [query, setQuery] = useState('')
  const [subject, setSubject] = useState('all')
  const [city, setCity] = useState('all')

  const debouncedQuery = useDebounced(query)

  const olympiads = useOlympiads(debouncedQuery, subject, city)
  const universities = useUniversities(debouncedQuery, city)
  const active = segment === 'olympiads' ? olympiads : universities

  const reset = () => {
    setQuery('')
    setSubject('all')
    setCity('all')
  }

  const switchSegment = (next: Segment) => {
    setSegment(next)
    // Города у олимпиад и вузов разные, общий фильтр после переключения
    // показал бы пустой список без всякой причины.
    setCity('all')
    setQuery('')
  }

  const cities = segment === 'olympiads' ? OLYMPIAD_CITIES : UNIVERSITY_CITIES

  const content = () => {
    if (active.isPending) return <CardSkeletons count={4} />

    if (active.isError) {
      return (
        <StateBlock icon="wifiOff" tone="error" title={t('state.errorTitle')} text={t('state.errorText')}>
          <Button stretched onClick={() => void active.refetch()}>
            {t('state.errorRetry')}
          </Button>
        </StateBlock>
      )
    }

    if (segment === 'olympiads') {
      const items = olympiads.data?.items ?? []
      if (items.length === 0) return empty
      return (
        <div className="list">
          {items.map((item) => (
            <button
              key={item.olympiad_id}
              type="button"
              className="row"
              onClick={() => sheets.open({ kind: 'oly', id: item.primary_profile.olympiad_profile_id })}
            >
              <Tile id={item.olympiad_id} name={item.name} shortName={item.short_name} color={item.color} />
              <span className="row-main">
                <span className="row-title">{item.name}</span>
                <span className="row-subtitle">{item.organizer}</span>
              </span>
              <span
                className={`level${item.kind === 'vsosh' ? ' level-vsosh' : item.kind === 'other' ? ' level-outside' : ''}`}
              >
                {levelLabel(item.primary_profile.level, item.kind, t('catalog.outsidePerechen'))}
              </span>
            </button>
          ))}
        </div>
      )
    }

    const items = universities.data?.items ?? []
    if (items.length === 0) return empty
    return (
      <div className="list">
        {items.map((item) => (
          <button
            key={item.id}
            type="button"
            className="row"
            onClick={() => sheets.open({ kind: 'vuz', id: item.id })}
          >
            <Tile id={item.id} name={item.name} shortName={item.short_name} color={item.color} />
            <span className="row-main">
              <span className="row-title">{item.name}</span>
              <span className="row-subtitle">
                {item.city}, {item.benefit_olympiads_count}{' '}
                {plural(item.benefit_olympiads_count, 'олимпиада', 'олимпиады', 'олимпиад')} с льготой
              </span>
            </span>
            {item.is_mine ? <span className="mine-badge">{t('catalog.mineBadge')}</span> : null}
            <Icon name="chevron" size={16} />
          </button>
        ))}
      </div>
    )
  }

  const empty = (
    <StateBlock icon="search" title={t('catalog.emptyTitle')} text={t('catalog.emptyText')}>
      <Button stretched onClick={reset}>
        {t('catalog.emptyReset')}
      </Button>
    </StateBlock>
  )

  return (
    <div className="screen">
      <div className="segment" role="tablist">
        <button
          type="button"
          role="tab"
          aria-selected={segment === 'olympiads'}
          className={segment === 'olympiads' ? 'segment-on' : ''}
          onClick={() => switchSegment('olympiads')}
        >
          {t('catalog.segmentOlympiads')}
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={segment === 'universities'}
          className={segment === 'universities' ? 'segment-on' : ''}
          onClick={() => switchSegment('universities')}
        >
          {t('catalog.segmentUniversities')}
        </button>
      </div>

      <Input
        type="search"
        value={query}
        placeholder={segment === 'olympiads' ? t('catalog.searchOlympiads') : t('catalog.searchUniversities')}
        aria-label={segment === 'olympiads' ? t('catalog.searchOlympiads') : t('catalog.searchUniversities')}
        iconBefore={<Icon name="search" size={16} />}
        onChange={(event) => setQuery(event.target.value)}
      />

      {segment === 'olympiads' ? (
        <div className="filter-row">
          <span className="filter-label">{t('catalog.filterSubject')}</span>
          <div className="chips">
            {SUBJECTS.map((item) => (
              <Chip key={item.value} active={subject === item.value} onClick={() => setSubject(item.value)}>
                {item.label}
              </Chip>
            ))}
          </div>
        </div>
      ) : null}

      <div className="filter-row">
        <span className="filter-label">
          {segment === 'olympiads' ? t('catalog.filterFinal') : t('catalog.filterCity')}
        </span>
        <div className="chips">
          {cities.map((value) => (
            <Chip key={value} active={city === value} onClick={() => setCity(value)}>
              {value === 'all' ? 'Все' : value}
            </Chip>
          ))}
        </div>
      </div>

      {content()}
    </div>
  )
}
