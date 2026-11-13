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
import * as z from 'zod'

export const errorRewriteRuleSchema = z.object({
  key: z.string().trim().default(''),
  enabled: z.boolean().default(false),
  priority: z.number().int().safe().default(0),
  source: z.preprocess(
    (value) => (value === undefined || value === '' ? 'all' : value),
    z.enum(['all', 'upstream', 'local'])
  ),
  match_type: z.preprocess(
    (value) => (value === undefined || value === '' ? 'contains_any' : value),
    z.enum(['exact_any', 'contains_any', 'regex_any'])
  ),
  patterns: z
    .array(z.string().trim())
    .transform((items) => items.filter(Boolean))
    .pipe(z.array(z.string()).min(1)),
  replace: z.string().trim().min(1),
})

export type ErrorRewriteRule = z.infer<typeof errorRewriteRuleSchema>
export type ErrorRewriteSettings = {
  ErrorMessageRewriteEnabled: boolean
  ErrorMessageRewriteRules: string
}

export function parseErrorRewriteRules(value: string): ErrorRewriteRule[] {
  let parsed: unknown
  try {
    parsed = JSON.parse(value.trim() || '[]')
  } catch {
    throw new Error('Invalid JSON string.')
  }
  const result = z.array(errorRewriteRuleSchema).safeParse(parsed)
  if (!result.success) throw new Error('Invalid error rewrite rules.')
  return result.data.sort((a, b) => {
    const priority = b.priority - a.priority
    if (priority !== 0) return priority
    if (a.key === b.key) return 0
    return a.key < b.key ? -1 : 1
  })
}

export function createErrorRewriteRule(): ErrorRewriteRule {
  return {
    key: '',
    enabled: true,
    priority: 100,
    source: 'all',
    match_type: 'contains_any',
    patterns: [],
    replace: '',
  }
}
