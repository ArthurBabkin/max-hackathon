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
import { pluralForm } from '@/lib/deadline'
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

/**
 * Текст со словом, склонённым по числу: ключи `<base>.one`, `.few`, `.many`
 * («1 олимпиада», «2 олимпиады», «5 олимпиад»). Число подставляется как `{count}`.
 */
export function countText(t: Translate, base: string, n: number, vars?: TextVars): string {
  return t(`${base}.${pluralForm(n)}` as TextKey, { count: n, ...vars })
}

/** Роль смотрящего — когда нужен не текст, а ветка в разметке. */
export function useRole(): 'kid' | 'parent' {
  return useSession().data?.member.role ?? 'kid'
}
