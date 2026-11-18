import { api } from '@/lib/api'

export interface UserAnalyticsDaily {
  date: string
  registrations: number
  active_users: number
  successful_logins: number
  failed_logins: number
  topup_orders: number
  redemptions: number
  redemption_quota: number
  paying_users: number
  revenue_by_currency: Record<string, number>
  redemption_revenue_cny: number
}

export interface UserAnalytics {
  from: number
  to: number
  generated_at: number
  timezone_offset_minutes: number
  login_coverage_start: number
  total_users: number
  enabled_users: number
  disabled_users: number
  invited_users: number
  new_users: number
  unknown_registration_time: number
  unknown_last_login_time: number
  dau: number
  mau: number
  successful_logins: number
  failed_logins: number
  topup_orders: number
  redemptions: number
  redemption_quota: number
  paying_users: number
  revenue_by_currency: { currency: string; amount: number }[]
  redemption_revenue_cny: number
  daily: UserAnalyticsDaily[]
}

export async function getUserAnalytics(params: {
  from: number
  to: number
  timezone_offset_minutes: number
}) {
  const response = await api.get<{ success: boolean; data: UserAnalytics }>(
    '/api/user/analytics',
    { params }
  )
  return response.data
}
