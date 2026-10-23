import { acousticCues, type SpeechBehaviorPlan, type SpeechGesture } from './speechBehavior'
import type { ResolvedPerformance } from '../types/performanceTypes'

// Cooldowns belong to the local performance session, surviving chat resets and
// character component remounts. Reservations are conservative after interruption.
let lastSmile = -Infinity,
  lastLaugh = -Infinity,
  precedingLaugh = false
export function admitPerformance(
  value: ResolvedPerformance | undefined,
  text: string,
  now: number,
  laughAvailable = false,
): ResolvedPerformance | undefined {
  if (
    !value ||
    value.intent?.version !== 'performance-intent/1' ||
    value.intent.text !== text ||
    !Array.isArray(value.intent.cues) ||
    !Array.isArray(value.resolved)
  ) {
    precedingLaugh = false
    return undefined
  }
  let smiled = false,
    laughed = false,
    heads = 0,
    lastHead = -Infinity
  let cues = value.resolved.slice(0, 16).filter((cue) => {
    if (
      !cue ||
      !Number.isFinite(cue.time) ||
      cue.time < 0 ||
      !Number.isFinite(cue.duration) ||
      cue.duration <= 0 ||
      cue.duration > 2.5 ||
      !value.intent.cues.some(
        (anchor) =>
          anchor?.kind === cue.kind &&
          typeof anchor.anchor === 'string' &&
          anchor.anchor.length > 0 &&
          text.split(anchor.anchor).length === 2,
      )
    )
      return false
    if (cue.kind === 'smile') {
      if (smiled || now + cue.time * 1000 - lastSmile < 30000 || value.intent.tone === 'serious')
        return false
      smiled = true
      return true
    }
    if (cue.kind === 'laugh') {
      if (
        !laughAvailable ||
        laughed ||
        precedingLaugh ||
        now + cue.time * 1000 - lastLaugh < 60000 ||
        !['playful', 'demonstration'].includes(value.intent.tone)
      )
        return false
      laughed = true
      return true
    }
    if (!['nod', 'shake', 'emphasis'].includes(cue.kind) || heads >= 8 || cue.time - lastHead < 2.2)
      return false
    lastHead = cue.time
    heads++
    return true
  })
  if (laughed) {
    cues = cues.filter((c) => c.kind !== 'smile')
    smiled = false
  }
  if (smiled) lastSmile = now + cues.find((c) => c.kind === 'smile')!.time * 1000
  if (laughed) lastLaugh = now + cues.find((c) => c.kind === 'laugh')!.time * 1000
  precedingLaugh = laughed
  return { ...value, resolved: cues }
}

export function directedPlan(
  samples: Float32Array,
  sampleRate: number,
  performance: ResolvedPerformance | undefined,
): SpeechBehaviorPlan {
  const duration = samples.length / sampleRate
  const gestures: SpeechGesture[] = []
  for (const cue of performance?.resolved ?? []) {
    if (!['nod', 'shake', 'emphasis'].includes(cue.kind) || cue.time + cue.duration > duration)
      continue
    gestures.push({
      time: cue.time,
      duration: cue.duration,
      kind: cue.kind as SpeechGesture['kind'],
      pitch: cue.kind === 'shake' ? 0 : cue.kind === 'emphasis' ? -0.55 : -1.05,
      yaw: cue.kind === 'shake' ? 1.5 : 0,
    })
  }
  return {
    version: 'speech-context/1',
    duration,
    cues: acousticCues(samples, sampleRate),
    gestures,
    performance,
  }
}
