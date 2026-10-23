import type { Rect } from './oralTypes'

export interface GazePackage {
  version: 'observed-iris/1'
  source_sha256: string
  motion_build_id: string
  crop: Rect
  eyes: { center: number[]; radii: number[]; width_px: number; bounds_px: number[][] }[]
  assets: Record<'sclera.png' | 'ownership.png', { file: string; sha256: string }>
  validation: { valid: boolean; hidden_iris_generated: boolean; neutral_source_exact: boolean }
}
export interface GazeImages {
  sclera: HTMLImageElement
  ownership: HTMLImageElement
}
export type GazeOffset = readonly [number, number]
