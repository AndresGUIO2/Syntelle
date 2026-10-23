import { easeMotion } from './headMotion'
import { randomSequence } from './timing'
import type { BehaviorSpeechState, SpeechBehaviorPlan } from './speechBehavior'
import type { ExpressionState } from '../types/expressionTypes'

export const expressionSettings = Object.freeze({
  version: 'speech-expressions/2',
  manual_ease_ms: 240,
  release_ms: 900,
  minimum_cue_gap_seconds: 3.2,
  emphasis_probability: 0.45,
})
interface ExpressionCue {
  time: number
  duration: number
  smile: number
  brows: number
}
const neutral = (): ExpressionState => ({ smile: 0, brows: 0 })
const clamp = (v: number) => (Number.isFinite(v) ? Math.max(0, Math.min(1, v)) : 0)

export function planExpressions(
  plan: SpeechBehaviorPlan | null,
  seed: number,
  directed = false,
  expressive = false,
): ExpressionCue[] {
  if (!plan) return []
  if (directed)
    return (plan.performance?.resolved ?? [])
      .filter((c) => c.kind === 'smile' || c.kind === 'emphasis')
      .map((c) => ({
        time: c.time,
        duration: c.duration,
        smile: c.kind === 'smile' ? 0.4 : 0,
        brows: c.kind === 'emphasis' ? (expressive ? 0.5 : 0.3) : 0,
      }))
  const random = randomSequence(seed ^ 0x46e8),
    cues: ExpressionCue[] = []
  for (const cue of plan.cues) {
    if (cue.kind === 'onset') {
      cues.push({ time: cue.time + 0.1, duration: 2.1, smile: 0, brows: 0.22 })
    } else if (
      (cue.kind === 'emphasis' || cue.kind === 'pause') &&
      random() < expressionSettings.emphasis_probability &&
      !cues.some(
        (other) => Math.abs(other.time - cue.time) < expressionSettings.minimum_cue_gap_seconds,
      )
    ) {
      cues.push({
        time: Math.max(0, cue.time - 0.12),
        duration: 1.5,
        smile: 0,
        brows: 0.35 + 0.15 * cue.strength,
      })
    }
  }
  return cues
}

export function expressionCueAt(cues: readonly ExpressionCue[], position: number): ExpressionState {
  const result = neutral()
  for (const cue of cues) {
    const t = (position - cue.time) / cue.duration
    if (t < 0 || t >= 1) continue
    const weight = t < 0.3 ? easeMotion(t / 0.3) : t < 0.6 ? 1 : 1 - easeMotion((t - 0.6) / 0.4)
    result.smile = Math.max(result.smile, cue.smile * weight)
    result.brows = Math.max(result.brows, cue.brows * weight)
  }
  return result
}

export function createExpressionDirector(
  seed: number,
  smileAvailable = false,
  directed = false,
  expressive = false,
) {
  let cachedPlan: SpeechBehaviorPlan | null = null,
    cues: ExpressionCue[] = []
  let manualFrom = neutral(),
    manualGoal = neutral(),
    manualAt = 0,
    manualCurrent = neutral()
  let previousAuto = neutral(),
    releaseFrom = neutral(),
    releaseAt = 0,
    wasActive = false
  let previousSession = -1
  let wasSpeaking = false,
    completionAt = -Infinity
  return {
    sample(
      ms: number,
      manual: ExpressionState,
      automatic: boolean,
      reduced: boolean,
      speech: BehaviorSpeechState,
      instantManual = false,
      mouthBusy = false,
    ): ExpressionState {
      if (
        !directed &&
        wasSpeaking &&
        !speech.active &&
        speech.plan &&
        speech.position >= speech.plan.duration - 0.15
      )
        completionAt = ms
      if (speech.active || mouthBusy || !automatic || reduced) completionAt = -Infinity
      wasSpeaking = speech.active
      const sinceCompletion = ms - completionAt
      const completionSmile =
        automatic && smileAvailable && !reduced && !mouthBusy
          ? 0.4 *
            easeMotion(sinceCompletion / 300) *
            (1 - easeMotion((sinceCompletion - 800) / 900))
          : 0
      const goal = {
        smile: smileAvailable && !mouthBusy ? clamp(manual.smile) : 0,
        brows: clamp(manual.brows),
      }
      if (goal.smile !== manualGoal.smile || goal.brows !== manualGoal.brows) {
        manualFrom = { ...manualCurrent }
        manualGoal = goal
        manualAt = ms
      }
      const blend =
        reduced || instantManual
          ? 1
          : easeMotion((ms - manualAt) / expressionSettings.manual_ease_ms)
      manualCurrent = {
        smile: manualFrom.smile + (goal.smile - manualFrom.smile) * blend,
        brows: manualFrom.brows + (goal.brows - manualFrom.brows) * blend,
      }
      const active = automatic && !reduced && speech.active
      if (wasActive !== active || previousSession !== speech.session) {
        releaseFrom = { ...previousAuto }
        releaseAt = ms
      }
      if (speech.session !== previousSession || speech.plan !== cachedPlan) {
        cues = planExpressions(speech.plan, seed, directed, expressive)
        cachedPlan = speech.plan
        previousSession = speech.session
      }
      const release = 1 - easeMotion((ms - releaseAt) / expressionSettings.release_ms)
      const target = active ? expressionCueAt(cues, speech.position) : neutral()
      previousAuto = reduced
        ? neutral()
        : {
            smile: releaseFrom.smile * release + target.smile * (1 - release),
            brows: releaseFrom.brows * release + target.brows * (1 - release),
          }
      wasActive = active
      return {
        smile:
          1 -
          (1 - manualCurrent.smile) *
            (1 - (directed && !mouthBusy && smileAvailable ? previousAuto.smile : completionSmile)),
        brows: 1 - (1 - manualCurrent.brows) * (1 - previousAuto.brows),
      }
    },
  }
}
