/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import { ErrorRewriteRuleDialog } from './error-rewrite-rule-dialog'
import type { ErrorRewriteRule } from './lib/error-rewrite'

type Props = {
  rules: ErrorRewriteRule[]
  onChange: (rules: ErrorRewriteRule[]) => void
}

export function ErrorRewriteVisualEditor(props: Props) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [editing, setEditing] = useState<{
    rule: ErrorRewriteRule | null
    index: number | null
  } | null>(null)
  const [deleting, setDeleting] = useState<number | null>(null)
  const sourceLabels = {
    all: t('All'),
    upstream: t('Upstream'),
    local: t('Local'),
  }
  const matchLabels = {
    exact_any: t('Exact match any'),
    contains_any: t('Contains any'),
    regex_any: t('Regex match any'),
  }
  return (
    <div className='space-y-4'>
      <Button
        type='button'
        onClick={() => setEditing({ rule: null, index: null })}
      >
        {t('Add rule')}
      </Button>
      <StaticDataTable
        data={props.rules}
        getRowKey={(_, index) => index}
        emptyContent={t('No rules configured.')}
        columns={[
          {
            id: 'enabled',
            header: t('Enabled'),
            cell: (rule, index) => (
              <Switch
                checked={rule.enabled}
                aria-label={t('Enable rule {{name}}', { name: rule.key })}
                onCheckedChange={(enabled) =>
                  props.onChange(
                    props.rules.map((item, itemIndex) =>
                      itemIndex === index ? { ...item, enabled } : item
                    )
                  )
                }
              />
            ),
          },
          { id: 'key', header: t('Rule name'), cell: (rule) => rule.key },
          {
            id: 'priority',
            header: t('Priority'),
            cell: (rule) => formatNumber(rule.priority, locale),
          },
          {
            id: 'source',
            header: t('Rule source'),
            cell: (rule) => sourceLabels[rule.source],
          },
          {
            id: 'match',
            header: t('Match type'),
            cell: (rule) => matchLabels[rule.match_type],
          },
          {
            id: 'patterns',
            header: t('Match patterns'),
            cell: (rule) => (
              <span className='block max-w-64 break-words whitespace-pre-wrap'>
                {rule.patterns.join('\n')}
              </span>
            ),
          },
          {
            id: 'replace',
            header: t('Replacement message'),
            cell: (rule) => (
              <span className='block max-w-64 break-words whitespace-pre-wrap'>
                {rule.replace}
              </span>
            ),
          },
          {
            id: 'actions',
            header: t('Actions'),
            cell: (rule, index) => (
              <div className='flex gap-2'>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  onClick={() => setEditing({ rule, index })}
                >
                  {t('Edit')}
                </Button>
                <Button
                  type='button'
                  size='sm'
                  variant='destructive'
                  onClick={() => setDeleting(index)}
                >
                  {t('Delete')}
                </Button>
              </div>
            ),
          },
        ]}
      />
      {editing && (
        <ErrorRewriteRuleDialog
          rule={editing.rule}
          otherKeys={props.rules
            .filter((_, index) => index !== editing.index)
            .map((rule) => rule.key)}
          onClose={() => setEditing(null)}
          onSave={(rule) => {
            const next = [...props.rules]
            if (editing.index === null) next.push(rule)
            else next[editing.index] = rule
            props.onChange(next)
            setEditing(null)
          }}
        />
      )}
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title={t('Delete rule')}
        desc={t('Delete rule {{name}}?', {
          name: deleting === null ? '' : (props.rules[deleting]?.key ?? ''),
        })}
        confirmText={t('Delete')}
        destructive
        handleConfirm={() => {
          props.onChange(props.rules.filter((_, index) => index !== deleting))
          setDeleting(null)
        }}
      />
    </div>
  )
}
