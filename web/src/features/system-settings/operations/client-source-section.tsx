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

const encoder = new TextEncoder()
const aliasRule = z.object({
  pattern: z
    .string()
    .trim()
    .min(1)
    .refine(
      (value) => encoder.encode(value).length <= 256 && !/\p{Cc}/u.test(value)
    ),
  alias: z
    .string()
    .trim()
    .min(1)
    .refine(
      (value) => encoder.encode(value).length <= 128 && !/\p{Cc}/u.test(value)
    ),
})
const aliasRules = z.array(aliasRule).max(100)
const schema = z.object({
  aliases: z.string().refine((value) => {
    if (encoder.encode(value).length > 65536) return false
    try {
      return aliasRules.safeParse(JSON.parse(value)).success
    } catch {
      return false
    }
  }, 'Invalid User-Agent aliases'),
})

export function ClientSourceSection(props: {
  defaultValues: { aliases: string }
}) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  const form = useForm<{ aliases: string }>({
    resolver: zodResolver(schema),
    defaultValues: props.defaultValues,
  })
  useResetForm(form, props.defaultValues)
  const submit = form.handleSubmit(async (values) => {
    await update.mutateAsync({
      key: 'client_source_setting.aliases',
      value: values.aliases,
    })
  })
  return (
    <SettingsSection title={t('Call source')}>
      <Form {...form}>
        <SettingsForm onSubmit={submit}>
          <SettingsPageFormActions
            onSave={submit}
            isSaving={update.isPending}
          />
          <FormField
            control={form.control}
            name='aliases'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('User-Agent aliases')}</FormLabel>
                <FormControl>
                  <JsonCodeEditor
                    value={field.value}
                    onChange={field.onChange}
                    name={field.name}
                    onBlur={field.onBlur}
                    textareaRef={field.ref}
                    disabled={update.isPending}
                    ariaLabel={t('User-Agent aliases')}
                    placeholder='[{"pattern":"cherry","alias":"Cherry Studio"}]'
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Match User-Agent by case-insensitive substring in order. The first match sets the display alias. Use [] for no aliases. Raw User-Agent is retained; aliases do not verify client identity.'
                  )}
                </FormDescription>
                <FormDescription>
                  {t(
                    'Up to 100 aliases, with pattern up to 256 bytes and alias up to 128 bytes.'
                  )}
                </FormDescription>
                <FormMessage>
                  {form.formState.errors.aliases
                    ? t('Invalid User-Agent aliases')
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
