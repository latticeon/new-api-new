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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { UserQuotaDialog } from '../user-quota-dialog'

beforeEach(() => {
  useSystemConfigStore
    .getState()
    .setConfig({ currency: { ...DEFAULT_CURRENCY_CONFIG } })
})
afterEach(() => vi.restoreAllMocks())

function renderQuota() {
  render(
    <UserQuotaDialog
      open
      onOpenChange={() => undefined}
      userId={2}
      currentQuota={500000}
      onSuccess={() => undefined}
    />
  )
}

it('requires a declared CNY amount for participation and preserves the reference on retry', async () => {
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValueOnce({
      data: { success: false, message: 'Temporary failure' },
    })
    .mockResolvedValueOnce({ data: { success: true } })
  renderQuota()
  const user = userEvent.setup()
  fireEvent.change(screen.getByRole('spinbutton', { name: /Amount/ }), {
    target: { value: '1' },
  })
  await user.click(
    screen.getByRole('switch', { name: 'Participate in arrival rebates' })
  )
  await user.click(screen.getByRole('button', { name: 'Confirm' }))
  expect(screen.getByRole('alert')).toHaveTextContent(
    'Enter a valid non-negative CNY amount for rebate participation'
  )
  expect(post).not.toHaveBeenCalled()
  fireEvent.change(
    screen.getByRole('spinbutton', { name: 'Declared received amount (CNY)' }),
    { target: { value: '12.5' } }
  )
  await user.click(screen.getByRole('button', { name: 'Confirm' }))
  await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Confirm' })).toBeEnabled()
  )
  await user.click(screen.getByRole('button', { name: 'Confirm' }))
  await waitFor(() => expect(post).toHaveBeenCalledTimes(2))
  expect(post.mock.calls[0]?.[1]).toMatchObject({
    mode: 'add',
    value: 500000,
    received_amount: 12.5,
    rebate_enabled: true,
    reference: expect.any(String),
  })
  expect(post.mock.calls[1]?.[1]).toEqual(post.mock.calls[0]?.[1])
})

it('removes receipt and participation fields when switching from addition to deduction', async () => {
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true } })
  renderQuota()
  const user = userEvent.setup()
  await user.click(
    screen.getByRole('switch', { name: 'Participate in arrival rebates' })
  )
  fireEvent.change(
    screen.getByRole('spinbutton', { name: 'Declared received amount (CNY)' }),
    { target: { value: '10' } }
  )
  await user.click(screen.getByRole('button', { name: 'Subtract' }))
  expect(screen.queryByRole('switch')).not.toBeInTheDocument()
  expect(
    screen.getByText('Deductions cannot earn rebates.')
  ).toBeInTheDocument()
  fireEvent.change(screen.getByRole('spinbutton', { name: /Amount/ }), {
    target: { value: '1' },
  })
  await user.click(screen.getByRole('button', { name: 'Confirm' }))
  await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
  expect(post.mock.calls[0]?.[1]).toEqual({
    id: 2,
    action: 'add_quota',
    mode: 'subtract',
    value: 500000,
  })
})
