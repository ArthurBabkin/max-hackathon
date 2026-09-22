/**
 * Тексты на «ты» и на «вы» (ТЗ F3).
 *
 * Словарь лежит в packages/shared/texts — его читает и Go-бот, поэтому копии
 * внутри apps/web нет. Типы выводятся прямо из JSON: `keyof typeof raw` даёт
 * литеральный union всех ключей, так что опечатка в `t('home.greetng')` —
 * ошибка компиляции, и генератор для этого не нужен.
 */

import raw from '@texts'
import type { Role } from '@contract'

type Dictionary = Record<string, { kid: string; parent: string }>

export type TextKey = keyof typeof raw

/** Значения подстановок. Числа приводятся к строке сами. */
export type TextVars = Record<string, string | number>

/**
 * Разрешённые плейсхолдеры. Список проверяется тестом по всему словарю:
 * новый `{плейсхолдер}` без записи здесь уронит сборку, а не всплывёт
 * на экране у пользователя.
 */
export const KNOWN_PLACEHOLDERS = [
  'student', // имя ученика, именительный падеж
  'student_gen', // родительный: «Цель Артёма», «В вузах Ольги»
  'student_dat', // дательный: «Предложить Артёму»
  'me', // имя того, кто смотрит
  'name', // имя третьего лица: кто отметил, кто предложил, кого удалили
  'creator', // имя создателя траектории
  'names', // перечисление имён через запятую
  'count',
  'date',
  'month',
  'direction',
  'grade',
  'region',
  'subject',
  'note',
  'stage',
  'title',
] as const

/**
 * Подстановка `{ключ}`. Значения нет — плейсхолдер остаётся видимым:
 * пустое место в строке легко пропустить при вычитке, а `{student}` на экране
 * не пропустишь.
 */
export function interpolate(template: string, vars: TextVars): string {
  return template.replace(/\{(\w+)\}/g, (match, name: string) =>
    name in vars ? String(vars[name]) : match,
  )
}

export function text(key: TextKey, role: Role, vars: TextVars = {}): string {
  const entry = (raw as Dictionary)[key as string]
  if (!entry) {
    if (import.meta.env.DEV) console.warn(`[voice] нет ключа: ${String(key)}`)
    return String(key)
  }
  return interpolate(role === 'parent' ? entry.parent : entry.kid, vars)
}
