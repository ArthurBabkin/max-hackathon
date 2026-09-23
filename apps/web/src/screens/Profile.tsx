/** Профиль ученика — экран H1, функция F49. */

import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Input } from '@maxhub/max-ui'
import { GRADES, type Grade } from '@contract'
import { usePatchProfile, useProfile, useServerVersion, useUniversities } from '@/api/queries'
import { Icon } from '@/ui/Icon'
import { CardSkeletons, Chip, StateBlock } from '@/ui/primitives'
import { useVoice } from '@/voice/useVoice'

/** Коммит сборки фронта; вне CI — «dev». */
const WEB_VERSION = ((import.meta.env.VITE_APP_VERSION as string | undefined) || 'dev').slice(0, 7)

/** Предметы онбординга (ТЗ F7). Придут справочником с сервера — разметка та же. */
const SUBJECTS = [
  { code: 'inf', name: 'Информатика' },
  { code: 'math', name: 'Математика' },
  { code: 'phys', name: 'Физика' },
  { code: 'chem', name: 'Химия' },
  { code: 'bio', name: 'Биология' },
  { code: 'soc', name: 'Обществознание' },
]

export function ProfileScreen() {
  const t = useVoice()
  const navigate = useNavigate()
  const profile = useProfile()
  const universities = useUniversities('', 'all')
  const save = usePatchProfile()
  const serverVersion = useServerVersion()

  const [name, setName] = useState('')
  const [grade, setGrade] = useState<Grade>(9)
  const [subjects, setSubjects] = useState<string[]>([])
  const [selectedUniversities, setSelectedUniversities] = useState<string[]>([])
  const [warning, setWarning] = useState<string | null>(null)

  // Форма заполняется, когда профиль приехал, и дальше живёт сама: иначе
  // фоновый перезапрос затирал бы то, что пользователь уже поправил.
  useEffect(() => {
    if (!profile.data) return
    setName(profile.data.student_name)
    setGrade(profile.data.grade as Grade)
    setSubjects(profile.data.subjects.map((s) => s.code))
    setSelectedUniversities(profile.data.universities.map((u) => u.id))
  }, [profile.data])

  if (profile.isPending) {
    return (
      <div className="screen">
        <CardSkeletons count={3} />
      </div>
    )
  }

  if (profile.isError || !profile.data) {
    return (
      <div className="screen">
        <StateBlock icon="wifiOff" tone="error" title={t('state.errorTitle')} text={t('state.errorText')}>
          <Button stretched onClick={() => void profile.refetch()}>
            {t('state.errorRetry')}
          </Button>
        </StateBlock>
      </div>
    )
  }

  const data = profile.data

  /** Мультивыбор, в котором нельзя снять последний элемент (ТЗ F7, F9). */
  const toggle = (list: string[], value: string, emptyMessage: string): string[] => {
    if (!list.includes(value)) {
      setWarning(null)
      return [...list, value]
    }
    if (list.length <= 1) {
      setWarning(emptyMessage)
      return list
    }
    setWarning(null)
    return list.filter((item) => item !== value)
  }

  const submit = () => {
    save.mutate(
      {
        student_name: name.trim(),
        grade,
        subject_codes: subjects,
        university_ids: selectedUniversities,
      },
      { onSuccess: () => navigate('/match') },
    )
  }

  const nameInvalid = name.trim().length === 0 || name.trim().length > 40

  return (
    <div className="screen">
      <button type="button" className="link back-link" onClick={() => navigate('/')}>
        <Icon name="back" size={16} />
        {t('profile.back')}
      </button>

      <h1 className="family-title">{t('profile.title')}</h1>
      <p className="family-subtitle">
        {data.other_member_names.length > 0
          ? t('profile.sharedNoteWith', { names: data.other_member_names.join(', ') })
          : t('profile.sharedNote')}
      </p>

      <div className="field">
        <p className="field-label">{t('profile.nameLabel')}</p>
        <Input
          value={name}
          maxLength={40}
          aria-label={t('profile.nameLabel')}
          aria-invalid={nameInvalid}
          hint={nameInvalid ? 'Имя от 1 до 40 символов' : undefined}
          onChange={(event) => setName(event.target.value)}
        />
      </div>

      <div className="field">
        <p className="field-label">{t('profile.gradeLabel')}</p>
        <div className="grades">
          {GRADES.map((value) => (
            <button
              key={value}
              type="button"
              className={grade === value ? 'grade grade-on' : 'grade'}
              aria-pressed={grade === value}
              onClick={() => setGrade(value)}
            >
              {value}
            </button>
          ))}
        </div>
      </div>

      <div className="field">
        <p className="field-label">{t('profile.regionLabel')}</p>
        <p className="field-value">{data.region_name}</p>
      </div>

      <div className="field">
        <p className="field-label">
          <span>{t('profile.subjectsLabel')}</span>
          <span>{t('profile.selectedCount', { count: subjects.length })}</span>
        </p>
        <div className="wrap-chips">
          {SUBJECTS.map((subject) => (
            <Chip
              key={subject.code}
              active={subjects.includes(subject.code)}
              onClick={() =>
                setSubjects((list) => toggle(list, subject.code, t('profile.needSubject')))
              }
            >
              {subjects.includes(subject.code) ? '✓ ' : ''}
              {subject.name}
            </Chip>
          ))}
        </div>
      </div>

      <div className="field">
        <p className="field-label">{t('profile.goalLabel')}</p>
        <p className="field-value">{data.direction_name ?? t('home.goalEmpty')}</p>
      </div>

      <div className="field">
        <p className="field-label">
          <span>{t('profile.universitiesLabel')}</span>
          <span>{t('profile.selectedCount', { count: selectedUniversities.length })}</span>
        </p>
        <div className="wrap-chips">
          {(universities.data?.items ?? []).map((university) => (
            <Chip
              key={university.id}
              active={selectedUniversities.includes(university.id)}
              onClick={() =>
                setSelectedUniversities((list) =>
                  toggle(list, university.id, t('profile.needUniversity')),
                )
              }
            >
              {selectedUniversities.includes(university.id) ? '✓ ' : ''}
              {university.short_name}
            </Chip>
          ))}
        </div>
      </div>

      {warning ? <p className="field-warning">{warning}</p> : null}

      <Button
        stretched
        loading={save.isPending}
        disabled={nameInvalid}
        iconBefore={<Icon name="spark" size={15} />}
        onClick={submit}
      >
        {t('profile.saveCta')}
      </Button>

      {/* По версиям видно, какие коммиты сейчас в проде. */}
      <p className="app-version">
        Версия: приложение {WEB_VERSION} · сервер {serverVersion.data ?? (serverVersion.isError ? '—' : '…')}
      </p>
    </div>
  )
}
