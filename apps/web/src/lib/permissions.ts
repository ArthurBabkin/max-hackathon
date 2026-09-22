/**
 * Права участника — матрица ТЗ §3.1.
 *
 * Источник истины — сервер: он считает права и кладёт готовые флаги в
 * `session.permissions`, он же проверяет их при каждом запросе. Приложение
 * рисует кнопки по этим флагам и защитой их не считает.
 *
 * `derivePermissions` — та же матрица на клиенте. Она нужна для моков
 * разработки: без неё переключение роли в dev не меняло бы интерфейс, и
 * родительский сценарий было бы нечем проверять. Плюс это исполняемая запись
 * таблицы §3.1, на которой стоят тесты.
 */

import type { Member, Permissions, Role } from '@contract'

export interface MemberFacts {
  role: Role
  is_creator: boolean
  /** Есть ли в траектории активный участник с ролью `kid`. */
  has_kid: boolean
}

/** Минимум от сессии, который нужен этим функциям. */
interface WithPermissions {
  permissions: Permissions
}

export function derivePermissions({ role, is_creator, has_kid }: MemberFacts): Permissions {
  const isKid = role === 'kid'
  // Родитель управляет трекером напрямую только пока ученика в траектории нет.
  // Как только ученик подключился, родитель переходит на предложения (ТЗ §3.2).
  const managesTrackerDirectly = isKid || !has_kid

  return {
    add_to_tracker: managesTrackerDirectly,
    remove_from_tracker: managesTrackerDirectly,
    propose: !isKid && has_kid,
    resolve_proposals: isKid,
    toggle_registered: true,
    edit_profile: true,
    invite: true,
    remove_members: is_creator,
    // Создатель не выходит из траектории, он её удаляет (ТЗ §3.2).
    leave: !is_creator,
    delete_trajectory: is_creator,
  }
}

/**
 * Что показывать на карточке олимпиады: «Добавить в трекер» или
 * «Предложить <имя>». Третьего варианта нет — смотреть карточку может любой,
 * и одно из двух действий доступно всегда.
 */
export function trackerAction(session: WithPermissions): 'add' | 'propose' {
  return session.permissions.add_to_tracker ? 'add' : 'propose'
}

/**
 * Показывать ли кнопку «Удалить» у участника.
 *
 * Сервер уже присылает `can_remove`, но проверяем и его, и собственные
 * условия: сервер отвечает за правило, клиент — за то, чтобы не нарисовать
 * кнопку там, где она заведомо не сработает.
 */
export function canRemoveMember(session: WithPermissions, member: Member): boolean {
  if (!session.permissions.remove_members) return false
  if (member.is_creator || member.is_me) return false
  return member.can_remove
}
