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
  act,
  render,
  renderHook,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { PerformanceMetricsData } from '@/features/performance-metrics/types'
import { api } from '@/lib/api'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { ModelCardGrid } from '../components/model-card-grid'
import { ModelPerfBadge } from '../components/model-perf-badge'
import { useModelPerformance } from '../hooks/use-model-performance'
import {
  projectModelBadge,
  projectModelMetrics,
} from '../lib/reference-metrics'
import type { PricingModel } from '../types'

const model: PricingModel = {
  id: 1,
  model_name: 'example-mini',
  enable_groups: ['default', 'premium'],
  quota_type: 0,
  model_ratio: 1,
  completion_ratio: 1,
}
const end = 1_800_000_000
const empty: PerformanceMetricsData['data'] = {
  model_name: model.model_name,
  summary: null,
  series: [],
  groups: [],
  window_end: end,
}
const measuredPoint = {
  ts: end,
  avg_ttft_ms: 0,
  avg_latency_ms: 0,
  success_rate: 0,
  avg_tps: 0,
  has_ttft: true,
  has_tps: true,
}
let client: QueryClient
beforeEach(() => {
  useSystemConfigStore
    .getState()
    .setConfig({ modelSquareReferenceDataEnabled: false })
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
})
afterEach(() => {
  client.clear()
  useSystemConfigStore
    .getState()
    .setConfig({ modelSquareReferenceDataEnabled: false })
  vi.restoreAllMocks()
})
function wrapper(props: { children: React.ReactNode }) {
  return (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
}

describe('model-square reference data', () => {
  it('fills a no-request model with stable 24-hour groups and matching card aggregates', () => {
    const result = projectModelMetrics(model, empty, true)
    const badge = projectModelBadge(model, undefined, true, end)
    expect(result).toEqual(projectModelMetrics(model, empty, true))
    expect(result.groups.map((group) => group.group)).toEqual([
      'default',
      'premium',
    ])
    expect(result.series).toHaveLength(24)
    expect(
      new Set(result.series.map((point) => point.avg_latency_ms)).size
    ).toBeGreaterThan(1)
    expect(result.summary?.avg_latency_ms).toBe(
      Math.round(
        result.series.reduce((sum, point) => sum + point.avg_latency_ms, 0) / 24
      )
    )
    expect(badge?.avg_latency_ms).toBe(result.summary?.avg_latency_ms)
    expect(badge?.avg_tps).toBe(result.summary?.avg_tps)
    expect(badge?.success_rate).toBe(result.summary?.success_rate)
    expect(
      badge?.recent_success_series?.map((point) => point.success_rate)
    ).toEqual(result.series.map((point) => point.success_rate))
    expect(
      result.series.every(
        (point) =>
          point.success_rate < 100 &&
          point.avg_latency_ms > point.avg_ttft_ms &&
          point.avg_tps > 0
      )
    ).toBe(true)
    expect(empty.summary).toBeNull()
    expect(empty.groups).toEqual([])
  })

  it('keeps overlapping hours identical when the 24-hour window advances', () => {
    const first = projectModelMetrics(model, empty, true)
    const next = projectModelMetrics(
      model,
      { ...empty, window_end: end + 3600 },
      true
    )
    expect(next.series.slice(0, 23)).toEqual(first.series.slice(1))
    expect(next.groups[0].series.slice(0, 23)).toEqual(
      first.groups[0].series.slice(1)
    )
  })

  it('retains real failures and sampled zero values while filling missing groups and hours', () => {
    const real = {
      ...empty,
      summary: { ...measuredPoint },
      series: [measuredPoint],
      groups: [{ group: 'default', ...measuredPoint, series: [measuredPoint] }],
    }
    const before = structuredClone(real)
    const result = projectModelMetrics(model, real, true)
    expect(result.summary).toMatchObject({
      success_rate: 0,
      avg_latency_ms: 0,
      avg_tps: 0,
    })
    expect(result.series.find((point) => point.ts === end)).toMatchObject(
      measuredPoint
    )
    expect(result.groups[0]).toMatchObject({
      success_rate: 0,
      avg_ttft_ms: 0,
      avg_tps: 0,
    })
    expect(result.groups[1].success_rate).toBeGreaterThan(99)
    expect(result.series).toHaveLength(24)
    expect(real).toEqual(before)
    expect(projectModelMetrics(model, real, false).summary).toBe(real.summary)
    expect(projectModelMetrics(model, real, false).series).toBe(real.series)
    expect(projectModelMetrics(model, real, false).groups).toBe(real.groups)
  })

  it('fills only explicitly unsampled TTFT and TPS without changing measured success or latency', () => {
    const point = {
      ...measuredPoint,
      has_ttft: false,
      has_tps: false,
      avg_latency_ms: 4500,
    }
    const real = {
      ...empty,
      summary: point,
      series: [point],
      groups: [{ group: 'default', ...point, series: [point] }],
    }
    const result = projectModelMetrics(model, real, true)
    expect(result.summary).toMatchObject({
      success_rate: 0,
      avg_latency_ms: 4500,
    })
    expect(result.summary?.avg_tps).toBeGreaterThan(0)
    expect(result.groups[0].avg_ttft_ms).toBeGreaterThan(0)
    expect(result.groups[0].avg_tps).toBeGreaterThan(0)
    const legacy = { ...measuredPoint, has_tps: undefined, has_ttft: undefined }
    expect(
      projectModelMetrics(model, { ...real, summary: legacy }, true).summary
        ?.avg_tps
    ).toBe(0)
  })

  it('uses slower reasoning profiles and does not invent token metrics for image models', () => {
    const fast = projectModelMetrics(model, empty, true)
    const reasoning = projectModelMetrics(
      { ...model, capabilities: ['reasoning'] },
      empty,
      true
    )
    expect(reasoning.groups[0].avg_ttft_ms).toBeGreaterThan(
      fast.groups[0].avg_ttft_ms
    )
    expect(reasoning.groups[0].avg_tps).toBeLessThan(fast.groups[0].avg_tps)
    const image = projectModelMetrics(
      { ...model, output_modalities: ['image'] },
      empty,
      true
    )
    expect(image.tokenMetricsApplicable).toBe(false)
    expect(
      image.series.every(
        (point) => point.avg_tps === 0 && point.avg_ttft_ms === 0
      )
    ).toBe(true)
    expect(image.summary?.avg_tps).toBe(0)
    expect(
      projectModelMetrics({ ...model, enable_groups: [] }, empty, true)
    ).toMatchObject({
      summary: null,
      groups: [],
      series: [],
    })
  })

  it('restores empty card display after disabling and never fills a failed API request', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: { models: [], window_end: end } },
    })
    render(<ModelCardGrid models={[model]} onModelClick={() => {}} />, {
      wrapper,
    })
    const cardMetrics = await screen.findByLabelText(
      'Performance metrics for the last 24 hours'
    )
    await waitFor(() => expect(get).toHaveBeenCalled())
    expect(within(cardMetrics).getByText('—s')).toBeVisible()
    act(() =>
      useSystemConfigStore
        .getState()
        .setConfig({ modelSquareReferenceDataEnabled: true })
    )
    await waitFor(() =>
      expect(within(cardMetrics).queryByText('—s')).not.toBeInTheDocument()
    )
    expect(screen.queryByText('Reference estimate')).not.toBeInTheDocument()
    act(() =>
      useSystemConfigStore
        .getState()
        .setConfig({ modelSquareReferenceDataEnabled: false })
    )
    await waitFor(() =>
      expect(within(cardMetrics).getByText('—s')).toBeVisible()
    )
    get.mockResolvedValue({ data: { success: false, message: 'Unavailable' } })
    act(() => {
      client.removeQueries({ queryKey: ['perf-metrics-summary'] })
      useSystemConfigStore
        .getState()
        .setConfig({ modelSquareReferenceDataEnabled: true })
    })
    await waitFor(() =>
      expect(
        client.getQueryState([
          'perf-metrics-summary',
          24,
          [model.model_name],
          true,
        ])?.status
      ).toBe('error')
    )
    expect(within(cardMetrics).getByText('—s')).toBeVisible()
  })

  it('renders measured zero latency, throughput and failure rate instead of placeholders', () => {
    const perf = projectModelBadge(
      model,
      {
        ...measuredPoint,
        recent_success_series: [{ ts: end, success_rate: 0 }],
      },
      true,
      end
    )
    render(<ModelPerfBadge perf={perf} />)
    expect(screen.getByText('0ms')).toBeVisible()
    expect(screen.getByText('0t/s')).toBeVisible()
    expect(screen.getByText('0.00%')).toBeVisible()
    expect(screen.queryByText('—s')).not.toBeInTheDocument()
  })

  it('withdraws estimates after a failed background refetch without changing cached measurements', async () => {
    useSystemConfigStore
      .getState()
      .setConfig({ modelSquareReferenceDataEnabled: true })
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValue({ data: { success: true, data: empty } })
    const hook = renderHook(() => useModelPerformance(model), { wrapper })
    await waitFor(() =>
      expect(hook.result.current.displayData?.groups).toHaveLength(2)
    )
    get.mockRejectedValue(new Error('Unavailable'))
    await act(async () => {
      await hook.result.current.refetch()
    })
    await waitFor(() => expect(hook.result.current.isError).toBe(true))
    expect(hook.result.current.displayData).toMatchObject({
      summary: null,
      groups: [],
      series: [],
    })
    expect(
      client.getQueryData<PerformanceMetricsData>([
        'perf-metrics',
        model.model_name,
      ])?.data
    ).toEqual(empty)
  })

  it('projects detail data on switch changes without modifying the query cache', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: empty },
    })
    const hook = renderHook(() => useModelPerformance(model), { wrapper })
    await waitFor(() => expect(hook.result.current.isSuccess).toBe(true))
    expect(hook.result.current.displayData?.summary).toBeNull()
    act(() =>
      useSystemConfigStore
        .getState()
        .setConfig({ modelSquareReferenceDataEnabled: true })
    )
    expect(hook.result.current.displayData?.groups).toHaveLength(2)
    expect(
      client.getQueryData<PerformanceMetricsData>([
        'perf-metrics',
        model.model_name,
      ])?.data
    ).toEqual(empty)
    act(() =>
      useSystemConfigStore
        .getState()
        .setConfig({ modelSquareReferenceDataEnabled: false })
    )
    expect(hook.result.current.displayData?.summary).toBeNull()
    expect(hook.result.current.displayData?.groups).toEqual([])
  })
})
