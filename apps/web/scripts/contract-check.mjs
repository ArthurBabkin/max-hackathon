#!/usr/bin/env node
/**
 * Проверка живого API на соответствие контракту packages/api-contract/openapi.yaml.
 *
 * Обходит все операции контракта, сверяет код ответа со списком документированных
 * и тело — со схемой этого кода. Нужна демо-траектория Артёма и Ольги
 * (сервис seed-demo в docker-compose) и дев-обход подписи initData —
 * поэтому только локально:
 *
 *   docker compose -f infra/docker-compose.yml up -d --build
 *   npm run contract:check                  # API_BASE=http://localhost:8081/api/v1
 *
 * Необратимые операции (удаление участника, выход из траектории) проверяются
 * только ответами-отказами: демо-данные после прогона остаются прежними.
 * Вопрос помощнику задаётся такой, чтобы ответ был шаблоном без вызова модели.
 */

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import yaml from 'js-yaml'

const API = (process.env.API_BASE ?? 'http://localhost:8081/api/v1').replace(/\/$/, '')
const specPath = fileURLToPath(new URL('../../../packages/api-contract/openapi.yaml', import.meta.url))
const spec = yaml.load(readFileSync(specPath, 'utf8'))

// --- Мини-валидатор JSON Schema: ровно то, что встречается в контракте ------------

function resolve(schema) {
  let s = schema
  while (s && s.$ref) {
    s = s.$ref
      .replace(/^#\//, '')
      .split('/')
      .reduce((node, key) => node[key], spec)
  }
  return s
}

const FORMATS = {
  uuid: /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i,
  date: /^\d{4}-\d{2}-\d{2}$/,
  'date-time': /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/,
  uri: /^https?:\/\/\S+$/,
}

function typeOf(v) {
  if (v === null) return 'null'
  if (Array.isArray(v)) return 'array'
  if (Number.isInteger(v)) return 'integer'
  return typeof v
}

/** Возвращает список расхождений: путь в теле и что не так. */
function validate(value, schema, path = '$') {
  const s = resolve(schema)
  if (!s) return []
  const errors = []
  if (s.allOf) for (const sub of s.allOf) errors.push(...validate(value, sub, path))
  if (s.oneOf || s.anyOf) {
    const variants = s.oneOf ?? s.anyOf
    const passed = variants.filter((sub) => validate(value, sub, path).length === 0).length
    if (s.oneOf ? passed !== 1 : passed === 0) {
      errors.push(`${path}: подходит вариантов ${passed} из ${variants.length} (${s.oneOf ? 'oneOf' : 'anyOf'})`)
    }
  }
  if (s.type) {
    const types = Array.isArray(s.type) ? s.type : [s.type]
    const t = typeOf(value)
    const ok = types.includes(t) || (t === 'integer' && types.includes('number'))
    if (!ok) return [...errors, `${path}: тип ${t}, ждали ${types.join('|')}`]
  }
  if (s.const !== undefined && value !== s.const) errors.push(`${path}: ${JSON.stringify(value)} ≠ ${s.const}`)
  if (s.enum && !s.enum.includes(value)) errors.push(`${path}: ${JSON.stringify(value)} не из ${s.enum.join(', ')}`)
  if (typeof value === 'string') {
    if (s.format && FORMATS[s.format] && !FORMATS[s.format].test(value)) errors.push(`${path}: не ${s.format}: ${value}`)
    if (s.minLength !== undefined && value.length < s.minLength) errors.push(`${path}: короче ${s.minLength}`)
    if (s.maxLength !== undefined && value.length > s.maxLength) errors.push(`${path}: длиннее ${s.maxLength}`)
    if (s.pattern && !new RegExp(s.pattern, 'u').test(value)) errors.push(`${path}: не по шаблону ${s.pattern}`)
  }
  if (typeof value === 'number') {
    if (s.minimum !== undefined && value < s.minimum) errors.push(`${path}: меньше ${s.minimum}`)
    if (s.maximum !== undefined && value > s.maximum) errors.push(`${path}: больше ${s.maximum}`)
  }
  if (Array.isArray(value)) {
    if (s.minItems !== undefined && value.length < s.minItems) errors.push(`${path}: элементов меньше ${s.minItems}`)
    if (s.maxItems !== undefined && value.length > s.maxItems) errors.push(`${path}: элементов больше ${s.maxItems}`)
    if (s.items) value.forEach((item, i) => errors.push(...validate(item, s.items, `${path}[${i}]`)))
  }
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    for (const key of s.required ?? []) if (!(key in value)) errors.push(`${path}: нет поля ${key}`)
    for (const [key, sub] of Object.entries(s.properties ?? {})) {
      if (key in value) errors.push(...validate(value[key], sub, `${path}.${key}`))
    }
    if (s.additionalProperties === false) {
      for (const key of Object.keys(value)) {
        if (!(key in (s.properties ?? {}))) errors.push(`${path}: лишнее поле ${key}`)
      }
    }
  }
  return errors
}

// --- Вызовы -----------------------------------------------------------------------

const covered = new Set()
const failures = []
let calls = 0

function operation(method, template) {
  const op = spec.paths[template]?.[method.toLowerCase()]
  if (!op) throw new Error(`в контракте нет ${method} ${template}`)
  return op
}

function responseContent(op, status) {
  let r = op.responses[String(status)]
  if (!r) return undefined
  if (r.$ref) r = resolve(r)
  return r.content ?? null
}

function responseSchema(op, status) {
  const content = responseContent(op, status)
  if (content === undefined) return undefined
  return content?.['application/json']?.schema ?? null
}

/**
 * Вызов операции контракта. template — путь из контракта, params — подстановки,
 * expect — код, который ждём именно в этом сценарии.
 */
async function call(method, template, { params = {}, query, body, token, expect } = {}) {
  const op = operation(method, template)
  const path = template.replace(/\{(\w+)\}/g, (_, name) => encodeURIComponent(params[name]))
  const qs = query ? `?${new URLSearchParams(query)}` : ''
  const headers = { 'Content-Type': 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`
  const res = await fetch(`${API}${path}${qs}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  calls++
  const label = `${method} ${template}${qs ? ` ${qs}` : ''} → ${res.status}`
  covered.add(`${method} ${template}`)
  const text = await res.text()
  // Файл календаря — не JSON: проверяем тип и начало файла.
  const calendar = responseContent(op, res.status)?.['text/calendar']
  if (calendar) {
    if (expect !== undefined && res.status !== expect) failures.push(`${label}: ждали ${expect}; ${text.slice(0, 200)}`)
    if (!res.headers.get('content-type')?.startsWith('text/calendar')) failures.push(`${label}: Content-Type ${res.headers.get('content-type')}`)
    if (!text.startsWith('BEGIN:VCALENDAR\r\n')) failures.push(`${label}: не файл календаря`)
    return { status: res.status, data: text }
  }
  let data = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      failures.push(`${label}: тело не JSON`)
      return { status: res.status, data: null }
    }
  }
  if (expect !== undefined && res.status !== expect) failures.push(`${label}: ждали ${expect}; ${text.slice(0, 200)}`)
  const schema = responseSchema(op, res.status)
  if (schema === undefined) {
    failures.push(`${label}: код не описан в контракте (есть ${Object.keys(op.responses).join(', ')})`)
  } else if (schema !== null) {
    for (const e of validate(data, schema)) failures.push(`${label}: ${e}`)
  } else if (text) {
    failures.push(`${label}: тела быть не должно`)
  }
  return { status: res.status, data }
}

function initData(id, name) {
  const user = JSON.stringify({ id, first_name: name })
  return new URLSearchParams({ user, auth_date: String(Math.floor(Date.now() / 1000)), hash: 'dev-stub-unsigned' }).toString()
}

async function login(id, name) {
  const r = await call('POST', '/session', { body: { init_data: initData(id, name), start_param: null }, expect: 200 })
  if (!r.data?.token) throw new Error(`вход ${name} не удался: ${JSON.stringify(r.data)}. Запущен ли seed-demo?`)
  return r.data.token
}

// --- Сценарий ---------------------------------------------------------------------

async function main() {
  const health = await fetch(`${API}/health`).catch(() => null)
  if (!health?.ok) throw new Error(`API не отвечает на ${API}/health`)

  // Сессия: неподписанная строка не от дев-диапазона и пользователь без траектории.
  await call('POST', '/session', { body: { init_data: 'user=%7B%22id%22%3A1%7D&hash=x' }, expect: 401 })
  await call('POST', '/session', { body: { init_data: initData(900000999, 'Никто') }, expect: 404 })
  await call('POST', '/session', { body: {}, expect: 400 })
  const kid = await login(900000001, 'Артём')
  const parent = await login(900000002, 'Ольга')

  await call('GET', '/home', { expect: 401 })
  await call('GET', '/home', { token: kid, expect: 200 })
  await call('GET', '/home', { token: parent, expect: 200 })

  const profile = (await call('GET', '/profile', { token: kid, expect: 200 })).data
  await call('PATCH', '/profile', { token: kid, body: { grade: profile.trajectory?.grade ?? profile.grade }, expect: 200 })
  await call('PATCH', '/profile', { token: kid, body: { subject_codes: [] }, expect: 400 })
  // Направления и город — теми же значениями: демо-данные не меняются.
  const goal = { direction_ids: profile.directions.map((d) => d.id), target_region_code: profile.target_region_code ?? '' }
  await call('PATCH', '/profile', { token: kid, body: goal, expect: 200 })
  await call('PATCH', '/profile', { token: kid, body: { target_region_code: '999' }, expect: 400 })
  const uniIds = (profile.universities ?? []).map((u) => u.id)
  await call('PUT', '/profile/universities', { token: kid, body: { university_ids: uniIds }, expect: 200 })
  await call('PUT', '/profile/universities', { token: kid, body: {}, expect: 400 })

  for (const filter of ['all', 'level1', 'soon', 'online']) {
    await call('GET', '/recommendations', { token: kid, query: { filter }, expect: 200 })
  }
  await call('GET', '/recommendations', { token: kid, query: { filter: 'unknown' }, expect: 400 })
  const recs = (await call('GET', '/recommendations', { token: kid, expect: 200 })).data

  const list = (await call('GET', '/olympiads', { token: kid, expect: 200 })).data
  await call('GET', '/olympiads', { token: kid, query: { q: 'проба' }, expect: 200 })
  await call('GET', '/olympiads', { token: kid, query: { q: 'ничего-такого-нет' }, expect: 200 })
  const profileId = list.items[0]?.primary_profile?.olympiad_profile_id ?? 'p669-8-informatika'
  await call('GET', '/olympiads/{id}', { token: kid, params: { id: profileId }, expect: 200 })
  await call('GET', '/olympiads/{id}', { token: parent, params: { id: 'p669-8-informatika' }, expect: 200 })
  await call('GET', '/olympiads/{id}', { token: kid, params: { id: 'nope' }, expect: 404 })

  const unis = (await call('GET', '/universities', { token: kid, expect: 200 })).data
  await call('GET', '/universities', { token: kid, query: { city: 'Москва' }, expect: 200 })
  await call('GET', '/universities/{id}', { token: kid, params: { id: unis.items[0].id }, expect: 200 })
  await call('GET', '/universities/{id}', { token: kid, params: { id: 'nope' }, expect: 404 })
  const directions = (await call('GET', '/directions', { token: kid, expect: 200 })).data

  // Направления вузов: фильтр каталога, олимпиады по направлению, сохранение.
  const direction = directions.items[0]?.id ?? 'nope'
  await call('GET', '/universities', { token: kid, query: { direction }, expect: 200 })
  let program = null
  for (const u of unis.items) {
    const detail = (await call('GET', '/universities/{id}', { token: kid, params: { id: u.id }, expect: 200 })).data
    program = detail.programs?.[0] ?? null
    if (program) break
  }
  if (program) {
    const params = { id: program.university_id }
    await call('GET', '/universities/{id}', { token: kid, params, query: { program: program.id }, expect: 200 })
    await call('GET', '/universities/{id}', { token: kid, params, query: { program: 'nope' }, expect: 404 })
    await call('PUT', '/profile/programs', { token: kid, body: { program_ids: [program.id] }, expect: 200 })
    await call('PUT', '/profile/programs', { token: kid, body: { program_ids: [] }, expect: 200 })
  }
  await call('PUT', '/profile/programs', { token: kid, body: {}, expect: 400 })
  await call('PUT', '/profile/universities', { token: kid, body: { university_ids: uniIds }, expect: 200 })

  // Трекер: добавить то, чего там нет, отметить, снять отметку, удалить.
  const tracker = (await call('GET', '/tracker', { token: kid, expect: 200 })).data
  const inTracker = new Set(tracker.items.map((i) => i.olympiad_profile_id))
  const candidate = [...(recs.items ?? []), ...(recs.outside ?? [])]
    .map((i) => i.olympiad_profile_id)
    .find((id) => id && !inTracker.has(id))
  if (candidate) {
    const added = (await call('POST', '/tracker', { token: kid, body: { olympiad_profile_id: candidate }, expect: 201 })).data
    await call('POST', '/tracker', { token: kid, body: { olympiad_profile_id: candidate }, expect: 200 })
    await call('PUT', '/tracker/{id}/registered', { token: kid, params: { id: added.id }, expect: 200 })
    await call('DELETE', '/tracker/{id}/registered', { token: kid, params: { id: added.id }, expect: 200 })
    await call('POST', '/tracker', { token: parent, body: { olympiad_profile_id: candidate }, expect: 403 })
    await call('DELETE', '/tracker/{id}', { token: kid, params: { id: added.id }, expect: 204 })
  } else {
    failures.push('нет олимпиады вне трекера — проверить добавление нечем')
  }
  await call('POST', '/tracker', { token: kid, body: { olympiad_profile_id: 'nope' }, expect: 404 })
  await call('POST', '/tracker', { token: kid, body: {}, expect: 400 })
  await call('DELETE', '/tracker/{id}', { token: kid, params: { id: '00000000-0000-4000-8000-000000000000' }, expect: 404 })
  await call('PUT', '/tracker/{id}/registered', { token: kid, params: { id: 'nope' }, expect: 404 })
  await call('DELETE', '/tracker/{id}/registered', { token: kid, params: { id: 'nope' }, expect: 404 })

  const month = new Date().toISOString().slice(0, 7)
  await call('GET', '/calendar', { token: kid, query: { month }, expect: 200 })
  await call('GET', '/calendar', { token: kid, query: { month: '2026-13' }, expect: 400 })

  // Выгрузка: ссылка своя у каждого, файл — по ней без сессии.
  const { url: calendarUrl } = (await call('GET', '/calendar/link', { token: kid, expect: 200 })).data
  const calendarToken = new URL(calendarUrl, 'http://x').searchParams.get('token')
  await call('GET', '/calendar.ics', { query: { token: calendarToken }, expect: 200 })
  await call('GET', '/calendar.ics', { query: { token: kid }, expect: 404 })
  await call('GET', '/calendar/link', { expect: 401 })

  // Предложения: родитель предлагает, ученик отказывается — трекер не меняется.
  const after = new Set((await call('GET', '/tracker', { token: kid, expect: 200 })).data.items.map((i) => i.olympiad_profile_id))
  const offer = list.items.map((i) => i.primary_profile?.olympiad_profile_id).find((id) => id && !after.has(id))
  const proposal = (await call('POST', '/proposals', { token: parent, body: { olympiad_profile_id: offer } })).data
  await call('POST', '/proposals', { token: kid, body: { olympiad_profile_id: offer }, expect: 403 })
  await call('POST', '/proposals/{id}/accept', { token: parent, params: { id: proposal.id }, expect: 403 })
  await call('POST', '/proposals/{id}/decline', { token: kid, params: { id: proposal.id }, expect: 200 })
  await call('POST', '/proposals/{id}/accept', { token: kid, params: { id: proposal.id }, expect: 409 })
  await call('POST', '/proposals/{id}/decline', { token: kid, params: { id: 'nope' }, expect: 404 })
  await call('POST', '/proposals', { token: parent, body: {}, expect: 400 })
  await call('POST', '/proposals', { token: parent, body: { olympiad_profile_id: 'nope' }, expect: 404 })
  await call('POST', '/proposals', { token: parent, body: { olympiad_profile_id: [...after][0] }, expect: 409 })

  // Семья: необратимое — только отказами.
  const family = (await call('GET', '/family', { token: kid, expect: 200 })).data
  const me = family.members.find((m) => m.is_me)
  await call('POST', '/family/invites', { token: kid, body: { role: 'parent' }, expect: 201 })
  await call('POST', '/family/invites', { token: kid, body: { role: 'kid' }, expect: 409 })
  await call('POST', '/family/invites', { token: kid, body: { role: 'admin' }, expect: 400 })
  await call('DELETE', '/family/members/{id}', { token: kid, params: { id: me.id }, expect: 403 })
  const other = family.members.find((m) => !m.is_me)
  if (other) await call('DELETE', '/family/members/{id}', { token: parent, params: { id: me.id }, expect: 403 })
  await call('DELETE', '/family/members/{id}', { token: kid, params: { id: '00000000-0000-4000-8000-000000000000' }, expect: 404 })
  await call('POST', '/family/leave', { token: kid, expect: 403 })

  // Помощник: вопросы вне базы — шаблон без вызова модели. Чат ученика
  // родитель не видит: для него такого чата нет.
  const chat = (await call('POST', '/ai/chats', { token: kid, body: { text: 'Какая завтра погода?' }, expect: 201 })).data?.chat
  await call('POST', '/ai/chats', { token: kid, body: { text: '' }, expect: 400 })
  await call('GET', '/ai/chats', { token: kid, expect: 200 })
  if (chat) {
    const id = { id: chat.id }
    await call('POST', '/ai/chats/{id}/messages', { token: kid, params: id, body: { text: 'А послезавтра?' }, expect: 200 })
    await call('POST', '/ai/chats/{id}/messages', { token: kid, params: id, body: { text: ' ' }, expect: 400 })
    await call('GET', '/ai/chats/{id}/messages', { token: kid, params: id, expect: 200 })
    await call('GET', '/ai/chats/{id}/messages', { token: parent, params: id, expect: 404 })
    await call('PATCH', '/ai/chats/{id}', { token: kid, params: id, body: { title: 'Погода' }, expect: 200 })
    await call('PATCH', '/ai/chats/{id}', { token: kid, params: id, body: { title: '' }, expect: 400 })
    await call('PATCH', '/ai/chats/{id}', { token: parent, params: id, body: { title: 'Моё' }, expect: 404 })
  }

  // Покрытие: каждая операция контракта вызвана хотя бы раз.
  for (const [template, ops] of Object.entries(spec.paths)) {
    for (const method of Object.keys(ops)) {
      if (method === 'parameters') continue
      if (!covered.has(`${method.toUpperCase()} ${template}`)) failures.push(`не проверено: ${method.toUpperCase()} ${template}`)
    }
  }
}

main()
  .catch((err) => failures.push(`прогон прерван: ${err.message}`))
  .finally(() => {
    const ops = Object.values(spec.paths).reduce((n, ops) => n + Object.keys(ops).filter((m) => m !== 'parameters').length, 0)
    console.log(`API ${API}: ${calls} вызовов, операций покрыто ${covered.size} из ${ops}`)
    if (failures.length) {
      console.error(`\nРасхождения с контрактом (${failures.length}):`)
      for (const f of failures) console.error(`  ✗ ${f}`)
      process.exit(1)
    }
    console.log('✓ ответы совпадают с контрактом')
  })
