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
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { ErrorRewriteSection } from '../error-rewrite-section'
import type { ErrorRewriteSettings } from '../lib/error-rewrite'

const rule = {
  key: 'quota',
  enabled: true,
  priority: 100,
  source: 'upstream',
  match_type: 'regex_any',
  patterns: ['(?i)quota'],
  replace: 'Provider unavailable',
}
let client: QueryClient

function renderEditor(values?: Partial<ErrorRewriteSettings>) {
  const defaults = {
    ErrorMessageRewriteEnabled: false,
    ErrorMessageRewriteRules: JSON.stringify([rule]),
    ...values,
  }
  const actions = document.createElement('div')
  document.body.append(actions)
  const result = render(
    <QueryClientProvider client={client}>
      <SettingsPageProvider actionsContainer={actions}>
        <ErrorRewriteSection defaultValues={defaults} />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
  return { ...result, defaults }
}

beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  vi.spyOn(api, 'put').mockResolvedValue({ data: { success: true } })
  vi.spyOn(toast, 'success').mockImplementation(() => '')
  vi.spyOn(toast, 'error').mockImplementation(() => '')
})
afterEach(() => {
  client.clear()
  vi.restoreAllMocks()
})

describe('error rewrite editor', () => {
  it('edits a rule source and replacement, toggles enabled, and preserves them in JSON', async () => {
    const user = userEvent.setup()
    renderEditor()
    await user.click(screen.getByRole('button', { name: 'Edit' }))
    const dialog = screen.getByRole('dialog')
    await user.click(
      within(dialog).getByRole('combobox', { name: 'Rule source' })
    )
    await user.click(screen.getByRole('option', { name: 'Local' }))
    await user.clear(within(dialog).getByLabelText('Replacement message'))
    await user.type(
      within(dialog).getByLabelText('Replacement message'),
      'Local quota warning'
    )
    await user.click(within(dialog).getByRole('button', { name: 'Apply rule' }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    expect(screen.getByRole('cell', { name: 'Local' })).toBeVisible()
    await user.click(screen.getByRole('switch', { name: 'Enable rule quota' }))
    expect(
      screen.getByRole('switch', { name: 'Enable rule quota' })
    ).not.toBeChecked()
    await user.click(
      screen.getByRole('button', { name: 'Switch to JSON editor' })
    )
    expect(
      JSON.parse(
        (
          screen.getByRole('textbox', {
            name: 'Error rewrite rules JSON',
          }) as HTMLTextAreaElement
        ).value
      )
    ).toEqual([
      {
        ...rule,
        source: 'local',
        enabled: false,
        replace: 'Local quota warning',
      },
    ])
  })

  it('edits and disables only the selected duplicate rule, then deletes only that row', async () => {
    const user = userEvent.setup()
    renderEditor({
      ErrorMessageRewriteRules: JSON.stringify([
        rule,
        { ...rule, replace: 'Second' },
      ]),
    })
    await user.click(screen.getAllByRole('button', { name: 'Edit' })[1])
    const dialog = screen.getByRole('dialog')
    await user.clear(within(dialog).getByLabelText('Replacement message'))
    await user.type(
      within(dialog).getByLabelText('Replacement message'),
      'Edited second'
    )
    await user.click(within(dialog).getByRole('button', { name: 'Apply rule' }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    const switches = screen.getAllByRole('switch', {
      name: 'Enable rule quota',
    })
    await user.click(switches[1])
    expect(switches[0]).toBeChecked()
    expect(switches[1]).not.toBeChecked()
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(2))
    expect(api.put).toHaveBeenLastCalledWith('/api/option/', {
      key: 'ErrorMessageRewriteRules',
      value: JSON.stringify([
        rule,
        { ...rule, enabled: false, replace: 'Edited second' },
      ]),
    })
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
    )
    await user.click(screen.getAllByRole('button', { name: 'Delete' })[1])
    await user.click(
      within(screen.getByRole('alertdialog')).getByRole('button', {
        name: 'Delete',
      })
    )
    expect(
      screen.getByRole('cell', { name: 'Provider unavailable' })
    ).toBeVisible()
    expect(
      screen.queryByRole('cell', { name: 'Edited second' })
    ).not.toBeInTheDocument()
  })

  it('edits an unnamed legacy rule and saves the global switch off without enabling the rule', async () => {
    const user = userEvent.setup()
    renderEditor({
      ErrorMessageRewriteEnabled: true,
      ErrorMessageRewriteRules: JSON.stringify([
        { match_type: '', patterns: [' ', 'quota'], replace: 'Legacy' },
      ]),
    })
    const ruleSwitch = screen.getByRole('switch', {
      name: 'Enable rule',
    })
    expect(ruleSwitch).not.toBeChecked()
    await user.click(screen.getByRole('button', { name: 'Edit' }))
    await user.click(
      within(screen.getByRole('dialog')).getByRole('button', {
        name: 'Apply rule',
      })
    )
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    await user.click(
      screen.getByRole('switch', { name: 'Enable error message rewriting' })
    )
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(2))
    expect(api.put).toHaveBeenNthCalledWith(1, '/api/option/', {
      key: 'ErrorMessageRewriteEnabled',
      value: 'false',
    })
    expect(api.put).toHaveBeenLastCalledWith('/api/option/', {
      key: 'ErrorMessageRewriteRules',
      value: JSON.stringify([
        {
          key: '',
          enabled: false,
          priority: 0,
          source: 'all',
          match_type: 'contains_any',
          patterns: ['quota'],
          replace: 'Legacy',
        },
      ]),
    })
  })

  it('adds a rule and requires confirmation before deleting it', async () => {
    const user = userEvent.setup()
    renderEditor({ ErrorMessageRewriteRules: '[]' })
    expect(screen.getByText('No rules configured.')).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    const dialog = screen.getByRole('dialog')
    await user.type(within(dialog).getByLabelText('Rule name'), 'maintenance')
    await user.type(
      within(dialog).getByLabelText('Match patterns'),
      'unavailable'
    )
    await user.type(
      within(dialog).getByLabelText('Replacement message'),
      'Try later'
    )
    await user.click(within(dialog).getByRole('button', { name: 'Apply rule' }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    expect(screen.getByRole('cell', { name: 'maintenance' })).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Delete' }))
    expect(
      screen.getByRole('cell', { name: 'maintenance', hidden: true })
    ).toBeInTheDocument()
    await user.click(
      within(screen.getByRole('alertdialog')).getByRole('button', {
        name: 'Delete',
      })
    )
    expect(screen.getByText('No rules configured.')).toBeVisible()
  })

  it('retains invalid JSON and blocks visual switching, preview, and saving', async () => {
    const user = userEvent.setup()
    const post = vi.spyOn(api, 'post')
    renderEditor({ ErrorMessageRewriteRules: '{broken' })
    await user.click(
      screen.getByRole('button', { name: 'Switch to visual editor' })
    )
    expect(screen.getByText('Invalid JSON string.')).toBeVisible()
    expect(
      screen.getByRole('textbox', { name: 'Error rewrite rules JSON' })
    ).toHaveValue('{broken')
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    await user.click(screen.getByRole('button', { name: 'Preview' }))
    expect(api.put).not.toHaveBeenCalled()
    expect(post).not.toHaveBeenCalled()
  })

  it('preserves drafts on a second-write failure without invalidating options', async () => {
    const user = userEvent.setup()
    const invalidate = vi.spyOn(client, 'invalidateQueries')
    vi.mocked(api.put)
      .mockResolvedValueOnce({ data: { success: true } })
      .mockResolvedValueOnce({
        data: { success: false, message: 'Switch rejected' },
      })
    renderEditor()
    await user.click(
      screen.getByRole('switch', { name: 'Enable error message rewriting' })
    )
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Switch rejected'
    )
    expect(
      screen.getByRole('switch', { name: 'Enable error message rewriting' })
    ).toBeChecked()
    expect(
      screen.getByRole('cell', { name: 'Provider unavailable' })
    ).toBeVisible()
    expect(toast.success).not.toHaveBeenCalled()
    expect(invalidate).not.toHaveBeenCalled()
  })

  it('disables editing during saves and invalidates only after both writes succeed', async () => {
    const user = userEvent.setup()
    const invalidate = vi.spyOn(client, 'invalidateQueries')
    let finish: (value: { data: { success: boolean } }) => void = () =>
      undefined
    vi.mocked(api.put).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve
        })
    )
    renderEditor()
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Edit' })).toBeDisabled()
    expect(invalidate).not.toHaveBeenCalled()
    finish({ data: { success: true } })
    await waitFor(() => expect(toast.success).toHaveBeenCalled())
    expect(api.put).toHaveBeenCalledTimes(2)
    expect(invalidate).toHaveBeenCalledTimes(1)
  })

  it('blocks enabling an empty rules list', async () => {
    const user = userEvent.setup()
    renderEditor({ ErrorMessageRewriteRules: '[]' })
    await user.click(
      screen.getByRole('switch', { name: 'Enable error message rewriting' })
    )
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    expect(
      await screen.findByText(
        'Add at least one rule before enabling error rewriting.'
      )
    ).toBeVisible()
    expect(api.put).not.toHaveBeenCalled()
  })

  it('previews upstream and local using the server response and hides stale results', async () => {
    const user = userEvent.setup()
    const post = vi.spyOn(api, 'post').mockResolvedValue({
      data: {
        success: true,
        data: { message: 'Server rewrite (request id: abc)' },
      },
    })
    renderEditor()
    fireEvent.change(screen.getByLabelText('Original error message'), {
      target: { value: 'QUOTA (request id: abc)' },
    })
    await user.click(screen.getByRole('button', { name: 'Preview' }))
    expect(
      await screen.findByText('Server rewrite (request id: abc)')
    ).toBeVisible()
    expect(post).toHaveBeenLastCalledWith(
      '/api/option/error-message-rewrite/preview',
      {
        message: 'QUOTA (request id: abc)',
        source: 'upstream',
        rules: JSON.stringify([rule]),
      }
    )
    await user.click(screen.getByRole('combobox', { name: 'Preview source' }))
    await user.click(screen.getByRole('option', { name: 'Local' }))
    expect(
      screen.queryByText('Server rewrite (request id: abc)')
    ).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Preview' }))
    await waitFor(() =>
      expect(post).toHaveBeenLastCalledWith(
        '/api/option/error-message-rewrite/preview',
        expect.objectContaining({ source: 'local' })
      )
    )
  })
})
