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

export type HotRestartStatus = {
  supported: boolean
  pending: boolean
  draining: boolean
  background: boolean
  last_error: string
  pid: number
  started_at: number
  version: string
}

type Response<T> = { success: boolean; message?: string; data?: T }

export async function getHotRestartStatus(): Promise<HotRestartStatus> {
  const response = await api.get<Response<HotRestartStatus>>(
    '/api/hot-restart/status'
  )
  if (response.data.success !== true || !response.data.data) {
    throw createServerError(response.data)
  }
  return response.data.data
}

export async function requestHotRestart(): Promise<void> {
  // Process control must never be automatically retried after a lost response.
  const response = await api.post<Response<null>>('/api/hot-restart')
  if (response.data.success !== true) throw createServerError(response.data)
}
