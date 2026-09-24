import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import { renderApp } from '@/test/render'
import { Tabbar } from './Tabbar'

describe('Tabbar', () => {
  it('ставит «Трекер» вторым, а «Подбор» — после «Каталога»', () => {
    renderApp(<Tabbar trackerCount={0} />)

    const nav = screen.getByRole('navigation', { name: 'Разделы приложения' })
    const labels = within(nav)
      .getAllByRole('link')
      .map((link) => link.textContent)

    expect(labels).toEqual(['Главная', 'Трекер', 'Каталог', 'Подбор', 'Семья'])
  })
})
