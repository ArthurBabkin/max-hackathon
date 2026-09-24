/**
 * Текст элемента так, как его слышит скринридер: без частей с aria-hidden
 * (значок вуза и т. п.). Для порядка строк, где textContent склеил бы значок
 * с названием.
 */
export function spokenText(el: Element): string {
  const copy = el.cloneNode(true) as Element
  copy.querySelectorAll('[aria-hidden="true"]').forEach((node) => node.remove())
  return copy.textContent?.trim() ?? ''
}
