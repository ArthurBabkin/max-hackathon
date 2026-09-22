/**
 * Нижнее меню (ТЗ §7.1). В MAX UI таббара нет, поэтому свой.
 *
 * Счётчик на «Трекере» — незарегистрированные плюс предложения, ждущие
 * ответа ученика (F31). У родителя предложения в счёт не идут: ответить
 * на них он не может, и цифра была бы обещанием несуществующего действия.
 */

import { NavLink } from 'react-router-dom'
import { Icon, type IconName } from './Icon'
import { useVoice } from '@/voice/useVoice'
import type { TextKey } from '@/voice/texts'

interface Tab {
  to: string
  labelKey: TextKey
  icon: IconName
}

const TABS: Tab[] = [
  { to: '/', labelKey: 'nav.home', icon: 'home' },
  { to: '/match', labelKey: 'nav.match', icon: 'target' },
  { to: '/catalog', labelKey: 'nav.catalog', icon: 'book' },
  { to: '/tracker', labelKey: 'nav.tracker', icon: 'calendar' },
  { to: '/family', labelKey: 'nav.family', icon: 'users' },
]

export function Tabbar({ trackerCount }: { trackerCount: number }) {
  const t = useVoice()

  return (
    <nav className="tabbar" aria-label="Разделы приложения">
      {TABS.map((tab) => (
        <NavLink
          key={tab.to}
          to={tab.to}
          end={tab.to === '/'}
          className={({ isActive }) => `tab${isActive ? ' tab-on' : ''}`}
        >
          <span className="tab-icon">
            <Icon name={tab.icon} size={21} />
            {tab.to === '/tracker' && trackerCount > 0 ? (
              <span className="tab-badge" aria-hidden="true">
                {trackerCount}
              </span>
            ) : null}
          </span>
          <span className="tab-label">{t(tab.labelKey)}</span>
          {tab.to === '/tracker' && trackerCount > 0 ? (
            <span className="sr-only">, требуют внимания: {trackerCount}</span>
          ) : null}
        </NavLink>
      ))}
    </nav>
  )
}
