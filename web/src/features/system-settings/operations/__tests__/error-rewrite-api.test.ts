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
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  previewErrorRewrite,
  saveErrorRewriteSettings,
} from '../error-rewrite-api'
import {
  createErrorRewriteRule,
  parseErrorRewriteRules,
} from '../lib/error-rewrite'

const rule = {
  key: 'upstream-quota',
  enabled: true,
  priority: 100,
  source: 'upstream',
  match_type: 'regex_any',
  patterns: ['(?i)quota'],
  replace: 'Provider unavailable',
}
const rules = JSON.stringify([rule])

beforeEach(() => {
  vi.restoreAllMocks()
})

describe('error rewrite persistence', () => {
  it.each([true, false])(
    'saves enabled=%s options in safe order',
    async (enabled) => {
      const put = vi
        .spyOn(api, 'put')
        .mockResolvedValue({ data: { success: true } })
      await saveErrorRewriteSettings({
        ErrorMessageRewriteEnabled: enabled,
        ErrorMessageRewriteRules: rules,
      })
      const expected = [
        { key: 'ErrorMessageRewriteRules', value: rules },
        { key: 'ErrorMessageRewriteEnabled', value: String(enabled) },
      ]
      if (!enabled) expected.reverse()
      expect(put.mock.calls.map((call) => call.slice(0, 2))).toEqual(
        expected.map((option) => ['/api/option/', option])
      )
    }
  )

  it('saves unnamed duplicate legacy rules without enabling missing switches', async () => {
    const legacy = JSON.stringify([
      {
        patterns: [' ', ' quota ', ''],
        replace: ' Try later ',
        match_type: '',
      },
      { key: '', enabled: false, patterns: ['quota'], replace: 'Second' },
    ])
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    await saveErrorRewriteSettings({
      ErrorMessageRewriteEnabled: false,
      ErrorMessageRewriteRules: legacy,
    })
    expect(put).toHaveBeenNthCalledWith(1, '/api/option/', {
      key: 'ErrorMessageRewriteEnabled',
      value: 'false',
    })
    expect(put).toHaveBeenNthCalledWith(2, '/api/option/', {
      key: 'ErrorMessageRewriteRules',
      value: JSON.stringify([
        {
          key: '',
          enabled: false,
          priority: 0,
          source: 'all',
          match_type: 'contains_any',
          patterns: ['quota'],
          replace: 'Try later',
        },
        {
          key: '',
          enabled: false,
          priority: 0,
          source: 'all',
          match_type: 'contains_any',
          patterns: ['quota'],
          replace: 'Second',
        },
      ]),
    })
    expect(createErrorRewriteRule().enabled).toBe(true)
  })

  it.each([true, false])(
    'stops enabled=%s sequence after a business failure',
    async (enabled) => {
      const put = vi.spyOn(api, 'put').mockResolvedValue({
        data: { success: false, message: 'Cannot save rules' },
      })
      await expect(
        saveErrorRewriteSettings({
          ErrorMessageRewriteEnabled: enabled,
          ErrorMessageRewriteRules: rules,
        })
      ).rejects.toThrow('Cannot save rules')
      expect(put).toHaveBeenCalledTimes(1)
    }
  )

  it('rejects the second write instead of reporting partial success', async () => {
    vi.spyOn(api, 'put')
      .mockResolvedValueOnce({ data: { success: true } })
      .mockResolvedValueOnce({
        data: { success: false, message: 'Switch rejected' },
      })
    await expect(
      saveErrorRewriteSettings({
        ErrorMessageRewriteEnabled: true,
        ErrorMessageRewriteRules: rules,
      })
    ).rejects.toThrow('Switch rejected')
  })

  it('does not send invalid JSON or broaden an invalid source', async () => {
    const put = vi.spyOn(api, 'put')
    for (const raw of [
      '{',
      '{}',
      JSON.stringify([{ ...rule, source: 'invalid' }]),
    ]) {
      await expect(
        saveErrorRewriteSettings({
          ErrorMessageRewriteEnabled: false,
          ErrorMessageRewriteRules: raw,
        })
      ).rejects.toThrow()
    }
    expect(put).not.toHaveBeenCalled()
  })
})

describe('error rewrite preview contract', () => {
  it('sends unsaved Go regex syntax and returns the server result verbatim', async () => {
    const post = vi.spyOn(api, 'post').mockResolvedValue({
      data: {
        success: true,
        data: { message: 'Provider unavailable (request id: abc)' },
      },
    })
    const request = {
      message: 'QUOTA (request id: abc)',
      source: 'upstream' as const,
      rules,
    }
    await expect(previewErrorRewrite(request)).resolves.toBe(
      'Provider unavailable (request id: abc)'
    )
    expect(post).toHaveBeenCalledWith(
      '/api/option/error-message-rewrite/preview',
      request
    )
  })

  it('preserves an empty successful preview message', async () => {
    vi.spyOn(api, 'post').mockResolvedValue({
      data: { success: true, data: { message: '' } },
    })
    await expect(
      previewErrorRewrite({ message: '', source: 'local', rules })
    ).resolves.toBe('')
  })

  it('rejects business and transport preview failures', async () => {
    vi.spyOn(api, 'post')
      .mockResolvedValueOnce({
        data: { success: false, message: 'Invalid Go regex' },
      })
      .mockRejectedValueOnce(new Error('Offline'))
    const request = { message: 'quota', source: 'local' as const, rules }
    await expect(previewErrorRewrite(request)).rejects.toThrow(
      'Invalid Go regex'
    )
    await expect(previewErrorRewrite(request)).rejects.toThrow('Offline')
  })
})

describe('error rewrite rule validation', () => {
  it('defaults missing legacy sources to all and orders priority before name', () => {
    const { source: _source, ...legacy } = rule
    const parsed = parseErrorRewriteRules(
      JSON.stringify([
        { ...legacy, key: 'z', priority: 1 },
        { ...rule, key: 'b' },
        { ...rule, key: 'a' },
      ])
    )
    expect(parsed.map((item) => item.key)).toEqual(['a', 'b', 'z'])
    expect(parsed[2].source).toBe('all')
  })

  it.each([
    [{ ...rule, patterns: [] }],
    [{ ...rule, patterns: [' ', ''] }],
    [{ ...rule, priority: 1.5 }],
    [{ ...rule, match_type: 'unknown' }],
    [{ ...rule, replace: ' ' }],
  ])('rejects invalid rule lists %#', (...items) => {
    expect(() => parseErrorRewriteRules(JSON.stringify(items))).toThrow()
  })
})
