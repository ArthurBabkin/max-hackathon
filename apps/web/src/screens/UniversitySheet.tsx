/** Карточка вуза — экран D3. Функции F25, F26. */

import { useState } from 'react'
import { Button } from '@maxhub/max-ui'
import { BENEFIT_LABELS, type Program } from '@contract'
import { useProfile, useSetPrograms, useSetUniversities, useUniversity } from '@/api/queries'
import { getWebApp } from '@/bridge'
import { byOlympiad } from '@/lib/catalog'
import { formatShortDate } from '@/lib/deadline'
import { UniversityOlympiadRow } from '@/ui/BenefitRow'
import { Icon } from '@/ui/Icon'
import { Sheet } from '@/ui/Sheet'
import { CardSkeletons, SourceTag, Tile } from '@/ui/primitives'
import type { SheetStack } from '@/ui/sheets'
import { useVoice } from '@/voice/useVoice'
import { ErrorState } from '@/ui/ErrorState'

/** Сколько олимпиад вуза видно сразу: у Иннополиса их больше шестидесяти. */
const PREVIEW = 8

/** Сколько направлений видно сразу: у КФУ их больше семидесяти. */
const PROGRAMS_PREVIEW = 5

export function UniversitySheet({ id, sheets }: { id: string; sheets: SheetStack }) {
  const t = useVoice()
  const [showAll, setShowAll] = useState(false)
  const [showAllPrograms, setShowAllPrograms] = useState(false)
  // Выбранное направление: олимпиады в карточке — с льготой именно на него.
  const [programId, setProgramId] = useState<string | null>(null)
  const query = useUniversity(id, programId)
  const { data: profile } = useProfile()
  const setUniversities = useSetUniversities()
  const setPrograms = useSetPrograms()

  const canGoBack = sheets.stack.length > 1

  if (query.isPending) {
    return (
      <Sheet label="Карточка вуза" canGoBack={canGoBack} onBack={sheets.back} onClose={sheets.closeAll}>
        <CardSkeletons count={2} />
      </Sheet>
    )
  }

  if (query.isError || !query.data) {
    return (
      <Sheet label="Карточка вуза" canGoBack={canGoBack} onBack={sheets.back} onClose={sheets.closeAll}>
        <ErrorState error={query.error} onRetry={() => void query.refetch()} />
      </Sheet>
    )
  }

  const university = query.data
  const current = profile?.universities.map((u) => u.id) ?? []
  const saved = profile?.programs.map((p) => p.id) ?? []
  const isMine = university.is_mine
  const olympiads = byOlympiad(university.olympiads)
  const shown = showAll ? olympiads : olympiads.slice(0, PREVIEW)
  const programs = university.programs
  const shownPrograms = showAllPrograms ? programs : programs.slice(0, PROGRAMS_PREVIEW)
  const chosen = programs.find((p) => p.id === programId) ?? null

  const toggle = () => {
    // Вузы выбирать не обязательно (F9): последний тоже можно убрать.
    const next = isMine ? current.filter((x) => x !== university.id) : [...current, university.id]
    setUniversities.mutate(next)
  }

  // Вуз сохранённого направления сервер добавит к вузам сам.
  const toggleProgram = (program: Program) => {
    const next = program.is_mine ? saved.filter((x) => x !== program.id) : [...saved, program.id]
    setPrograms.mutate(next)
  }

  const choose = (next: string | null) => {
    setProgramId(next)
    setShowAll(false)
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

      {programs.length > 0 ? (
        // Направление — это то, куда поступают: льготы по олимпиадам у
        // разных направлений одного вуза разные.
        <section className="block" data-tour="university-programs">
          <h3 className="block-head">{t('university.programsTitle')}</h3>
          <p className="block-text">{t('university.programsHint')}</p>
          <div className="programs">
            {shownPrograms.map((program) => (
              <div key={program.id} className={`program${program.id === programId ? ' program-on' : ''}`}>
                <button
                  type="button"
                  className="program-main"
                  aria-pressed={program.id === programId}
                  onClick={() => choose(program.id === programId ? null : program.id)}
                >
                  <span className="program-name">{program.name}</span>
                  <span className="program-meta">{[program.code, program.faculty].filter(Boolean).join(' · ')}</span>
                  <span className="program-meta">
                    {t('university.programOlympiads', { count: program.olympiads_count })}
                    {program.budget_places !== null
                      ? `, ${t('university.programPlaces', { count: program.budget_places })}`
                      : ''}
                    {program.is_mine ? (
                      <span className="program-saved">
                        <Icon name="check" size={12} />
                        {t('university.programSaved')}
                      </span>
                    ) : null}
                  </span>
                </button>
                <button
                  type="button"
                  className={`program-save${program.is_mine ? ' program-save-on' : ''}`}
                  aria-pressed={program.is_mine}
                  aria-label={
                    program.is_mine
                      ? t('university.programUnsave', { name: program.name })
                      : t('university.programSave', { name: program.name })
                  }
                  disabled={setPrograms.isPending}
                  onClick={() => toggleProgram(program)}
                >
                  <Icon name={program.is_mine ? 'check' : 'plus'} size={16} />
                </button>
              </div>
            ))}
          </div>
          {programs.length > shownPrograms.length ? (
            <button type="button" className="link show-more" onClick={() => setShowAllPrograms(true)}>
              {t('university.showAll', { count: programs.length })}
            </button>
          ) : null}
        </section>
      ) : (
        <section className="block">
          <h3 className="block-head">{t('university.directions')}</h3>
          <div className="directions">
            {university.directions.map((direction) => (
              <span key={direction} className="direction">
                {direction}
              </span>
            ))}
          </div>
        </section>
      )}

      {/* Олимпиады, дающие льготу в этом вузе, с переходом в карточку — F26. */}
      <section className="block" data-tour="university-olympiads" aria-busy={query.isPlaceholderData}>
        <h3 className="block-head">
          {t('university.olympiadsTitle')}
          <SourceTag kind="fact" />
        </h3>
        {chosen ? (
          <p className="program-filter">
            <span>{t('university.programChosen', { name: chosen.name })}</span>
            <button type="button" className="link" onClick={() => choose(null)}>
              {t('university.allPrograms')}
            </button>
          </p>
        ) : null}
        {chosen && olympiads.length === 0 && !query.isPlaceholderData ? (
          <p className="block-text">{t('university.programNoOlympiads')}</p>
        ) : null}
        {/* Строка на олимпиаду, а не на профиль (D3): предметы через запятую,
            льгота — лучшая из профилей; открывается профиль ученика. */}
        {shown.map((row) => (
          <UniversityOlympiadRow
            key={row.olympiad_id}
            id={row.open_profile_id}
            olympiadId={row.olympiad_id}
            name={row.first.name}
            subtitle={row.subjects}
            label={BENEFIT_LABELS[row.benefit]}
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
          {isMine ? t('university.inMine') : t('university.addToMine')}
        </Button>
      </div>

      <p className="lock">
        <Icon name="users" size={12} />
        {t('university.sharedNote')}
      </p>
    </Sheet>
  )
}
