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

import { SettingsPageProvider } from '../../components/settings-page-context'
import { APIMaintenanceSection } from '../api-maintenance-section'

afterEach(() => vi.restoreAllMocks())

it('saves the maintenance message before enabling and restores the saved settings', async () => {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const actions = document.createElement('div')
  document.body.append(actions)
  const wrap = (enabled: boolean, message: string) => (
    <QueryClientProvider client={client}>
      <SettingsPageProvider actionsContainer={actions}>
        <APIMaintenanceSection defaultValues={{ enabled, message }} />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
  const view = render(wrap(false, ''))
  const user = userEvent.setup()
  expect(
    screen.getByRole('switch', { name: 'Pause new API executions' })
  ).not.toBeChecked()
  await user.type(
    screen.getByRole('textbox', { name: 'Maintenance message' }),
    'Scheduled work'
  )
  await user.click(
    screen.getByRole('switch', { name: 'Pause new API executions' })
  )
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() => expect(put).toHaveBeenCalledTimes(2))
  expect(put.mock.calls[0]?.[1]).toEqual({
    key: 'maintenance_setting.message',
    value: 'Scheduled work',
  })
  expect(put.mock.calls[1]?.[1]).toEqual({
    key: 'maintenance_setting.enabled',
    value: true,
  })
  view.rerender(wrap(true, 'Scheduled work'))
  expect(
    screen.getByRole('switch', { name: 'Pause new API executions' })
  ).toBeChecked()
  expect(
    screen.getByRole('textbox', { name: 'Maintenance message' })
  ).toHaveValue('Scheduled work')
  client.clear()
  actions.remove()
})
