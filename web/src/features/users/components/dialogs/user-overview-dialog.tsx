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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatQuota } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getUser } from '../../api'

export function UserOverviewDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const query = useQuery({
    queryKey: ['users', props.userId, 'overview'],
    enabled: props.open,
    queryFn: async () => {
      const result = requireServerSuccess(await getUser(props.userId))
      if (!result.data) throw new Error(t('User not found'))
      return result.data
    },
  })
  const user = query.data
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Account overview')}
      description={t('Read-only account and invitation information')}
    >
      {query.isPending && <LoadingState />}
      {query.isError && <ErrorState onRetry={() => void query.refetch()} />}
      {user && (
        <>
          <dl className='grid min-w-0 grid-cols-2 gap-3 text-sm [&_dd]:break-all'>
            <dt>{t('Username')}</dt>
            <dd>{user.username}</dd>
            <dt>{t('Display Name')}</dt>
            <dd>{user.display_name || '—'}</dd>
            <dt>{t('Email')}</dt>
            <dd>{user.email || '—'}</dd>
            <dt>{t('Email verification status')}</dt>
            <dd>
              {user.email ? t('Verification not recorded') : t('Not bound')}
            </dd>
            <dt>{t('Requests')}</dt>
            <dd>{formatNumber(user.request_count, locale)}</dd>
            <dt>{t('User Group')}</dt>
            <dd>{user.group}</dd>
            <dt>{t('Available Balance')}</dt>
            <dd>{formatQuota(user.quota)}</dd>
            <dt>{t('Total Used')}</dt>
            <dd>{formatQuota(user.used_quota)}</dd>
            <dt>{t('Invitation code')}</dt>
            <dd className='flex items-center gap-2'>
              {user.aff_code || '—'}
              {user.aff_code && <CopyButton value={user.aff_code} />}
            </dd>
            <dt>{t('Pending invitation quota')}</dt>
            <dd>{formatQuota(user.aff_quota ?? 0)}</dd>
            <dt>{t('Inviter ID')}</dt>
            <dd>{user.inviter_id || '—'}</dd>
            <dt>{t('Invited users')}</dt>
            <dd>{formatNumber(user.aff_count ?? 0, locale)}</dd>
            <dt>{t('Total invitation earnings')}</dt>
            <dd>{formatQuota(user.aff_history_quota ?? 0)}</dd>
          </dl>
        </>
      )}
    </Dialog>
  )
}
