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
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { JsonCodeEditor } from '@/components/json-code-editor'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'

import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const rule = z
  .object({
    path_prefix: z
      .string()
      .startsWith('/')
      .regex(/^[^?#\r\n]+$/),
    allow_origins: z
      .array(
        z.string().refine((origin) => {
          if (origin === '*') return true
          try {
            const url = new URL(origin)
            return (
              ['http:', 'https:'].includes(url.protocol) &&
              Boolean(url.hostname) &&
              !url.hostname.includes('*') &&
              !url.username &&
              !url.password &&
              !url.search &&
              !url.hash &&
              (url.pathname === '/' || url.pathname === '') &&
              !/[\s\\,?#]/.test(origin)
            )
          } catch {
            return false
          }
        })
      )
      .max(100),
    allow_headers: z
      .array(z.string().regex(/^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/))
      .max(100),
  })
  .strict()
const rulesSchema = z
  .array(rule)
  .max(100)
  .refine(
    (rules) =>
      new Set(rules.map((entry) => entry.path_prefix)).size === rules.length
  )
const schema = z.object({
  rules: z
    .string()
    .max(65536)
    .refine((value) => {
      try {
        return rulesSchema.safeParse(JSON.parse(value)).success
      } catch {
        return false
      }
    }, 'Invalid CORS rules'),
})

export function CORSSection(props: { defaultValues: { rules: string } }) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  const form = useForm<{ rules: string }>({
    resolver: zodResolver(schema),
    defaultValues: props.defaultValues,
  })
  useResetForm(form, props.defaultValues)
  const submit = form.handleSubmit(async (values) => {
    await update.mutateAsync({ key: 'cors_setting.rules', value: values.rules })
  })
  return (
    <SettingsSection title={t('Cross-origin requests')}>
      <Form {...form}>
        <SettingsForm onSubmit={submit}>
          <SettingsPageFormActions
            onSave={submit}
            isSaving={update.isPending}
          />
          <FormField
            control={form.control}
            name='rules'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('CORS rules')}</FormLabel>
                <FormControl>
                  <JsonCodeEditor
                    value={field.value}
                    onChange={field.onChange}
                    name={field.name}
                    onBlur={field.onBlur}
                    textareaRef={field.ref}
                    disabled={update.isPending}
                    ariaLabel={t('CORS rules')}
                    heightClassName='h-96 min-h-96 max-h-96'
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'The longest path_prefix wins. Set allow_origins and allow_headers to allowed lists; * allows any, and [] denies access. Origins include scheme and port, without a path. Cross-origin cookies are not enabled.'
                  )}
                </FormDescription>
                <FormMessage>
                  {form.formState.errors.rules
                    ? t('Invalid CORS rules')
                    : undefined}
                </FormMessage>
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
