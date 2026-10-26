/** Карточка олимпиады — экраны C4, C5, C6. Функции F17–F23, F28. */

import { useState } from 'react'
import { Button } from '@maxhub/max-ui'
import type { OlympiadDetail, Source } from '@contract'
import { useAddToTracker, useOlympiad, usePropose, useSession } from '@/api/queries'
import { formatShortDate } from '@/lib/deadline'
import { levelLabel } from '@/lib/level'
import { stageSubtitle } from '@/lib/stages'
import { getWebApp } from '@/bridge'
import { trackerAction } from '@/lib/permissions'
import { BenefitRow } from '@/ui/BenefitRow'
import { BenefitTable } from '@/ui/BenefitTable'
import { Icon } from '@/ui/Icon'
import { Sheet } from '@/ui/Sheet'
import { CardSkeletons, ListToggle, Pill, SourceLine, SourceTag, Tile } from '@/ui/primitives'
import type { SheetStack } from '@/ui/sheets'
import { useVoice } from '@/voice/useVoice'
import { ErrorState } from '@/ui/ErrorState'

/** Таймлайн этапов: пройденные серым, ближайший выделен (F21). */
function Stages({ detail }: { detail: OlympiadDetail }) {
  const t = useVoice()
  return (
    <ol className="timeline">
      {detail.stages.map((stage) => (
        <li key={stage.id} className={`stage stage-${stage.state}`}>
          <b>{stage.title}</b>
          <span>{stageSubtitle(stage, t)}</span>
        </li>
      ))}
    </ol>
  )
}

/** Сколько плашек уровней видно сразу: у НТО их двадцать одна. */
const LEVELS_PREVIEW = 6

/**
 * Уровни по профилям (F17). Плашка — переключатель: у профилей одной
 * олимпиады свои сроки и льготы, нажатие открывает карточку профиля на месте
 * этой (#135). Порядок не меняется при переключении, иначе плашки прыгают;
 * длинный список свёрнут до шести, открытый профиль виден и в свёрнутом.
 */
function Levels({ profiles, onSelect }: { profiles: OlympiadDetail['profiles']; onSelect: (id: string) => void }) {
  const t = useVoice()
  const [expanded, setExpanded] = useState(false)
  // Две-три лишние плашки прятать незачем.
  const long = profiles.length > LEVELS_PREVIEW + 2
  const preview = profiles.slice(0, LEVELS_PREVIEW)
  const current = profiles.find((p) => p.is_mine)
  if (current && !preview.includes(current)) preview.splice(LEVELS_PREVIEW - 1, 1, current)
  const shown = long && !expanded ? preview : profiles
  return (
    <>
      <div className="levels">
        {shown.map((profile) => (
          <button
            key={profile.olympiad_profile_id}
            type="button"
            className={`level-item${profile.is_mine ? ' level-item-mine' : ''}`}
            aria-current={profile.is_mine ? 'true' : undefined}
            onClick={() => {
              if (!profile.is_mine) onSelect(profile.olympiad_profile_id)
            }}
          >
            {profile.subject_name}: {profile.level ?? '—'}
          </button>
        ))}
      </div>
      {long ? (
        <ListToggle
          expanded={expanded}
          label={t('olympiad.allProfiles', { count: profiles.length })}
          onToggle={() => setExpanded(!expanded)}
        />
      ) : null}
    </>
  )
}

/**
 * Источники льгот под таблицей (F18): у каждого вуза свои правила приёма —
 * ссылка на документ своего вуза; вузы без источника — «данные уточняются»
 * с их названиями, а не пометкой на весь блок.
 */
function BenefitSources({ rows }: { rows: OlympiadDetail['benefits'] }) {
  const t = useVoice()
  const counted = rows.filter((row) => row.winner || row.prizer)
  const sources = new Map<string, Source>()
  for (const row of counted) if (row.source) sources.set(row.source.id, row.source)
  const unknown = counted.filter((row) => !row.source).map((row) => row.university_nick)
  if (counted.length === 0) return null
  if (sources.size === 0) return <p className="fine">{t('olympiad.benefitsUnknown')}</p>
  return (
    <>
      {[...sources.values()].map((source) => (
        <SourceLine
          key={source.id}
          title={
            source.verified_at
              ? t('olympiad.sourceVerified', {
                  title: source.title,
                  date: formatShortDate(source.verified_at) ?? '',
                })
              : source.title
          }
          url={source.url}
        />
      ))}
      {unknown.length > 0 ? (
        <p className="fine">{t('olympiad.benefitsUnknownAt', { names: unknown.join(', ') })}</p>
      ) : null}
    </>
  )
}

export function OlympiadSheet({ id, sheets }: { id: string; sheets: SheetStack }) {
  const t = useVoice()
  const { data: session } = useSession()
  const query = useOlympiad(id)
  const add = useAddToTracker()
  const propose = usePropose()

  const canGoBack = sheets.stack.length > 1
  const action = session ? trackerAction(session) : 'add'

  if (query.isPending) {
    return (
      <Sheet label={t('sheet.olympiad')} canGoBack={canGoBack} onBack={sheets.back} onClose={sheets.closeAll}>
        <CardSkeletons count={2} />
      </Sheet>
    )
  }

  if (query.isError || !query.data) {
    return (
      <Sheet label={t('sheet.olympiad')} canGoBack={canGoBack} onBack={sheets.back} onClose={sheets.closeAll}>
        <ErrorState error={query.error} onRetry={() => void query.refetch()} />
      </Sheet>
    )
  }

  const detail = query.data
  const pending = detail.proposal_status === 'pending'
  // Пока грузится выбранный профиль, на экране ещё прежний — добавлять рано.
  const busy = add.isPending || propose.isPending || query.isPlaceholderData
  // У олимпиады несколько профилей — в шапке видно, какой открыт.
  const severalProfiles = detail.profiles.length > 1

  const mainButton = () => {
    if (detail.in_tracker) {
      return (
        <Button stretched variant="secondary" disabled iconBefore={<Icon name="check" size={16} />}>
          {t('olympiad.inTracker')}
        </Button>
      )
    }
    if (pending) {
      return (
        <Button stretched variant="secondary" disabled iconBefore={<Icon name="clock" size={16} />}>
          {t('olympiad.proposalPending')}
        </Button>
      )
    }
    return (
      <Button
        stretched
        loading={busy}
        iconBefore={<Icon name={action === 'propose' ? 'send' : 'plus'} size={16} />}
        onClick={() =>
          action === 'propose'
            ? propose.mutate(detail.olympiad_profile_id)
            : add.mutate(detail.olympiad_profile_id)
        }
      >
        {action === 'propose' ? t('olympiad.proposeCta') : t('olympiad.addCta')}
      </Button>
    )
  }

  const lockNote = detail.in_tracker
    ? t('olympiad.remindAll')
    : action === 'propose'
      ? t('olympiad.kidDecides')
      : t('olympiad.remindAfterAdd')

  return (
    <Sheet
      label={detail.name}
      canGoBack={canGoBack}
      onBack={sheets.back}
      onClose={sheets.closeAll}
      footer={
        <>
          <div data-tour="track-action">{mainButton()}</div>
          <p className="lock lock-center">
            <Icon name="bell" size={12} />
            {lockNote}
          </p>
        </>
      }
      header={
        <>
          <Tile
            id={detail.olympiad_id}
            name={detail.name}
            shortName={detail.short_name}
            color={detail.color}
            size="lg"
            filled
          />
          <div className="sheet-title">
            <h2>{detail.name}</h2>
            {severalProfiles ? <p className="sheet-title-profile">{detail.subject_name}</p> : null}
            <p>{detail.organizer}</p>
          </div>
        </>
      }
    >
      <div className="oly-card-tags sheet-tags">
        <span className={`level${detail.kind === 'vsosh' ? ' level-vsosh' : detail.kind === 'other' ? ' level-outside' : ''}`}>
          {levelLabel(detail.kind, detail.level, t)}
        </span>
        {detail.format ? <span className="pill pill-ok">{detail.format}</span> : null}
        <Pill deadlineAt={detail.deadline_at} doneLabel={t('pill.done')} />
      </div>
      {detail.registration_closed ? (
        <div className="closed-note" role="note">
          <Icon name="clock" size={15} />
          <p>
            <b>{t('olympiad.registrationClosedTitle')}</b>
            {t('olympiad.registrationClosedText')}
          </p>
        </div>
      ) : null}

      {/* Что это за олимпиада и где её сайт (F22) — первым делом, до льгот. */}
      {detail.description || detail.official_url ? (
        <section className="block">
          <h3 className="block-head">{t('olympiad.aboutTitle')}</h3>
          {detail.description ? <p className="block-text">{detail.description}</p> : null}
          {detail.official_url ? (
            <button type="button" className="about-link" onClick={() => getWebApp().openLink(detail.official_url!)}>
              <Icon name="external" size={15} />
              {t('olympiad.officialSite')}
            </button>
          ) : null}
        </section>
      ) : null}

      {/* Уровень профиля — F17. У ВсОШ уровней нет, вместо них этапы. */}
      {detail.kind === 'vsosh' ? (
        <section className="block">
          <h3 className="block-head">
            {t('olympiad.vsoshStagesTitle')}
            <SourceTag kind={detail.stages_are_demo ? 'demo' : 'fact'} />
          </h3>
          <Stages detail={detail} />
          <p className="fine">{t('olympiad.vsoshHint')}</p>
        </section>
      ) : detail.kind === 'perechen' ? (
        <section className="block">
          <h3 className="block-head">
            {t('olympiad.levelsTitle')}
            <SourceTag kind="fact" />
          </h3>
          <Levels profiles={detail.profiles} onSelect={(profileId) => sheets.replace({ kind: 'oly', id: profileId })} />
          {severalProfiles ? <p className="fine">{t('olympiad.levelsHint')}</p> : null}
          {detail.profiles_source ? (
            <SourceLine title={detail.profiles_source.title} url={detail.profiles_source.url} />
          ) : null}
        </section>
      ) : (
        <section className="block">
          <h3 className="block-head">{t('olympiad.outsideTitle')}</h3>
          <p className="block-text">{t('olympiad.outsideText')}</p>
        </section>
      )}

      {/* Этапы и даты — F21, сразу под уровнем: когда регистрация, важно не
          меньше льгот. У ВсОШ этапы уже показаны выше вместо уровней. */}
      {detail.kind !== 'vsosh' ? (
        <section className="block">
          <h3 className="block-head">
            {t('olympiad.stagesTitle')}
            <SourceTag kind={detail.stages_are_demo ? 'demo' : 'fact'} />
          </h3>
          <Stages detail={detail} />
        </section>
      ) : null}

      {/* Льгота и условия в вузах ученика — F18, F19: вузы таблицей, своё у
          вуза — под его строкой, общее для всех — после таблицы. */}
      <section className="block">
        <h3 className="block-head">
          {t('olympiad.benefitsTitle')}
          {detail.benefits_source ? <SourceTag kind="fact" /> : null}
        </h3>
        <BenefitTable
          rows={detail.benefits}
          columns={detail.benefit_columns}
          onOpen={(universityId) => sheets.open({ kind: 'vuz', id: universityId })}
        />
        {detail.conditions.length > 0 ? (
          <div className="benefit-everywhere">
            {/* Подпись — только когда есть вузы, к которым «во всех» относится. */}
            {detail.benefits.some((row) => row.winner || row.prizer) ? (
              <p className="benefit-everywhere-head">{t('benefits.everywhere')}</p>
            ) : null}
            <ul className="conditions">
              {detail.conditions.map((condition) => (
                <li key={condition}>
                  <Icon name="check" size={13} strokeWidth={2.6} />
                  <span>{condition}</span>
                </li>
              ))}
            </ul>
          </div>
        ) : null}
        <BenefitSources rows={detail.benefits} />
      </section>

      {/* Почему подходит — F20. */}
      <section className="block">
        <h3 className="block-head">
          {t('olympiad.whyTitle')}
          <SourceTag kind="recommendation" />
        </h3>
        <p className="block-text">{detail.why}</p>
      </section>

      {/* Где ещё даёт льготу — F23: вузы базы, кроме вузов ученика. */}
      {detail.benefit_universities.length > 0 ? (
        <section className="block">
          <h3 className="block-head">{t('olympiad.whereTitle')}</h3>
          {detail.benefit_universities.map((row) => (
            <BenefitRow
              key={row.university_id}
              data={row}
              onOpen={(universityId) => sheets.open({ kind: 'vuz', id: universityId })}
            />
          ))}
        </section>
      ) : null}
    </Sheet>
  )
}
