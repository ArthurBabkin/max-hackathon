import { useEffect, useState } from 'react'

/**
 * Отложенное значение. Поиск в каталоге по ТЗ F27 срабатывает по мере ввода
 * с задержкой 250 мс — иначе на каждый символ уходил бы запрос.
 */
export function useDebounced<T>(value: T, delay = 250): T {
  const [delayed, setDelayed] = useState(value)

  useEffect(() => {
    const timer = setTimeout(() => setDelayed(value), delay)
    return () => clearTimeout(timer)
  }, [value, delay])

  return delayed
}
