/**
 * Туториал показывается один раз на устройстве. Отметка — в localStorage:
 * это удобство, а не данные, поэтому сервер о ней не знает. В приватном
 * режиме или вебвью без хранилища доступ бросает исключение — тогда туториал
 * просто покажется ещё раз.
 */

// v3 — переходы между шагами нажимает сам пользователь: видевшим прежний
// обзор, где туториал нажимал за него, покажем новый.
const KEY = 'traektoria.tutorial.v3'

export function tutorialSeen(): boolean {
  try {
    return localStorage.getItem(KEY) === 'seen'
  } catch {
    return false
  }
}

export function markTutorialSeen(): void {
  try {
    localStorage.setItem(KEY, 'seen')
  } catch {
    // Хранилища нет — покажем ещё раз, ничего страшного.
  }
}

/**
 * Показывать ли туториал при запуске. Новичку — если он открыл приложение
 * сам или общей кнопкой бота «Открыть» (параметр `home`). Пришедшего из бота
 * в конкретный раздел — трекер, семью, карточку олимпиады — не перебиваем.
 */
export function shouldShowTutorial(startParam: string | null | undefined): boolean {
  return !tutorialSeen() && (!startParam || startParam === 'home')
}
