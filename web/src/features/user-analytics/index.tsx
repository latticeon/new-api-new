import { useQuery } from '@tanstack/react-query'
import { VChart } from '@visactor/react-vchart'
import { CalendarDays, LoaderCircle } from 'lucide-react'
import { useMemo, useState } from 'react'
import type { DateRange } from 'react-day-picker'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  getUserAnalytics,
  type UserAnalyticsDaily,
} from '@/features/user-analytics/api'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'
import { VCHART_OPTION } from '@/lib/vchart'

const RANGE_OPTIONS = [7, 30, 90, 365]

function startOfLocalDay(date: Date) {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate())
}

function endOfLocalDay(date: Date) {
  return new Date(
    date.getFullYear(),
    date.getMonth(),
    date.getDate(),
    23,
    59,
    59
  )
}

function Stat(props: {
  title: string
  value?: number
  loading: boolean
  format: (value: number) => string
}) {
  return (
    <Card size='sm'>
      <CardHeader>
        <CardTitle className='text-muted-foreground text-xs font-medium'>
          {props.title}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {props.loading || props.value == null ? (
          <Skeleton className='h-7 w-20' />
        ) : (
          <div className='text-2xl font-semibold tabular-nums'>
            {props.format(props.value)}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function UserAnalyticsPage() {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [rangeDays, setRangeDays] = useState(30)
  const [customRange, setCustomRange] = useState<DateRange>()
  const [calendarOpen, setCalendarOpen] = useState(false)
  const dateRange = useMemo(() => {
    if (customRange?.from && customRange.to) {
      return { from: customRange.from, to: customRange.to }
    }
    const end = startOfLocalDay(new Date())
    const from = new Date(end)
    from.setDate(from.getDate() - (rangeDays - 1))
    return { from, to: end }
  }, [customRange, rangeDays])
  const requestParams = useMemo(
    () => ({
      from: Math.floor(dateRange.from.getTime() / 1000),
      to: Math.floor(endOfLocalDay(dateRange.to).getTime() / 1000),
      timezone_offset_minutes: -new Date().getTimezoneOffset(),
    }),
    [dateRange]
  )
  const query = useQuery({
    queryKey: ['admin', 'user-analytics', requestParams],
    queryFn: async () =>
      requireServerSuccess(await getUserAnalytics(requestParams)),
    staleTime: 60_000,
  })
  const data = query.data?.data
  const number = (value: number) => formatNumber(value, locale)
  const activitySpec = useMemo(
    () => ({
      type: 'line' as const,
      data: {
        values: (data?.daily ?? []).flatMap((row) => [
          {
            date: row.date,
            series: t('Registrations'),
            value: row.registrations,
          },
          {
            date: row.date,
            series: t('Active users'),
            value: row.active_users,
          },
        ]),
      },
      xField: 'date',
      yField: 'value',
      seriesField: 'series',
      point: { visible: true },
      legends: { visible: true, orient: 'bottom' },
    }),
    [data?.daily, t]
  )
  const paymentSpec = useMemo(
    () => ({
      type: 'bar' as const,
      data: {
        values: (data?.daily ?? []).flatMap((row) => [
          {
            date: row.date,
            series: t('Top-up orders'),
            value: row.topup_orders,
          },
          {
            date: row.date,
            series: t('Code redemptions'),
            value: row.redemptions,
          },
          {
            date: row.date,
            series: t('Paying users'),
            value: row.paying_users,
          },
        ]),
      },
      xField: 'date',
      yField: 'value',
      seriesField: 'series',
      legends: { visible: true, orient: 'bottom' },
    }),
    [data?.daily, t]
  )
  const formatRange = (range: DateRange) => {
    if (!range.from) return t('Select date range')
    const from = range.from.toLocaleDateString(locale)
    return range.to ? `${from} - ${range.to.toLocaleDateString(locale)}` : from
  }
  const getOrderRevenue = (row: UserAnalyticsDaily) => {
    const amount = row.revenue_by_currency?.UNKNOWN
    return amount ? `${number(amount)} ${t('Unknown currency')}` : '-'
  }

  return (
    <main className='space-y-5 p-4 md:p-6'>
      <header className='flex flex-wrap items-center justify-between gap-3'>
        <h1 className='text-xl font-semibold'>{t('User analytics')}</h1>
        <div className='flex flex-wrap items-center gap-2'>
          <div
            className='flex items-center rounded-md border p-0.5'
            role='group'
            aria-label={t('Date range')}
          >
            {RANGE_OPTIONS.map((days) => (
              <Button
                key={days}
                size='sm'
                variant={
                  !customRange && rangeDays === days ? 'secondary' : 'ghost'
                }
                onClick={() => {
                  setCustomRange(undefined)
                  setRangeDays(days)
                }}
              >
                {t('{{count}} days', { count: days })}
              </Button>
            ))}
          </div>
          <Popover open={calendarOpen} onOpenChange={setCalendarOpen}>
            <PopoverTrigger render={<Button variant='outline' size='sm' />}>
              <CalendarDays className='size-4' />
              <span>
                {customRange?.from
                  ? formatRange(customRange)
                  : t('Custom range')}
              </span>
            </PopoverTrigger>
            <PopoverContent align='end' className='w-auto p-0'>
              <Calendar
                mode='range'
                selected={customRange}
                onSelect={(value) => {
                  setCustomRange(value)
                  if (value?.from && value.to) setCalendarOpen(false)
                }}
                disabled={{ after: new Date() }}
                numberOfMonths={1}
              />
            </PopoverContent>
          </Popover>
        </div>
      </header>

      {query.isError ? (
        <ErrorState
          title={t('Unable to load user analytics')}
          onRetry={() => void query.refetch()}
        />
      ) : (
        <>
          <section className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
            <Stat
              title={t('Total users')}
              value={data?.total_users}
              loading={query.isLoading}
              format={number}
            />
            <Stat
              title={t('New users')}
              value={data?.new_users}
              loading={query.isLoading}
              format={number}
            />
            <Stat
              title={t('DAU')}
              value={data?.dau}
              loading={query.isLoading}
              format={number}
            />
            <Stat
              title={t('MAU')}
              value={data?.mau}
              loading={query.isLoading}
              format={number}
            />
            <Stat
              title={t('Top-up orders')}
              value={data?.topup_orders}
              loading={query.isLoading}
              format={number}
            />
            <Stat
              title={t('Code redemptions')}
              value={data?.redemptions}
              loading={query.isLoading}
              format={number}
            />
            <Stat
              title={t('Redeemed quota')}
              value={data?.redemption_quota}
              loading={query.isLoading}
              format={number}
            />
            <Stat
              title={t('Paying users')}
              value={data?.paying_users}
              loading={query.isLoading}
              format={number}
            />
          </section>

          <section className='grid gap-4 lg:grid-cols-2'>
            <Card className='min-w-0'>
              <CardHeader>
                <CardTitle>{t('Registration and login activity')}</CardTitle>
              </CardHeader>
              <CardContent className='h-72 min-w-0'>
                {data?.daily.length ? (
                  <VChart spec={activitySpec} option={VCHART_OPTION} />
                ) : (
                  <EmptyState title={t('No data')} className='min-h-48' />
                )}
              </CardContent>
            </Card>
            <Card className='min-w-0'>
              <CardHeader>
                <CardTitle>{t('Recharge activity')}</CardTitle>
              </CardHeader>
              <CardContent className='h-72 min-w-0'>
                {data?.daily.length ? (
                  <VChart spec={paymentSpec} option={VCHART_OPTION} />
                ) : (
                  <EmptyState title={t('No data')} className='min-h-48' />
                )}
              </CardContent>
            </Card>
          </section>

          <section className='space-y-3'>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <h2 className='text-base font-semibold'>
                {t('Recharge totals')}
              </h2>
              <div className='flex flex-wrap gap-2 text-sm'>
                {data?.revenue_by_currency.map((item) => (
                  <span
                    key={item.currency}
                    className='rounded-md border px-2 py-1'
                  >
                    {number(item.amount)}{' '}
                    {item.currency === 'UNKNOWN'
                      ? t('Unknown currency')
                      : item.currency}
                  </span>
                ))}
                <span className='rounded-md border px-2 py-1'>
                  {number(data?.redemption_revenue_cny ?? 0)} CNY{' '}
                  {t('Paid code redemptions')}
                </span>
              </div>
            </div>
            <div className='overflow-x-auto rounded-lg border'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Date')}</TableHead>
                    <TableHead>{t('Registrations')}</TableHead>
                    <TableHead>{t('Active users')}</TableHead>
                    <TableHead>{t('Successful logins')}</TableHead>
                    <TableHead>{t('Failed logins')}</TableHead>
                    <TableHead>{t('Top-up orders')}</TableHead>
                    <TableHead>{t('Code redemptions')}</TableHead>
                    <TableHead>{t('Redeemed quota')}</TableHead>
                    <TableHead>{t('Paying users')}</TableHead>
                    <TableHead>{t('Order amount')}</TableHead>
                    <TableHead>{t('Paid code amount (CNY)')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data?.daily.map((row) => (
                    <TableRow key={row.date}>
                      <TableCell>{row.date}</TableCell>
                      <TableCell>{number(row.registrations)}</TableCell>
                      <TableCell>{number(row.active_users)}</TableCell>
                      <TableCell>{number(row.successful_logins)}</TableCell>
                      <TableCell>{number(row.failed_logins)}</TableCell>
                      <TableCell>{number(row.topup_orders)}</TableCell>
                      <TableCell>{number(row.redemptions)}</TableCell>
                      <TableCell>{number(row.redemption_quota)}</TableCell>
                      <TableCell>{number(row.paying_users)}</TableCell>
                      <TableCell>{getOrderRevenue(row)}</TableCell>
                      <TableCell>
                        {number(row.redemption_revenue_cny)}
                      </TableCell>
                    </TableRow>
                  ))}
                  {!query.isLoading && !data?.daily.length && (
                    <TableRow>
                      <TableCell colSpan={11} className='h-24 text-center'>
                        {t('No data')}
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </div>
          </section>
          {query.isFetching && (
            <div className='text-muted-foreground flex items-center gap-2 text-xs'>
              <LoaderCircle className='size-3 animate-spin' />
              {t('Updating')}
            </div>
          )}
        </>
      )}
    </main>
  )
}

export { UserAnalyticsPage }
