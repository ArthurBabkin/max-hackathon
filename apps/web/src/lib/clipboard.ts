/**
 * Копирование в буфер обмена. Clipboard API есть не везде: во встроенных
 * браузерах мессенджеров он бывает выключен или отказывает без разрешения.
 * Тогда — старый способ через выделение текста. false — не вышло никак,
 * ссылка остаётся на экране, её можно выделить руками.
 */
export async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
      return true
    }
  } catch {
    // Пробуем запасной способ ниже.
  }

  const area = document.createElement('textarea')
  area.value = text
  area.setAttribute('readonly', '')
  area.style.cssText = 'position:fixed;top:0;left:0;opacity:0'
  document.body.append(area)
  area.select()
  try {
    return document.execCommand('copy')
  } catch {
    return false
  } finally {
    area.remove()
  }
}
