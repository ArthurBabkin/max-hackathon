import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { getWebApp } from '@/bridge'
import { makeSession, renderApp } from '@/test/render'
import { FaqScreen } from './Faq'

it('показывает разделы, раскрывает ответ и открывает первоисточник через MAX', async () => {
  const openLink = vi.spyOn(getWebApp(), 'openLink').mockImplementation(() => {})
  renderApp(<FaqScreen />, { route: '/faq' })

  expect(screen.getByRole('heading', { name: 'Льготы при поступлении' })).toBeInTheDocument()
  await userEvent.click(screen.getByText('Что такое БВИ?'))
  expect(screen.getByText(/только в одном вузе и на одной программе/)).toBeVisible()

  await userEvent.click(screen.getAllByRole('button', { name: /статья 71/ })[0]!)
  expect(openLink).toHaveBeenCalledWith(expect.stringMatching(/^https:\/\/www\.consultant\.ru\//))
})

it('поиск оставляет подходящие вопросы, пустой результат — сброс', async () => {
  renderApp(<FaqScreen />, { route: '/faq' })
  const search = screen.getByRole('searchbox', { name: 'Поиск по вопросам' })

  await userEvent.type(search, 'ЕГЭ')
  expect(screen.getByText('Нужно ли сдавать ЕГЭ, если есть диплом?')).toBeInTheDocument()
  expect(screen.queryByText('Как удалить свои данные?')).not.toBeInTheDocument()

  await userEvent.clear(search)
  await userEvent.type(search, 'квазиэкспонента')
  await userEvent.click(screen.getByRole('button', { name: 'Сбросить поиск' }))
  expect(screen.getByText('Как удалить свои данные?')).toBeInTheDocument()
})

it('родителю отвечает на «вы» и о ребёнке по имени', async () => {
  renderApp(<FaqScreen />, { route: '/faq', session: makeSession({ role: 'parent' }) })
  const question = screen.getByText('Нужно ли сдавать ЕГЭ, если есть диплом?')
  await userEvent.click(question)
  expect(within(question.closest('details')!).getByText(/в вузах Артёма/)).toBeVisible()
})
