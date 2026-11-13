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
import { HotRestartPanel } from '@/features/hot-restart/HotRestartPanel'

import { SystemBehaviorSection } from '../general/system-behavior-section'
import { EmailSettingsSection } from '../integrations/email-settings-section'
import { MonitoringSettingsSection } from '../integrations/monitoring-settings-section'
import { WorkerSettingsSection } from '../integrations/worker-settings-section'
import { LogSettingsSection } from '../maintenance/log-settings-section'
import { PerformanceSection } from '../maintenance/performance-section'
import { UpdateCheckerSection } from '../maintenance/update-checker-section'
import type { OperationsSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'
import { APIMaintenanceSection } from './api-maintenance-section'
import {
  arrivalSettingsValues,
  rebateSettingsValues,
} from './arrival-rebate-settings'
import { ArrivalSettingsSection } from './arrival-settings-section'
import { ErrorRewriteSection } from './error-rewrite-section'
import { LogCleanupSection } from './log-cleanup-section'
import { RebateSettingsSection } from './rebate-settings-section'

const OPERATIONS_SECTIONS = [
  {
    id: 'hot-restart',
    titleKey: 'Linux graceful hot restart',
    build: () => <HotRestartPanel />,
  },
  {
    id: 'arrival',
    titleKey: 'Arrival auto-grouping',
    build: (settings: OperationsSettings) => (
      <ArrivalSettingsSection defaultValues={arrivalSettingsValues(settings)} />
    ),
  },
  {
    id: 'rebates',
    titleKey: 'Global arrival rebates',
    build: (settings: OperationsSettings) => (
      <RebateSettingsSection defaultValues={rebateSettingsValues(settings)} />
    ),
  },
  {
    id: 'log-cleanup',
    titleKey: 'Database log automatic cleanup',
    build: (settings: OperationsSettings) => (
      <LogCleanupSection
        defaultValues={{
          'log_setting.auto_cleanup_enabled':
            settings['log_setting.auto_cleanup_enabled'],
          'log_setting.cleanup_retention_days':
            settings['log_setting.cleanup_retention_days'],
          'log_setting.cleanup_scope': settings['log_setting.cleanup_scope'],
        }}
      />
    ),
  },
  {
    id: 'api-maintenance',
    titleKey: 'API maintenance',
    build: (settings: OperationsSettings) => (
      <APIMaintenanceSection
        defaultValues={{
          enabled: settings['maintenance_setting.enabled'] ?? false,
          message: settings['maintenance_setting.message'] ?? '',
        }}
      />
    ),
  },
  {
    id: 'behavior',
    titleKey: 'System Behavior',
    build: (settings: OperationsSettings) => (
      <SystemBehaviorSection
        defaultValues={{
          DefaultCollapseSidebar: settings.DefaultCollapseSidebar,
          DemoSiteEnabled: settings.DemoSiteEnabled,
          SelfUseModeEnabled: settings.SelfUseModeEnabled,
          NormalizeResponseModelEnabled:
            settings['general_setting.normalize_response_model_enabled'] ??
            false,
        }}
      />
    ),
  },
  {
    id: 'error-rewrite',
    titleKey: 'Error message rewriting',
    build: (settings: OperationsSettings) => (
      <ErrorRewriteSection
        defaultValues={{
          ErrorMessageRewriteEnabled: settings.ErrorMessageRewriteEnabled,
          ErrorMessageRewriteRules: settings.ErrorMessageRewriteRules,
        }}
      />
    ),
  },
  {
    id: 'alerts',
    titleKey: 'Model performance metrics',
    build: (settings: OperationsSettings) => (
      <MonitoringSettingsSection
        defaultValues={{
          'perf_metrics_setting.enabled':
            settings['perf_metrics_setting.enabled'] ?? true,
          'perf_metrics_setting.flush_interval':
            settings['perf_metrics_setting.flush_interval'] ?? 5,
          'perf_metrics_setting.bucket_time':
            settings['perf_metrics_setting.bucket_time'] ?? 'hour',
          'perf_metrics_setting.retention_days':
            settings['perf_metrics_setting.retention_days'] ?? 0,
        }}
      />
    ),
  },
  {
    id: 'email',
    titleKey: 'Email & Notifications',
    build: (settings: OperationsSettings) => (
      <EmailSettingsSection
        defaultValues={{
          QuotaRemindEnabled: settings.QuotaRemindEnabled,
          QuotaRemindThreshold: settings.QuotaRemindThreshold,
          EmailProvider: settings.EmailProvider,
          ResendApiKey: settings.ResendApiKey,
          ResendKeyConfigured: settings.ResendKeyConfigured,
          ResendFrom: settings.ResendFrom,
          SMTPServer: settings.SMTPServer,
          SMTPPort: settings.SMTPPort,
          SMTPAccount: settings.SMTPAccount,
          SMTPFrom: settings.SMTPFrom,
          SMTPToken: settings.SMTPToken,
          SMTPSSLEnabled: settings.SMTPSSLEnabled,
          SMTPStartTLSEnabled: settings.SMTPStartTLSEnabled,
          SMTPInsecureSkipVerify: settings.SMTPInsecureSkipVerify,
          SMTPForceAuthLogin: settings.SMTPForceAuthLogin,
        }}
      />
    ),
  },
  {
    id: 'worker',
    titleKey: 'Worker Proxy',
    build: (settings: OperationsSettings) => (
      <WorkerSettingsSection
        defaultValues={{
          WorkerUrl: settings.WorkerUrl,
          WorkerValidKey: settings.WorkerValidKey,
          WorkerAllowHttpImageRequestEnabled:
            settings.WorkerAllowHttpImageRequestEnabled,
        }}
      />
    ),
  },
  {
    id: 'logs',
    titleKey: 'Log Maintenance',
    build: (settings: OperationsSettings) => (
      <LogSettingsSection
        defaultValues={{
          LogConsumeEnabled: Boolean(settings.LogConsumeEnabled),
          'log_setting.user_export_enabled':
            settings['log_setting.user_export_enabled'] ?? true,
          'log_setting.max_range_days':
            settings['log_setting.max_range_days'] ?? 365,
          'system_task_setting.retention_days':
            settings['system_task_setting.retention_days'] ?? 30,
        }}
      />
    ),
  },
  {
    id: 'performance',
    titleKey: 'Performance',
    build: (settings: OperationsSettings) => (
      <PerformanceSection
        defaultValues={{
          'performance_setting.disk_cache_enabled':
            settings['performance_setting.disk_cache_enabled'] ?? false,
          'performance_setting.disk_cache_threshold_mb':
            settings['performance_setting.disk_cache_threshold_mb'] ?? 10,
          'performance_setting.disk_cache_max_size_mb':
            settings['performance_setting.disk_cache_max_size_mb'] ?? 1024,
          'performance_setting.disk_cache_path':
            settings['performance_setting.disk_cache_path'] ?? '',
          'performance_setting.monitor_enabled':
            settings['performance_setting.monitor_enabled'] ?? false,
          'performance_setting.monitor_cpu_threshold':
            settings['performance_setting.monitor_cpu_threshold'] ?? 90,
          'performance_setting.monitor_memory_threshold':
            settings['performance_setting.monitor_memory_threshold'] ?? 90,
          'performance_setting.monitor_disk_threshold':
            settings['performance_setting.monitor_disk_threshold'] ?? 95,
        }}
      />
    ),
  },
  {
    id: 'update-checker',
    titleKey: 'System maintenance',
    build: (
      _settings: OperationsSettings,
      currentVersion?: string | null,
      startTime?: number | null
    ) => (
      <UpdateCheckerSection
        currentVersion={currentVersion}
        startTime={startTime}
      />
    ),
  },
] as const

export type OperationsSectionId = (typeof OPERATIONS_SECTIONS)[number]['id']

const operationsRegistry = createSectionRegistry<
  OperationsSectionId,
  OperationsSettings,
  [string | null | undefined, number | null | undefined]
>({
  sections: OPERATIONS_SECTIONS,
  defaultSection: 'behavior',
  basePath: '/system-settings/operations',
  urlStyle: 'path',
})

export const OPERATIONS_SECTION_IDS = operationsRegistry.sectionIds
export const OPERATIONS_DEFAULT_SECTION = operationsRegistry.defaultSection
export const getOperationsSectionNavItems =
  operationsRegistry.getSectionNavItems
export const getOperationsSectionContent = operationsRegistry.getSectionContent
export const getOperationsSectionMeta = operationsRegistry.getSectionMeta
