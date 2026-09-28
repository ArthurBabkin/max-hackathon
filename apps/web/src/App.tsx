/**
 * Оболочка мини-приложения: шапка, вкладки, стек нижних листов и системная
 * кнопка «Назад» MAX.
 */

import { Suspense, lazy, useEffect, useState } from 'react'
import { Button, IconButton } from '@maxhub/max-ui'
import { Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { useSession, useTracker } from './api/queries'
import { BOT_URL, getWebApp, openMaxLink } from './bridge'
import { trackerBadgeCount } from './lib/derive'
import { useStartRoute } from './lib/startParam'
import { shouldShowTutorial } from './lib/tutorial'
import { errorKind } from './api/errors'
import { ErrorState } from './ui/ErrorState'
import { Icon, Logo } from './ui/Icon'
import { Tabbar } from './ui/Tabbar'
import { Toaster } from './ui/Toaster'
import { StateBlock } from './ui/primitives'
import { Tutorial } from './ui/Tutorial'
import { useSheetStack } from './ui/sheets'
import { text } from './voice/texts'
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
      <p className="splash-title">{text('app.title', 'kid')}</p>
      <p className="splash-text">{text('app.splashCaption', 'kid')}</p>
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

  // Туториал — новичку (правило в lib/tutorial). ?tutorial=1 — пройти заново.
  const [tutorial, setTutorial] = useState(() => shouldShowTutorial(getWebApp().initDataUnsafe.start_param))
  const replayTutorial = new URLSearchParams(location.search).has('tutorial')
  useEffect(() => {
    if (!replayTutorial) return
    setTutorial(true)
    navigate(location.pathname, { replace: true })
  }, [replayTutorial, location.pathname, navigate])

  const hasSheet = sheets.stack.length > 0
  // Раздел из кнопки бота: «Трекер», «Семья», карточка олимпиады.
  useStartRoute(session.isSuccess)

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
    // 404 на POST /session — траектории ещё нет: анкета в чате бота не
    // пройдена. Это не сбой, а следующий шаг, поэтому без красной иконки и
    // с кнопкой в чат; вернулся из чата — вход проверяется сам (useSession).
    const noTrajectory = errorKind(session.error) === 'notFound'
    return (
      <div className="app app-state">
        {noTrajectory ? (
          <StateBlock icon="target" title={t('state.noTrajectoryTitle')} text={t('state.noTrajectoryText')}>
            <Button stretched iconBefore={<Icon name="send" size={16} />} onClick={() => openMaxLink(BOT_URL)}>
              {t('state.noTrajectoryBot')}
            </Button>
            <Button
              stretched
              variant="secondary"
              iconBefore={<Icon name="refresh" size={16} />}
              onClick={() => void session.refetch()}
            >
              {t('state.noTrajectoryRetry')}
            </Button>
          </StateBlock>
        ) : (
          <ErrorState error={session.error} onRetry={() => void session.refetch()} />
        )}
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
          <span className="sr-only">{t('app.toHome')}</span>
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
            {/* MAX открывает приложение с данными запуска в hash
                (`#WebAppData=…`), и хеш-роутер видит в них путь. Мост MAX к
                этому моменту их уже прочитал — меняем адрес на главную, иначе
                без кнопки «Спросить» и с лишней «Назад» открылась бы главная. */}
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Suspense>
      </main>

      {showFab ? (
        <button type="button" className="fab" data-tour="ai" onClick={() => sheets.open({ kind: 'ai', id: '' })}>
          <Icon name="spark" size={15} />
          {t('ai.openFab')}
        </button>
      ) : null}

      <Tabbar trackerCount={badge} />

      <SheetHost sheets={sheets} />

      <Toaster />

      {tutorial ? <Tutorial onDone={() => setTutorial(false)} /> : null}
    </div>
  )
}
