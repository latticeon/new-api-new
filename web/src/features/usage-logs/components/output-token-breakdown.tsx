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
import { useTranslation } from 'react-i18next'

import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import type { LogOtherData } from '../types'
import { DetailRow } from './dialogs/log-detail-layout'

/** Only recorded observations are shown; older logs never acquire an inferred split. */
export function OutputTokenBreakdown(props: {
  other: LogOtherData | null
  details?: boolean
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const entries = [
    { label: t('Reasoning'), value: props.other?.reasoning_tokens },
    { label: t('Body'), value: props.other?.text_tokens },
  ].filter(
    (entry): entry is { label: string; value: number } =>
      typeof entry.value === 'number' &&
      Number.isSafeInteger(entry.value) &&
      entry.value >= 0
  )
  if (entries.length === 0) return null

  if (props.details) {
    return entries.map((entry) => (
      <DetailRow
        key={entry.label}
        label={entry.label}
        value={formatNumber(entry.value, locale)}
        mono
      />
    ))
  }

  return (
    <span className='text-muted-foreground flex min-w-0 flex-wrap gap-x-2 gap-y-0.5 text-xs'>
      {entries.map((entry) => (
        <span key={entry.label}>
          {entry.label}: {formatNumber(entry.value, locale)}
        </span>
      ))}
    </span>
  )
}
