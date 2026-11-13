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
import { api } from '@/lib/api'
import { createServerError } from '@/lib/server-error-message'

import { updateSystemOption } from '../api'
import type { UpdateOptionRequest } from '../types'
import {
  parseErrorRewriteRules,
  type ErrorRewriteSettings,
} from './lib/error-rewrite'

export async function saveErrorRewriteSettings(
  settings: ErrorRewriteSettings
): Promise<void> {
  const rules = JSON.stringify(
    parseErrorRewriteRules(settings.ErrorMessageRewriteRules)
  )
  const rulesOption: UpdateOptionRequest = {
    key: 'ErrorMessageRewriteRules',
    value: rules,
  }
  const enabledOption: UpdateOptionRequest = {
    key: 'ErrorMessageRewriteEnabled',
    value: String(settings.ErrorMessageRewriteEnabled),
  }
  const options = settings.ErrorMessageRewriteEnabled
    ? [rulesOption, enabledOption]
    : [enabledOption, rulesOption]
  // One mutation owns both writes. Never invalidate/reset the draft between them.
  for (const option of options) {
    const response = await updateSystemOption(option)
    if (response.success !== true) throw createServerError(response)
  }
}

export type ErrorRewritePreviewRequest = {
  message: string
  source: 'upstream' | 'local'
  rules: string
}

export async function previewErrorRewrite(
  request: ErrorRewritePreviewRequest
): Promise<string> {
  const response = await api.post<{
    success: boolean
    message?: string
    data?: { message: string }
  }>('/api/option/error-message-rewrite/preview', request)
  if (
    response.data.success !== true ||
    typeof response.data.data?.message !== 'string'
  ) {
    throw createServerError(response.data)
  }
  return response.data.data.message
}
