/** Каталог — экраны D1, D2 и D5. Функции F24, F25, F27, F66. */

import { useSearchParams } from 'react-router-dom'
import { useState } from 'react'
import { Button, Input } from '@maxhub/max-ui'
import type { OlympiadListItem, Profile } from '@contract'
import { useOlympiads, useProfile, useTracker, useUniversities } from '@/api/queries'
import { groupByLevel } from '@/lib/catalog'
import { useDebounced } from '@/lib/useDebounced'
import { Icon } from '@/ui/Icon'
import { CardSkeletons, Chip, StateBlock, Tile } from '@/ui/primitives'
import { useSheetStack } from '@/ui/sheets'
import type { TextKey } from '@/voice/texts'
import { useVoice } from '@/voice/useVoice'
import { plural } from '@/lib/deadline'
import { ErrorState } from '@/ui/ErrorState'

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
  { value: 'chem', label: 'Химия' },
  { value: 'bio', label: 'Биология' },
  { value: 'soc', label: 'Обществознание' },
]

/** Сколько строк группы видно сразу: дальше — «Показать ещё». */
const GROUP_PREVIEW = 5

const UNIVERSITY_CITIES = ['all', 'Казань', 'Иннополис', 'Москва', 'Санкт-Петербург', 'Долгопрудный']

function levelLabel(level: string | null, kind: string, outside: string): string {
  if (kind === 'vsosh') return 'ВсОШ'
  if (kind === 'other') return outside
  return level ? `${level} уровень` : 'уровень уточняется'
}

/** Мои вузы и сколько направлений в них учитываются: «ВШЭ, КФУ · 2 направления». */
function mineSummary(profile: Profile): string {
  const names = profile.universities.map((u) => u.nick).join(', ')
  const count = new Set(profile.universities.flatMap((u) => u.target_directions.map((d) => d.id))).size
  return count > 0 ? `${names} · ${count} ${plural(count, 'направление', 'направления', 'направлений')}` : names
}

/** Льгота в моих вузах в строке каталога: метка сильнейшей, дальше — вузы. */
function MyBenefitLine({ groups, allMine }: { groups: OlympiadListItem['my_benefits']; allMine: boolean }) {
  const t = useVoice()
  const [first, ...rest] = groups
  if (!first) return null
  const where = allMine && rest.length === 0 ? t('catalog.mineAll') : first.universities.join(', ')
  return (
    <span className="row-benefit">
      <span className={`benefit-value ${first.benefit === 'score100' ? 'benefit-score' : 'benefit-bvi'}`}>
        {first.benefit_label}
      </span>{' '}
      {where}
      {rest.map((g) => ` · ${g.benefit_label}: ${g.universities.join(', ')}`).join('')}
    </span>
  )
}

export function CatalogScreen() {
  const t = useVoice()
  const sheets = useSheetStack()

  // «Найти вузы» из «Подбора» открывает каталог сразу на вузах.
  const [params] = useSearchParams()
  const [segment, setSegment] = useState<Segment>(params.get('segment') === 'universities' ? 'universities' : 'olympiads')
  const [query, setQuery] = useState('')
  // Пока пользователь не выбрал сам — предмет ученика, как на макете D1:
  // семьдесят олимпиад разом никто не читает.
  const [pickedSubject, setSubject] = useState<string | null>(null)
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const profile = useProfile()
  const tracker = useTracker()
  const mySubject = profile.data?.subjects.map((s) => s.code).find((code) => SUBJECTS.some((s) => s.value === code))
  const subject = pickedSubject ?? mySubject ?? 'all'
  const tracked = new Set(tracker.data?.items.map((item) => item.olympiad_id))
  // Уже в трекере или ждёт ответа на предложение — добавить её нельзя.
  // Туториал открывает олимпиаду, у которой кнопка «Добавить» ещё есть.
  const taken = new Set([...tracked, ...(tracker.data?.proposals.map((p) => p.olympiad_id) ?? [])])
  const [city, setCity] = useState('all')
  // «Ведут в мои вузы и на мои направления» (F66); без вузов не включить.
  const [mineOn, setMine] = useState(false)
  const myUniversities = profile.data?.universities ?? []
  const mine = mineOn && myUniversities.length > 0

  const debouncedQuery = useDebounced(query)

  const olympiads = useOlympiads(debouncedQuery, subject, mine)
  const universities = useUniversities(debouncedQuery, city)
  const active = segment === 'olympiads' ? olympiads : universities

  const reset = () => {
    setQuery('')
    setSubject('all')
    setCity('all')
  }

  const switchSegment = (next: Segment) => {
    setSegment(next)
    setQuery('')
  }

  const content = () => {
    if (active.isPending) return <CardSkeletons count={4} />

    if (active.isError) {
      return (
        <ErrorState error={active.error} onRetry={() => void active.refetch()} />
      )
    }

    if (segment === 'olympiads') {
      const items = olympiads.data?.items ?? []
      if (items.length === 0) return mine && !query.trim() ? mineEmpty : empty
      return groupByLevel(items).map((group) => {
        const open = query.trim() !== '' || expanded.has(group.key)
        const visible = open ? group.items : group.items.slice(0, GROUP_PREVIEW)
        const hidden = group.items.length - visible.length
        return (
          <section key={group.key} className="catalog-group" data-tour="olympiad-group">
            <h3 className="catalog-group-head">
              {t(`catalog.group.${group.key}` as TextKey)}
              <span className="catalog-group-count">{group.items.length}</span>
            </h3>
            <div className="list">
              {visible.map((item) => (
                <button
                  key={item.olympiad_id}
                  type="button"
                  className="row"
                  data-tour="olympiad-row"
                  data-free={taken.has(item.olympiad_id) ? undefined : ''}
                  onClick={() => sheets.open({ kind: 'oly', id: item.primary_profile.olympiad_profile_id })}
                >
                  <Tile id={item.olympiad_id} name={item.name} shortName={item.short_name} color={item.color} />
                  <span className="row-main">
                    <span className="row-title">{item.name}</span>
                    <span className="row-subtitle">{item.organizer}</span>
                    {mine ? (
                      <MyBenefitLine
                        groups={item.my_benefits}
                        allMine={
                          myUniversities.length > 1 &&
                          item.my_benefits[0]?.universities.length === myUniversities.length
                        }
                      />
                    ) : null}
                    {tracked.has(item.olympiad_id) ? (
                      <span className="row-tracked">
                        <Icon name="check" size={12} />
                        {t('catalog.inTracker')}
                      </span>
                    ) : item.registration_closed ? (
                      <span className="row-closed">{t('catalog.registrationClosed')}</span>
                    ) : null}
                  </span>
                  <span
                    className={`level${item.kind === 'vsosh' ? ' level-vsosh' : item.kind === 'other' ? ' level-outside' : ''}`}
                  >
                    {levelLabel(item.primary_profile.level, item.kind, t('catalog.outsidePerechen'))}
                  </span>
                </button>
              ))}
            </div>
            {hidden > 0 ? (
              <button
                type="button"
                className="link show-more"
                onClick={() => setExpanded((prev) => new Set(prev).add(group.key))}
              >
                {t('catalog.showMore', { count: hidden })}
              </button>
            ) : null}
          </section>
        )
      })
    }

    const items = universities.data?.items ?? []
    if (items.length === 0) return empty
    return (
      <div className="list" data-tour="university-list">
        {items.map((item) => (
          <button
            key={item.id}
            type="button"
            className="row"
            data-tour="university-row"
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

  // Пусто с фильтром «мои»: по предмету — предложить все предметы, иначе —
  // проверить направления в вузах.
  const mineEmpty =
    subject !== 'all' ? (
      <StateBlock icon="search" title={t('catalog.mineEmptyTitle')} text={t('catalog.mineEmptySubject')}>
        <Button stretched onClick={() => setSubject('all')}>
          {t('catalog.mineAllSubjects')}
        </Button>
      </StateBlock>
    ) : (
      <StateBlock icon="search" title={t('catalog.mineEmptyTitle')} text={t('catalog.mineEmptyAll')}>
        <Button stretched onClick={() => setMine(false)}>
          {t('catalog.mineOff')}
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
          data-tour="catalog-olympiads"
          onClick={() => switchSegment('olympiads')}
        >
          {t('catalog.segmentOlympiads')}
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={segment === 'universities'}
          className={segment === 'universities' ? 'segment-on' : ''}
          data-tour="catalog-universities"
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
        <div className={`mine-switch${myUniversities.length === 0 ? ' mine-switch-off' : ''}`}>
          <button
            type="button"
            role="switch"
            aria-checked={mine}
            disabled={myUniversities.length === 0}
            onClick={() => setMine((on) => !on)}
          >
            <span className="mine-switch-text">
              <b>{t('catalog.mineTitle')}</b>
              {profile.data && myUniversities.length > 0 ? <span>{mineSummary(profile.data)}</span> : null}
            </span>
            <span className="switch" aria-hidden />
          </button>
          {profile.data && myUniversities.length === 0 ? (
            <button type="button" className="link mine-switch-link" onClick={() => switchSegment('universities')}>
              {t('catalog.mineNoUniversities')}
              <Icon name="chevron" size={14} />
            </button>
          ) : null}
        </div>
      ) : null}

      {segment === 'olympiads' ? (
        <div className="filter-row" data-tour="catalog-subjects">
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

      {segment === 'universities' ? (
        <div className="filter-row">
          <span className="filter-label">{t('catalog.filterCity')}</span>
          <div className="chips">
            {UNIVERSITY_CITIES.map((value) => (
              <Chip key={value} active={city === value} onClick={() => setCity(value)}>
                {value === 'all' ? 'Все' : value}
              </Chip>
            ))}
          </div>
        </div>
      ) : null}

      {content()}
    </div>
  )
}
