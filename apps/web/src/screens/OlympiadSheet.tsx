/** Карточка олимпиады — экраны C4, C5, C6. Функции F17–F23, F28. */

import { Button } from '@maxhub/max-ui'
import type { OlympiadDetail } from '@contract'
import { useAddToTracker, useOlympiad, usePropose, useSession } from '@/api/queries'
import { formatShortDate } from '@/lib/deadline'
import { getWebApp } from '@/bridge'
import { trackerAction } from '@/lib/permissions'
import { BenefitRow } from '@/ui/BenefitRow'
import { Icon } from '@/ui/Icon'
import { Sheet } from '@/ui/Sheet'
import { CardSkeletons, Pill, SourceLine, SourceTag, StateBlock, Tile } from '@/ui/primitives'
import type { SheetStack } from '@/ui/sheets'
import { useVoice } from '@/voice/useVoice'

function levelLabel(detail: OlympiadDetail): string {
  if (detail.kind === 'vsosh') return 'ВсОШ'
  if (detail.kind === 'other') return 'вне перечня'
  return detail.level ? `${detail.level} уровень` : 'уровень уточняется'
}

/** Таймлайн этапов: пройденные серым, ближайший выделен (F21). */
function Stages({ detail }: { detail: OlympiadDetail }) {
  return (
    <ol className="timeline">
      {detail.stages.map((stage) => (
        <li key={stage.id} className={`stage stage-${stage.state}`}>
          <b>{stage.title}</b>
          <span>{stage.subtitle}</span>
        </li>
      ))}
    </ol>
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
      <Sheet label="Карточка олимпиады" canGoBack={canGoBack} onBack={sheets.back} onClose={sheets.closeAll}>
        <CardSkeletons count={2} />
      </Sheet>
    )
  }

  if (query.isError || !query.data) {
    return (
      <Sheet label="Карточка олимпиады" canGoBack={canGoBack} onBack={sheets.back} onClose={sheets.closeAll}>
        <StateBlock icon="wifiOff" tone="error" title={t('state.errorTitle')} text={t('state.errorText')}>
          <Button stretched onClick={() => void query.refetch()}>
            {t('state.errorRetry')}
          </Button>
        </StateBlock>
      </Sheet>
    )
  }

  const detail = query.data
  const pending = detail.proposal_status === 'pending'
  const busy = add.isPending || propose.isPending

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
            <p>{detail.organizer}</p>
          </div>
        </>
      }
    >
      <div className="oly-card-tags sheet-tags">
        <span className={`level${detail.kind === 'vsosh' ? ' level-vsosh' : detail.kind === 'other' ? ' level-outside' : ''}`}>
          {levelLabel(detail)}
        </span>
        {detail.format ? <span className="pill pill-ok">{detail.format}</span> : null}
        <Pill deadlineAt={detail.deadline_at} doneLabel={t('pill.done')} />
      </div>

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
          <div className="levels">
            {detail.profiles.map((profile) => (
              <span key={profile.olympiad_profile_id} className={`level-item${profile.is_mine ? ' level-item-mine' : ''}`}>
                {profile.subject_name}: {profile.level ?? '—'}
              </span>
            ))}
          </div>
          <p className="fine">{t('olympiad.levelsHint', { subject: detail.subject_name.toLowerCase() })}</p>
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

      {/* Льгота в вузах ученика — F18. */}
      <section className="block">
        <h3 className="block-head">
          {t('olympiad.benefitsTitle')}
          {detail.benefits_source ? <SourceTag kind="fact" /> : null}
        </h3>
        {detail.benefits.map((row) => (
          <BenefitRow
            key={row.university_id}
            data={row}
            onOpen={(universityId) => sheets.open({ kind: 'vuz', id: universityId })}
          />
        ))}
        {detail.benefits_source ? (
          <SourceLine
            title={`${detail.benefits_source.title}${
              detail.benefits_source.verified_at
                ? `. Проверено ${formatShortDate(detail.benefits_source.verified_at)}`
                : ''
            }`}
            url={detail.benefits_source.url}
          />
        ) : (
          <p className="fine">{t('olympiad.benefitsUnknown')}</p>
        )}
      </section>

      {/* Условия подтверждения — F19. */}
      <section className="block">
        <h3 className="block-head">{t('olympiad.conditionsTitle')}</h3>
        <ul className="conditions">
          {detail.conditions.map((condition) => (
            <li key={condition}>
              <Icon name="check" size={13} strokeWidth={2.6} />
              <span>{condition}</span>
            </li>
          ))}
        </ul>
      </section>

      {/* Почему подходит — F20. */}
      <section className="block">
        <h3 className="block-head">
          {t('olympiad.whyTitle')}
          <SourceTag kind="recommendation" />
        </h3>
        <p className="block-text">{detail.why}</p>
      </section>

      {/* Этапы и даты — F21. У ВсОШ они уже показаны выше. */}
      {detail.kind !== 'vsosh' ? (
        <section className="block">
          <h3 className="block-head">
            {t('olympiad.stagesTitle')}
            <SourceTag kind={detail.stages_are_demo ? 'demo' : 'fact'} />
          </h3>
          <Stages detail={detail} />
        </section>
      ) : null}

      {/* Где даёт льготу — F23. */}
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

      <div className="sheet-actions">
        {mainButton()}
        {detail.official_url ? (
          <Button
            stretched
            variant="secondary"
            iconAfter={<Icon name="external" size={14} />}
            onClick={() => getWebApp().openLink(detail.official_url!)}
          >
            {t('olympiad.officialSite')}
          </Button>
        ) : null}
      </div>

      <p className="lock">
        <Icon name="bell" size={12} />
        {lockNote}
      </p>
    </Sheet>
  )
}
