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
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { UserOverviewDialog } from '../user-overview-dialog'

const clients: QueryClient[] = []
afterEach(() => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  vi.restoreAllMocks()
})

function renderOverview() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  clients.push(client)
  return render(
    <QueryClientProvider client={client}>
      <UserOverviewDialog open onOpenChange={() => {}} userId={7} />
    </QueryClientProvider>
  )
}

it('loads the protected account API and shows read-only invitation and usage facts without inventing email verification', async () => {
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        id: 7,
        username: 'invite-owner',
        display_name: 'Owner',
        email: 'owner@example.com',
        request_count: 1234,
        group: 'default',
        quota: 1000000,
        used_quota: 500000,
        aff_code: 'direct-code',
        aff_count: 3,
        aff_quota: 250000,
        aff_history_quota: 750000,
        inviter_id: 2,
        status: 1,
        role: 1,
      },
    },
  })
  renderOverview()
  expect(await screen.findByText('owner@example.com')).toBeVisible()
  expect(get).toHaveBeenCalledWith('/api/user/7')
  expect(screen.getByText('1,234')).toBeVisible()
  expect(screen.getByText('Verification not recorded')).toBeVisible()
  expect(screen.getByText('direct-code')).toBeVisible()
  expect(screen.getByText('Pending invitation quota')).toBeVisible()
  expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Save Changes' })
  ).not.toBeInTheDocument()
})

it('does not display protected data when the server denies access', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: false, message: 'No permission' },
  })
  renderOverview()
  expect(await screen.findByRole('button', { name: 'Retry' })).toBeVisible()
  expect(screen.queryByText('Pending invitation quota')).not.toBeInTheDocument()
})
