import type { StreamSettings } from '../streaming/types'
export interface Publication {
  id: string
  workspace_id: string
  name: string
  character_id: string
  package_hash: string
  settings: StreamSettings
  portrait_available?: boolean
  created_at: string
  revoked_at?: string | null
}
export interface WidgetAppearance {
  placement: 'inline' | 'floating'
  position: 'bottom-right' | 'bottom-left'
  theme: 'system' | 'light' | 'dark'
  language: 'en-US' | 'es-CO'
  launcher_label: string
}
export interface Integration {
  id: string
  workspace_id: string
  name: string
  publication_id: string
  widget_id: string
  allowed_origins: string[]
  capabilities: string[]
  settings: StreamSettings
  appearance?: WidgetAppearance
  daily_limit_seconds?: number
  revision: number
  published_revision: number
  enabled: boolean
  created_at: string
  updated_at: string
}
export interface IntegrationKey {
  id: string
  name: string
  integration_id?: string
  status: string
  revoked_at: string | null
}
export interface BillingPlan {
  id: string
  name: string
  amount: number
  currency: string
  interval: string
  included_minutes: number
  concurrency: number
  max_session_seconds: number
}
export interface Topup {
  id: string
  name: string
  amount: number
  currency: string
  minutes: number
}
export interface Billing {
  configured: boolean
  environment: 'sandbox'
  client_token?: string
  reason: string
  can_manage: boolean
  catalog: { plans: BillingPlan[]; topups: Topup[] }
  subscription: null | {
    id: string
    plan: string
    status: string
    current_period_start: string
    current_period_end: string
    cancel_at_period_end: boolean
    pending_plan: string
  }
  review_required: boolean
}
export interface Usage {
  included_seconds_remaining: number
  topup_seconds_remaining: number
  used_seconds: number
  reserved_seconds: number
  active_sessions: number
  concurrency_limit: number
  max_session_seconds: number
  period_start: string
  period_end: string
  by_integration: { integration_id: string; seconds: number; sessions: number }[]
}
