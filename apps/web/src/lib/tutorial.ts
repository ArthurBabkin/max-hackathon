/**
 * Туториал показывается один раз на устройстве. Отметка — в localStorage:
 * это удобство, а не данные, поэтому сервер о ней не знает. В приватном
 * режиме или вебвью без хранилища доступ бросает исключение — тогда туториал
 * просто покажется ещё раз.
 */

const KEY = 'traektoria.tutorial.v1'

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
