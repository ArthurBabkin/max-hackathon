/** Семья — экраны G1, G2, G3. Функции F38–F44, F47, F48. */

import { useState } from 'react'
import { Button } from '@maxhub/max-ui'
import type { Role } from '@contract'
import { useCreateInvite, useFamily, useLeaveTrajectory, useRemoveMember, useSession } from '@/api/queries'
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
  const leave = useLeaveTrajectory()

  const [copied, setCopied] = useState(false)
  const [left, setLeft] = useState(false)

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
  const lastInvite = invites.at(-1)
  const hasKid = members.some((member) => member.role === 'kid')

  // Роль приглашённого выбирает тот, кто создаёт ссылку. Если ученик уже
  // подключён, вторую роль «ученик» выдать нельзя (ТЗ F42).
  const inviteRole: Role = hasKid ? 'parent' : 'kid'

  const copy = async () => {
    if (!lastInvite) return
    if (!(await copyText(lastInvite.url))) return
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  // Не вышло поделиться — кладём ссылку в буфер. Открывать её самому нельзя:
  // приглашение одноразовое и досталось бы тому, кто его создал.
  const share = async () => {
    if (!lastInvite) return
    if (!(await shareLink(lastInvite.url))) await copy()
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
                className="member-remove"
                disabled={removeMember.isPending}
                onClick={() => removeMember.mutate(member.id)}
              >
                {t('family.removeMember')}
              </button>
            ) : null}
          </div>
        ))}
      </section>

      {/* Каждая ссылка срабатывает один раз — F39. */}
      {lastInvite ? (
        <section className="invite-box">
          <p className="invite-title">{t('family.linkTitle')}</p>
          <p className="invite-text">{t('family.linkText')}</p>
          <p className="invite-url">{lastInvite.url}</p>
          <div className="invite-actions">
            <Button stretched iconBefore={<Icon name="send" size={15} />} onClick={() => void share()}>
              {t('family.linkShare')}
            </Button>
            <Button stretched variant="secondary" onClick={() => void copy()}>
              {copied ? t('toast.inviteCopied') : t('family.linkCopy')}
            </Button>
          </div>
        </section>
      ) : null}

      {invites.length > 1 ? (
        <p className="fine fine-center">{t('family.linksWaiting', { count: invites.length })}</p>
      ) : null}

      <Button
        stretched
        variant={invites.length > 0 ? 'secondary' : 'primary'}
        loading={createInvite.isPending}
        iconBefore={<Icon name="link" size={16} />}
        data-tour="invite"
        onClick={() => createInvite.mutate(inviteRole)}
      >
        {invites.length > 0
          ? t('family.inviteMoreCta')
          : hasKid
            ? t('family.inviteParent')
            : t('family.inviteKid')}
      </Button>

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
            onClick={() => leave.mutate(undefined, { onSuccess: () => setLeft(true) })}
          >
            {left ? t('toast.left') : t('family.leaveCta')}
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
