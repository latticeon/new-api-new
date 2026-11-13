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
import type { ComponentProps } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import type { ErrorRewriteRule } from './lib/error-rewrite'

type Props = Omit<
  ComponentProps<typeof SelectTrigger>,
  'value' | 'onChange'
> & {
  value: ErrorRewriteRule['source']
  onChange: (value: ErrorRewriteRule['source']) => void
  preview?: boolean
}

export function ErrorRewriteSourceSelect(props: Props) {
  const { t } = useTranslation()
  const { value, onChange, preview, ...triggerProps } = props
  const labels = { all: t('All'), upstream: t('Upstream'), local: t('Local') }
  return (
    <Select
      value={value}
      onValueChange={(nextValue) => {
        if (
          nextValue === 'all' ||
          nextValue === 'upstream' ||
          nextValue === 'local'
        ) {
          onChange(nextValue)
        }
      }}
      disabled={props.disabled}
    >
      <SelectTrigger
        aria-label={preview ? t('Preview source') : t('Rule source')}
        className='w-full'
        {...triggerProps}
      >
        <SelectValue>{labels[value]}</SelectValue>
      </SelectTrigger>
      <SelectContent>
        {!preview && <SelectItem value='all'>{labels.all}</SelectItem>}
        <SelectItem value='upstream'>{labels.upstream}</SelectItem>
        <SelectItem value='local'>{labels.local}</SelectItem>
      </SelectContent>
    </Select>
  )
}
