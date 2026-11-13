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
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { LogCleanupSection } from '../log-cleanup-section'

let client: QueryClient
function renderSettings() {
  const actions = document.createElement('div')
  document.body.append(actions)
  render(
    <QueryClientProvider client={client}>
      <SettingsPageProvider actionsContainer={actions}>
        <LogCleanupSection
          defaultValues={{
            'log_setting.auto_cleanup_enabled': false,
            'log_setting.cleanup_retention_days': 30,
            'log_setting.cleanup_scope': 'consume_error',
          }}
        />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
}
beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url === '/api/system-task/list' ? [] : null,
    },
  }))
  vi.spyOn(api, 'put').mockResolvedValue({ data: { success: true } })
})
afterEach(() => {
  client.clear()
  vi.restoreAllMocks()
})

test('cleanup is off by default and only safe scopes can be saved with valid retention', async () => {
  const user = userEvent.setup()
  renderSettings()
  const enabled = screen.getByRole('switch', {
    name: 'Automatic database log cleanup',
  })
  expect(enabled).not.toBeChecked()
  expect(
    screen
      .getAllByRole('option')
      .map((option) => (option as HTMLOptionElement).value)
  ).toEqual(['consume_error', 'consume', 'error'])
  const days = screen.getByRole('spinbutton', { name: 'Log retention (days)' })
  fireEvent.change(days, { target: { value: '0' } })
  await user.click(
    screen.getByRole('button', { name: 'Save log cleanup settings' })
  )
  await waitFor(() => expect(days).toHaveAttribute('aria-invalid', 'true'))
  expect(api.put).not.toHaveBeenCalled()
  fireEvent.change(days, { target: { value: '7' } })
  await user.selectOptions(
    screen.getByRole('combobox', { name: 'Log cleanup scope' }),
    'error'
  )
  await user.click(enabled)
  await user.click(
    screen.getByRole('button', { name: 'Save log cleanup settings' })
  )
  await waitFor(() => expect(api.put).toHaveBeenCalledTimes(3))
  expect(vi.mocked(api.put).mock.calls.map((call) => call[1])).toEqual([
    { key: 'log_setting.cleanup_retention_days', value: 7 },
    { key: 'log_setting.cleanup_scope', value: 'error' },
    { key: 'log_setting.auto_cleanup_enabled', value: true },
  ])
})

test('manual cleanup confirms protected records, sends selected scope and blocks duplicates', async () => {
  const user = userEvent.setup()
  vi.spyOn(api, 'post').mockResolvedValue({
    data: {
      success: true,
      data: {
        task_id: 'cleanup-1',
        status: 'running',
        state: { total: 4, processed: 1, progress: 25, remaining: 3 },
      },
    },
  })
  renderSettings()
  await user.selectOptions(
    screen.getByRole('combobox', { name: 'Log cleanup scope' }),
    'error'
  )
  await user.click(screen.getByRole('button', { name: 'Clean logs' }))
  const dialog = await screen.findByRole('alertdialog')
  expect(
    within(dialog).getByText(
      /Financial and security audit records are protected/
    )
  ).toBeVisible()
  await user.click(within(dialog).getByRole('button', { name: 'Delete logs' }))
  await waitFor(() =>
    expect(api.post).toHaveBeenCalledWith(
      '/api/system-task/log-cleanup',
      null,
      {
        params: { target_timestamp: expect.any(Number), scope: 'error' },
      }
    )
  )
  expect(await screen.findByText('1 of 4 log entries processed.')).toBeVisible()
  expect(screen.getByRole('button', { name: 'Cleaning...' })).toBeDisabled()
})

test('finished cleanup results and failures remain visible after reopening', async () => {
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/system-task/list'
          ? [
              {
                task_id: 'done',
                status: 'succeeded',
                created_at: 1700000000,
                result: { deleted_count: 12 },
              },
              {
                task_id: 'failed',
                status: 'failed',
                created_at: 1700000001,
                error: 'Cleanup database unavailable',
              },
            ]
          : null,
    },
  }))
  renderSettings()
  expect(await screen.findByText('Deleted log entries: 12')).toBeVisible()
  expect(screen.getByText('Cleanup database unavailable')).toBeVisible()
})

test('result query failure is visible instead of looking like empty history', async () => {
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: url !== '/api/system-task/list',
      message: 'unavailable',
      data: null,
    },
  }))
  renderSettings()
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Failed to load log cleanup results'
  )
  expect(screen.queryByText('No log cleanup tasks yet')).not.toBeInTheDocument()
})
