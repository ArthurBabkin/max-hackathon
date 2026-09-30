/** Помощник — экраны F1–F5. Функции F35–F37, F58–F60. */

import { useEffect, useRef, useState } from 'react'
import { IconButton, Input } from '@maxhub/max-ui'
import type { AiChat } from '@contract'
import { ApiError } from '@/api/errors'
import { useAiChatMessages, useAiChats, useAskAi, useRenameAiChat } from '@/api/queries'
import { getWebApp } from '@/bridge'
import { daysLeft, formatDay } from '@/lib/deadline'
import { Icon, Logo } from '@/ui/Icon'
import { Sheet } from '@/ui/Sheet'
import { SourceTag } from '@/ui/primitives'
import type { SheetStack } from '@/ui/sheets'
import { useVoice, type Translate } from '@/voice/useVoice'

/** Подсказки-вопросы: с чего начать, если не знаешь, что спросить. Тексты — в словаре. */
const SUGGESTIONS = ['ai.suggest.1', 'ai.suggest.2', 'ai.suggest.3'] as const

/** Новый чат в адресе: `?sheet=ai:new`. Пустой `ai` — последний чат. */
const NEW_CHAT = 'new'

/** Название чата — не длиннее 60 символов (F59), как проверяет сервер. */
const MAX_TITLE = 60

/** День последней реплики под названием чата в списке. */
function lastDay(chat: AiChat, t: Translate): string {
  const days = daysLeft(chat.last_message_at)
  if (days === 0) return t('ai.today')
  if (days === -1) return t('ai.yesterday')
  return formatDay(chat.last_message_at) ?? ''
}

export function AiSheet({ chatId, sheets }: { chatId: string; sheets: SheetStack }) {
  const t = useVoice()
  const [draft, setDraft] = useState('')
  const [view, setView] = useState<'chat' | 'list'>('chat')
  // Черновик названия; `null` — полоса чата не в режиме переименования.
  const [renaming, setRenaming] = useState<string | null>(null)
  const chats = useAiChats()
  const ask = useAskAi()
  const rename = useRenameAiChat()
  const logRef = useRef<HTMLDivElement>(null)

  // Открытый чат: `null` — новый, ещё без реплик; `undefined` — список чатов
  // ещё не пришёл, и какой чат последний, пока неизвестно.
  const list = chats.data?.items
  const active: string | null | undefined =
    chatId === NEW_CHAT ? null : chatId || (list ? (list[0]?.id ?? null) : undefined)
  const current = list?.find((c) => c.id === active)
  const messages = useAiChatMessages(typeof active === 'string' ? active : null)
  const items = messages.data?.items ?? []

  // Ответ на вопрос приходит секундами позже: если за это время открыли
  // другой чат, переключать обратно на созданный нельзя.
  const chatIdRef = useRef(chatId)
  chatIdRef.current = chatId

  const pending = ask.isPending && ask.variables.chatId === active
  const failed = ask.isError && ask.variables.chatId === active ? ask.error : null

  // Новый ответ должен быть виден сразу, без ручной прокрутки.
  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight })
  }, [items.length, pending, active])

  // Чат из адреса не нашёлся — чужой или его нет. Открываем свой последний.
  const missing = messages.error instanceof ApiError && messages.error.status === 404
  useEffect(() => {
    if (missing) sheets.replace({ kind: 'ai', id: '' })
  }, [missing, sheets])

  const open = (id: string) => {
    setView('chat')
    setRenaming(null)
    if (id !== chatId) sheets.replace({ kind: 'ai', id })
  }

  const send = (text: string) => {
    const value = text.trim()
    if (!value || ask.isPending || active === undefined) return
    const from = chatId
    setDraft('')
    ask.mutate(
      { chatId: active, text: value },
      {
        onSuccess: ({ chat }) => {
          if (active === null && chatIdRef.current === from) sheets.replace({ kind: 'ai', id: chat.id })
        },
        // Вопрос не должен пропадать: возвращаем его в поле, если там пусто.
        onError: () => setDraft((now) => now || value),
      },
    )
  }

  const saveTitle = () => {
    const title = (renaming ?? '').trim()
    if (!current || !title) return
    if (title === current.title) {
      setRenaming(null)
      return
    }
    rename.mutate({ id: current.id, title }, { onSuccess: () => setRenaming(null) })
  }

  const cancelRename = () => {
    rename.reset()
    setRenaming(null)
  }

  const asked = new Set(items.filter((m) => m.role === 'user').map((m) => m.text))
  if (pending) asked.add(ask.variables.text)
  const left = SUGGESTIONS.map((key) => t(key)).filter((q) => !asked.has(q))
  const title = active === null ? t('ai.newChat') : (current?.title ?? '…')

  return (
    <Sheet
      tall
      label={t('ai.title')}
      canGoBack={sheets.stack.length > 1}
      onBack={sheets.back}
      onClose={sheets.closeAll}
      header={
        <>
          <Logo size={34} />
          <div className="sheet-title">
            <h2 className="sheet-title-sm">{t('ai.title')}</h2>
            <p>{t('ai.subtitle')}</p>
          </div>
        </>
      }
    >
      <div className="ai-bar">
        {renaming !== null ? (
          <>
            <input
              className="ai-title-field"
              value={renaming}
              maxLength={MAX_TITLE}
              aria-label={t('ai.renameField')}
              autoFocus
              onFocus={(event) => event.target.select()}
              onChange={(event) => setRenaming(event.target.value)}
              onKeyDown={(event) => {
                // Escape закрывает только поле, а не весь лист.
                event.stopPropagation()
                if (event.key === 'Enter') {
                  event.preventDefault()
                  saveTitle()
                } else if (event.key === 'Escape') {
                  cancelRename()
                }
              }}
            />
            <button
              type="button"
              className="ai-bar-button ai-bar-button-primary"
              aria-label={t('ai.renameSave')}
              disabled={!renaming.trim() || rename.isPending}
              onClick={saveTitle}
            >
              <Icon name="check" size={15} strokeWidth={2.6} />
            </button>
            <button type="button" className="ai-bar-button" aria-label={t('ai.renameCancel')} onClick={cancelRename}>
              <Icon name="close" size={14} />
            </button>
          </>
        ) : view === 'list' ? (
          <button type="button" className="ai-current" aria-label={t('ai.backToChat')} onClick={() => setView('chat')}>
            <Icon name="back" size={14} />
            <span className="ai-current-title">{t('ai.chats')}</span>
          </button>
        ) : (
          <>
            <button type="button" className="ai-current" onClick={() => setView('list')}>
              <Icon name="list" size={14} />
              <span className="sr-only">{t('ai.chats')}: </span>
              <span className="ai-current-title">{title}</span>
              <Icon name="down" size={13} />
            </button>
            {current ? (
              <button
                type="button"
                className="ai-bar-button"
                aria-label={t('ai.rename')}
                onClick={() => setRenaming(current.title)}
              >
                <Icon name="pencil" size={14} />
              </button>
            ) : null}
            <button type="button" className="ai-bar-button" aria-label={t('ai.newChat')} onClick={() => open(NEW_CHAT)}>
              <Icon name="plus" size={15} />
            </button>
          </>
        )}
      </div>
      {rename.isError && renaming !== null ? (
        <p className="ai-error" role="alert">
          {rename.error.message}
        </p>
      ) : null}

      {view === 'list' ? (
        <div className="ai-log">
          <p className="lock lock-center">
            <Icon name="eyeOff" size={12} />
            {t('ai.chatsPrivacy')}
          </p>
          <button type="button" className="ai-new" onClick={() => open(NEW_CHAT)}>
            <Icon name="plus" size={15} />
            {t('ai.newChat')}
          </button>
          <ul className="ai-chats" aria-label={t('ai.chats')}>
            {(list ?? []).map((chat) => {
              const on = chat.id === active
              return (
                <li key={chat.id}>
                  <button
                    type="button"
                    className={`ai-chat${on ? ' ai-chat-on' : ''}`}
                    aria-current={on ? 'true' : undefined}
                    onClick={() => open(chat.id)}
                  >
                    <span className="ai-chat-main">
                      <b>{chat.title}</b>
                      <small>{lastDay(chat, t)}</small>
                    </span>
                    {on ? <Icon name="check" size={14} strokeWidth={2.6} /> : null}
                  </button>
                </li>
              )
            })}
          </ul>
        </div>
      ) : (
        <>
          <div className="ai-log" ref={logRef}>
            <p className="lock lock-center">
              <Icon name="eyeOff" size={12} />
              {t('ai.privacy')}
            </p>

            <div className="ai-message ai-message-bot">{t('ai.intro')}</div>

            {items.map((message) => (
              <div
                key={message.id}
                className={`ai-message ${
                  message.role === 'user'
                    ? 'ai-message-user'
                    : message.refused
                      ? 'ai-message-bot ai-message-warn'
                      : 'ai-message-bot'
                }`}
              >
                {message.role === 'assistant' && !message.refused ? <SourceTag kind="fact" /> : null}
                <p>{message.text}</p>

                {message.card_refs.map((ref) => (
                  <button
                    key={`${ref.type}-${ref.id}`}
                    type="button"
                    className="source-line"
                    onClick={() => sheets.open({ kind: ref.type === 'olympiad' ? 'oly' : 'vuz', id: ref.id })}
                  >
                    <Icon name="doc" size={12} />
                    {t('ai.cardRef', { title: ref.title })}
                  </button>
                ))}

                {message.sources.map((source) => (
                  <button
                    key={source.id}
                    type="button"
                    className="source-line"
                    onClick={() => getWebApp().openLink(source.url)}
                  >
                    <Icon name="external" size={12} />
                    {source.title}
                  </button>
                ))}
              </div>
            ))}

            {pending ? (
              // Сервер возвращает вопрос вместе с ответом, а ответ идёт секунды —
              // до тех пор показываем отправленный текст сами.
              <div className="ai-message ai-message-user">
                <p>{ask.variables.text}</p>
              </div>
            ) : null}
            {pending ? (
              <div className="ai-message ai-message-bot ai-dots" aria-label={t('ai.typing')}>
                <span />
                <span />
                <span />
              </div>
            ) : null}
          </div>

          {left.length > 0 ? (
            <div className="chips ai-suggestions" data-tour="ai-suggest">
              {left.map((question) => (
                <button key={question} type="button" className="chip chip-suggest" onClick={() => send(question)}>
                  {question}
                </button>
              ))}
            </div>
          ) : null}

          {failed ? (
            <p className="ai-error" role="alert">
              {failed.message}
            </p>
          ) : null}

          {/* Не <form>: IconButton из MAX UI принудительно ставит себе
              type="button", поэтому отправки по Enter через submit не случится.
              Enter обрабатывается на поле напрямую. */}
          <div className="ai-input" data-tour="ai-compose">
            <Input
              value={draft}
              // Длина вопроса ограничена 500 символами — ТЗ §6.4.
              maxLength={500}
              placeholder={t('ai.inputPlaceholder')}
              aria-label={t('ai.inputPlaceholder')}
              onChange={(event) => setDraft(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' && !event.shiftKey) {
                  event.preventDefault()
                  send(draft)
                }
              }}
            />
            <IconButton disabled={!draft.trim() || ask.isPending || active === undefined} onClick={() => send(draft)}>
              <Icon name="send" size={16} title={t('ai.send')} />
            </IconButton>
          </div>
        </>
      )}
    </Sheet>
  )
}
