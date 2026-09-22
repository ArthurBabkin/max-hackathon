/** Карточка вуза — экран D3. Функции F25, F26. */

import { Button } from '@maxhub/max-ui'
import { BENEFIT_LABELS } from '@contract'
import { useProfile, useSetUniversities, useUniversity } from '@/api/queries'
import { getWebApp } from '@/bridge'
import { formatShortDate } from '@/lib/deadline'
import { UniversityOlympiadRow } from '@/ui/BenefitRow'
import { Icon } from '@/ui/Icon'
import { Sheet } from '@/ui/Sheet'
import { CardSkeletons, SourceTag, StateBlock, Tile } from '@/ui/primitives'
import type { SheetStack } from '@/ui/sheets'
import { useVoice } from '@/voice/useVoice'

export function UniversitySheet({ id, sheets }: { id: string; sheets: SheetStack }) {
  const t = useVoice()
  const query = useUniversity(id)
  const { data: profile } = useProfile()
  const setUniversities = useSetUniversities()

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
        <StateBlock icon="wifiOff" tone="error" title={t('state.errorTitle')} text={t('state.errorText')}>
          <Button stretched onClick={() => void query.refetch()}>
            {t('state.errorRetry')}
          </Button>
        </StateBlock>
      </Sheet>
    )
  }

  const university = query.data
  const current = profile?.universities.map((u) => u.id) ?? []
  const isMine = university.is_mine

  const toggle = () => {
    const next = isMine ? current.filter((x) => x !== university.id) : [...current, university.id]
    // Хотя бы один вуз обязателен (F9, F49): без него не из чего считать льготы.
    if (next.length === 0) return
    setUniversities.mutate(next)
  }

  const lastOne = isMine && current.length <= 1

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

      {/* Олимпиады, дающие льготу в этом вузе, с переходом в карточку — F26. */}
      <section className="block">
        <h3 className="block-head">
          {t('university.olympiadsTitle')}
          <SourceTag kind="fact" />
        </h3>
        {university.olympiads.map((olympiad) => (
          <UniversityOlympiadRow
            key={olympiad.olympiad_profile_id}
            id={olympiad.olympiad_profile_id}
            olympiadId={olympiad.olympiad_id ?? olympiad.olympiad_profile_id}
            name={olympiad.name}
            subtitle={olympiad.subject_name ?? ''}
            label={olympiad.benefit_label ?? BENEFIT_LABELS[olympiad.benefit]}
            shortName={olympiad.short_name}
            color={olympiad.color}
            onOpen={(profileId) => sheets.open({ kind: 'oly', id: profileId })}
          />
        ))}
      </section>

      {university.ege_note ? (
        <section className="block">
          <h3 className="block-head">{t('university.egeTitle')}</h3>
          <p className="block-text">{t('university.egeText', { note: university.ege_note })}</p>
        </section>
      ) : null}

      {university.rules_url ? (
        <button
          type="button"
          className="rules-row"
          onClick={() => getWebApp().openLink(university.rules_url!)}
        >
          <span>
            <Icon name="doc" size={15} />
            {t('university.rules')}
          </span>
          <span className="rules-date">
            {university.rules_verified_at
              ? t('university.verifiedAt', {
                  date: formatShortDate(university.rules_verified_at) ?? university.rules_verified_at,
                })
              : t('olympiad.benefitsUnknown')}
          </span>
        </button>
      ) : null}

      <div className="sheet-actions">
        <Button
          stretched
          variant={isMine ? 'secondary' : 'primary'}
          disabled={lastOne}
          loading={setUniversities.isPending}
          iconBefore={<Icon name={isMine ? 'check' : 'plus'} size={16} />}
          onClick={toggle}
        >
          {isMine ? t('university.inMine') : t('university.addToMine')}
        </Button>
      </div>

      <p className="lock">
        <Icon name="users" size={12} />
        {lastOne ? t('profile.needUniversity') : t('university.sharedNote')}
      </p>
    </Sheet>
  )
}
