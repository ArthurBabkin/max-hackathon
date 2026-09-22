/**
 * Склонение русских имён в родительный и дательный падеж.
 *
 * MAX отдаёт только именительный падеж (`initDataUnsafe.user.first_name`),
 * а ТЗ F3 требует от родительского голоса «Цель Артёма» и «Предложить Артёму».
 * Поэтому склоняем сами.
 *
 * Пол пользователя платформа не отдаёт, но для имён он почти и не нужен:
 * образец склонения задаёт окончание, а не пол. Никита и Ольга склоняются
 * одинаково, Илья и Катя — тоже.
 *
 * Берём только эти два падежа: по всему словарю нужны именно они.
 */

/** Имена, которые не подчиняются правилам. */
const EXCEPTIONS: Record<string, { gen: string; dat: string }> = {
  // Беглая гласная в основе.
  павел: { gen: 'Павла', dat: 'Павлу' },
  лев: { gen: 'Льва', dat: 'Льву' },
  пётр: { gen: 'Петра', dat: 'Петру' },
  петр: { gen: 'Петра', dat: 'Петру' },
  // Женское на -ь склоняется по третьему склонению, а не как Игорь.
  любовь: { gen: 'Любови', dat: 'Любови' },
}

/** После этих согласных вместо «ы» пишется «и». */
const HUSHING = 'гкхжчшщ'

/** Окончания, после которых имя не склоняется: Отто, Мэри, Нино. */
const INDECLINABLE_ENDINGS = 'оеэуюиы'

const CYRILLIC = /[а-яёА-ЯЁ]/

type Case = 'gen' | 'dat'

function decline(name: string, grammaticalCase: Case): string {
  const trimmed = name.trim()
  if (!trimmed) return trimmed

  // Латиница и всё нерусское не склоняем — правила к ним неприменимы.
  if (!CYRILLIC.test(trimmed)) return trimmed

  const exception = EXCEPTIONS[trimmed.toLowerCase()]
  if (exception) return exception[grammaticalCase]

  const last = trimmed.slice(-1).toLowerCase()
  const stem = trimmed.slice(0, -1)
  const beforeLast = trimmed.slice(-2, -1).toLowerCase()

  // Мария, Ксения — в обоих падежах «-ии».
  if (trimmed.length > 2 && trimmed.slice(-2).toLowerCase() === 'ия') {
    return `${trimmed.slice(0, -1)}и`
  }

  if (last === 'а') {
    if (grammaticalCase === 'dat') return `${stem}е`
    return `${stem}${HUSHING.includes(beforeLast) ? 'и' : 'ы'}`
  }

  if (last === 'я') {
    return `${stem}${grammaticalCase === 'gen' ? 'и' : 'е'}`
  }

  if (last === 'й' || last === 'ь') {
    return `${stem}${grammaticalCase === 'gen' ? 'я' : 'ю'}`
  }

  if (INDECLINABLE_ENDINGS.includes(last)) return trimmed

  // Осталась согласная: Артём, Иван, Тимур.
  return `${trimmed}${grammaticalCase === 'gen' ? 'а' : 'у'}`
}

/** Кого? чего? — «Цель Артёма», «В вузах Ольги». */
export function genitive(name: string): string {
  return decline(name, 'gen')
}

/** Кому? чему? — «Предложить Артёму». */
export function dative(name: string): string {
  return decline(name, 'dat')
}
