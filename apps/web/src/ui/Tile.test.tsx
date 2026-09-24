import { render } from '@testing-library/react'
import { expect, it } from 'vitest'
import { Tile } from './primitives'

// «МФТИ», «СПбГУ» не влезают в маленький значок тем же кеглем, что «КФУ»:
// такие подписи помечены, чтобы значок мог их уменьшить.
it('помечает длинную подпись значка', () => {
  const { container } = render(
    <>
      <Tile id="kfu" name="Казанский федеральный университет" shortName="КФУ" />
      <Tile id="mipt" name="Московский физико-технический институт" shortName="МФТИ" />
    </>,
  )
  const [short, long] = container.querySelectorAll('.tile')
  expect(short).not.toHaveClass('tile-long')
  expect(long).toHaveClass('tile-long')
})
