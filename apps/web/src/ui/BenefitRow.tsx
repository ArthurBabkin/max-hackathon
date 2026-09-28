/**
 * Строка льготы: вуз слева, что он даёт — справа.
 *
 * Вузы в «Где ещё даёт льготу» карточки олимпиады (F23); свои вузы ученика
 * там же — таблицей (BenefitTable). Зеркальная строка — в карточке вуза.
 */

import type { BenefitKind, BenefitRow as BenefitRowData } from '@contract'
import { NO_BENEFIT_LABEL } from '@contract'
import { countText, useVoice } from '@/voice/useVoice'
import { Icon } from './Icon'
import { BenefitValue, Tile } from './primitives'

export interface BenefitRowProps {
  data: BenefitRowData
  onOpen?: (universityId: string) => void
}

export function BenefitRow({ data, onOpen }: BenefitRowProps) {
  const t = useVoice()
  // «Где ещё даёт льготу» (F65): на скольких направлениях вуза.
  const coverage =
    data.directions_count > 0 && data.directions_total > 0
      ? ` · ${t('university.onDirections', { count: data.directions_count, total: countText(t, 'count.directionsGen', data.directions_total) })}`
      : ''
  const body = (
    <>
      <span className="benefit-main">
        <Tile
          id={data.university_id}
          name={data.university_name}
          shortName={data.university_short_name}
          color={data.color}
          filled
        />
        <span className="benefit-text">
          <b>{data.university_name}</b>
          <span>
            {data.city}
            {coverage}
          </span>
          {data.conditions?.map((condition) => (
            <span key={condition} className="benefit-cond">
              {condition}
            </span>
          ))}
        </span>
      </span>
      <BenefitValue kind={data.benefit} label={data.benefit_label ?? NO_BENEFIT_LABEL} />
      {onOpen ? <Icon name="chevron" size={15} /> : null}
    </>
  )

  if (!onOpen) return <div className="benefit">{body}</div>

  return (
    <button type="button" className="benefit" onClick={() => onOpen(data.university_id)}>
      {body}
    </button>
  )
}

/** Строка олимпиады в карточке вуза — зеркальная к BenefitRow (F26). */
export function UniversityOlympiadRow({
  id,
  olympiadId,
  name,
  subtitle,
  label,
  benefit,
  shortName,
  color,
  onOpen,
}: {
  id: string
  olympiadId: string
  name: string
  subtitle: string
  label: string
  /** Вид льготы — цвет плашки, как в таблице льгот. */
  benefit?: BenefitKind
  shortName?: string | null
  color?: string | null
  onOpen: (olympiadProfileId: string) => void
}) {
  return (
    <button type="button" className="benefit" onClick={() => onOpen(id)}>
      <span className="benefit-main">
        <Tile id={olympiadId} name={name} shortName={shortName} color={color} filled />
        <span className="benefit-text">
          <b>{name}</b>
          <span>{subtitle}</span>
        </span>
      </span>
      <BenefitValue kind={benefit ?? 'bvi'} label={label} />
      <Icon name="chevron" size={15} />
    </button>
  )
}
