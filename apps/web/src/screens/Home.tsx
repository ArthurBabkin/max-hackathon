/** Главная (экраны C1 и C2): цель, прогресс, следующий шаг, ближайшие сроки. */

import { useNavigate } from 'react-router-dom'
import { Button } from '@maxhub/max-ui'
import { useHome } from '@/api/queries'
import { daysLabel, daysLeft, formatToday, plural } from '@/lib/deadline'
import { Icon } from '@/ui/Icon'
import { CardSkeletons, Note, Section, SourceLine } from '@/ui/primitives'
import { TrackerRow } from '@/ui/TrackerRow'
import { useSheetStack } from '@/ui/sheets'
import { useRole, useVoice } from '@/voice/useVoice'
import { ErrorState } from '@/ui/ErrorState'

/** Кольцо прогресса регистраций в карточке цели. */
function ProgressRing({ percent }: { percent: number }) {
  const radius = 24
  const circumference = 2 * Math.PI * radius
  return (
    <svg width="62" height="62" viewBox="0 0 62 62" aria-hidden="true" className="ring">
      <circle cx="31" cy="31" r={radius} stroke="rgba(255,255,255,.25)" strokeWidth="7" fill="none" />
      <circle
        cx="31"
        cy="31"
        r={radius}
        stroke="#fff"
        strokeWidth="7"
        fill="none"
        strokeLinecap="round"
        strokeDasharray={circumference.toFixed(2)}
        strokeDashoffset={(circumference * (1 - percent / 100)).toFixed(2)}
        transform="rotate(-90 31 31)"
      />
      <text x="31" y="35.5" textAnchor="middle" fontSize="13" fontWeight="800" fill="#fff">
        {percent}%
      </text>
    </svg>
  )
}

/** «Олимпиады простыми словами» — блок для родителя (F47). */
function Explainer() {
  const t = useVoice()
  const navigate = useNavigate()
  return (
    <section className="block block-card">
      <h3 className="block-head">
        {t('explainer.title')}
        <span className="tag tag-fact">{t('tag.fact')}</span>
      </h3>
      <ul className="explainer">
        <li>{t('explainer.bvi')}</li>
        <li>{t('explainer.score100')}</li>
        <li>{t('explainer.confirm')}</li>
        <li>{t('explainer.oneVuz')}</li>
      </ul>
      <SourceLine title={t('explainer.source')} />
      <button type="button" className="link explainer-more" onClick={() => navigate('/faq')}>
        {t('faq.explainerMore')}
        <Icon name="chevron" size={14} />
      </button>
    </section>
  )
}

export function HomeScreen() {
  const t = useVoice()
  const role = useRole()
  const navigate = useNavigate()
  const sheets = useSheetStack()
  const home = useHome()

  const openOlympiad = (id: string) => sheets.open({ kind: 'oly', id })

  if (home.isPending) {
    return (
      <div className="screen">
        <CardSkeletons count={3} />
      </div>
    )
  }

  if (home.isError || !home.data) {
    return (
      <ErrorState error={home.error} onRetry={() => void home.refetch()} />
    )
  }

  const data = home.data
  const trajectory = data.trajectory
  const percent = data.tracker_count ? Math.round((data.registered_count / data.tracker_count) * 100) : 0
  const next = data.next_step
  const nextDays = daysLeft(next?.deadline_at)

  return (
    <div className="screen">
      <div className="hello">
        <div>
          <p className="hello-date">{formatToday()}</p>
          <h1 className="hello-title">{t('home.greeting')}</h1>
        </div>
        <button type="button" className="avatar" onClick={() => navigate('/profile')}>
          {trajectory.student_name.slice(0, 1)}
          <span className="sr-only">{t('home.openProfile')}</span>
        </button>
      </div>

      <section className="goal">
        <span className="goal-pattern" aria-hidden="true">
          <svg viewBox="0 0 96 64">
            <rect x="64" y="0" width="32" height="16" fill="#8EF0FB" opacity=".9" />
            <rect x="48" y="16" width="16" height="16" fill="#FF8DBE" />
            <rect x="80" y="16" width="16" height="16" fill="#fff" opacity=".18" />
            <rect x="64" y="32" width="16" height="16" fill="#fff" opacity=".12" />
          </svg>
        </span>
        <p className="goal-label">
          {t('home.goalLabel')}, {trajectory.grade} класс
        </p>
        <h2 className="goal-title">{trajectory.directions.map((d) => d.name).join(', ') || t('home.goalEmpty')}</h2>
        <div className="goal-stats">
          <ProgressRing percent={percent} />
          <ul>
            <li>
              <b>{data.tracker_count}</b>{' '}
              {plural(data.tracker_count, 'олимпиада', 'олимпиады', 'олимпиад')} в трекере
            </li>
            <li>
              <b>
                {data.registered_count} из {data.tracker_count}
              </b>{' '}
              {plural(data.registered_count, 'регистрация', 'регистрации', 'регистраций')}
            </li>
            <li>
              <b>{data.universities_count}</b>{' '}
              {plural(data.universities_count, 'вуз', 'вуза', 'вузов')} в цели
            </li>
          </ul>
        </div>
      </section>

      {next ? (
        <div className="next-step">
          <span className="next-step-icon">
            <Icon name="clock" size={17} />
          </span>
          <div className="next-step-main">
            <p className="next-step-label">
              {t('home.nextStepLabel')}
              {nextDays !== null && nextDays >= 0 ? `, осталось ${daysLabel(nextDays)}` : ''}
            </p>
            <p className="next-step-title">
              {next.stage_title} {next.stage_kind === 'registration' ? 'на' : ''} «{next.olympiad_name}»
            </p>
          </div>
          <Button size="small" onClick={() => openOlympiad(next.olympiad_profile_id)}>
            {t('home.nextStepOpen')}
          </Button>
        </div>
      ) : data.tracker_count > 0 ? (
        <div className="next-step next-step-done">
          <span className="next-step-icon">
            <Icon name="check" size={17} />
          </span>
          <div className="next-step-main">
            <p className="next-step-label">{t('home.allDoneTitle')}</p>
            <p className="next-step-title">{t('home.allDoneText')}</p>
          </div>
        </div>
      ) : null}

      <Section
        title={t('home.upcomingTitle')}
        action={
          <button type="button" className="link" onClick={() => navigate('/tracker')}>
            {t('home.upcomingAll')}
          </button>
        }
      />

      {data.upcoming.length > 0 ? (
        <div className="list" data-tour="upcoming">
          {data.upcoming.map((item) => (
            <TrackerRow key={item.id} item={item} onOpen={openOlympiad} />
          ))}
        </div>
      ) : (
        <div className="list list-empty">
          <p className="state-text">{t('home.emptyTracker')}</p>
          <Button size="small" onClick={() => navigate('/match')}>
            {t('home.emptyTrackerCta')}
          </Button>
        </div>
      )}

      {role === 'parent' ? <Explainer /> : null}

      <Section title={t('home.quickTitle')} />
      <div className="quick">
        <button type="button" onClick={() => navigate('/match')}>
          <span className="quick-icon">
            <Icon name="target" size={17} />
          </span>
          {t('home.quickMatch')}
        </button>
        <button type="button" onClick={() => navigate('/catalog')}>
          <span className="quick-icon">
            <Icon name="book" size={17} />
          </span>
          {t('home.quickCatalog')}
        </button>
        <button type="button" onClick={() => sheets.open({ kind: 'ai', id: '' })}>
          <span className="quick-icon">
            <Icon name="spark" size={17} />
          </span>
          {t('home.quickAi')}
        </button>
        <button type="button" onClick={() => navigate('/family')}>
          <span className="quick-icon">
            <Icon name="users" size={17} />
          </span>
          {t('home.quickFamily')}
        </button>
      </div>

      <button type="button" className="faq-entry" onClick={() => navigate('/faq')}>
        <span className="quick-icon">
          <Icon name="doc" size={17} />
        </span>
        <span className="faq-entry-text">
          <b>{t('faq.entryTitle')}</b>
          <span>{t('faq.entryText')}</span>
        </span>
        <Icon name="chevron" size={16} />
      </button>

      <Note>{t('home.disclaimer')}</Note>
    </div>
  )
}
