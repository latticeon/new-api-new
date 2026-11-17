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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { mapStatusDataToConfig } from '@/lib/status-query'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { MonitoringSettingsSection } from '../monitoring-settings-section'

const initialStore = useSystemConfigStore.getState()
const clients: QueryClient[] = []
const actionContainers: HTMLDivElement[] = []

afterEach(() => {
  clients.forEach((client) => client.clear())
  clients.length = 0
  useSystemConfigStore.setState(initialStore, true)
  actionContainers.forEach((container) => container.remove())
  actionContainers.length = 0
  window.localStorage.clear()
  vi.restoreAllMocks()
})

function renderMonitoringSettings(enabled: boolean) {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  clients.push(client)
  const actions = document.createElement('div')
  document.body.append(actions)
  actionContainers.push(actions)
  render(
    <QueryClientProvider client={client}>
      <SettingsPageProvider actionsContainer={actions}>
        <MonitoringSettingsSection
          defaultValues={{
            'perf_metrics_setting.enabled': enabled,
            'perf_metrics_setting.reference_data_enabled': false,
            'perf_metrics_setting.flush_interval': 5,
            'perf_metrics_setting.bucket_time': 'hour',
            'perf_metrics_setting.retention_days': 0,
          }}
        />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
  return client
}

it('saves the reference-data option and invalidates cached public status', async () => {
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const client = renderMonitoringSettings(true)
  client.setQueryData(['status'], {
    model_square_reference_data_enabled: false,
  })
  window.localStorage.setItem('status', '{}')
  const user = userEvent.setup()
  const toggle = screen.getByRole('switch', {
    name: 'Fill missing model-square metrics with reference data',
  })

  expect(toggle).not.toBeChecked()
  await user.click(toggle)
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/option/', {
      key: 'perf_metrics_setting.reference_data_enabled',
      value: true,
    })
  )
  await waitFor(() =>
    expect(client.getQueryState(['status'])?.isInvalidated).toBe(true)
  )
  expect(put).toHaveBeenCalledTimes(1)
  expect(window.localStorage.getItem('status')).toBeNull()
})

it('keeps reference estimates enabled and editable when collection is switched off', async () => {
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  renderMonitoringSettings(true)
  const user = userEvent.setup()
  const referenceToggle = screen.getByRole('switch', {
    name: 'Fill missing model-square metrics with reference data',
  })
  await user.click(referenceToggle)
  await user.click(
    screen.getByRole('switch', { name: 'Enable model performance metrics' })
  )

  expect(referenceToggle).toBeChecked()
  expect(referenceToggle).not.toHaveAttribute('aria-disabled', 'true')
  expect(
    screen.getByRole('spinbutton', { name: 'Flush interval (minutes)' })
  ).toBeDisabled()
  await user.click(referenceToggle)
  expect(referenceToggle).not.toBeChecked()
  await user.click(referenceToggle)
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() => expect(put).toHaveBeenCalledTimes(2))
  expect(put).toHaveBeenCalledWith('/api/option/', {
    key: 'perf_metrics_setting.enabled',
    value: false,
  })
  expect(put).toHaveBeenCalledWith('/api/option/', {
    key: 'perf_metrics_setting.reference_data_enabled',
    value: true,
  })
})

it.each([
  { value: true, expected: true },
  { value: false, expected: false },
  { value: undefined, expected: false },
  { value: 'true', expected: false },
  { value: 1, expected: false },
])(
  'maps public reference-data flag $value to $expected, clearing stale config',
  ({ value, expected }) => {
    const store = useSystemConfigStore.getState()
    store.setConfig({ modelSquareReferenceDataEnabled: true })

    store.setConfig(
      mapStatusDataToConfig({ model_square_reference_data_enabled: value })
    )

    expect(
      useSystemConfigStore.getState().config.modelSquareReferenceDataEnabled
    ).toBe(expected)
  }
)
