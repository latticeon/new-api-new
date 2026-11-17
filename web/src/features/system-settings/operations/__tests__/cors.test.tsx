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
import { CORSSection } from '../cors-section'
import { DEFAULT_CORS_RULES } from '../cors-settings'

afterEach(() => vi.restoreAllMocks())

it('rejects malformed rules and saves allowed origins and request headers together', async () => {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const actions = document.createElement('div')
  document.body.append(actions)
  const view = render(
    <QueryClientProvider client={client}>
      <SettingsPageProvider actionsContainer={actions}>
        <CORSSection defaultValues={{ rules: DEFAULT_CORS_RULES }} />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  const editor = screen.getByRole('textbox', { name: 'CORS rules' })
  fireEvent.input(editor, {
    target: {
      value:
        '[{"path_prefix":"/","allow_origins":["https://client.example/private"],"allow_headers":[]}]',
    },
  })
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(await screen.findByText('Invalid CORS rules')).toBeInTheDocument()
  expect(put).not.toHaveBeenCalled()
  const rules =
    '[{"path_prefix":"/v1/","allow_origins":["https://client.example:8443"],"allow_headers":["Authorization","X-Client-Session"]}]'
  fireEvent.input(editor, { target: { value: rules } })
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/option/', {
      key: 'cors_setting.rules',
      value: rules,
    })
  )
  view.unmount()
  client.clear()
  actions.remove()
})
