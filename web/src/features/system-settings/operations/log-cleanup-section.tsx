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
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatTimestampToDate } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'

import { listSystemTasks } from '../api'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { HistoryLogCleanup } from '../maintenance/history-log-cleanup'
import { safeNumberFieldProps } from '../utils/numeric-field'

const schema = z.object({
  enabled: z.boolean(),
  days: z.number().int().min(1).max(36500),
  scope: z.enum(['consume_error', 'consume', 'error']),
})
type Values = z.infer<typeof schema>
export type LogCleanupSettings = {
  'log_setting.auto_cleanup_enabled': boolean
  'log_setting.cleanup_retention_days': number
  'log_setting.cleanup_scope': Values['scope']
}

export function LogCleanupSection(props: {
  defaultValues: LogCleanupSettings
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const update = useUpdateOption()
  const defaults = useMemo(
    () => ({
      enabled: props.defaultValues['log_setting.auto_cleanup_enabled'],
      days: props.defaultValues['log_setting.cleanup_retention_days'],
      scope: props.defaultValues['log_setting.cleanup_scope'],
    }),
    [props.defaultValues]
  )
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: defaults,
  })
  useEffect(() => {
    form.reset(defaults)
  }, [form, defaults])
  const tasks = useQuery({
    queryKey: ['system-tasks', 'log-cleanup-results'],
    queryFn: async () =>
      requireServerSuccess(await listSystemTasks(5, { type: 'log_cleanup' })),
    refetchInterval: 5000,
  })
  const onSubmit = async (values: Values) => {
    // Disable first; enable only after retention/scope have been persisted.
    const changes = [
      ['log_setting.auto_cleanup_enabled', false],
      ['log_setting.cleanup_retention_days', values.days],
      ['log_setting.cleanup_scope', values.scope],
      ['log_setting.auto_cleanup_enabled', values.enabled],
    ] as const
    const saved = { ...props.defaultValues }
    for (const [key, value] of changes) {
      if (saved[key] === value) continue
      await update.mutateAsync({ key, value })
      Object.assign(saved, { [key]: value })
    }
  }
  const statuses = {
    pending: t('Pending'),
    running: t('Running'),
    succeeded: t('Succeeded'),
    failed: t('Failed'),
  }
  return (
    <SettingsSection title={t('Database log cleanup')}>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Runs daily in bounded batches using system tasks. Disabled by default. Only consumption/error logs can be removed; financial, rebate and security audit records are protected. Disk log files are not affected.'
        )}
      </p>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={update.isPending || form.formState.isSubmitting}
            saveLabel='Save log cleanup settings'
          />
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Automatic database log cleanup')}</FormLabel>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </SettingsSwitchItem>
            )}
          />
          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='days'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Log retention (days)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      max={36500}
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
              name='scope'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Log cleanup scope')}</FormLabel>
                  <FormControl>
                    <NativeSelect {...field}>
                      <NativeSelectOption value='consume_error'>
                        {t('Consumption and error logs')}
                      </NativeSelectOption>
                      <NativeSelectOption value='consume'>
                        {t('Consumption logs only')}
                      </NativeSelectOption>
                      <NativeSelectOption value='error'>
                        {t('Error logs only')}
                      </NativeSelectOption>
                    </NativeSelect>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
      <HistoryLogCleanup scope={form.watch('scope')} />
      <section
        aria-label={t('Recent log cleanup results')}
        className='flex flex-col gap-3'
      >
        <h4 className='font-medium'>{t('Recent log cleanup results')}</h4>
        {tasks.isPending && <p role='status'>{t('Loading...')}</p>}
        {tasks.isError && (
          <p role='alert'>{t('Failed to load log cleanup results')}</p>
        )}
        {tasks.data?.data?.length === 0 && (
          <p>{t('No log cleanup tasks yet')}</p>
        )}
        {tasks.data?.data?.map((task) => (
          <div key={task.task_id} className='rounded-md border p-3 text-sm'>
            <p>
              {formatTimestampToDate(task.created_at)} · {statuses[task.status]}
            </p>
            <p>
              {t('Deleted log entries: {{count}}', {
                count: formatNumber(
                  Number(
                    task.result?.deleted_count ?? task.state?.processed ?? 0
                  ),
                  locale
                ),
              })}
            </p>
            {task.error && <p className='text-destructive'>{t(task.error)}</p>}
          </div>
        ))}
        <Button
          type='button'
          variant='outline'
          disabled={tasks.isFetching}
          onClick={() => void tasks.refetch()}
        >
          {t('Refresh')}
        </Button>
      </section>
    </SettingsSection>
  )
}
