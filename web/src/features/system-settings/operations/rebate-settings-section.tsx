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
  rebateSettingsSchema,
  type RebateSettingsValues,
} from './arrival-rebate-settings'

export function RebateSettingsSection(props: {
  defaultValues: RebateSettingsValues
}) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  const settings = useSettingsForm({
    defaultValues: props.defaultValues,
    resolver: zodResolver(rebateSettingsSchema),
    onSubmit: async (_values, changed) => {
      const entries = Object.entries(changed).sort(([a], [b]) => {
        if (a === 'enabled') return changed.enabled ? 1 : -1
        if (b === 'enabled') return changed.enabled ? -1 : 1
        return 0
      })
      for (const [key, value] of entries) {
        await update.mutateAsync({
          key: `rebate_setting.${key}`,
          value: value as string | number | boolean,
        })
      }
    },
  })
  const form = settings.form
  const save = () => settings.handleSubmit().catch(() => undefined)

  return (
    <SettingsSection title={t('Global arrival rebates')}>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Rules remain editable when globally disabled. Only direct inviters receive rebates.'
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
                  controlId='rebate-enabled'
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  label={t('Enable global rebates')}
                />
              )}
            />
            <FormField
              control={form.control}
              name='rule'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Rebate frequency')}</FormLabel>
                  <FormControl>
                    <NativeSelect {...field}>
                      <NativeSelectOption value='first'>
                        {t('First qualifying arrival')}
                      </NativeSelectOption>
                      <NativeSelectOption value='every'>
                        {t('Every qualifying arrival')}
                      </NativeSelectOption>
                    </NativeSelect>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='percent'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Rebate percentage (%)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={100}
                      step='any'
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='max_per_invitee'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Lifetime cap per invitee (quota units)')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={Number.MAX_SAFE_INTEGER}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Zero means unlimited; pending and paid rebates share this cap.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='quota_per_yuan'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Reward quota units per CNY')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={Number.MAX_SAFE_INTEGER}
                      step='any'
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='destination'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Rebate destination')}</FormLabel>
                  <FormControl>
                    <NativeSelect {...field}>
                      <NativeSelectOption value='wallet'>
                        {t('Wallet balance')}
                      </NativeSelectOption>
                      <NativeSelectOption value='affiliate'>
                        {t('Affiliate balance')}
                      </NativeSelectOption>
                    </NativeSelect>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='online'
              render={({ field }) => (
                <SettingsSwitchField
                  controlId='rebate-online'
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  label={t('Online top-ups — actual paid order amount')}
                />
              )}
            />
            <FormField
              control={form.control}
              name='subscription'
              render={({ field }) => (
                <SettingsSwitchField
                  controlId='rebate-subscription'
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  label={t('Subscription purchases — actual paid order amount')}
                />
              )}
            />
            <FormField
              control={form.control}
              name='redemption'
              render={({ field }) => (
                <SettingsSwitchField
                  controlId='rebate-redemption'
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  label={t('Redemption codes — registered sale price (CNY)')}
                />
              )}
            />
            <FormField
              control={form.control}
              name='admin'
              render={({ field }) => (
                <SettingsSwitchField
                  controlId='rebate-admin'
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  label={t(
                    'Administrator additions — declared received amount (CNY)'
                  )}
                  description={t(
                    'Registered amounts are administrator declarations, not verified online payments. Deductions cannot earn rebates.'
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
