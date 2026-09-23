/** Помощник — экраны F1 и F2. Функции F35–F37. */

import { useEffect, useRef, useState } from 'react'
import { IconButton, Input } from '@maxhub/max-ui'
import { useAiMessages, useAskAi } from '@/api/queries'
import { getWebApp } from '@/bridge'
import { Icon, Logo } from '@/ui/Icon'
import { Sheet } from '@/ui/Sheet'
import { SourceTag } from '@/ui/primitives'
import type { SheetStack } from '@/ui/sheets'
import { useRole, useVoice } from '@/voice/useVoice'

/** Подсказки-вопросы: с чего начать, если не знаешь, что спросить. */
const SUGGESTIONS: Record<'kid' | 'parent', string[]> = {
  kid: [
    'Какие льготы даёт «Высшая проба» в моих вузах?',
    'Чем БВИ отличается от 100 баллов?',
    'Можно ли поступить в ИТМО по Innopolis Open?',
  ],
  parent: [
    'Какие льготы даёт «Высшая проба» в вузах Артёма?',
    'Чем БВИ отличается от 100 баллов?',
    'Можно ли поступить в ИТМО по Innopolis Open?',
  ],
}

export function AiSheet({ sheets }: { sheets: SheetStack }) {
  const t = useVoice()
  const role = useRole()
  const [draft, setDraft] = useState('')
  const messages = useAiMessages()
  const ask = useAskAi()
  const logRef = useRef<HTMLDivElement>(null)

  const items = messages.data?.items ?? []

  // Новый ответ должен быть виден сразу, без ручной прокрутки.
  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight })
  }, [items.length, ask.isPending])

  const send = (text: string) => {
    const value = text.trim()
    if (!value || ask.isPending) return
    setDraft('')
    ask.mutate(value)
  }

  const asked = new Set(items.filter((m) => m.role === 'user').map((m) => m.text))
  if (ask.isPending) asked.add(ask.variables)
  const left = SUGGESTIONS[role].filter((q) => !asked.has(q))

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

        {ask.isPending ? (
          // Сервер возвращает вопрос вместе с ответом, а ответ идёт секунды —
          // до тех пор показываем отправленный текст сами.
          <div className="ai-message ai-message-user">
            <p>{ask.variables}</p>
          </div>
        ) : null}
        {ask.isPending ? (
          <div className="ai-message ai-message-bot ai-dots" aria-label="Помощник печатает">
            <span />
            <span />
            <span />
          </div>
        ) : null}
      </div>

      {left.length > 0 ? (
        <div className="chips ai-suggestions">
          {left.map((question) => (
            <button key={question} type="button" className="chip chip-suggest" onClick={() => send(question)}>
              {question}
            </button>
          ))}
        </div>
      ) : null}

      {/* Не <form>: IconButton из MAX UI принудительно ставит себе
          type="button", поэтому отправки по Enter через submit не случится.
          Enter обрабатывается на поле напрямую. */}
      <div className="ai-input">
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
        <IconButton disabled={!draft.trim() || ask.isPending} onClick={() => send(draft)}>
          <Icon name="send" size={16} title="Отправить" />
        </IconButton>
      </div>
    </Sheet>
  )
}
