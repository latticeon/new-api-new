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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { requestHotRestart, type HotRestartStatus } from '../api'
import { HotRestartPanel } from '../HotRestartPanel'

const ready: HotRestartStatus = {
  supported: true,
  pending: false,
  draining: false,
  background: true,
  last_error: '',
  pid: 1234,
  started_at: 1,
  version: 'test-version',
}
let client: QueryClient

function renderPanel(
  status: Partial<HotRestartStatus> = {},
  failFirst = false
) {
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: { ...ready, ...status } },
  })
  if (failFirst) get.mockRejectedValueOnce(new Error('status unavailable'))
  return render(
    <QueryClientProvider client={client}>
      <HotRestartPanel />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
})
afterEach(() => {
  client.clear()
  vi.restoreAllMocks()
})

describe('hot restart control', () => {
  it('requires confirmation and disables repeat requests while the mutation is pending', async () => {
    const user = userEvent.setup()
    let finish!: () => void
    vi.spyOn(api, 'post').mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = () => resolve({ data: { success: true } })
        })
    )
    renderPanel()
    const trigger = await screen.findByRole('button', {
      name: 'Request hot restart',
    })
    expect(screen.getByText('test-version')).toBeVisible()
    await user.click(trigger)
    expect(api.post).not.toHaveBeenCalled()
    const dialog = screen.getByRole('alertdialog')
    await user.click(
      within(dialog).getByRole('button', { name: 'Start replacement' })
    )
    expect(
      within(dialog).getByRole('button', { name: 'Start replacement' })
    ).toBeDisabled()
    expect(api.post).toHaveBeenCalledExactlyOnceWith('/api/hot-restart')
    finish()
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
  })

  it.each([{ supported: false }, { pending: true }, { draining: true }])(
    'disables process control for unavailable or draining status %j',
    async (status) => {
      renderPanel(status)
      expect(
        await screen.findByRole('button', { name: 'Request hot restart' })
      ).toBeDisabled()
      if (status.supported === false) {
        expect(
          screen.getByText(/Requires Linux, HOT_RESTART_ENABLED/)
        ).toBeVisible()
      }
    }
  )

  it('shows rollback failure details without treating the previous service as unavailable', async () => {
    renderPanel({ last_error: 'Replacement readiness timed out' })
    expect(
      await screen.findByText('Replacement readiness timed out')
    ).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Request hot restart' })
    ).toBeEnabled()
  })

  it('provides a retry when the status endpoint fails', async () => {
    renderPanel({}, true)
    expect(await screen.findByText('status unavailable')).toBeVisible()
    await userEvent.setup().click(screen.getByRole('button', { name: 'Retry' }))
    expect(
      await screen.findByRole('button', { name: 'Request hot restart' })
    ).toBeEnabled()
  })

  it('rejects business failure from restart instead of reporting success', async () => {
    vi.spyOn(api, 'post').mockResolvedValue({
      data: { success: false, message: 'Hot restart is already in progress' },
    })
    await expect(requestHotRestart()).rejects.toThrow(
      'Hot restart is already in progress'
    )
  })
})
