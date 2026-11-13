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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import {
  ARRIVAL_REBATE_DEFAULT_OPTIONS,
  arrivalSettingsValues,
} from '../arrival-rebate-settings'
import { ArrivalSettingsSection } from '../arrival-settings-section'

const clients: QueryClient[] = []
const actionContainers: HTMLElement[] = []
afterEach(() => {
  vi.restoreAllMocks()
  clients.splice(0).forEach((client) => client.clear())
  actionContainers.splice(0).forEach((container) => container.remove())
})

function renderSettings() {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  clients.push(client)
  const actions = document.createElement('div')
  document.body.append(actions)
  actionContainers.push(actions)
  return render(
    <QueryClientProvider client={client}>
      <SettingsPageProvider actionsContainer={actions}>
        <ArrivalSettingsSection
          defaultValues={arrivalSettingsValues(ARRIVAL_REBATE_DEFAULT_OPTIONS)}
        />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
}

it('rejects an enabled arrival rule without a positive threshold and target, then saves the rule before enabling', async () => {
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  renderSettings()
  const user = userEvent.setup()
  await user.click(
    screen.getByRole('switch', { name: 'Enable arrival auto-grouping' })
  )
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(
    await screen.findByText('Enter a positive arrival threshold')
  ).toBeInTheDocument()
  expect(screen.getByText('Select a target group')).toBeInTheDocument()
  expect(put).not.toHaveBeenCalled()
  fireEvent.change(
    screen.getByRole('spinbutton', { name: 'Arrival threshold (quota units)' }),
    { target: { value: '500000' } }
  )
  await user.type(screen.getByRole('textbox', { name: 'Target group' }), 'vip')
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() => expect(put).toHaveBeenCalledTimes(3))
  expect(put.mock.calls.at(-1)?.[1]).toEqual({
    key: 'arrival_setting.enabled',
    value: true,
  })
})
