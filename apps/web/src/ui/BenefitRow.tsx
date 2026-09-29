/**
 * Строка льготы: вуз слева, что он даёт — справа.
 *
 * Вузы в «Где ещё даёт льготу» карточки олимпиады (F23); свои вузы ученика
 * там же — таблицей (BenefitTable). Зеркальная строка — в карточке вуза.
 */

import { useState } from 'react'
import type { BenefitKind, BenefitRow as BenefitRowData } from '@contract'
import { NO_BENEFIT_LABEL } from '@contract'
import { countText, useVoice } from '@/voice/useVoice'
import { Icon } from './Icon'
import { BenefitValue, ListToggle, Tile } from './primitives'

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

/**
 * Сколько знаков подписи видно в свёрнутой строке — пара строк на телефоне.
 * Считается длина, а не число профилей: у НТО профиль бывает длиной в абзац.
 */
const SUBJECTS_BUDGET = 70

/** Профили, что влезают в свёрнутую подпись; хотя бы один. */
function previewSubjects(subjects: string[]): string[] {
  const out: string[] = []
  let length = 0
  for (const subject of subjects) {
    if (out.length > 0 && length + 2 + subject.length > SUBJECTS_BUDGET) break
    length += (out.length > 0 ? 2 : 0) + subject.length
    out.push(subject)
  }
  return out
}

/**
 * Строка олимпиады в карточке вуза — зеркальная к BenefitRow (F26). Под
 * названием — профили олимпиады; длинный список свёрнут до пары строк,
 * остальные раскрывает кнопка под строкой: в самой строке её не вложить,
 * строка — кнопка.
 */
export function UniversityOlympiadRow({
  id,
  olympiadId,
  name,
  subjects,
  coverage = '',
  label,
  benefit,
  shortName,
  color,
  onOpen,
}: {
  id: string
  olympiadId: string
  name: string
  subjects: string[]
  /** Хвост подписи: « · на 2 из 19 направлений». */
  coverage?: string
  label: string
  /** Вид льготы — цвет плашки, как в таблице льгот. */
  benefit?: BenefitKind
  shortName?: string | null
  color?: string | null
  onOpen: (olympiadProfileId: string) => void
}) {
  const t = useVoice()
  const [expanded, setExpanded] = useState(false)
  const preview = previewSubjects(subjects)
  const long = preview.length < subjects.length
  const shown =
    long && !expanded
      ? t('university.subjectsMore', { subjects: preview.join(', '), count: subjects.length - preview.length })
      : subjects.join(', ')

  return (
    <div className="benefit-item">
      <button type="button" className="benefit" onClick={() => onOpen(id)}>
        <span className="benefit-main">
          <Tile id={olympiadId} name={name} shortName={shortName} color={color} filled />
          <span className="benefit-text">
            <b>{name}</b>
            <span>
              {shown}
              {coverage}
            </span>
          </span>
        </span>
        <BenefitValue kind={benefit ?? 'bvi'} label={label} />
        <Icon name="chevron" size={15} />
      </button>
      {long ? <ListToggle expanded={expanded} count={subjects.length} onToggle={() => setExpanded(!expanded)} /> : null}
    </div>
  )
}
