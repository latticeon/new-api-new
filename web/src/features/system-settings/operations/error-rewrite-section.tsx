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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { JsonCodeEditor } from '@/components/json-code-editor'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { handleServerError } from '@/lib/handle-server-error'
import { getServerErrorMessage } from '@/lib/server-error-message'

import {
  SettingsForm,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { saveErrorRewriteSettings } from './error-rewrite-api'
import { ErrorRewritePreview } from './error-rewrite-preview'
import { ErrorRewriteVisualEditor } from './error-rewrite-visual-editor'
import {
  parseErrorRewriteRules,
  type ErrorRewriteRule,
  type ErrorRewriteSettings,
} from './lib/error-rewrite'

type Props = { defaultValues: ErrorRewriteSettings }

export function ErrorRewriteSection(props: Props) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const [jsonMode, setJsonMode] = useState(() => {
    try {
      parseErrorRewriteRules(props.defaultValues.ErrorMessageRewriteRules)
      return false
    } catch {
      return true
    }
  })
  const schema = z.object({
    ErrorMessageRewriteEnabled: z.boolean(),
    ErrorMessageRewriteRules: z.string().superRefine((value, context) => {
      try {
        parseErrorRewriteRules(value)
      } catch (error) {
        context.addIssue({
          code: 'custom',
          message: t(
            error instanceof Error
              ? error.message
              : 'Invalid error rewrite rules.'
          ),
        })
      }
    }),
  })
  const form = useForm<ErrorRewriteSettings>({
    resolver: zodResolver(schema),
    defaultValues: props.defaultValues,
  })
  const rulesText = useWatch({
    control: form.control,
    name: 'ErrorMessageRewriteRules',
  })
  let rules: ErrorRewriteRule[] = []
  try {
    rules = parseErrorRewriteRules(rulesText)
  } catch {
    /* Keep invalid JSON intact for repair. */
  }
  const save = useMutation({
    mutationFn: saveErrorRewriteSettings,
    onSuccess: (_, values) => {
      form.reset(values)
      void client.invalidateQueries({ queryKey: ['system-options'] })
      toast.success(t('Saved successfully'))
    },
    onError: (error) => handleServerError(error),
    meta: { errorToast: false },
  })
  const onSubmit = form.handleSubmit((values) => {
    if (
      values.ErrorMessageRewriteEnabled &&
      parseErrorRewriteRules(values.ErrorMessageRewriteRules).length === 0
    ) {
      form.setError('ErrorMessageRewriteRules', {
        message: t('Add at least one rule before enabling error rewriting.'),
      })
      return
    }
    save.mutate(values)
  })
  return (
    <SettingsSection title={t('Error message rewriting')}>
      <Form {...form}>
        <SettingsForm onSubmit={onSubmit}>
          <SettingsPageFormActions
            onSave={onSubmit}
            isSaving={save.isPending}
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'Rules run by descending priority, then ascending name. The first match wins. Legacy rules apply to all sources; choose upstream to avoid changing local quota errors.'
            )}
          </p>
          <fieldset disabled={save.isPending} className='min-w-0 space-y-5'>
            <FormField
              control={form.control}
              name='ErrorMessageRewriteEnabled'
              render={({ field }) => (
                <SettingsSwitchField
                  controlId='error-rewrite-enabled'
                  label={t('Enable error message rewriting')}
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  disabled={save.isPending}
                />
              )}
            />
            <Button
              type='button'
              variant='outline'
              onClick={() => {
                if (jsonMode) {
                  try {
                    parseErrorRewriteRules(rulesText)
                  } catch (error) {
                    form.setError('ErrorMessageRewriteRules', {
                      message: t(
                        error instanceof Error
                          ? error.message
                          : 'Invalid error rewrite rules.'
                      ),
                    })
                    return
                  }
                }
                form.clearErrors('ErrorMessageRewriteRules')
                setJsonMode((current) => !current)
              }}
            >
              {jsonMode
                ? t('Switch to visual editor')
                : t('Switch to JSON editor')}
            </Button>
            <FormField
              control={form.control}
              name='ErrorMessageRewriteRules'
              render={({ field }) => (
                <FormItem>
                  {jsonMode ? (
                    <>
                      <FormLabel>{t('Error rewrite rules JSON')}</FormLabel>
                      <FormControl>
                        <JsonCodeEditor
                          value={field.value}
                          onChange={field.onChange}
                          name={field.name}
                          onBlur={field.onBlur}
                          textareaRef={field.ref}
                          ariaLabel={t('Error rewrite rules JSON')}
                          disabled={save.isPending}
                          aria-invalid={Boolean(
                            form.formState.errors.ErrorMessageRewriteRules
                          )}
                        />
                      </FormControl>
                    </>
                  ) : (
                    <ErrorRewriteVisualEditor
                      rules={rules}
                      onChange={(nextRules) => {
                        field.onChange(
                          JSON.stringify(
                            parseErrorRewriteRules(JSON.stringify(nextRules)),
                            null,
                            2
                          )
                        )
                        form.clearErrors('ErrorMessageRewriteRules')
                      }}
                    />
                  )}
                  <FormMessage />
                </FormItem>
              )}
            />
          </fieldset>
          {save.isError && (
            <p role='alert' className='text-destructive text-sm'>
              {getServerErrorMessage(save.error)}
            </p>
          )}
        </SettingsForm>
      </Form>
      <ErrorRewritePreview rules={rulesText} />
    </SettingsSection>
  )
}
