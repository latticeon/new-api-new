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
import { z } from 'zod'

const quota = z
  .number({ error: 'Enter a valid non-negative quota amount' })
  .int('Enter a valid non-negative quota amount')
  .min(0, 'Enter a valid non-negative quota amount')
  .max(Number.MAX_SAFE_INTEGER, 'Enter a valid non-negative quota amount')

export const arrivalSettingsSchema = z
  .object({
    enabled: z.boolean(),
    include_admin: z.boolean(),
    mode: z.enum(['single', 'cumulative']),
    threshold: quota,
    source_group: z.string().trim(),
    target_group: z.string().trim(),
  })
  .superRefine((value, ctx) => {
    if (!value.enabled) return
    if (value.threshold <= 0) {
      ctx.addIssue({
        code: 'custom',
        path: ['threshold'],
        message: 'Enter a positive arrival threshold',
      })
    }
    if (!value.target_group) {
      ctx.addIssue({
        code: 'custom',
        path: ['target_group'],
        message: 'Select a target group',
      })
    }
  })

export const rebateSettingsSchema = z.object({
  enabled: z.boolean(),
  rule: z.enum(['first', 'every']),
  percent: z
    .number({ error: 'Enter a rebate percentage from 0 to 100' })
    .min(0, 'Enter a rebate percentage from 0 to 100')
    .max(100, 'Enter a rebate percentage from 0 to 100'),
  max_per_invitee: quota,
  quota_per_yuan: z
    .number({ error: 'Enter a valid non-negative quota amount' })
    .min(0, 'Enter a valid non-negative quota amount')
    .max(Number.MAX_SAFE_INTEGER, 'Enter a valid non-negative quota amount'),
  destination: z.enum(['wallet', 'affiliate']),
  online: z.boolean(),
  subscription: z.boolean(),
  redemption: z.boolean(),
  admin: z.boolean(),
})

export type ArrivalSettingsValues = z.infer<typeof arrivalSettingsSchema>
export type RebateSettingsValues = z.infer<typeof rebateSettingsSchema>
export type ArrivalRebateOptions = {
  [
    K in keyof ArrivalSettingsValues as `arrival_setting.${K}`
  ]: ArrivalSettingsValues[K]
} & {
  [
    K in keyof RebateSettingsValues as `rebate_setting.${K}`
  ]: RebateSettingsValues[K]
}

export const ARRIVAL_REBATE_DEFAULT_OPTIONS: ArrivalRebateOptions = {
  'arrival_setting.enabled': false,
  'arrival_setting.include_admin': false,
  'arrival_setting.mode': 'cumulative',
  'arrival_setting.threshold': 0,
  'arrival_setting.source_group': '',
  'arrival_setting.target_group': '',
  'rebate_setting.enabled': false,
  'rebate_setting.rule': 'first',
  'rebate_setting.percent': 0,
  'rebate_setting.max_per_invitee': 0,
  'rebate_setting.quota_per_yuan': 500000,
  'rebate_setting.destination': 'wallet',
  'rebate_setting.online': true,
  'rebate_setting.subscription': true,
  'rebate_setting.redemption': false,
  'rebate_setting.admin': false,
}

export function arrivalSettingsValues(
  settings: ArrivalRebateOptions
): ArrivalSettingsValues {
  return {
    enabled: settings['arrival_setting.enabled'],
    include_admin: settings['arrival_setting.include_admin'],
    mode: settings['arrival_setting.mode'],
    threshold: settings['arrival_setting.threshold'],
    source_group: settings['arrival_setting.source_group'],
    target_group: settings['arrival_setting.target_group'],
  }
}

export function rebateSettingsValues(
  settings: ArrivalRebateOptions
): RebateSettingsValues {
  return {
    enabled: settings['rebate_setting.enabled'],
    rule: settings['rebate_setting.rule'],
    percent: settings['rebate_setting.percent'],
    max_per_invitee: settings['rebate_setting.max_per_invitee'],
    quota_per_yuan: settings['rebate_setting.quota_per_yuan'],
    destination: settings['rebate_setting.destination'],
    online: settings['rebate_setting.online'],
    subscription: settings['rebate_setting.subscription'],
    redemption: settings['rebate_setting.redemption'],
    admin: settings['rebate_setting.admin'],
  }
}
