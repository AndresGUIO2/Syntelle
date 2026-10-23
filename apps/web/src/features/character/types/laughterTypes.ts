import type { Rect } from './oralTypes'
export interface LaughterPackage {
  version: 'registered-laughter/1'
  source_sha256: string
  motion_build_id: string
  oral_build_id: string
  crop: Rect
  frames: readonly { file: string; sha256: string }[]
  validation: { valid: boolean }
  review_status: 'development-composition-accepted'
  generated_dental_installed: false
  audio: readonly {
    file: string
    sha256: string
    voice: string
    language: string
    region: string
    duration_ms: number
  }[]
}
