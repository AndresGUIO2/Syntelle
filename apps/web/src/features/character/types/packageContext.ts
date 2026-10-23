import type { PosePackage } from './oralTypes'
import type { MotionPackage } from './motionTypes'
import type { IdleMotionProfile } from '../animation/idlePosture'
import type { GazePackage } from './gazeTypes'
import type { ExpressionPackage } from './expressionTypes'
import type { LaughterPackage } from './laughterTypes'

export interface TrialSelection {
  readonly character?: string | null
  readonly oral?: string | null
  readonly trial?: string | null
  readonly head?: string | null
  readonly render?: string | null
}

export interface OralContext {
  readonly package: PosePackage
  readonly character: string | null
  readonly completed: boolean
  readonly semantic: boolean
  readonly assetUrl: (name: string) => string
}

export interface MotionContext {
  readonly expressive?: boolean
  readonly performance?: boolean
  readonly conversationalAttention?: boolean
  readonly laughter?: LaughterPackage
  readonly definition: MotionPackage | undefined
  readonly idle?: IdleMotionProfile
  readonly attention?: GazePackage
  readonly expressions?: ExpressionPackage
  readonly assetUrl: (name: string) => string
}
