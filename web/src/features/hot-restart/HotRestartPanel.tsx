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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { getServerErrorMessage } from '@/lib/server-error-message'

import { SettingsSection } from '../system-settings/components/settings-section'
import { getHotRestartStatus, requestHotRestart } from './api'

const statusKey = ['hot-restart', 'status'] as const

// Mount only inside an existing root-administrator settings route. The API
// independently enforces RootAuth; hiding this component is not authorization.
export function HotRestartPanel() {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const client = useQueryClient()
  const [confirming, setConfirming] = useState(false)
  const status = useQuery({
    queryKey: statusKey,
    queryFn: getHotRestartStatus,
    refetchInterval: 2000,
    meta: { errorToast: false },
  })
  const restart = useMutation({
    mutationFn: requestHotRestart,
    retry: false,
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: statusKey })
      setConfirming(false)
    },
    onError: (error) => handleServerError(error),
    meta: { errorToast: false },
  })

  if (status.isPending) return <LoadingState />
  if (status.isError) {
    return (
      <ErrorState
        description={getServerErrorMessage(status.error)}
        onRetry={() => void status.refetch()}
      />
    )
  }

  const data = status.data
  const disabled =
    !data.supported || data.pending || data.draining || restart.isPending
  let phase = t('Ready for hot restart')
  if (!data.supported) {
    phase = t('Hot restart is unavailable')
  } else if (data.draining) {
    phase = t('Draining existing connections')
  } else if (data.pending) {
    phase = t('Replacement startup or connection drain in progress')
  }

  return (
    <SettingsSection title={t('Linux graceful hot restart')}>
      <p className='text-muted-foreground text-sm'>
        {t(
          'A ready replacement accepts new connections while the previous worker drains HTTP, SSE and WebSocket connections. Long-lived connections delay the next restart.'
        )}
      </p>
      <p role='status'>{phase}</p>
      {!data.supported && (
        <Alert>
          <AlertDescription>
            {t(
              'Requires Linux, HOT_RESTART_ENABLED=true and a stable SESSION_SECRET. Other platforms are not supported.'
            )}
          </AlertDescription>
        </Alert>
      )}
      <dl className='grid min-w-0 grid-cols-1 gap-2 text-sm sm:grid-cols-2'>
        <div>
          <dt className='text-muted-foreground'>{t('Worker PID')}</dt>
          <dd>{formatNumber(data.pid, locale)}</dd>
        </div>
        <div>
          <dt className='text-muted-foreground'>{t('Version')}</dt>
          <dd className='break-all'>{data.version}</dd>
        </div>
        <div>
          <dt className='text-muted-foreground'>
            {t('Background leadership')}
          </dt>
          <dd>
            {data.background ? t('Active') : t('Waiting for previous worker')}
          </dd>
        </div>
      </dl>
      {data.last_error && (
        <Alert variant='destructive'>
          <AlertDescription className='break-words'>
            {t(data.last_error)}
          </AlertDescription>
        </Alert>
      )}
      <p className='text-muted-foreground text-sm'>
        {t(
          'Startup failure or readiness timeout retains the previous service. Forced termination, host failure, proxy timeouts and incompatible database upgrades are not covered.'
        )}
      </p>
      <div>
        <Button disabled={disabled} onClick={() => setConfirming(true)}>
          {t('Request hot restart')}
        </Button>
      </div>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={t('Confirm hot restart')}
        desc={t(
          'Start a replacement worker? Existing streams stay on the previous worker until they finish. Business background ownership transfers only after admitted jobs finish.'
        )}
        confirmText={t('Start replacement')}
        disabled={disabled}
        isLoading={restart.isPending}
        handleConfirm={() => restart.mutate()}
      />
    </SettingsSection>
  )
}
