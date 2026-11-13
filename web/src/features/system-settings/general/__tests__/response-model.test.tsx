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
import { SystemBehaviorSection } from '../system-behavior-section'

afterEach(() => vi.restoreAllMocks())

it('keeps normalization opt-in, resets edits and saves the dotted option with pending controls disabled', async () => {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  let resolveSave: (value: { data: { success: boolean } }) => void = () => {}
  const put = vi.spyOn(api, 'put').mockImplementation(
    () =>
      new Promise((resolve) => {
        resolveSave = resolve
      })
  )
  const actions = document.createElement('div')
  document.body.append(actions)
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPageProvider actionsContainer={actions}>
        <SystemBehaviorSection
          defaultValues={{
            DefaultCollapseSidebar: false,
            NormalizeResponseModelEnabled: false,
            DemoSiteEnabled: false,
            SelfUseModeEnabled: false,
          }}
        />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  const toggle = screen.getByRole('switch', {
    name: 'Normalize response model names',
  })
  expect(toggle).not.toBeChecked()
  await user.click(toggle)
  expect(toggle).toBeChecked()
  await user.click(screen.getByRole('button', { name: 'Reset' }))
  expect(toggle).not.toBeChecked()
  expect(put).not.toHaveBeenCalled()
  await user.click(toggle)
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/option/', {
      key: 'general_setting.normalize_response_model_enabled',
      value: true,
    })
  )
  expect(toggle).toHaveAttribute('aria-disabled', 'true')
  expect(screen.getByRole('button', { name: 'Reset' })).toBeDisabled()
  expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
  resolveSave({ data: { success: true } })
  await waitFor(() =>
    expect(toggle).not.toHaveAttribute('aria-disabled', 'true')
  )
  view.unmount()
  client.clear()
  actions.remove()
})
