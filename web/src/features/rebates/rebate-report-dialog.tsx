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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DataTablePagination,
  StaticDataTable,
  useDataTable,
} from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatQuota, formatTimestampToDate } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

import {
  getRebateOverview,
  getRebateRows,
  retryRebate,
  type RebateEvent,
  type RebateFilter,
  type RebateParticipant,
} from './api'

type View = 'ledger' | 'ranking' | 'invitees'
const paginationColumns: ColumnDef<RebateEvent & RebateParticipant>[] = []

export function RebateReportDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  inviterId?: number
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const queryClient = useQueryClient()
  const evidenceLabels: Record<string, string> = {
    provider_payment: t('Order payment amount'),
    registered_sale: t('Registered code sale price'),
    registered_receipt: t('Registered administrator receipt'),
  }
  const [view, setView] = useState<View>('ledger')
  const [filter, setFilter] = useState<RebateFilter>({
    sort: 'created_at',
    order: 'desc',
  })
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: 10 })
  const params = {
    ...filter,
    inviter_id: props.inviterId,
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
  }
  const overview = useQuery({
    queryKey: ['rebates', 'overview', params],
    queryFn: () => getRebateOverview(params),
    enabled: props.open,
  })
  const rows = useQuery({
    queryKey: ['rebates', view, params],
    queryFn: () => getRebateRows<RebateEvent & RebateParticipant>(view, params),
    enabled: props.open,
  })
  const retry = useMutation({
    mutationFn: retryRebate,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['rebates'] }),
    onError: (error) => handleServerError(error),
  })
  const table = useDataTable({
    data: rows.data?.items ?? [],
    columns: paginationColumns,
    totalCount: rows.data?.total ?? 0,
    manualPagination: true,
    pagination,
    onPaginationChange: setPagination,
  })
  const changeFilter = (
    name: keyof RebateFilter,
    value: string | number | undefined
  ) => {
    setFilter((previous) => ({ ...previous, [name]: value }))
    setPagination((previous) => ({ ...previous, pageIndex: 0 }))
  }
  const options = {
    view: [
      { value: 'ledger', label: t('Reward ledger') },
      { value: 'ranking', label: t('Inviter ranking') },
      { value: 'invitees', label: t('Invited users') },
    ],
    source: [
      { value: 'all', label: t('All sources') },
      { value: 'online', label: t('Online recharge') },
      { value: 'subscription', label: t('Subscription') },
      { value: 'redemption', label: t('Redemption code') },
      { value: 'admin', label: t('Administrator adjustment') },
    ],
    status: [
      { value: 'all', label: t('All statuses') },
      { value: 'paid', label: t('Paid') },
      { value: 'pending', label: t('Pending') },
      { value: 'skipped', label: t('Skipped') },
    ],
    sort:
      view === 'ledger'
        ? [
            { value: 'created_at', label: t('Time') },
            { value: 'reward', label: t('Reward') },
            { value: 'percent', label: t('Rebate percentage') },
            { value: 'inviter_id', label: t('Inviter ID') },
            { value: 'user_id', label: t('User ID') },
          ]
        : [
            { value: 'reward', label: t('Reward') },
            { value: 'events', label: t('Events') },
            { value: 'pending', label: t('Pending') },
            { value: 'username', label: t('Username') },
          ],
    order: [
      { value: 'desc', label: t('Descending') },
      { value: 'asc', label: t('Ascending') },
    ],
  }
  const selectValues = {
    view,
    source: filter.source || 'all',
    status: filter.status || 'all',
    sort: filter.sort,
    order: filter.order,
  }
  const selectLabels = {
    view: t('Report'),
    source: t('Source'),
    status: t('Status'),
    sort: t('Sort by'),
    order: t('Sort order'),
  }
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Invitation and rebate reports')}
      description={t(
        'Statistics use the durable business ledger. Missing historical details are not reconstructed.'
      )}
      contentClassName='sm:max-w-6xl'
    >
      <div className='space-y-4'>
        {props.inviterId && (
          <p>
            {t('Inviter ID')}: {props.inviterId}
          </p>
        )}
        <div className='grid gap-3 sm:grid-cols-3 lg:grid-cols-5'>
          {(Object.keys(options) as (keyof typeof options)[]).map((name) => (
            <div key={name} className='space-y-1'>
              <Label htmlFor={`rebate-${name}`}>{selectLabels[name]}</Label>
              <Select
                items={options[name]}
                value={selectValues[name]}
                onValueChange={(value) => {
                  if (!value) return
                  if (name === 'view') {
                    setView(value as View)
                    changeFilter(
                      'sort',
                      value === 'ledger' ? 'created_at' : 'reward'
                    )
                  } else changeFilter(name, value === 'all' ? undefined : value)
                }}
              >
                <SelectTrigger id={`rebate-${name}`}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {options[name].map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          ))}
          <div className='space-y-1'>
            <Label htmlFor='rebate-search'>{t('Search')}</Label>
            <Input
              id='rebate-search'
              value={filter.search ?? ''}
              onChange={(event) => changeFilter('search', event.target.value)}
            />
          </div>
          <div className='space-y-1'>
            <Label htmlFor='rebate-start'>{t('Start date')}</Label>
            <Input
              id='rebate-start'
              type='date'
              onChange={(event) =>
                changeFilter(
                  'start',
                  event.target.value
                    ? Math.floor(
                        new Date(`${event.target.value}T00:00:00`).getTime() /
                          1000
                      )
                    : undefined
                )
              }
            />
          </div>
          <div className='space-y-1'>
            <Label htmlFor='rebate-end'>{t('End date')}</Label>
            <Input
              id='rebate-end'
              type='date'
              onChange={(event) =>
                changeFilter(
                  'end',
                  event.target.value
                    ? Math.floor(
                        new Date(`${event.target.value}T23:59:59`).getTime() /
                          1000
                      )
                    : undefined
                )
              }
            />
          </div>
        </div>
        {overview.data && (
          <dl className='grid grid-cols-2 gap-3 text-sm sm:grid-cols-5'>
            <div>
              <dt>{t('Events')}</dt>
              <dd>{formatNumber(overview.data.events, locale)}</dd>
            </div>
            <div>
              <dt>{t('Invited users')}</dt>
              <dd>{formatNumber(overview.data.invitees, locale)}</dd>
            </div>
            <div>
              <dt>{t('Paid rewards')}</dt>
              <dd>{formatQuota(overview.data.paid_reward)}</dd>
            </div>
            <div>
              <dt>{t('Pending rewards')}</dt>
              <dd>{formatQuota(overview.data.pending_reward)}</dd>
            </div>
            <div>
              <dt>{t('Skipped')}</dt>
              <dd>{formatNumber(overview.data.skipped, locale)}</dd>
            </div>
          </dl>
        )}
        {(rows.isPending || overview.isPending) && <LoadingState />}
        {(rows.isError || overview.isError) && (
          <ErrorState
            onRetry={() => {
              void rows.refetch()
              void overview.refetch()
            }}
          />
        )}
        {rows.data && view === 'ledger' && (
          <StaticDataTable
            data={rows.data.items}
            getRowKey={(row) => row.id}
            emptyContent={t('No data')}
            columns={[
              {
                id: 'time',
                header: t('Time'),
                cell: (row) => formatTimestampToDate(row.created_at),
              },
              {
                id: 'identity',
                header: t('Inviter / invitee'),
                cell: (row) => (
                  <div>
                    {row.inviter_name || '—'} (#{row.inviter_id}) →{' '}
                    {row.invitee_name} (#{row.user_id})
                  </div>
                ),
              },
              {
                id: 'source',
                header: t('Source reference'),
                cell: (row) => (
                  <div>
                    {options.source.find(
                      (source) => source.value === row.source
                    )?.label ?? row.source}
                    <div>{row.reference}</div>
                  </div>
                ),
              },
              {
                id: 'amount',
                header: t('Amount evidence'),
                cell: (row) => (
                  <div>
                    {evidenceLabels[row.amount_evidence] ??
                      t('Amount unavailable')}
                    <div>
                      {row.amount || '—'} {row.currency}
                    </div>
                    <div>
                      {t('Calculation amount (CNY)')}: {row.amount_cny || '—'}
                    </div>
                    <div>
                      {t('CNY conversion rate')}: {row.cny_rate || '—'}
                    </div>
                  </div>
                ),
              },
              {
                id: 'reward',
                header: t('Reward'),
                cell: (row) => (
                  <div>
                    {formatNumber(row.percent, locale)}% ·{' '}
                    {formatQuota(row.reward)}
                    <div>
                      {row.destination === 'wallet'
                        ? t('Wallet balance')
                        : t('Pending invitation quota')}
                    </div>
                  </div>
                ),
              },
              {
                id: 'status',
                header: t('Status / reason'),
                cell: (row) => (
                  <div>
                    {options.status.find(
                      (status) => status.value === row.status
                    )?.label ?? row.status}
                    <div>{row.reason || '—'}</div>
                    {row.status === 'pending' && (
                      <Button
                        size='sm'
                        variant='outline'
                        disabled={retry.isPending}
                        onClick={() => retry.mutate(row.id)}
                      >
                        {t('Retry reward')}
                      </Button>
                    )}
                  </div>
                ),
              },
              {
                id: 'snapshot',
                header: t('Configuration snapshot'),
                cell: (row) => (
                  <details>
                    <summary>{t('Details')}</summary>
                    <pre className='max-w-80 text-xs break-all whitespace-pre-wrap'>
                      {row.rebate_snapshot || '—'}
                    </pre>
                  </details>
                ),
              },
            ]}
          />
        )}
        {rows.data && view !== 'ledger' && (
          <StaticDataTable
            data={rows.data.items}
            getRowKey={(row) => row.id}
            emptyContent={t('No data')}
            columns={[
              {
                id: 'user',
                header: t('Username'),
                cell: (row) => `${row.username} (#${row.id})`,
              },
              {
                id: 'inviter',
                header: t('Inviter ID'),
                cell: (row) => row.inviter_id || '—',
              },
              {
                id: 'events',
                header: t('Events'),
                cell: (row) => formatNumber(row.events, locale),
              },
              {
                id: 'reward',
                header: t('Paid rewards'),
                cell: (row) => formatQuota(row.reward),
              },
              {
                id: 'pending',
                header: t('Pending rewards'),
                cell: (row) => formatQuota(row.pending),
              },
            ]}
          />
        )}
        <DataTablePagination table={table.table} compact />
      </div>
    </Dialog>
  )
}
