import { fireEvent, render } from '@testing-library/react'
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

it('показывает логотип вместо подписи, если он есть', () => {
  const { container } = render(<Tile id="hse" name="НИУ ВШЭ" shortName="ВШЭ" />)
  const img = container.querySelector('img')
  expect(img).toHaveAttribute('src', '/logos/hse.png')
  expect(container.querySelector('.tile')).not.toHaveTextContent('ВШЭ')
})

it('без логотипа показывает подпись', () => {
  const { container } = render(<Tile id="kfu" name="КФУ" shortName="КФУ" />)
  expect(container.querySelector('img')).toBeNull()
  expect(container.querySelector('.tile')).toHaveTextContent('КФУ')
})

it('если логотип не загрузился, возвращает подпись', () => {
  const { container } = render(<Tile id="hse" name="НИУ ВШЭ" shortName="ВШЭ" />)
  fireEvent.error(container.querySelector('img')!)
  expect(container.querySelector('img')).toBeNull()
  expect(container.querySelector('.tile')).toHaveTextContent('ВШЭ')
})
