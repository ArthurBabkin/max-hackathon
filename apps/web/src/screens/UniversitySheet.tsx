/** Карточка вуза — экран D3. Функции F25, F26. */

import { useRef, useState } from 'react'
import { Button } from '@maxhub/max-ui'
import { BENEFIT_LABELS, type UniversityDetail } from '@contract'
import { useProfile, useSession, useSetUniversities, useSetUniversityDirections, useUniversity } from '@/api/queries'
import { getWebApp } from '@/bridge'
import { byOlympiad } from '@/lib/catalog'
import { formatShortDate } from '@/lib/deadline'
import { UniversityOlympiadRow } from '@/ui/BenefitRow'
import { Icon } from '@/ui/Icon'
import { Sheet } from '@/ui/Sheet'
import { Chip, CardSkeletons, SourceTag, Tile } from '@/ui/primitives'
import type { SheetStack } from '@/ui/sheets'
import { countText, useVoice } from '@/voice/useVoice'
import { ErrorState } from '@/ui/ErrorState'

/** Сколько олимпиад вуза видно сразу: у Иннополиса их больше шестидесяти. */
const PREVIEW = 8

/** Сколько направлений видно сразу: у КФУ их сорок. */
const DIRECTIONS_PREVIEW = 5

type Olympiads = 'mine' | 'all'

/**
 * Направления вуза (D3, F65): галочка — «моё», выбор сохраняется сразу. Мои
 * сверху, за ними из цели, дальше — где больше олимпиад с льготой. Открыта
 * из каталога с фильтром (F67) — направление фильтра первым.
 */
function Directions({
  university,
  canEdit,
  focus,
}: {
  university: UniversityDetail
  canEdit: boolean
  focus?: string
}) {
  const t = useVoice()
  const [showAll, setShowAll] = useState(false)
  const save = useSetUniversityDirections(university.id)
  // Порядок запоминается при открытии: отмеченная строка не уезжает из-под
  // пальца, хотя сервер ставит выбранное первым.
  const order = useRef<string[]>(
    focus && university.offered_directions.some((d) => d.id === focus) ? [focus] : [],
  )
  for (const d of university.offered_directions) if (!order.current.includes(d.id)) order.current.push(d.id)
  const all = [...university.offered_directions].sort(
    (a, b) => order.current.indexOf(a.id) - order.current.indexOf(b.id),
  )
  if (all.length === 0) return null
  const mine = all.filter((d) => d.is_mine)
  const shown = showAll ? all : all.slice(0, DIRECTIONS_PREVIEW)
  const toggle = (id: string) => {
    const ids = mine.map((d) => d.id)
    save.mutate(ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id])
  }

  return (
    <section className="block">
      <h3 className="block-head">
        {t('university.directions')}
        {mine.length > 0 ? (
          <span className="block-head-note">{t('university.directionsMine', { count: mine.length, total: all.length })}</span>
        ) : null}
      </h3>
      {shown.map((d) => (
        <button
          key={d.id}
          type="button"
          role="checkbox"
          aria-checked={d.is_mine}
          className="uni-direction"
          disabled={!canEdit || save.isPending}
          onClick={() => toggle(d.id)}
        >
          <span className={`checkbox${d.is_mine ? ' checkbox-on' : ''}`}>
            {d.is_mine ? <Icon name="check" size={12} strokeWidth={3} /> : null}
          </span>
          <span className="uni-direction-text">
            <b>{d.name}</b>
            <span>
              {d.code}
              {d.programs > 0 ? ` · ${countText(t, 'count.programs', d.programs)}` : ''}
              {university.target_basis === 'goal' && d.is_goal ? (
                <em className="uni-direction-goal"> · {t('university.directionGoal')}</em>
              ) : null}
            </span>
          </span>
          {d.status === 'to_check' ? (
            <span className="uni-direction-count uni-direction-check">{t('university.directionToCheck')}</span>
          ) : d.benefit_olympiads_count > 0 ? (
            <span className="uni-direction-count">
              {countText(t, 'count.olympiads', d.benefit_olympiads_count)}
            </span>
          ) : null}
        </button>
      ))}
      {all.length > shown.length ? (
        <button type="button" className="link show-more" onClick={() => setShowAll(true)}>
          {t('university.allDirections', { count: all.length })}
        </button>
      ) : null}
      {university.target_basis === 'goal' ? (
        <p className="block-hint">{t('university.directionsHintGoal')}</p>
      ) : university.target_basis === 'university' ? (
        <p className="block-hint">{t('university.directionsHintNone')}</p>
      ) : null}
    </section>
  )
}

export function UniversitySheet({ id, focus, sheets }: { id: string; focus?: string; sheets: SheetStack }) {
  const t = useVoice()
  const [showAll, setShowAll] = useState(false)
  const [mode, setMode] = useState<Olympiads | null>(null)
  const query = useUniversity(id)
  const { data: profile } = useProfile()
  const { data: session } = useSession()
  const setUniversities = useSetUniversities()

  const canGoBack = sheets.stack.length > 1

  if (query.isPending) {
    return (
      <Sheet label={t('sheet.university')} canGoBack={canGoBack} onBack={sheets.back} onClose={sheets.closeAll}>
        <CardSkeletons count={2} />
      </Sheet>
    )
  }

  if (query.isError || !query.data) {
    return (
      <Sheet label={t('sheet.university')} canGoBack={canGoBack} onBack={sheets.back} onClose={sheets.closeAll}>
        <ErrorState error={query.error} onRetry={() => void query.refetch()} />
      </Sheet>
    )
  }

  const university = query.data
  const current = profile?.universities.map((u) => u.id) ?? []
  const isMine = university.is_mine
  const chosen = university.offered_directions.filter((d) => d.is_mine).length
  const allOlympiads = byOlympiad(university.olympiads)
  // «На мои направления» — льгота на них; фильтр есть, только если льготы
  // считаются по направлениям и они проверены.
  const myOlympiads = byOlympiad(
    university.olympiads.flatMap((o) =>
      o.my_benefit ? [{ ...o, benefit: o.my_benefit, benefit_label: o.my_benefit_label ?? BENEFIT_LABELS[o.my_benefit] }] : [],
    ),
  )
  const byDirections = university.target_basis !== 'university' && !university.target_unverified
  const shownMode: Olympiads = mode ?? (byDirections && myOlympiads.length > 0 ? 'mine' : 'all')
  const olympiads = byDirections && shownMode === 'mine' ? myOlympiads : allOlympiads
  const shown = showAll ? olympiads : olympiads.slice(0, PREVIEW)
  const coverage = (rows: UniversityDetail['olympiads']) => {
    const best = rows.reduce((a, r) => (r.directions_count > a.directions_count ? r : a), rows[0]!)
    return best.directions_total > 0 && best.directions_count > 0
      ? ` · ${t('university.onDirections', { count: best.directions_count, total: countText(t, 'count.directionsGen', best.directions_total) })}`
      : ''
  }

  const toggle = () => {
    // Вузы выбирать не обязательно (F9): последний тоже можно убрать.
    const next = isMine ? current.filter((x) => x !== university.id) : [...current, university.id]
    setUniversities.mutate(next)
  }

  return (
    <Sheet
      label={university.name}
      canGoBack={canGoBack}
      onBack={sheets.back}
      onClose={sheets.closeAll}
      header={
        <>
          <Tile
            id={university.id}
            name={university.name}
            shortName={university.short_name}
            color={university.color}
            size="lg"
            filled
          />
          <div className="sheet-title">
            <h2>{university.name}</h2>
            <p className="sheet-title-city">
              <Icon name="pin" size={12} />
              {university.city}
            </p>
          </div>
        </>
      }
    >
      {/* Что это за вуз, его сайт и правила приёма — первым делом. */}
      <section className="block">
        <h3 className="block-head">{t('university.aboutTitle')}</h3>
        {university.description ? <p className="block-text">{university.description}</p> : null}
        {university.site_url ? (
          <button type="button" className="about-link" onClick={() => getWebApp().openLink(university.site_url!)}>
            <Icon name="external" size={15} />
            {t('university.site')}
          </button>
        ) : null}
        {university.rules_url ? (
          <button type="button" className="about-link" onClick={() => getWebApp().openLink(university.rules_url!)}>
            <Icon name="doc" size={15} />
            {t('university.rules')}
            <span className="rules-date">
              {university.rules_verified_at
                ? t('university.verifiedAt', {
                    date: formatShortDate(university.rules_verified_at) ?? university.rules_verified_at,
                  })
                : t('olympiad.benefitsUnknown')}
            </span>
          </button>
        ) : null}
      </section>

      <Directions university={university} canEdit={session?.permissions.edit_profile !== false} focus={focus} />

      {/* Олимпиады, дающие льготу в этом вузе, с переходом в карточку — F26. */}
      <section className="block" data-tour="university-olympiads">
        <h3 className="block-head">
          {t('university.olympiadsTitle')}
          <SourceTag kind="fact" />
        </h3>
        {byDirections ? (
          <div className="chips chips-inline">
            <Chip active={shownMode === 'mine'} onClick={() => setMode('mine')}>
              {t('university.olympiadsMine', { count: myOlympiads.length })}
            </Chip>
            <Chip active={shownMode === 'all'} onClick={() => setMode('all')}>
              {t('university.olympiadsAll', { count: allOlympiads.length })}
            </Chip>
          </div>
        ) : null}
        {university.target_unverified ? <p className="block-hint">{t('university.olympiadsUnverified')}</p> : null}
        {byDirections && shownMode === 'mine' && myOlympiads.length === 0 ? (
          <p className="block-hint">{t('university.olympiadsMineEmpty')}</p>
        ) : null}
        {/* Строка на олимпиаду, а не на профиль (D3): предметы через запятую,
            льгота — лучшая из профилей; открывается профиль ученика. */}
        {shown.map((row) => (
          <UniversityOlympiadRow
            key={row.olympiad_id}
            id={row.open_profile_id}
            olympiadId={row.olympiad_id}
            name={row.first.name}
            subtitle={shownMode === 'all' ? row.subjects + coverage(row.rows) : row.subjects}
            label={row.label}
            benefit={row.benefit}
            shortName={row.first.short_name}
            color={row.first.color}
            onOpen={(profileId) => sheets.open({ kind: 'oly', id: profileId })}
          />
        ))}
        {olympiads.length > shown.length ? (
          <button type="button" className="link show-more" onClick={() => setShowAll(true)}>
            {t('university.showAll', { count: olympiads.length })}
          </button>
        ) : null}
      </section>

      {university.ege_note ? (
        <section className="block">
          <h3 className="block-head">{t('university.egeTitle')}</h3>
          <p className="block-text">{t('university.egeText', { note: university.ege_note })}</p>
        </section>
      ) : null}

      <div className="sheet-actions">
        <Button
          stretched
          variant={isMine ? 'secondary' : 'primary'}
          loading={setUniversities.isPending}
          iconBefore={<Icon name={isMine ? 'check' : 'plus'} size={16} />}
          onClick={toggle}
        >
          {!isMine
            ? t('university.addToMine')
            : chosen > 0
              ? t('university.inMineWith', {
                  count: countText(t, 'count.directions', chosen),
                })
              : t('university.inMine')}
        </Button>
      </div>

      <p className="lock">
        <Icon name="users" size={12} />
        {t('university.sharedNote')}
      </p>
    </Sheet>
  )
}
