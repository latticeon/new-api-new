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
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { handleServerError } from '@/lib/handle-server-error'
import { getServerErrorMessage } from '@/lib/server-error-message'

import { previewErrorRewrite } from './error-rewrite-api'
import { ErrorRewriteSourceSelect } from './error-rewrite-source-select'
import { parseErrorRewriteRules } from './lib/error-rewrite'

type Props = { rules: string }

export function ErrorRewritePreview(props: Props) {
  const { t } = useTranslation()
  const [message, setMessage] = useState('')
  const [source, setSource] = useState<'upstream' | 'local'>('upstream')
  const preview = useMutation({
    mutationFn: previewErrorRewrite,
    onError: (error) => handleServerError(error),
    meta: { errorToast: false },
  })
  const currentResult =
    preview.variables?.message === message &&
    preview.variables?.source === source &&
    preview.variables?.rules === props.rules
  return (
    <div className='space-y-3 rounded-xl border p-4'>
      <h4 className='font-medium'>{t('Rule preview')}</h4>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Preview uses the server regex engine and the current unsaved rules.'
        )}
      </p>
      <Label htmlFor='error-rewrite-preview-source'>
        {t('Preview source')}
      </Label>
      <ErrorRewriteSourceSelect
        id='error-rewrite-preview-source'
        preview
        value={source}
        onChange={(value) => {
          if (value !== 'all') setSource(value)
        }}
        disabled={preview.isPending}
      />
      <Label htmlFor='error-rewrite-preview-message'>
        {t('Original error message')}
      </Label>
      <Textarea
        id='error-rewrite-preview-message'
        value={message}
        onChange={(event) => setMessage(event.target.value)}
        disabled={preview.isPending}
        rows={3}
      />
      <Button
        type='button'
        variant='outline'
        disabled={preview.isPending}
        onClick={() => {
          try {
            parseErrorRewriteRules(props.rules)
            preview.mutate({ message, source, rules: props.rules })
          } catch (error) {
            handleServerError(
              new Error(
                t(
                  error instanceof Error
                    ? error.message
                    : 'Invalid error rewrite rules.'
                )
              )
            )
          }
        }}
      >
        {preview.isPending ? t('Loading...') : t('Preview')}
      </Button>
      <div
        role='status'
        aria-live='polite'
        className='break-words whitespace-pre-wrap'
      >
        {currentResult && preview.isSuccess && preview.data}
        {currentResult && preview.isError && (
          <p className='text-destructive'>
            {getServerErrorMessage(preview.error)}
          </p>
        )}
      </div>
    </div>
  )
}
