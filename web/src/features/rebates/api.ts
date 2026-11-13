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
import { requireServerSuccess } from '@/lib/server-error-message'

export interface RebateEvent {
  id: number
  source: string
  reference: string
  user_id: number
  inviter_id: number
  inviter_name: string
  invitee_name: string
  created_at: number
  amount: string
  currency: string
  cny_rate: string
  amount_cny: string
  amount_evidence: string
  percent: number
  reward: number
  destination: string
  status: string
  reason: string
  rebate_snapshot: string
}
export interface RebateParticipant {
  id: number
  username: string
  inviter_id: number
  events: number
  reward: number
  pending: number
}
export interface RebateOverview {
  events: number
  invitees: number
  paid_reward: number
  pending_reward: number
  skipped: number
}
export interface RebateRule {
  enabled: boolean
  rule: string
  percent: number
  max_per_invitee: number
  quota_per_yuan: number
  destination: string
  online: boolean
  subscription: boolean
  redemption: boolean
  admin: boolean
}
export interface SelfRebateRule {
  mode: string
  percent: number | null
  effective: RebateRule
  compliance_confirmed: boolean
}
export interface RebateFilter {
  inviter_id?: number
  search?: string
  source?: string
  status?: string
  start?: number
  end?: number
  sort?: string
  order?: string
  p?: number
  page_size?: number
}
export async function getRebateOverview(params: RebateFilter) {
  const response = await api.get('/api/rebates/overview', { params })
  return requireServerSuccess<{ data: RebateOverview; success: boolean }>(
    response.data
  ).data
}
export async function getRebateRows<T>(
  view: 'ledger' | 'ranking' | 'invitees',
  params: RebateFilter
) {
  const response = await api.get(`/api/rebates/${view}`, { params })
  return requireServerSuccess<{
    data: { items: T[]; total: number }
    success: boolean
  }>(response.data).data
}
export async function retryRebate(id: number) {
  const response = await api.post(`/api/rebates/${id}/retry`)
  return requireServerSuccess(response.data)
}
export async function setRebateOverride(
  id: number,
  mode: string,
  percent: number | null
) {
  const response = await api.put(`/api/user/${id}/rebate`, { mode, percent })
  return requireServerSuccess(response.data)
}
export async function getSelfRebateRule(): Promise<SelfRebateRule> {
  const response = await api.get('/api/user/self/rebate')
  return requireServerSuccess<{ data: SelfRebateRule; success: boolean }>(
    response.data
  ).data
}
