/**
 * Оболочка мини-приложения: шапка, вкладки, стек нижних листов и системная
 * кнопка «Назад» MAX.
 */

import { Suspense, lazy, useEffect, useState } from 'react'
import { Button, IconButton } from '@maxhub/max-ui'
import { Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { useColorScheme } from '@maxhub/max-ui'
import { useSession, useTracker } from './api/queries'
import { getWebApp } from './bridge'
import { trackerBadgeCount } from './lib/derive'
import { useStartRoute } from './lib/startParam'
import { tutorialSeen } from './lib/tutorial'
import { Icon, Logo } from './ui/Icon'
import { Tabbar } from './ui/Tabbar'
import { StateBlock } from './ui/primitives'
import { Tutorial } from './ui/Tutorial'
import { useSheetStack } from './ui/sheets'
import { useVoice } from './voice/useVoice'
import { HomeScreen } from './screens/Home'
import { MatchScreen } from './screens/Match'
import { SheetHost } from './screens/SheetHost'

// Каталог, семья и профиль в первый экран не попадают — грузим отдельным
// чанком, чтобы уложиться в бюджет 2,5 с на 4G (ТЗ §12).
const CatalogScreen = lazy(() => import('./screens/Catalog').then((m) => ({ default: m.CatalogScreen })))
const TrackerScreen = lazy(() => import('./screens/Tracker').then((m) => ({ default: m.TrackerScreen })))
const FamilyScreen = lazy(() => import('./screens/Family').then((m) => ({ default: m.FamilyScreen })))
const ProfileScreen = lazy(() => import('./screens/Profile').then((m) => ({ default: m.ProfileScreen })))
const FaqScreen = lazy(() => import('./screens/Faq').then((m) => ({ default: m.FaqScreen })))

/** Вкладки, на которых показывается кнопка «Спросить» (ТЗ §7.1). */
const FAB_ROUTES = ['/', '/match', '/catalog']

function Splash() {
  return (
    <div className="splash">
      <Logo size={72} />
      <p className="splash-title">Траектория</p>
      <p className="splash-text">Собираем подборку…</p>
      <span className="splash-progress" aria-hidden="true">
        <i />
      </span>
    </div>
  )
}

export function App() {
  const session = useSession()
  const { data: tracker } = useTracker(session.isSuccess)
  const sheets = useSheetStack()
  const location = useLocation()
  const navigate = useNavigate()
  const t = useVoice()
  const colorScheme = useColorScheme()

  // Туториал — новичку, который открыл приложение сам. Пришедшего по кнопке
  // бота в конкретный раздел не перебиваем. ?tutorial=1 — пройти заново.
  const [tutorial, setTutorial] = useState(() => !tutorialSeen() && !getWebApp().initDataUnsafe.start_param)
  const replayTutorial = new URLSearchParams(location.search).has('tutorial')
  useEffect(() => {
    if (!replayTutorial) return
    setTutorial(true)
    navigate(location.pathname, { replace: true })
  }, [replayTutorial, location.pathname, navigate])

  const hasSheet = sheets.stack.length > 0
  // Раздел из кнопки бота: «Трекер», «Семья», карточка олимпиады.
  useStartRoute(session.isSuccess)

  // Токены темы и компоненты MAX UI должны переключаться вместе. Провайдер —
  // единственный источник правды: он же учтёт настройку, пришедшую из MAX,
  // а не только системную.
  useEffect(() => {
    document.documentElement.dataset.theme = colorScheme
  }, [colorScheme])
  const isRoot = location.pathname === '/'

  // Системная кнопка «Назад» MAX ведёт по истории браузера: стек листов и
  // вкладки живут в адресе, поэтому одно нажатие снимает ровно один шаг.
  useEffect(() => {
    const backButton = getWebApp().BackButton
    const handler = () => navigate(-1)

    backButton.onClick(handler)
    if (hasSheet || !isRoot) backButton.show()
    else backButton.hide()

    // Снимать обработчик обязательно: в StrictMode эффекты вызываются дважды,
    // и без очистки «Назад» отматывал бы историю на два шага.
    return () => backButton.offClick(handler)
  }, [hasSheet, isRoot, navigate])

  if (session.isPending) return <Splash />

  if (session.isError) {
    return (
      <div className="app">
        <StateBlock icon="wifiOff" tone="error" title={t('state.errorTitle')} text={t('state.errorText')}>
          <Button stretched iconBefore={<Icon name="refresh" size={16} />} onClick={() => void session.refetch()}>
            {t('state.errorRetry')}
          </Button>
        </StateBlock>
      </div>
    )
  }

  const showFab = FAB_ROUTES.includes(location.pathname) && !hasSheet
  const badge = trackerBadgeCount(tracker, session.data?.member.role ?? 'kid')

  return (
    <div className="app">
      <header className="app-top">
        <IconButton variant="ghost" size="small" onClick={() => navigate('/')}>
          <Logo size={26} />
          <span className="sr-only">На главную</span>
        </IconButton>
        <div className="app-title">
          <b>{t('app.title')}</b>
          <span>{t('app.viewerSubtitle')}</span>
        </div>
        <IconButton variant="ghost" size="small" onClick={() => navigate('/profile')}>
          <Icon name="dots" size={16} title={t('home.openProfile')} />
        </IconButton>
      </header>

      <main className={`app-body${showFab ? ' app-body-fab' : ''}`}>
        <Suspense fallback={<div className="screen-loading">{t('state.loading')}</div>}>
          <Routes>
            <Route path="/" element={<HomeScreen />} />
            <Route path="/match" element={<MatchScreen />} />
            <Route path="/catalog" element={<CatalogScreen />} />
            <Route path="/tracker" element={<TrackerScreen />} />
            <Route path="/family" element={<FamilyScreen />} />
            <Route path="/profile" element={<ProfileScreen />} />
            <Route path="/faq" element={<FaqScreen />} />
            <Route path="*" element={<HomeScreen />} />
          </Routes>
        </Suspense>
      </main>

      {showFab ? (
        <button type="button" className="fab" onClick={() => sheets.open({ kind: 'ai', id: '' })}>
          <Icon name="spark" size={15} />
          {t('ai.openFab')}
        </button>
      ) : null}

      <Tabbar trackerCount={badge} />

      <SheetHost sheets={sheets} />

      {tutorial ? <Tutorial onDone={() => setTutorial(false)} /> : null}
    </div>
  )
}
