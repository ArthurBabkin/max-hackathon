import { render } from '@testing-library/react'
import { expect, it } from 'vitest'
import { Tile } from './primitives'

// «ИТМО», «СПбГУ», «ПМГМУ» не влезали в плитку кеглем «КФУ»: кегль считается
// от ширины плитки и числа букв, поэтому плитка знает, сколько в подписи букв.
it('передаёт в стили число букв подписи', () => {
  const { container } = render(
    <>
      <Tile id="kfu" name="Казанский федеральный университет" shortName="КФУ" />
      <Tile id="x-spbgu" name="Санкт-Петербургский университет" shortName="СПбГУ" />
    </>,
  )
  const [short, long] = container.querySelectorAll<HTMLElement>('.tile')
  expect(short!.style.getPropertyValue('--tile-chars')).toBe('3')
  expect(long!.style.getPropertyValue('--tile-chars')).toBe('5')
})
