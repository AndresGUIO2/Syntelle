import type { Rect } from './oralTypes'
import type { RotationProfile } from '../animation/headMotion'
import type { BehaviorSettings } from '../animation/speechBehavior'
import type { BreathingSettings } from '../animation/breathing'
import type { ProjectionRig } from './projectionTypes'
import type { GazeImages } from './gazeTypes'
export interface MotionPackage {
  build_id: string
  original_sha256: string
  oral_build_id: string
  canvas: { width: number; height: number }
  foreground: string
  depth: string
  behavior?: BehaviorSettings
  breathing?: BreathingSettings
  animation_seed?: number
  projection?: ProjectionRig
  eyes: { crop: Rect; open: string; closed: string; mask: string; curves: number[][] }
  anatomy: { head_pivot: number[]; chin_y: number; face_width: number; face_height: number }
  blink: {
    closing_ms: number
    closed_ms: number
    opening_ms: number
    minimum_interval_ms: number
    maximum_interval_ms: number
  }
  motion: {
    yaw_degrees: number
    pitch_degrees: number
    roll_degrees: number
    breath_period_ms: number
    breath_face_height_fraction: number
  }
  rig?: {
    version: string
    grid: number
    vertices: number[][]
    pivot: number[]
    bounds: { idle: number[]; speaking: number[] }
    posture?: RotationProfile
    geometry_validation: { valid: boolean; minimum_area_ratio: number }
    source_sha256: string
    previous_build_id: string
  }
}
export interface MotionImages {
  expressionField?: HTMLImageElement
  smileMaterial?: HTMLImageElement
  gaze?: GazeImages
  geometry?: ArrayBuffer
  foreground: HTMLImageElement
  depth: HTMLImageElement
  opened: HTMLImageElement
  closed: HTMLImageElement
  mask: HTMLImageElement
}
export interface Background {
  id: string
  name: string
  image: string
  thumbnail: string
  source_sha256: string
}
