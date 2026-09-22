/**
 * Голос интерфейса: «ты» ученику, «вы» родителю (ТЗ F3).
 *
 * Имя ученика приходит из MAX в именительном падеже, а родителю нужно
 * «Цель Артёма» и «Предложить Артёму», поэтому падежи считаются здесь один
 * раз и подставляются во все ключи сами. Вызывающему коду остаётся передать
 * только то, что знает он: имя третьего лица, число, дату.
 */

import { useMemo } from 'react'
import { useSession } from '@/api/queries'
import { dative, genitive } from '@/lib/declension'
import { text, type TextKey, type TextVars } from './texts'

export type Translate = (key: TextKey, vars?: TextVars) => string

export function useVoice(): Translate {
  const { data: session } = useSession()

  return useMemo(() => {
    const role = session?.member.role ?? 'kid'
    const student = session?.trajectory.student_name ?? ''
    const base: TextVars = {
      student,
      student_gen: genitive(student),
      student_dat: dative(student),
      me: session?.user.first_name ?? '',
    }
    return (key, vars) => text(key, role, vars ? { ...base, ...vars } : base)
  }, [session])
}

/** Роль смотрящего — когда нужен не текст, а ветка в разметке. */
export function useRole(): 'kid' | 'parent' {
  return useSession().data?.member.role ?? 'kid'
}
