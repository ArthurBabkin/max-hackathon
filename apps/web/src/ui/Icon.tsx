/**
 * Иконки. Набор перенесён из прототипа (объект `P` в docs/prototype/index.html)
 * один в один, чтобы экраны совпадали с макетами.
 *
 * Библиотеку иконок не берём: здесь их два десятка, а любая библиотека тянет
 * в бандл тысячи — при бюджете 2,5 с на 4G это не окупается.
 */

import type { SVGProps } from 'react'

export const ICON_PATHS = {
  back: 'M15 18l-6-6 6-6',
  chevron: 'M9 6l6 6-6 6',
  down: 'M6 9l6 6 6-6',
  close: 'M18 6L6 18M6 6l12 12',
  dots: 'M5 12h.01M12 12h.01M19 12h.01',
  search: 'M11 4a7 7 0 100 14a7 7 0 100-14zM20 20l-4-4',
  check: 'M5 12l5 5L20 7',
  plus: 'M12 5v14M5 12h14',
  home: 'M3 11l9-7 9 7v9a1 1 0 01-1 1h-5v-6H9v6H4a1 1 0 01-1-1z',
  target: 'M12 3a9 9 0 100 18a9 9 0 100-18zM12 8a4 4 0 100 8a4 4 0 100-8zM12 11.5v1',
  calendar: 'M4 6h16v14H4zM4 10h16M8 3v4M16 3v4',
  list: 'M9 6h11M9 12h11M9 18h11M4 6h.01M4 12h.01M4 18h.01',
  users: 'M9 11a4 4 0 100-8a4 4 0 000 8zM2 21v-1a6 6 0 0112 0v1M16 3.5a4 4 0 010 7.5M22 21v-1a6 6 0 00-4-5.6',
  spark: 'M12 3l1.9 5.1L19 10l-5.1 1.9L12 17l-1.9-5.1L5 10l5.1-1.9zM19 16l.8 2.2L22 19l-2.2.8L19 22l-.8-2.2L16 19l2.2-.8z',
  external: 'M14 4h6v6M20 4l-9 9M19 14v5a1 1 0 01-1 1H5a1 1 0 01-1-1V6a1 1 0 011-1h5',
  doc: 'M7 3h7l5 5v12a1 1 0 01-1 1H7a1 1 0 01-1-1V4a1 1 0 011-1zM14 3v5h5',
  bell: 'M6 8a6 6 0 0112 0c0 7 3 9 3 9H3s3-2 3-9M10.3 21a1.94 1.94 0 003.4 0',
  eyeOff: 'M3 3l18 18M10.6 5.1A10.5 10.5 0 0112 5c6.5 0 10 7 10 7a17 17 0 01-3.2 4.2M6.6 6.6A17 17 0 002 12s3.5 7 10 7a10 10 0 005.4-1.6M9.9 9.9a3 3 0 004.2 4.2',
  clock: 'M12 3a9 9 0 100 18a9 9 0 100-18zM12 7v5l3 2',
  shield: 'M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z',
  link: 'M10 14a5 5 0 007 0l3-3a5 5 0 00-7-7l-1 1M14 10a5 5 0 00-7 0l-3 3a5 5 0 007 7l1-1',
  send: 'M4 12l16-8-6 16-2-7z',
  pencil: 'M4 20h4L19 9a2.8 2.8 0 00-4-4L4 16zM13.5 6.5l4 4',
  refresh: 'M20 11a8 8 0 10-2.3 5.7M20 4v7h-7',
  alert: 'M10.3 3.9L1.8 18a2 2 0 001.7 3h17a2 2 0 001.7-3L13.7 3.9a2 2 0 00-3.4 0zM12 9v4M12 17h.01',
  wifiOff: 'M3 3l18 18M8.5 16.5a5 5 0 017 0M5 12.9a10 10 0 015.2-2.8M19 12.9a10 10 0 00-2.4-1.7M2 8.8a15 15 0 014.2-2.7M22 8.8A15 15 0 0010.5 5.1M12 20h.01',
  book: 'M4 5a2 2 0 012-2h13v16H6a2 2 0 00-2 2zM4 5v16M8 7h7',
  pin: 'M12 21s-7-6.5-7-12a7 7 0 0114 0c0 5.5-7 12-7 12zM12 7a2 2 0 100 4a2 2 0 000-4z',
  out: 'M15 4h4a1 1 0 011 1v14a1 1 0 01-1 1h-4M10 17l5-5-5-5M15 12H4',
  inbox: 'M4 13l3-8h10l3 8v6H4zM4 13h5l1 2h4l1-2h5',
  award: 'M12 3a6 6 0 100 12a6 6 0 100-12zM8.5 14L7 21l5-3 5 3-1.5-7',
  trash: 'M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3',
  up: 'M6 15l6-6 6 6',
} as const

export type IconName = keyof typeof ICON_PATHS

export interface IconProps extends Omit<SVGProps<SVGSVGElement>, 'name'> {
  name: IconName
  size?: number
  strokeWidth?: number
  /** Подпись для скринридера. Без неё иконка считается декоративной. */
  title?: string
}

export function Icon({ name, size = 18, strokeWidth, title, ...rest }: IconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill="none"
      stroke="currentColor"
      // У многоточия штрихи вырождаются в точки — им нужна своя толщина.
      strokeWidth={strokeWidth ?? (name === 'dots' ? 3.2 : 2)}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden={title ? undefined : true}
      role={title ? 'img' : undefined}
      {...rest}
    >
      {title ? <title>{title}</title> : null}
      <path d={ICON_PATHS[name]} />
    </svg>
  )
}

/** Логотип «Траектории»: четыре ступени вверх — та же метафора, что в названии. */
export function Logo({ size = 40 }: { size?: number }) {
  const id = `logo-gradient-${size}`
  return (
    <svg viewBox="0 0 48 48" width={size} height={size} aria-hidden="true">
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#7B3BFF" />
          <stop offset="1" stopColor="#1A6DFF" />
        </linearGradient>
      </defs>
      <rect width="48" height="48" rx="14" fill={`url(#${id})`} />
      <rect x="9" y="31" width="8" height="8" rx="1.5" fill="#fff" />
      <rect x="17" y="23" width="8" height="8" rx="1.5" fill="#fff" opacity=".85" />
      <rect x="25" y="15" width="8" height="8" rx="1.5" fill="#8EF0FB" />
      <rect x="33" y="7" width="8" height="8" rx="1.5" fill="#FF8DBE" />
    </svg>
  )
}
