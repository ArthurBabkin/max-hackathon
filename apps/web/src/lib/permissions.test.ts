import { describe, expect, it } from 'vitest'
import { canRemoveMember, derivePermissions, trackerAction } from './permissions'
import type { Member, Role } from '@contract'

/**
 * Матрица прав из ТЗ §3.1 в исполняемом виде. Сервер считает то же самое и
 * отдаёт флаги в сессии — здесь проверяется, что таблица понята верно.
 */
describe('derivePermissions — матрица ТЗ §3.1', () => {
  const кейс = (role: Role, is_creator: boolean, has_kid: boolean) =>
    derivePermissions({ role, is_creator, has_kid })

  describe('добавление и удаление из трекера', () => {
    it('ученик добавляет сам', () => {
      expect(кейс('kid', false, true).add_to_tracker).toBe(true)
      expect(кейс('kid', false, true).remove_from_tracker).toBe(true)
    })

    it('родитель при ученике в траектории не добавляет, а предлагает', () => {
      const p = кейс('parent', true, true)
      expect(p.add_to_tracker).toBe(false)
      expect(p.remove_from_tracker).toBe(false)
      expect(p.propose).toBe(true)
    })

    it('родитель без ученика в траектории добавляет сам (§3.2)', () => {
      const p = кейс('parent', true, false)
      expect(p.add_to_tracker).toBe(true)
      expect(p.remove_from_tracker).toBe(true)
      expect(p.propose).toBe(false)
    })
  })

  describe('предложения', () => {
    it('отвечает на предложение только ученик', () => {
      expect(кейс('kid', false, true).resolve_proposals).toBe(true)
      expect(кейс('parent', true, true).resolve_proposals).toBe(false)
    })

    it('ученик не предлагает сам себе', () => {
      expect(кейс('kid', true, true).propose).toBe(false)
    })
  })

  describe('общие для обеих ролей', () => {
    it.each<[Role, boolean, boolean]>([
      ['kid', false, true],
      ['kid', true, true],
      ['parent', false, true],
      ['parent', true, false],
    ])('%s, создатель=%s: отметка, профиль и приглашения доступны', (role, creator, kid) => {
      const p = кейс(role, creator, kid)
      expect(p.toggle_registered).toBe(true)
      expect(p.edit_profile).toBe(true)
      expect(p.invite).toBe(true)
    })
  })

  describe('только создатель', () => {
    it('удаляет участников и траекторию', () => {
      expect(кейс('parent', true, true).remove_members).toBe(true)
      expect(кейс('parent', true, true).delete_trajectory).toBe(true)
      expect(кейс('kid', false, true).remove_members).toBe(false)
      expect(кейс('kid', false, true).delete_trajectory).toBe(false)
    })

    it('создатель не может выйти — только удалить траекторию', () => {
      expect(кейс('parent', true, true).leave).toBe(false)
      expect(кейс('kid', false, true).leave).toBe(true)
    })

    it('роль на права создателя не влияет: создателем бывает и ученик', () => {
      expect(кейс('kid', true, true).remove_members).toBe(true)
      expect(кейс('kid', true, true).leave).toBe(false)
    })
  })
})

describe('trackerAction — какую кнопку рисовать на карточке', () => {
  const сессия = (role: Role, has_kid: boolean) =>
    ({ permissions: derivePermissions({ role, is_creator: false, has_kid }) })

  it('ученик добавляет', () => {
    expect(trackerAction(сессия('kid', true))).toBe('add')
  })

  it('родитель при ученике предлагает', () => {
    expect(trackerAction(сессия('parent', true))).toBe('propose')
  })

  it('родитель без ученика добавляет', () => {
    expect(trackerAction(сессия('parent', false))).toBe('add')
  })
})

describe('canRemoveMember', () => {
  const участник = (over: Partial<Member> = {}): Member => ({
    id: 'm-2',
    name: 'Игорь',
    role: 'parent',
    is_creator: false,
    is_me: false,
    can_remove: true,
    joined_at: '2026-09-01T10:00:00Z',
    ...over,
  })

  const создатель = { permissions: derivePermissions({ role: 'parent', is_creator: true, has_kid: true }) }
  const приглашённый = { permissions: derivePermissions({ role: 'kid', is_creator: false, has_kid: true }) }

  it('создатель удаляет приглашённого', () => {
    expect(canRemoveMember(создатель, участник())).toBe(true)
  })

  it('создателя удалить нельзя даже создателю', () => {
    expect(canRemoveMember(создатель, участник({ is_creator: true }))).toBe(false)
  })

  it('себя удалить нельзя', () => {
    expect(canRemoveMember(создатель, участник({ is_me: true }))).toBe(false)
  })

  it('не создатель не удаляет никого', () => {
    expect(canRemoveMember(приглашённый, участник())).toBe(false)
  })

  it('запрет сервера перевешивает: can_remove=false — кнопки нет', () => {
    expect(canRemoveMember(создатель, участник({ can_remove: false }))).toBe(false)
  })
})
