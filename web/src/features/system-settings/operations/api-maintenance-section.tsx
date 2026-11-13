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

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({ enabled: z.boolean(), message: z.string().max(2000) })
type Values = z.infer<typeof schema>

export function APIMaintenanceSection(props: { defaultValues: Values }) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: props.defaultValues,
  })
  useResetForm(form, props.defaultValues)

  const submit = async (values: Values) => {
    // Save the message before enabling to avoid an intermediate stale message.
    if (values.message !== props.defaultValues.message) {
      await update.mutateAsync({
        key: 'maintenance_setting.message',
        value: values.message,
      })
    }
    if (values.enabled !== props.defaultValues.enabled) {
      await update.mutateAsync({
        key: 'maintenance_setting.enabled',
        value: values.enabled,
      })
    }
  }

  return (
    <SettingsSection title={t('API maintenance')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(submit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(submit)}
            isSaving={update.isPending}
          />
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Pause new API executions')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Existing generations may finish. Management and existing task queries and downloads remain available.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={update.isPending}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />
          <FormField
            control={form.control}
            name='message'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Maintenance message')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    maxLength={2000}
                    disabled={update.isPending}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Returned to API clients with service unavailable status. Leave empty to use the default message.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
