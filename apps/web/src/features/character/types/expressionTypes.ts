import type { Rect } from './oralTypes'

export interface ReferenceLaughterPackage {
  version: 'reference-laughter/1'
  source_sha256: string
  motion_build_id: string
  oral_build_id: string
  reference_sha256: string
  crop: Rect
  frames: readonly { file: string; sha256: string }[]
  validation: { valid: boolean }
  dental_ready: boolean
  eyes: {
    crop: Rect
    profiles: readonly {
      curves: readonly number[][]
      iris_offsets: readonly number[][]
    }[]
  }
  oral: {
    inner: readonly number[][][]
    outer: readonly number[][][]
    lower_translation: readonly number[][]
    cavity: { file: string; sha256: string }
    observed_upper?: { file: string; sha256: string }
    observed_lower?: { file: string; sha256: string }
    jaw_limit_px: number
    contact_shadow?: { version: 'aperture-contact-shadow/1'; strength: number; width_px: number }
  }
}

export interface ReferenceLaughterImages {
  frames: readonly HTMLImageElement[]
  cavity: HTMLImageElement
  upper?: HTMLImageElement
  lower?: HTMLImageElement
}

export interface ExpressionState {
  smile: number
  brows: number
}
export interface ExpressionPackage {
  version: 'local-expressions/1' | 'local-expressions/2'
  source_sha256: string
  motion_build_id: string
  oral_build_id: string
  crop: Rect
  field_size: number[]
  field: { file: string; sha256: string }
  units_per_byte: number
  inverse_iterations: number
  validation: { valid: boolean; minimum_jacobian: number; maximum_inverse_residual_px: number }
  smile?: {
    version: 'registered-closed-smile/1'
    closed_mouth_only: true
    reference_sha256: string
    material: { file: string; sha256: string }
  }
  reference_laughter?: ReferenceLaughterPackage
}
