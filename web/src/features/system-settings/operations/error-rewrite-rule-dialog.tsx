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
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

import { SettingsSwitchField } from '../components/settings-form-layout'
import { ErrorRewriteSourceSelect } from './error-rewrite-source-select'
import {
  createErrorRewriteRule,
  type ErrorRewriteRule,
} from './lib/error-rewrite'

type Props = {
  rule: ErrorRewriteRule | null
  otherKeys: string[]
  onSave: (rule: ErrorRewriteRule) => void
  onClose: () => void
}

export function ErrorRewriteRuleDialog(props: Props) {
  const { t } = useTranslation()
  const schema = z.object({
    key: props.rule
      ? z.string().trim()
      : z
          .string()
          .trim()
          .min(1, t('Required'))
          .refine(
            (key) => !props.otherKeys.includes(key),
            t('Rule names must be unique.')
          ),
    priority: z
      .number()
      .int(t('Invalid error rewrite rules.'))
      .safe(t('Invalid error rewrite rules.')),
    source: z.enum(['all', 'upstream', 'local']),
    enabled: z.boolean(),
    match_type: z.enum(['exact_any', 'contains_any', 'regex_any']),
    patternsText: z.string().trim().min(1, t('Required')),
    replace: z.string().trim().min(1, t('Required')),
  })
  type Values = z.infer<typeof schema>
  const initial = props.rule ?? createErrorRewriteRule()
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { ...initial, patternsText: initial.patterns.join('\n') },
  })
  const matchLabels = {
    exact_any: t('Exact match any'),
    contains_any: t('Contains any'),
    regex_any: t('Regex match any'),
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
      title={props.rule ? t('Edit rule') : t('Add rule')}
      description={t(
        'Enter one pattern per line. Any matching pattern triggers the rule.'
      )}
      footer={
        <>
          <Button type='button' variant='outline' onClick={props.onClose}>
            {t('Cancel')}
          </Button>
          <Button type='submit' form='error-rewrite-rule-form'>
            {t('Apply rule')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id='error-rewrite-rule-form'
          className='space-y-4'
          onSubmit={(event) => {
            event.stopPropagation()
            void form.handleSubmit((values) => {
              const { patternsText, ...rule } = values
              props.onSave({
                ...rule,
                patterns: patternsText
                  .split('\n')
                  .map((item) => item.trim())
                  .filter(Boolean),
              })
            })(event)
          }}
        >
          <FormField
            control={form.control}
            name='key'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Rule name')}</FormLabel>
                <FormControl>
                  <Input {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchField
                controlId='error-rewrite-rule-enabled'
                label={t('Enable rule')}
                checked={field.value}
                onCheckedChange={field.onChange}
              />
            )}
          />
          <FormField
            control={form.control}
            name='priority'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Priority')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    type='number'
                    step={1}
                    onChange={(event) =>
                      field.onChange(event.target.valueAsNumber)
                    }
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='source'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Rule source')}</FormLabel>
                <FormControl>
                  <ErrorRewriteSourceSelect
                    value={field.value}
                    onChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='match_type'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Match type')}</FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue>{matchLabels[field.value]}</SelectValue>
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {Object.entries(matchLabels).map(([value, label]) => (
                      <SelectItem key={value} value={value}>
                        {label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='patternsText'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Match patterns')}</FormLabel>
                <FormControl>
                  <Textarea {...field} rows={5} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='replace'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Replacement message')}</FormLabel>
                <FormControl>
                  <Textarea {...field} rows={3} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}
