export const AVA_VOICE = 'en-US-Ava:DragonHDLatestNeural'

export interface StreamSettings {
  character: 'syntelle'
  instructions: string
  language: 'en-US' | 'es-CO'
  voice: string
  background: 'light-studio' | 'dark-lounge' | 'natural-space'
  blur: number
  blinking: boolean
  movement: boolean
  automatic_expressions: boolean
  smile: number
  brows: number
  reduced_motion: boolean
}

export interface SavedConfiguration {
  id: string
  name: string
  package_hash: string
  settings: StreamSettings
  created_at: string
  updated_at: string
}

export interface Account {
  id: string
  email: string
  name: string
}

export interface IntegrationKey {
  id: string
  name: string
  created_at: string
  last_used_at?: string | null
}

export interface AuthStatus {
  email: { configured: boolean; reason?: string }
  google: { configured: boolean; reason?: string }
  github: { configured: boolean; reason?: string }
}

export type Envelope = {
  type: string
  session_id: string
  command_id?: string
  data?: unknown
}

export function defaultSettings(): StreamSettings {
  return {
    character: 'syntelle',
    instructions:
      'Be a clear, friendly assistant. Match the depth and length to the question. Do not invent company knowledge.',
    language: 'en-US',
    voice: AVA_VOICE,
    background: 'natural-space',
    blur: 0,
    blinking: true,
    movement: true,
    automatic_expressions: true,
    smile: 0,
    brows: 0,
    reduced_motion: false,
  }
}
