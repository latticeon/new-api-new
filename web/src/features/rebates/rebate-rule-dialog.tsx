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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatQuota } from '@/lib/format'

import { getSelfRebateRule, setRebateOverride } from './api'

const schema = z.object({
  mode: z.enum(['inherit', 'on', 'off']),
  percent: z
    .string()
    .refine(
      (value) =>
        value.trim() === '' ||
        (Number.isFinite(Number(value)) &&
          Number(value) >= 0 &&
          Number(value) <= 100),
      'Enter a percentage between 0 and 100'
    ),
})
type Values = z.infer<typeof schema>

export function RebateOverrideDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
  mode?: string
  percent?: number | null
}) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    values: {
      mode:
        props.mode === 'on' || props.mode === 'off' ? props.mode : 'inherit',
      percent: props.percent == null ? '' : String(props.percent),
    },
  })
  const mutation = useMutation({
    mutationFn: (values: Values) =>
      setRebateOverride(
        props.userId,
        values.mode,
        values.percent.trim() === '' ? null : Number(values.percent)
      ),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ['users'] })
      props.onOpenChange(false)
    },
  })
  const modes = [
    { value: 'inherit', label: t('Follow global') },
    { value: 'on', label: t('Enabled') },
    { value: 'off', label: t('Disabled') },
  ]
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Inviter rebate override')}
      description={t(
        'Only enablement and percentage are overridden. Source switches, first/every rule and cumulative cap still apply.'
      )}
    >
      <Form {...form}>
        <form
          className='space-y-4'
          onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
        >
          <FormField
            control={form.control}
            name='mode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Rebate mode')}</FormLabel>
                <Select
                  items={modes}
                  value={field.value}
                  onValueChange={field.onChange}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {modes.map((mode) => (
                      <SelectItem key={mode.value} value={mode.value}>
                        {mode.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='percent'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Rebate percentage')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    type='number'
                    step='any'
                    min={0}
                    max={100}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Leave empty to inherit the global percentage. Explicit 0% produces no reward.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <Button type='submit' disabled={mutation.isPending}>
            {t('Save')}
          </Button>
        </form>
      </Form>
    </Dialog>
  )
}

export function SelfRebateRuleDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const query = useQuery({
    queryKey: ['self-rebate-rule'],
    queryFn: getSelfRebateRule,
    enabled: props.open,
  })
  const rule = query.data?.effective
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('My effective rebate rule')}
    >
      {query.isPending && <LoadingState />}
      {query.isError && <ErrorState onRetry={() => void query.refetch()} />}
      {rule && (
        <dl className='grid grid-cols-2 gap-3 text-sm'>
          <dt>{t('Status')}</dt>
          <dd>{rule.enabled ? t('Enabled') : t('Disabled')}</dd>
          <dt>{t('Rebate percentage')}</dt>
          <dd>{formatNumber(rule.percent, locale)}%</dd>
          <dt>{t('Rule')}</dt>
          <dd>
            {rule.rule === 'first'
              ? t('First qualifying payment')
              : t('Every qualifying payment')}
          </dd>
          <dt>{t('Reward quota per CNY')}</dt>
          <dd>{formatQuota(rule.quota_per_yuan)}</dd>
          <dt>{t('Cumulative cap per invitee')}</dt>
          <dd>
            {rule.max_per_invitee
              ? formatQuota(rule.max_per_invitee)
              : t('Unlimited')}
          </dd>
          <dt>{t('Reward destination')}</dt>
          <dd>
            {rule.destination === 'wallet'
              ? t('Wallet balance')
              : t('Pending invitation quota')}
          </dd>
          <dt>{t('Online recharge')}</dt>
          <dd>{rule.online ? t('Enabled') : t('Disabled')}</dd>
          <dt>{t('Subscription')}</dt>
          <dd>{rule.subscription ? t('Enabled') : t('Disabled')}</dd>
          <dt>{t('Redemption code')}</dt>
          <dd>{rule.redemption ? t('Enabled') : t('Disabled')}</dd>
          <dt>{t('Administrator adjustment')}</dt>
          <dd>{rule.admin ? t('Enabled') : t('Disabled')}</dd>
          {!query.data?.compliance_confirmed && (
            <dd className='col-span-2'>
              {t(
                'Rebates require administrator confirmation of payment compliance terms.'
              )}
            </dd>
          )}
        </dl>
      )}
    </Dialog>
  )
}
