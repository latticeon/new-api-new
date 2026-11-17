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
import { ClientSourceSection } from '../client-source-section'

afterEach(() => vi.restoreAllMocks())

it('rejects empty alias patterns, saves ordered rules, and resets when settings reload', async () => {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const actions = document.createElement('div')
  document.body.append(actions)
  const content = (aliases: string) => (
    <QueryClientProvider client={client}>
      <SettingsPageProvider actionsContainer={actions}>
        <ClientSourceSection defaultValues={{ aliases }} />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
  const view = render(content('[]'))
  const user = userEvent.setup()
  const editor = screen.getByRole('textbox', { name: 'User-Agent aliases' })
  fireEvent.input(editor, {
    target: { value: '[{"pattern":" ","alias":"Client"}]' },
  })
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(
    await screen.findByText('Invalid User-Agent aliases')
  ).toBeInTheDocument()
  expect(put).not.toHaveBeenCalled()
  const aliases =
    '[{"pattern":"Cherry","alias":"Cherry Studio"},{"pattern":"curl/","alias":"curl"}]'
  fireEvent.input(editor, { target: { value: aliases } })
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/option/', {
      key: 'client_source_setting.aliases',
      value: aliases,
    })
  )
  const reloaded = '[{"pattern":"curl/","alias":"Terminal"}]'
  view.rerender(content(reloaded))
  await waitFor(() => expect(editor).toHaveValue(reloaded))
  view.unmount()
  client.clear()
  actions.remove()
})
