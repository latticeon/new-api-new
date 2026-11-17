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
import {
  formatLatency,
  formatThroughput,
} from '@/features/performance-metrics/lib/format'
import type {
  PerformanceGroup,
  PerformanceMetricsData,
  PerformanceSeriesPoint,
  PerformanceSummary,
  PerfModelSummary,
  SuccessRatePoint,
} from '@/features/performance-metrics/types'

import type { PricingModel } from '../types'
import { hashStringToSeed, seededRandom } from './seed'

type Metric = 'avg_ttft_ms' | 'avg_latency_ms' | 'avg_tps' | 'success_rate'
export type ReferenceProvenance = { reference_fields?: Metric[] }
export type DisplayPoint = PerformanceSeriesPoint & ReferenceProvenance
export type DisplayGroup = Omit<PerformanceGroup, 'series'> &
  ReferenceProvenance & {
    series: DisplayPoint[]
  }
export type DisplayMetrics = {
  summary?: (PerformanceSummary & ReferenceProvenance) | null
  series: DisplayPoint[]
  groups: DisplayGroup[]
  hasReference: boolean
  tokenMetricsApplicable: boolean
}
export type DisplayBadgeMetrics = Omit<
  PerfModelSummary,
  'model_name' | 'recent_success_series'
> &
  ReferenceProvenance & {
    window_start?: number
    window_end?: number
    recent_success_series?: (SuccessRatePoint & { reference?: boolean })[]
    hasReference?: boolean
    tokenMetricsApplicable?: boolean
  }

type Profile = { ttft: number; tps: number; latency: number; tokens: boolean }

// Illustrative presets only, not provider benchmarks or measured service levels.
// Catalog capabilities take precedence over name heuristics. No request volume
// or accounting events are invented. Aggregates give every reference hour equal weight.
function referenceProfile(model: PricingModel): Profile {
  const name = model.model_name.toLowerCase()
  const textOutput = model.output_modalities?.includes('text')
  if (model.capabilities?.includes('embeddings') || /embed|rerank/.test(name)) {
    return { ttft: 0, tps: 0, latency: 160, tokens: false }
  }
  if (
    !textOutput &&
    (model.output_modalities?.includes('video') ||
      /sora|veo|kling|pika|video|wan-/.test(name))
  ) {
    return { ttft: 0, tps: 0, latency: 85_000, tokens: false }
  }
  if (
    !textOutput &&
    (model.output_modalities?.includes('image') ||
      /dall-e|dalle|imagen|flux|image|midjourney/.test(name))
  ) {
    return { ttft: 0, tps: 0, latency: 18_000, tokens: false }
  }
  if (
    !textOutput &&
    (model.output_modalities?.includes('audio') ||
      /whisper|tts|voice|audio|suno/.test(name))
  ) {
    return { ttft: 0, tps: 0, latency: 6_000, tokens: false }
  }
  if (
    model.capabilities?.includes('reasoning') ||
    /(?:^|[/_-])o[134](?:[-_]|$)|reason|thinking|deepseek-r/.test(name)
  ) {
    return { ttft: 2_800, tps: 42, latency: 0, tokens: true }
  }
  if (/flash|haiku|mini|small|nano|fast/.test(name)) {
    return { ttft: 310, tps: 125, latency: 0, tokens: true }
  }
  if (/opus|ultra|405b|70b|large/.test(name)) {
    return { ttft: 950, tps: 48, latency: 0, tokens: true }
  }
  return { ttft: 620, tps: 76, latency: 0, tokens: true }
}

export function hasReference(values: ReferenceProvenance): boolean {
  return Boolean(values.reference_fields?.length)
}

export function formatDisplayLatency(value: number, sampled = false): string {
  return sampled && value === 0 ? '0ms' : formatLatency(value)
}

export function formatDisplayThroughput(
  value: number,
  sampled = false
): string {
  return sampled && value === 0 ? '0 t/s' : formatThroughput(value)
}

function referencePoint(
  model: PricingModel,
  group: string,
  ts: number
): DisplayPoint {
  const profile = referenceProfile(model)
  const base = seededRandom(
    hashStringToSeed(`${model.model_name}:${group}:reference-v1`)
  )
  const hour = seededRandom(
    hashStringToSeed(`${model.model_name}:${group}:${ts}:reference-v1`)
  )
  const modelFactor = 0.8 + base() * 0.5
  const hourOfDay = (ts / 3600) % 24
  const load = 1 + Math.sin(((hourOfDay - 8) * Math.PI) / 12) * 0.1
  const variation = 0.92 + hour() * 0.16
  const spike = hour() < 0.055 ? 1.25 + hour() * 0.4 : 1
  const ttft = Math.round(profile.ttft * modelFactor * load * variation * spike)
  const tps =
    Math.round((profile.tps / (modelFactor * load * variation)) * 100) / 100
  const latency = profile.tokens
    ? ttft + ((300 + base() * 400) / tps) * 1000
    : profile.latency * modelFactor * load * variation * spike
  const success = 99.1 + base() * 0.8 - hour() * 0.25 - (spike > 1 ? 0.6 : 0)
  return {
    ts,
    avg_ttft_ms: ttft,
    avg_latency_ms: Math.round(latency),
    avg_tps: tps,
    success_rate: Math.round(success * 100) / 100,
    reference_fields: profile.tokens
      ? ['avg_ttft_ms', 'avg_latency_ms', 'avg_tps', 'success_rate']
      : ['avg_latency_ms', 'success_rate'],
  }
}

function aggregate(points: DisplayPoint[]): DisplayPoint {
  const count = points.length || 1
  return {
    ts: points[0]?.ts ?? 0,
    avg_ttft_ms: Math.round(
      points.reduce((sum, point) => sum + point.avg_ttft_ms, 0) / count
    ),
    avg_latency_ms: Math.round(
      points.reduce((sum, point) => sum + point.avg_latency_ms, 0) / count
    ),
    avg_tps: points.reduce((sum, point) => sum + point.avg_tps, 0) / count,
    success_rate:
      points.reduce((sum, point) => sum + point.success_rate, 0) / count,
    reference_fields: [
      ...new Set(points.flatMap((point) => point.reference_fields ?? [])),
    ],
  }
}

// Explicit false means unsampled; absent flags from an older server must not
// cause a valid zero measurement to be overwritten.
function fillSample<
  T extends PerformanceSummary & { has_ttft?: boolean; avg_ttft_ms?: number },
>(real: T, fallback: DisplayPoint, tokens: boolean): T & ReferenceProvenance {
  const result: T & ReferenceProvenance = { ...real, reference_fields: [] }
  if (tokens && real.has_tps === false) {
    result.avg_tps = fallback.avg_tps
    result.reference_fields?.push('avg_tps')
  }
  if (tokens && real.has_ttft === false && 'avg_ttft_ms' in real) {
    result.avg_ttft_ms = fallback.avg_ttft_ms
    result.reference_fields?.push('avg_ttft_ms')
  }
  return result
}

function fillSeries(
  real: PerformanceSeriesPoint[],
  fallback: DisplayPoint[],
  tokens: boolean
): DisplayPoint[] {
  const byHour = new Map(
    fallback.map((point) => [Math.floor(point.ts / 3600), point])
  )
  const occupied = new Set(real.map((point) => Math.floor(point.ts / 3600)))
  const points: DisplayPoint[] = real.map((point) => {
    const reference = byHour.get(Math.floor(point.ts / 3600))
    return reference ? fillSample(point, reference, tokens) : point
  })
  for (const point of fallback) {
    if (!occupied.has(Math.floor(point.ts / 3600))) points.push(point)
  }
  return points.sort((a, b) => a.ts - b.ts)
}

export function projectModelMetrics(
  model: PricingModel,
  real: PerformanceMetricsData['data'],
  enabled: boolean
): DisplayMetrics {
  const tokens = referenceProfile(model).tokens
  if (!enabled) {
    return {
      ...real,
      series: real.series ?? [],
      hasReference: false,
      tokenMetricsApplicable: true,
    }
  }
  const end = real.window_end ?? Math.floor(Date.now() / 1000)
  const lastHour = end - (end % 3600)
  const firstHour = lastHour - 23 * 3600
  const groups = [...new Set(model.enable_groups.filter(Boolean))].sort()
  const references = groups.map<DisplayGroup>((group) => {
    const series = Array.from({ length: 24 }, (_, index) =>
      referencePoint(model, group, firstHour + index * 3600)
    )
    return { group, ...aggregate(series), series }
  })
  // A catalog without enabled groups does not acquire a fictitious group.
  const modelSeries = Array.from({ length: 24 }, (_, index) =>
    aggregate(references.map((group) => group.series[index]))
  )
  const modelReference = aggregate(modelSeries)
  const realGroups = new Map(real.groups.map((group) => [group.group, group]))
  const projectedGroups: DisplayGroup[] = references.map((reference) => {
    const measured = realGroups.get(reference.group)
    if (!measured) return reference
    realGroups.delete(reference.group)
    return {
      ...fillSample(measured, aggregate(reference.series), tokens),
      series: fillSeries(measured.series, reference.series, tokens),
    }
  })
  // Keep permitted real groups even if the public catalog changed since sampling.
  for (const measured of realGroups.values()) projectedGroups.push(measured)
  const series: DisplayPoint[] =
    references.length > 0
      ? fillSeries(real.series ?? [], modelSeries, tokens)
      : (real.series ?? [])
  let summary: DisplayMetrics['summary'] = real.summary
  if (references.length > 0) {
    summary = real.summary
      ? fillSample(real.summary, modelReference, tokens)
      : modelReference
  }
  return {
    summary,
    series,
    groups: projectedGroups,
    hasReference:
      Boolean(summary && hasReference(summary)) ||
      series.some(hasReference) ||
      projectedGroups.some(
        (group) => hasReference(group) || group.series.some(hasReference)
      ),
    tokenMetricsApplicable: tokens,
  }
}

export function projectModelBadge(
  model: PricingModel,
  real: DisplayBadgeMetrics | undefined,
  enabled: boolean,
  windowEnd?: number
): DisplayBadgeMetrics | undefined {
  if (!enabled) return real
  const reference = projectModelMetrics(
    model,
    {
      model_name: model.model_name,
      groups: [],
      window_end: windowEnd,
    },
    true
  )
  if (!reference.summary) return real
  const fallback = reference.summary
  const summary =
    real && Number.isFinite(real.success_rate)
      ? fillSample(
          real,
          { ...aggregate(reference.series), ...fallback },
          reference.tokenMetricsApplicable
        )
      : fallback
  const end = windowEnd ?? Math.floor(Date.now() / 1000)
  const lastHour = end - (end % 3600)
  const existing = (real?.recent_success_series ?? []).filter(
    (point) => point.ts >= lastHour - 23 * 3600 && point.ts <= lastHour + 3599
  )
  const occupied = new Set(existing.map((point) => Math.floor(point.ts / 3600)))
  const series = [
    ...existing,
    ...reference.series
      .filter((point) => !occupied.has(Math.floor(point.ts / 3600)))
      .map((point) => ({
        ts: point.ts,
        success_rate: point.success_rate,
        reference: true,
      })),
  ]
    .sort((a, b) => a.ts - b.ts)
    .slice(-24)
  return {
    ...real,
    ...summary,
    recent_success_series: series,
    hasReference:
      hasReference(summary) || series.some((point) => point.reference),
    tokenMetricsApplicable: reference.tokenMetricsApplicable,
  }
}
