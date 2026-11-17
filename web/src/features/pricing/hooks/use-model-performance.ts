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
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'

import { getPerfMetrics } from '@/features/performance-metrics/api'
import { requireServerSuccess } from '@/lib/server-error-message'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { projectModelMetrics } from '../lib/reference-metrics'
import type { PricingModel } from '../types'

export function useModelPerformance(model: PricingModel) {
  const enabled = useSystemConfigStore(
    (state) => state.config.modelSquareReferenceDataEnabled === true
  )
  const query = useQuery({
    queryKey: ['perf-metrics', model.model_name],
    queryFn: async () =>
      requireServerSuccess(await getPerfMetrics(model.model_name, 24)),
    staleTime: 60 * 1000,
    refetchInterval: 60 * 1000,
  })
  const data = useMemo(
    () =>
      query.data
        ? projectModelMetrics(model, query.data.data, enabled && !query.isError)
        : undefined,
    [model, query.data, query.isError, enabled]
  )
  return { ...query, displayData: data }
}
