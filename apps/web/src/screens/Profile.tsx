/** Профиль ученика — экран H1, функции F49 и F61 (тема оформления). */

import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Input } from '@maxhub/max-ui'
import { GRADES, type Grade, type ProfilePatch } from '@contract'
import { districts, regions } from '@regions'
import { useDirections, usePatchProfile, useProfile, useServerVersion, useUniversities } from '@/api/queries'
import { Icon } from '@/ui/Icon'
import { CardSkeletons, Chip } from '@/ui/primitives'
import { ThemeSetting } from '@/ui/ThemeSetting'
import { readThemeChoice, setThemeChoice, type ThemeChoice } from '@/ui/theme'
import { useVoice } from '@/voice/useVoice'
import { ErrorState } from '@/ui/ErrorState'

/** Коммит сборки фронта; вне CI — «dev». */
const WEB_VERSION = ((import.meta.env.VITE_APP_VERSION as string | undefined) || 'dev').slice(0, 7)

/** Предметы онбординга (ТЗ F7). Придут справочником с сервера — разметка та же. */
/** Опыт в олимпиадах (онбординг v2, SPEC 6): от него зависит вес уровня олимпиады. */
const EXPERIENCES = ['none', 'school', 'region'] as const
type Experience = (typeof EXPERIENCES)[number]

type PlacePatch = NonNullable<ProfilePatch['places']>[number]

/** Одно место «Где учиться»: регион целиком (city = null) или город в нём. */
const samePlace = (a: PlacePatch, b: PlacePatch) =>
  a.region_code === b.region_code && (a.city ?? null) === (b.city ?? null)

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
  const directions = useDirections()
  const save = usePatchProfile()
  const serverVersion = useServerVersion()

  const [name, setName] = useState('')
  const [grade, setGrade] = useState<Grade>(9)
  const [region, setRegion] = useState('')
  const [selectedDirections, setSelectedDirections] = useState<string[]>([])
  /** Где учиться, в порядке выбора; пусто — не важно. */
  const [places, setPlaces] = useState<PlacePatch[]>([])
  const [experience, setExperience] = useState<Experience | null>(null)
  const [subjects, setSubjects] = useState<string[]>([])
  const [selectedUniversities, setSelectedUniversities] = useState<string[]>([])
  const [warning, setWarning] = useState<string | null>(null)
  const [theme, setTheme] = useState<ThemeChoice>(readThemeChoice)

  // Форма заполняется, когда профиль приехал, и дальше живёт сама: иначе
  // фоновый перезапрос затирал бы то, что пользователь уже поправил.
  useEffect(() => {
    if (!profile.data) return
    setName(profile.data.student_name)
    setGrade(profile.data.grade as Grade)
    setRegion(profile.data.region_code)
    setSelectedDirections(profile.data.directions.map((d) => d.id))
    setPlaces(profile.data.places.map((p) => ({ region_code: p.region_code, city: p.city })))
    setExperience(profile.data.experience ?? null)
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
        <ErrorState error={profile.error} onRetry={() => void profile.refetch()} />
      </div>
    )
  }

  const data = profile.data

  /** Мультивыбор, в котором нельзя снять последний элемент (ТЗ F7). */
  const toggleRequired = (list: string[], value: string, emptyMessage: string): string[] => {
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

  /** Мультивыбор, который может остаться пустым: направления и вузы (F8, F9). */
  const toggle = (list: string[], value: string): string[] =>
    list.includes(value) ? list.filter((item) => item !== value) : [...list, value]

  const submit = () => {
    // Тема живёт на устройстве и сервера не ждёт: применяется сразу.
    setThemeChoice(theme)
    save.mutate(
      {
        student_name: name.trim(),
        grade,
        region_code: region,
        direction_ids: selectedDirections,
        places,
        ...(experience ? { experience } : {}),
        subject_codes: subjects,
        university_ids: selectedUniversities,
      },
      { onSuccess: () => navigate('/match') },
    )
  }

  /** Подпись места: город или название региона. */
  const placeName = (place: PlacePatch) =>
    place.city ?? regions.find((r) => r.code === place.region_code)?.name ?? place.region_code

  const addRegion = (code: string) => {
    if (!code) return
    const place = { region_code: code, city: null }
    setPlaces((list) => (list.some((p) => samePlace(p, place)) ? list : [...list, place]))
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

      {/* Нативный список: 89 регионов по округам, на телефоне — системный пикер.
          «Не указан» — «Не важно» в боте: время напоминаний московское. */}
      <label className="field">
        <span className="field-label">{t('profile.regionLabel')}</span>
        <select className="field-select" value={region} onChange={(event) => setRegion(event.target.value)}>
          <option value="">{t('profile.regionNone')}</option>
          {districts.map((district) => (
            <optgroup key={district.n} label={`${district.name} округ`}>
              {regions
                .filter((r) => r.district === district.n)
                .map((r) => (
                  <option key={r.code} value={r.code}>
                    {r.name}
                  </option>
                ))}
            </optgroup>
          ))}
        </select>
      </label>

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
                setSubjects((list) => toggleRequired(list, subject.code, t('profile.needSubject')))
              }
            >
              {subjects.includes(subject.code) ? '✓ ' : ''}
              {subject.name}
            </Chip>
          ))}
        </div>
      </div>

      <div className="field">
        <p className="field-label">
          <span>{t('profile.goalLabel')}</span>
          <span>{t('profile.selectedCount', { count: selectedDirections.length })}</span>
        </p>
        <div className="wrap-chips" role="group" aria-label={t('profile.goalLabel')}>
          {(directions.data?.items ?? []).map((d) => (
            <Chip
              key={d.id}
              active={selectedDirections.includes(d.id)}
              onClick={() => setSelectedDirections((list) => toggle(list, d.id))}
            >
              {selectedDirections.includes(d.id) ? '✓ ' : ''}
              {d.name}
            </Chip>
          ))}
        </div>
        <p className="field-note">{t('profile.goalHint')}</p>
      </div>

      <div className="field">
        <p className="field-label">{t('profile.experienceLabel')}</p>
        <div className="wrap-chips" role="group" aria-label={t('profile.experienceLabel')}>
          {EXPERIENCES.map((value) => (
            <Chip key={value} active={experience === value} onClick={() => setExperience(value)}>
              {experience === value ? '✓ ' : ''}
              {t(`profile.experience.${value}`)}
            </Chip>
          ))}
        </div>
      </div>

      <div className="field">
        <p className="field-label">
          <span>{t('profile.targetLabel')}</span>
          <span>{t('profile.selectedCount', { count: places.length })}</span>
        </p>
        <div className="wrap-chips" role="group" aria-label={t('profile.targetLabel')}>
          <Chip active={places.length === 0} onClick={() => setPlaces([])}>
            {places.length === 0 ? '✓ ' : ''}
            {t('profile.targetAny')}
          </Chip>
          {places.map((place) => (
            <Chip
              key={`${place.region_code}:${place.city ?? ''}`}
              active
              aria-label={t('profile.placeRemove', { place: placeName(place) })}
              onClick={() => setPlaces((list) => list.filter((p) => !samePlace(p, place)))}
            >
              {placeName(place)} ✕
            </Chip>
          ))}
        </div>
        {/* Нативный список: выбор добавляет регион целиком и сбрасывается. */}
        <select
          className="field-select"
          value=""
          aria-label={t('profile.placesAdd')}
          onChange={(event) => addRegion(event.target.value)}
        >
          <option value="">{t('profile.placesAdd')}</option>
          {districts.map((district) => (
            <optgroup key={district.n} label={`${district.name} округ`}>
              {regions
                .filter((r) => r.district === district.n)
                .map((r) => (
                  <option key={r.code} value={r.code}>
                    {r.name}
                  </option>
                ))}
            </optgroup>
          ))}
        </select>
        <p className="field-note">{t('profile.placesHint')}</p>
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
                setSelectedUniversities((list) => toggle(list, university.id))
              }
            >
              {selectedUniversities.includes(university.id) ? '✓ ' : ''}
              {university.short_name}
            </Chip>
          ))}
        </div>
      </div>

      {warning ? <p className="field-warning">{warning}</p> : null}

      <ThemeSetting value={theme} onChange={setTheme} />

      <Button
        stretched
        className="profile-save"
        loading={save.isPending}
        disabled={nameInvalid}
        iconBefore={<Icon name="spark" size={15} />}
        innerClassNames={{ content: 'button-wrap' }}
        onClick={submit}
      >
        {t('profile.saveCta')}
      </Button>

      <button type="button" className="link tutorial-replay" onClick={() => navigate('/?tutorial=1')}>
        <Icon name="book" size={15} />
        {t('tutorial.replay')}
      </button>

      {/* По версиям видно, какие коммиты сейчас в проде. */}
      <p className="app-version">
        Версия: приложение {WEB_VERSION} · сервер {serverVersion.data ?? (serverVersion.isError ? '—' : '…')}
      </p>
    </div>
  )
}
