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
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { cn } from '@/lib/utils'

import { adjustUserQuota } from '../api'
import type { QuotaAdjustMode } from '../types'

interface UserQuotaDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
  currentQuota: number
  onSuccess: () => void
}

export function UserQuotaDialog(props: UserQuotaDialogProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<QuotaAdjustMode>('add')
  const [amount, setAmount] = useState('')
  const [loading, setLoading] = useState(false)
  const [receivedAmount, setReceivedAmount] = useState('')
  const [rebateEnabled, setRebateEnabled] = useState(false)
  const [receiptError, setReceiptError] = useState(false)
  const receiptRequest = useRef<{
    signature: string
    reference: string
  } | null>(null)
  const submitting = useRef(false)

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'

  const amountValue = Number.parseFloat(amount) || 0
  const quotaValue = parseQuotaFromDollars(Math.abs(amountValue))

  const getPreviewText = () => {
    const current = props.currentQuota
    const val = quotaValue
    switch (mode) {
      case 'add':
        return `${t('Current quota')}: ${formatQuota(current)}  +${formatQuota(val)} = ${formatQuota(current + val)}`
      case 'subtract':
        return `${t('Current quota')}: ${formatQuota(current)}  -${formatQuota(val)} = ${formatQuota(current - val)}`
      case 'override': {
        const overrideQuota = parseQuotaFromDollars(amountValue)
        return `${t('Current quota')}: ${formatQuota(current)} → ${formatQuota(overrideQuota)}`
      }
      default:
        return ''
    }
  }

  const handleConfirm = async () => {
    if (!amount && mode !== 'override') return
    if (quotaValue <= 0 && mode !== 'override') return
    if (submitting.current) return
    const declaredAmount = receivedAmount === '' ? null : Number(receivedAmount)
    if (
      mode === 'add' &&
      ((declaredAmount !== null &&
        (!Number.isFinite(declaredAmount) ||
          declaredAmount < 0 ||
          declaredAmount > Number.MAX_SAFE_INTEGER)) ||
        (rebateEnabled && declaredAmount === null))
    ) {
      setReceiptError(true)
      return
    }
    setReceiptError(false)
    submitting.current = true

    setLoading(true)
    try {
      const value =
        mode === 'override' ? parseQuotaFromDollars(amountValue) : quotaValue
      let receiptFields = {}
      if (mode === 'add') {
        const signature = JSON.stringify([
          props.userId,
          value,
          declaredAmount,
          rebateEnabled,
        ])
        if (receiptRequest.current?.signature !== signature) {
          receiptRequest.current = { signature, reference: crypto.randomUUID() }
        }
        receiptFields = {
          received_amount: declaredAmount,
          rebate_enabled: rebateEnabled,
          reference: receiptRequest.current.reference,
        }
      }
      const result = await adjustUserQuota({
        id: props.userId,
        action: 'add_quota',
        mode,
        value: mode === 'override' ? value : Math.abs(value),
        ...receiptFields,
      })
      if (result.success) {
        toast.success(t('Quota adjusted successfully'))
        setAmount('')
        setMode('add')
        setReceivedAmount('')
        setRebateEnabled(false)
        receiptRequest.current = null
        props.onOpenChange(false)
        props.onSuccess()
      } else {
        handleServerError(result, t('Failed to adjust quota'))
      }
    } catch (e: unknown) {
      handleServerError(e, t('Failed to adjust quota'))
    } finally {
      setLoading(false)
      submitting.current = false
    }
  }

  const handleCancel = () => {
    if (submitting.current) return
    setReceivedAmount('')
    setRebateEnabled(false)
    setReceiptError(false)
    receiptRequest.current = null
    setAmount('')
    setMode('add')
    props.onOpenChange(false)
  }

  const placeholder = tokensOnly
    ? t('Enter amount in tokens')
    : t('Enter amount in {{currency}}', { currency: currencyLabel })

  return (
    <Dialog
      open={props.open}
      onOpenChange={(open) => {
        if (!open) handleCancel()
        else props.onOpenChange(true)
      }}
      title={t('Adjust Quota')}
      description={t('Select an operation mode and enter the amount')}
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={handleCancel} disabled={loading}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={loading}>
            {loading ? t('Processing...') : t('Confirm')}
          </Button>
        </>
      }
    >
      <div className='space-y-4'>
        <div className='text-muted-foreground text-sm'>{getPreviewText()}</div>

        <div className='space-y-2'>
          <Label>{t('Mode')}</Label>
          <div className='flex gap-1'>
            {(['add', 'subtract', 'override'] as const).map((m) => (
              <Button
                key={m}
                type='button'
                variant='outline'
                size='sm'
                disabled={loading}
                className={cn(
                  mode === m &&
                    'bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground'
                )}
                onClick={() => {
                  setMode(m)
                  setAmount('')
                  setReceivedAmount('')
                  setRebateEnabled(false)
                  setReceiptError(false)
                  receiptRequest.current = null
                }}
              >
                {m === 'add' && t('Add')}
                {!(m === 'add') && m === 'subtract' && t('Subtract')}
                {!(m === 'add') && !(m === 'subtract') && t('Override')}
              </Button>
            ))}
          </div>
        </div>

        <div className='space-y-2'>
          <Label htmlFor='quota-adjust-amount'>
            {t('Amount')} ({currencyLabel})
          </Label>
          <Input
            id='quota-adjust-amount'
            disabled={loading}
            type='number'
            step={tokensOnly ? 1 : 0.000001}
            min={mode === 'override' ? undefined : 0}
            placeholder={placeholder}
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleConfirm()
            }}
          />
        </div>
        {mode === 'add' && (
          <>
            <div className='space-y-2'>
              <Label htmlFor='quota-received-amount'>
                {t('Declared received amount (CNY)')}
              </Label>
              <Input
                id='quota-received-amount'
                type='number'
                min={0}
                max={Number.MAX_SAFE_INTEGER}
                step='any'
                disabled={loading}
                value={receivedAmount}
                aria-invalid={receiptError}
                aria-describedby='quota-receipt-description'
                onChange={(event) => {
                  setReceivedAmount(event.target.value)
                  setReceiptError(false)
                }}
              />
              <p
                id='quota-receipt-description'
                className='text-muted-foreground text-sm'
              >
                {t(
                  'Optional administrator declaration, not a verified online payment. Required for rebate participation; quota is entered separately.'
                )}
              </p>
              {receiptError && (
                <p role='alert' className='text-destructive text-sm'>
                  {t(
                    'Enter a valid non-negative CNY amount for rebate participation'
                  )}
                </p>
              )}
            </div>
            <div className='flex items-center justify-between gap-4'>
              <Label htmlFor='quota-rebate-enabled'>
                {t('Participate in arrival rebates')}
              </Label>
              <Switch
                id='quota-rebate-enabled'
                disabled={loading}
                checked={rebateEnabled}
                onCheckedChange={setRebateEnabled}
              />
            </div>
          </>
        )}
        {mode === 'subtract' && (
          <p className='text-muted-foreground text-sm'>
            {t('Deductions cannot earn rebates.')}
          </p>
        )}
      </div>
    </Dialog>
  )
}
