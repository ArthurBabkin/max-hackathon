/** Семья — экраны G1, G2, G3. Функции F38–F44, F47, F48. */

import { useEffect, useState } from 'react'
import { Button } from '@maxhub/max-ui'
import type { Invite, Role } from '@contract'
import {
  useCreateInvite,
  useFamily,
  useLeaveTrajectory,
  useRemoveMember,
  useRevokeInvite,
  useSession,
} from '@/api/queries'
import { shareLink } from '@/bridge'
import { copyText } from '@/lib/clipboard'
import { canRemoveMember } from '@/lib/permissions'
import { Icon } from '@/ui/Icon'
import { CardSkeletons, Hint, SourceLine } from '@/ui/primitives'
import { useRole, useVoice } from '@/voice/useVoice'
import { ErrorState } from '@/ui/ErrorState'

export function FamilyScreen() {
  const t = useVoice()
  const role = useRole()
  const { data: session } = useSession()
  const family = useFamily()
  const createInvite = useCreateInvite()
  const removeMember = useRemoveMember()
  const revokeInvite = useRevokeInvite()
  const leave = useLeaveTrajectory()

  // id ссылки, которую только что скопировали: кнопка на 2 секунды говорит «скопирована».
  const [copied, setCopied] = useState<string | null>(null)
  const [left, setLeft] = useState(false)
  // Удаление и выход не отменить, поэтому срабатывают со второго нажатия.
  // Здесь id участника, id ссылки или 'leave'; через 4 секунды без ответа кнопка
  // возвращается в обычный вид.
  const [armed, setArmed] = useState<string | null>(null)

  useEffect(() => {
    if (!armed) return
    const timer = setTimeout(() => setArmed(null), 4000)
    return () => clearTimeout(timer)
  }, [armed])

  if (family.isPending) {
    return (
      <div className="screen">
        <CardSkeletons count={2} />
      </div>
    )
  }

  if (family.isError || !family.data) {
    return (
      <div className="screen">
        <ErrorState error={family.error} onRetry={() => void family.refetch()} />
      </div>
    )
  }

  const { members, invites } = family.data
  const creator = members.find((member) => member.is_creator)
  const hasKid = members.some((member) => member.role === 'kid')

  // Роль приглашённого выбирает тот, кто создаёт ссылку. Если ученик уже
  // подключён, вторую роль «ученик» выдать нельзя (ТЗ F42).
  const inviteRoles: Role[] = hasKid ? ['parent'] : ['kid', 'parent']

  const copy = async (invite: Invite) => {
    if (!(await copyText(invite.url))) return
    setCopied(invite.id)
    setTimeout(() => setCopied((id) => (id === invite.id ? null : id)), 2000)
  }

  // Не вышло поделиться — кладём ссылку в буфер. Открывать её самому нельзя:
  // приглашение одноразовое и досталось бы тому, кто его создал.
  const share = async (invite: Invite) => {
    if (!(await shareLink(invite.url))) await copy(invite)
  }

  return (
    <div className="screen">
      <h1 className="family-title">{t('family.title')}</h1>
      {creator ? (
        <p className="family-subtitle">{t('family.subtitle', { creator: creator.name })}</p>
      ) : null}

      <Hint>{t('family.remindNote')}</Hint>

      <section className="members">
        <h2 className="members-head">
          <Icon name="users" size={13} />
          {t('family.membersTitle', { count: members.length })}
        </h2>
        {members.map((member) => (
          <div key={member.id} className="member">
            <span className="member-avatar" style={{ background: member.color ?? 'var(--p)' }}>
              {member.name.slice(0, 1)}
            </span>
            <span className="member-main">
              <span className="member-name">
                {member.name}
                {member.is_creator ? <span className="crown">{t('family.creatorBadge')}</span> : null}
                {member.is_me ? <span className="mine-badge">{t('family.youBadge')}</span> : null}
              </span>
              <span className="member-role">
                {member.role === 'kid' ? t('family.roleKid') : t('family.roleParent')}
              </span>
            </span>
            {session && canRemoveMember(session, member) ? (
              <button
                type="button"
                className={armed === member.id ? 'member-remove member-remove-armed' : 'member-remove'}
                disabled={removeMember.isPending}
                onClick={() => (armed === member.id ? removeMember.mutate(member.id) : setArmed(member.id))}
              >
                {armed === member.id ? t('family.removeConfirm') : t('family.removeMember')}
              </button>
            ) : null}
          </div>
        ))}
      </section>

      {/* Каждая ссылка срабатывает один раз — F39. Неиспользованную можно
          отозвать: автор ссылки или создатель траектории (ТЗ §16). Свежие сверху. */}
      {invites.length > 0 ? (
        <section className="invites">
          <h2 className="members-head">
            <Icon name="link" size={13} />
            {t('family.linksWaiting', { count: invites.length })}
          </h2>
          <p className="invite-text">{t('family.linkText')}</p>
          {[...invites].reverse().map((invite) => (
            <div key={invite.id} className="invite-box">
              <p className="invite-title">
                {invite.role === 'kid' ? t('family.linkForKid') : t('family.linkForParent')}
              </p>
              <p className="invite-url">{invite.url}</p>
              <div className="invite-actions">
                <Button stretched iconBefore={<Icon name="send" size={15} />} onClick={() => void share(invite)}>
                  {t('family.linkShare')}
                </Button>
                <Button stretched variant="secondary" onClick={() => void copy(invite)}>
                  {copied === invite.id ? t('toast.inviteCopied') : t('family.linkCopy')}
                </Button>
                {invite.can_revoke ? (
                  <button
                    type="button"
                    className={`member-remove invite-revoke${armed === invite.id ? ' member-remove-armed' : ''}`}
                    disabled={revokeInvite.isPending}
                    onClick={() => (armed === invite.id ? revokeInvite.mutate(invite.id) : setArmed(invite.id))}
                  >
                    {armed === invite.id ? t('family.linkRevokeConfirm') : t('family.linkRevoke')}
                  </button>
                ) : null}
              </div>
            </div>
          ))}
        </section>
      ) : null}

      <div className="invite-buttons" data-tour="invite">
        {inviteRoles.map((inviteRole, i) => (
          <Button
            key={inviteRole}
            stretched
            variant={invites.length === 0 && i === 0 ? 'primary' : 'secondary'}
            loading={createInvite.isPending && createInvite.variables === inviteRole}
            disabled={createInvite.isPending}
            iconBefore={<Icon name="link" size={16} />}
            onClick={() => createInvite.mutate(inviteRole)}
          >
            {inviteRole === 'kid' ? t('family.inviteKid') : t('family.inviteParent')}
          </Button>
        ))}
      </div>

      {/* Объяснение льгот простыми словами доступно родителю и здесь — F47. */}
      {role === 'parent' ? (
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
        </section>
      ) : null}

      {session?.permissions.leave ? (
        <>
          <Button
            stretched
            variant="secondary"
            className="leave-button"
            disabled={left}
            loading={leave.isPending}
            iconBefore={<Icon name="out" size={15} />}
            onClick={() =>
              armed === 'leave'
                ? leave.mutate(undefined, { onSuccess: () => setLeft(true) })
                : setArmed('leave')
            }
          >
            {left ? t('toast.left') : armed === 'leave' ? t('family.leaveConfirm') : t('family.leaveCta')}
          </Button>
          {creator ? (
            <p className="fine fine-center">{t('family.leaveHint', { creator: creator.name })}</p>
          ) : null}
        </>
      ) : (
        <p className="fine fine-center">{t('family.deleteHint')}</p>
      )}
    </div>
  )
}
