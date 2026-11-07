import type { StreamSettings } from '../streaming/types'
export interface WidgetMetadata {
  id: string
  name: string
  allowed_origins: string[]
  available: boolean
  reason?: string
  turnstile_site_key: string
  appearance: {
    placement: 'inline' | 'floating'
    position?: 'bottom-left' | 'bottom-right'
    theme: 'system' | 'light' | 'dark'
    language: 'en-US' | 'es-CO'
    launcher_label: string
  }
  player: {
    character_name: string
    background: StreamSettings['background']
    blur: number
    poster?: string | null
  }
}
export interface WidgetBootstrap {
  bootstrap_id: string
  nonce: string
  expires_at: string
}
export const WIDGET_CHANNEL = 'syntelle-widget-v1'

export function acceptsParentMessage(
  event: Pick<MessageEvent, 'source' | 'origin' | 'data'>,
  parent: Window,
  origins: readonly string[],
  nonce: string,
): boolean {
  return (
    event.source === parent &&
    origins.includes(event.origin) &&
    event.data?.channel === WIDGET_CHANNEL &&
    event.data?.nonce === nonce
  )
}
