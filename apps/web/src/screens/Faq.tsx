/** «Вопросы и ответы»: олимпиады, льготы, подготовка и сама «Траектория». */

import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Input } from '@maxhub/max-ui'
import { getWebApp } from '@/bridge'
import { FAQ, faqKey, searchFaq } from '@/lib/faq'
import { Icon } from '@/ui/Icon'
import { StateBlock } from '@/ui/primitives'
import { useSheetStack } from '@/ui/sheets'
import { useVoice } from '@/voice/useVoice'

export function FaqScreen() {
  const t = useVoice()
  const navigate = useNavigate()
  const sheets = useSheetStack()
  const [query, setQuery] = useState('')

  const sections = searchFaq(query, t)
  const searching = sections !== FAQ

  return (
    <div className="screen">
      <button type="button" className="link back-link" onClick={() => navigate('/')}>
        <Icon name="back" size={16} />
        {t('faq.back')}
      </button>

      <h1 className="family-title">{t('faq.title')}</h1>
      <p className="family-subtitle">{t('faq.subtitle')}</p>

      <Input
        type="search"
        value={query}
        placeholder={t('faq.search')}
        aria-label={t('faq.search')}
        iconBefore={<Icon name="search" size={16} />}
        onChange={(event) => setQuery(event.target.value)}
      />

      {sections.length === 0 ? (
        <StateBlock icon="search" title={t('faq.empty')}>
          <Button stretched variant="secondary" onClick={() => setQuery('')}>
            {t('faq.resetSearch')}
          </Button>
        </StateBlock>
      ) : (
        sections.map((section) => (
          <section key={section.id} className="block block-card faq-section">
            <h3 className="block-head">{t(section.titleKey)}</h3>
            {section.items.map((item) => (
              // При поиске найденное раскрыто сразу: иначе пришлось бы тыкать в каждый.
              <details key={`${item.id}${searching ? '-found' : ''}`} className="faq-item" open={searching}>
                <summary>
                  {t(faqKey(item.id, 'q'))}
                  <Icon name="chevron" size={16} />
                </summary>
                <p>{t(faqKey(item.id, 'a'))}</p>
                {item.source ? (
                  <button
                    type="button"
                    className="source-line"
                    onClick={() => getWebApp().openLink(item.source!.url)}
                  >
                    <Icon name="doc" size={13} />
                    {item.source.title}
                  </button>
                ) : null}
              </details>
            ))}
          </section>
        ))
      )}

      <section className="block block-card">
        <h3 className="block-head">{t('faq.askTitle')}</h3>
        <p className="block-text">{t('faq.askText')}</p>
        <Button stretched iconBefore={<Icon name="spark" size={15} />} onClick={() => sheets.open({ kind: 'ai', id: '' })}>
          {t('faq.askCta')}
        </Button>
      </section>
    </div>
  )
}
