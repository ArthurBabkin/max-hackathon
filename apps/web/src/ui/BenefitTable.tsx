/**
 * Таблица «Льгота и условия в твоих вузах» (F18, F19).
 *
 * Вузы — строками: сколько бы их ни было, таблица растёт вниз. Столбцы —
 * закрытый набор, его выбирает сервер (`benefit_columns`). Всё редкое — класс
 * диплома, другой предмет ЕГЭ — плашкой под строкой своего вуза, новых
 * столбцов не появляется. Жёлтым — только то, что не как в большинстве вузов.
 * Кто олимпиаду не учитывает — одной строкой под таблицей.
 */

import type { BenefitColumn, BenefitGrant, BenefitRow } from '@contract'
import { outliers } from '@/lib/benefits'
import { useVoice } from '@/voice/useVoice'
import type { TextKey } from '@/voice/texts'
import { Tile } from './primitives'

const HEADS: Record<BenefitColumn, TextKey> = {
  winner: 'benefits.colWinner',
  prizer: 'benefits.colPrizer',
  ege: 'benefits.colEge',
  extra_points: 'benefits.colExtra',
}

const GRANT_CLASS: Record<BenefitGrant['kind'], string> = {
  bvi: 'benefit-bvi',
  score100: 'benefit-score',
  extra_points: 'benefit-extra',
}

export interface BenefitTableProps {
  rows: BenefitRow[]
  columns: BenefitColumn[]
  onOpen: (universityId: string) => void
}

export function BenefitTable({ rows, columns, onOpen }: BenefitTableProps) {
  const t = useVoice()
  const counted = rows.filter((row) => row.winner || row.prizer)
  const notCounted = rows.filter((row) => !row.winner && !row.prizer)

  const grantOf = (row: BenefitRow, column: BenefitColumn) => (column === 'prizer' ? row.prizer : row.winner)
  const ege = (row: BenefitRow) => {
    if (row.ege_min == null) return '—'
    // Диапазон — тоже нижний порог, просто разный по программам: «от 75–90».
    const count = row.ege_max != null ? `${row.ege_min}–${row.ege_max}` : row.ege_min
    return t('benefits.egeFrom', { count })
  }
  const label = (row: BenefitRow, column: BenefitColumn) =>
    column === 'ege' ? ege(row) : (grantOf(row, column)?.label ?? t('benefits.none'))

  // Отметки считаются по каждому столбцу отдельно, по тем же подписям, что видны.
  const differs = new Map(columns.map((column) => [column, outliers(counted.map((row) => label(row, column)))]))
  const anyDiffers = [...differs.values()].some((marks) => marks.size > 0)
  const egeShown = columns.includes('ege')
  const anyUnnamed = egeShown && counted.some((row) => row.ege_min == null)
  const byDirections = counted.some((row) => row.directions.length > 0)
  const legend = [
    anyDiffers ? t('benefits.legendDiffers') : null,
    anyUnnamed ? t('benefits.legendNoEge') : null,
    byDirections ? t('benefits.legendDirections') : null,
  ].filter(Boolean)

  const cell = (row: BenefitRow, column: BenefitColumn) => {
    if (column === 'ege') {
      // Диапазон без пояснения непонятен — пояснение в той же ячейке.
      const range = row.ege_min != null && row.ege_max != null
      return (
        <>
          <span className="benefit-table-ege">{label(row, column)}</span>
          {range ? <span className="benefit-table-ege-note">{t('benefits.egeRange')}</span> : null}
        </>
      )
    }
    const grant = grantOf(row, column)
    return (
      <span className={`benefit-value ${grant ? GRANT_CLASS[grant.kind] : 'benefit-none'}`}>
        {label(row, column)}
      </span>
    )
  }

  return (
    <>
      {counted.length > 0 ? (
        <table className="benefit-table">
          <colgroup>
            <col className="benefit-table-uni" />
            {columns.map((column) => (
              <col key={column} />
            ))}
          </colgroup>
          <thead>
            <tr>
              <th scope="col">{t('benefits.colUniversity')}</th>
              {columns.map((column) => (
                <th key={column} scope="col">
                  {t(HEADS[column])}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {counted.flatMap((row, i) => {
              const tr = (
                <tr key={row.university_id}>
                  <th scope="row">
                    <button type="button" onClick={() => onOpen(row.university_id)}>
                      <Tile
                        id={row.university_id}
                        name={row.university_name}
                        shortName={row.university_short_name}
                        color={row.color}
                        filled
                      />
                      <span>{row.university_nick}</span>
                    </button>
                    {row.directions.length > 0 ? (
                      <>
                        {' '}
                        <span className="benefit-table-directions">{row.directions.join(', ')}</span>
                      </>
                    ) : null}
                  </th>
                  {columns.map((column) => {
                    const marked = differs.get(column)!.has(i)
                    return (
                      <td key={column} className={marked ? 'benefit-table-differs' : undefined}>
                        {cell(row, column)}
                        {marked ? <span className="sr-only">{t('benefits.differs')}</span> : null}
                      </td>
                    )
                  })}
                </tr>
              )
              // Слабее льгота на других моих направлениях — подстрокой со своими столбцами.
              const others = row.other_directions.flatMap((other, j) => {
                const { winner, prizer } = other
                if (!winner) return []
                return [
                  <tr key={`${row.university_id}-${j}`} className="benefit-table-sub">
                    <th scope="row">{other.directions.join(', ')}</th>
                    {columns.map((column) => {
                      const grant = column === 'winner' ? winner : column === 'prizer' ? prizer : null
                      return (
                        <td key={column}>
                          {column === 'ege' ? null : (
                            <span className={`benefit-value ${grant ? GRANT_CLASS[grant.kind] : 'benefit-none'}`}>
                              {grant?.label ?? t('benefits.none')}
                            </span>
                          )}
                        </td>
                      )
                    })}
                  </tr>,
                ]
              })
              if (!row.conditions?.length) return [tr, ...others]
              return [
                tr,
                ...others,
                <tr key={`${row.university_id}-note`} className="benefit-table-note">
                  <td colSpan={columns.length + 1}>
                    {row.conditions.map((condition) => (
                      <p key={condition}>{condition}</p>
                    ))}
                  </td>
                </tr>,
              ]
            })}
          </tbody>
        </table>
      ) : null}

      {notCounted.length > 0 ? (
        <p className="benefit-table-skip">
          {t(notCounted.length === 1 ? 'benefits.notCountedOne' : 'benefits.notCountedMany', {
            names: notCounted.map((row) => row.university_nick).join(', '),
          })}
        </p>
      ) : null}

      {legend.length > 0 ? (
        <p className="benefit-table-legend">
          {anyDiffers ? <span className="benefit-table-swatch" aria-hidden="true" /> : null}
          {legend.join(' ')}
        </p>
      ) : null}
    </>
  )
}
