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
import { useTranslation } from 'react-i18next'

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
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import {
  SettingsForm,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'
import {
  arrivalSettingsSchema,
  type ArrivalSettingsValues,
} from './arrival-rebate-settings'

export function ArrivalSettingsSection(props: {
  defaultValues: ArrivalSettingsValues
}) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  const settings = useSettingsForm({
    defaultValues: props.defaultValues,
    resolver: zodResolver(arrivalSettingsSchema),
    onSubmit: async (_values, changed) => {
      // Persist the rule before enabling it; disable first when turning it off.
      const entries = Object.entries(changed).sort(([a], [b]) => {
        if (a === 'enabled') return changed.enabled ? 1 : -1
        if (b === 'enabled') return changed.enabled ? -1 : 1
        return 0
      })
      for (const [key, value] of entries) {
        await update.mutateAsync({
          key: `arrival_setting.${key}`,
          value: value as string | number | boolean,
        })
      }
    },
  })
  const form = settings.form
  const save = () => settings.handleSubmit().catch(() => undefined)

  return (
    <SettingsSection title={t('Arrival auto-grouping')}>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Counts successful wallet arrivals, not current balance. Subscription payments and periodic resets do not count.'
        )}
      </p>
      <Form {...form}>
        <SettingsForm
          onSubmit={(event) => {
            event.preventDefault()
            void save()
          }}
        >
          <SettingsPageFormActions
            onSave={save}
            onReset={settings.handleReset}
            isSaving={settings.isSubmitting}
          />
          <fieldset disabled={settings.isSubmitting} className='contents'>
            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <SettingsSwitchField
                  controlId='arrival-enabled'
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  label={t('Enable arrival auto-grouping')}
                />
              )}
            />
            <FormField
              control={form.control}
              name='mode'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Arrival threshold mode')}</FormLabel>
                  <FormControl>
                    <NativeSelect {...field}>
                      <NativeSelectOption value='single'>
                        {t('Single arrival')}
                      </NativeSelectOption>
                      <NativeSelectOption value='cumulative'>
                        {t('Cumulative arrivals')}
                      </NativeSelectOption>
                    </NativeSelect>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='threshold'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Arrival threshold (quota units)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={Number.MAX_SAFE_INTEGER}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='source_group'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Source group')}</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormDescription>
                    {t('Leave empty to allow any source group')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='target_group'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Target group')}</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='include_admin'
              render={({ field }) => (
                <SettingsSwitchField
                  controlId='arrival-admin'
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  label={t('Count administrator additions')}
                  description={t(
                    'Only positive quota changes count; deductions never count as arrivals.'
                  )}
                />
              )}
            />
          </fieldset>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
