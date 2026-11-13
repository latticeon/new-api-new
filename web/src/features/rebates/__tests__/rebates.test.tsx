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
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import * as serverErrors from '@/lib/handle-server-error'

import { RebateReportDialog } from '../rebate-report-dialog'
import {
  RebateOverrideDialog,
  SelfRebateRuleDialog,
} from '../rebate-rule-dialog'

const clients: QueryClient[] = []
afterEach(() => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  vi.restoreAllMocks()
})
function renderWithQuery(element: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  clients.push(client)
  return render(
    <QueryClientProvider client={client}>{element}</QueryClientProvider>
  )
}

it('shows distinct registered amount evidence, identities and snapshots and sends page/search/date filters to the server', async () => {
  const get = vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: String(url).endsWith('/overview')
        ? {
            events: 12,
            invitees: 3,
            paid_reward: 100,
            pending_reward: 0,
            skipped: 1,
          }
        : {
            total: 12,
            items: [
              {
                id: 9,
                source: 'redemption',
                reference: 'code-9',
                user_id: 2,
                inviter_id: 1,
                inviter_name: 'alice',
                invitee_name: 'bob',
                created_at: 1700000000,
                amount: '20',
                currency: 'CNY',
                cny_rate: '1',
                amount_cny: '20',
                amount_evidence: 'registered_sale',
                percent: 5,
                reward: 100,
                destination: 'wallet',
                status: 'paid',
                reason: '',
                rebate_snapshot: '{"rule":"first"}',
              },
            ],
          },
    },
  }))
  renderWithQuery(
    <RebateReportDialog open onOpenChange={() => {}} inviterId={1} />
  )
  expect(await screen.findByText('Registered code sale price')).toBeVisible()
  expect(screen.queryByText('Order payment amount')).not.toBeInTheDocument()
  expect(screen.getByText(/alice.*bob/)).toBeVisible()
  expect(screen.getByText('code-9')).toBeVisible()
  await userEvent.click(screen.getByRole('button', { name: 'Go to next page' }))
  await waitFor(() =>
    expect(get).toHaveBeenCalledWith('/api/rebates/ledger', {
      params: expect.objectContaining({ p: 2, inviter_id: 1 }),
    })
  )
  fireEvent.change(screen.getByLabelText('Search'), {
    target: { value: 'bob' },
  })
  fireEvent.change(screen.getByLabelText('Start date'), {
    target: { value: '2026-01-01' },
  })
  await waitFor(() =>
    expect(get).toHaveBeenCalledWith('/api/rebates/ledger', {
      params: expect.objectContaining({
        p: 1,
        search: 'bob',
        start: Math.floor(new Date('2026-01-01T00:00:00').getTime() / 1000),
      }),
    })
  )
  await userEvent.click(await screen.findByText('Details'))
  expect(screen.getByText('{"rule":"first"}')).toBeVisible()
})

it('shows protected report errors without fabricated data when the server denies access', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: false, message: 'No permission' },
  })
  renderWithQuery(<RebateReportDialog open onOpenChange={() => {}} />)
  expect(await screen.findByRole('button', { name: 'Retry' })).toBeVisible()
  expect(screen.queryByRole('table')).not.toBeInTheDocument()
})

it('saves explicit zero separately from an empty inherited percentage', async () => {
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const close = vi.fn()
  const rendered = renderWithQuery(
    <RebateOverrideDialog
      open
      onOpenChange={close}
      userId={7}
      mode='on'
      percent={0}
    />
  )
  expect(screen.getByLabelText('Rebate percentage')).toHaveValue(0)
  await userEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/user/7/rebate', {
      mode: 'on',
      percent: 0,
    })
  )
  rendered.unmount()
  renderWithQuery(
    <RebateOverrideDialog
      open
      onOpenChange={close}
      userId={7}
      mode='inherit'
      percent={null}
    />
  )
  await userEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() =>
    expect(put).toHaveBeenLastCalledWith('/api/user/7/rebate', {
      mode: 'inherit',
      percent: null,
    })
  )
})

it('does not report save success when the override API rejects the change', async () => {
  vi.spyOn(api, 'put').mockResolvedValue({
    data: { success: false, message: 'No permission' },
  })
  const close = vi.fn()
  renderWithQuery(<RebateOverrideDialog open onOpenChange={close} userId={7} />)
  await userEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(api.put).toHaveBeenCalled())
  expect(close).not.toHaveBeenCalled()
})

it('loads only the current account effective rule and exposes no editing controls', async () => {
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        mode: 'on',
        percent: 0,
        compliance_confirmed: true,
        effective: {
          enabled: true,
          percent: 0,
          rule: 'first',
          max_per_invitee: 0,
          quota_per_yuan: 500000,
          destination: 'wallet',
          online: true,
          subscription: true,
          redemption: false,
          admin: false,
        },
      },
    },
  })
  renderWithQuery(<SelfRebateRuleDialog open onOpenChange={() => {}} />)
  expect(await screen.findByText('0%')).toBeVisible()
  expect(get).toHaveBeenCalledWith('/api/user/self/rebate')
  expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()
})

it('keeps a failed pending reward retry visible and reports the server reason', async () => {
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: String(url).endsWith('/overview')
        ? {
            events: 1,
            invitees: 1,
            paid_reward: 0,
            pending_reward: 100,
            skipped: 0,
          }
        : {
            total: 1,
            items: [
              {
                id: 9,
                source: 'online',
                reference: 'order-9',
                inviter_id: 1,
                user_id: 2,
                inviter_name: 'alice',
                invitee_name: 'bob',
                status: 'pending',
                reason: 'wallet unavailable',
                reward: 100,
                percent: 5,
                destination: 'wallet',
                created_at: 1700000000,
                amount_evidence: 'provider_payment',
                amount: '20',
                currency: 'CNY',
                amount_cny: '20',
                cny_rate: '1',
              },
            ],
          },
    },
  }))
  const post = vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: false, message: 'Reward remains pending' },
  })
  const handleError = vi
    .spyOn(serverErrors, 'handleServerError')
    .mockImplementation(() => {})
  renderWithQuery(<RebateReportDialog open onOpenChange={() => {}} />)
  await userEvent.click(
    await screen.findByRole('button', { name: 'Retry reward' })
  )
  await waitFor(() => expect(handleError).toHaveBeenCalled())
  expect(post).toHaveBeenCalledWith('/api/rebates/9/retry')
  expect(screen.getByText('wallet unavailable')).toBeVisible()
  expect(screen.getByRole('button', { name: 'Retry reward' })).toBeEnabled()
})
