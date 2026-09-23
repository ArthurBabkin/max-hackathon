/**
 * Строка льготы: вуз слева, что он даёт — справа.
 *
 * Используется в карточке олимпиады дважды («Льгота в твоих вузах» и
 * «Где даёт льготу») и в карточке вуза, поэтому вынесена отдельно.
 */

import type { BenefitRow as BenefitRowData } from '@contract'
import { NO_BENEFIT_LABEL } from '@contract'
import { Icon } from './Icon'
import { Tile } from './primitives'

/** Класс подписи по виду льготы — БВИ выделяется сильнее остальных. */
function benefitClass(data: BenefitRowData): string {
  if (!data.benefit) return 'benefit-value benefit-none'
  if (data.benefit === 'bvi' || data.benefit === 'bvi_winners') return 'benefit-value benefit-bvi'
  if (data.benefit === 'score100') return 'benefit-value benefit-score'
  return 'benefit-value benefit-extra'
}

export interface BenefitRowProps {
  data: BenefitRowData
  onOpen?: (universityId: string) => void
}

export function BenefitRow({ data, onOpen }: BenefitRowProps) {
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
          <span>{data.city}</span>
          {/* Особенности вуза в карточке олимпиады (F19). */}
          {data.conditions?.map((condition) => (
            <span key={condition} className="benefit-condition">
              {condition}
            </span>
          ))}
        </span>
      </span>
      <span className={benefitClass(data)}>{data.benefit_label ?? NO_BENEFIT_LABEL}</span>
    </>
  )

  // С особенностями строка выше обычной: значок и льгота держатся верха.
  const className = data.conditions?.length ? 'benefit benefit-tall' : 'benefit'
  if (!onOpen) return <div className={className}>{body}</div>

  return (
    <button type="button" className={className} onClick={() => onOpen(data.university_id)}>
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
  shortName,
  color,
  onOpen,
}: {
  id: string
  olympiadId: string
  name: string
  subtitle: string
  label: string
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
      <span className="benefit-value benefit-bvi">{label}</span>
      <Icon name="chevron" size={15} />
    </button>
  )
}
