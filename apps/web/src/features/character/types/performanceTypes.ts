export type PerformanceKind = 'nod' | 'shake' | 'emphasis' | 'smile' | 'laugh'
export interface PerformanceIntent {
  version: 'performance-intent/1'
  text: string
  tone: 'neutral' | 'serious' | 'playful' | 'demonstration'
  cues: readonly { kind: PerformanceKind; anchor: string }[]
}
export interface ResolvedPerformance {
  intent: PerformanceIntent
  omitted: readonly string[]
  timing?: string
  resolved?: readonly { kind: PerformanceKind; time: number; duration: number }[]
}
