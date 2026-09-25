/**
 * Всплывающее сообщение об ошибке действия: сохранить профиль, добавить в
 * трекер, создать ссылку. Экран при этом не меняется — без сообщения сбой
 * был бы незаметен. Одно сообщение за раз: новое заменяет старое.
 *
 * Подтверждение (showInfoToast) — там, где результат действия уходит с
 * экрана: добавленная из «Подбора» олимпиада исчезает из списка.
 */

import { useSyncExternalStore } from 'react'

export interface ErrorToast {
  id: number
  error: unknown
  /** Текст подтверждения; есть — это не ошибка. */
  info?: string
}

let current: ErrorToast | null = null
let nextId = 1
const listeners = new Set<() => void>()

function emit(): void {
  for (const listener of listeners) listener()
}

export function showErrorToast(error: unknown): void {
  current = { id: nextId++, error }
  emit()
}

export function showInfoToast(text: string): void {
  current = { id: nextId++, error: null, info: text }
  emit()
}

export function dismissToast(id?: number): void {
  if (!current || (id !== undefined && current.id !== id)) return
  current = null
  emit()
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function useErrorToast(): ErrorToast | null {
  return useSyncExternalStore(subscribe, () => current)
}
