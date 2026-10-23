import { easeMotion, type HeadPose } from './headMotion'
import { randomSequence } from './timing'
import { gestureAt, type BehaviorSpeechState } from './speechBehavior'
import type { GazeOffset, GazePackage } from '../types/gazeTypes'
import { createSpeakingAttention } from './speakingAttention'

export const attentionSettings = Object.freeze({
  version: 'viewer-attention/1',
  degrees: [2.4, 0.8, 0.25] as const,
  transition_ms: [3000, 6000] as const,
  hold_ms: [1000, 3000] as const,
  glance_interval_ms: [8000, 14000] as const,
  eye_lead_ms: 150,
  glance_move_ms: [80, 140] as const,
  glance_hold_ms: [350, 800] as const,
  glance_return_ms: [120, 220] as const,
  counter_rotation: [-0.09, 0.14] as const,
  head_follow_probability: 0.35,
  head_follow_ms: 1800,
  gesture_blend_ms: 600,
})

// One seeded attention timeline drives both eyes and head, independent of frame rate.
export function attentionAt(ms: number, seed: number, expressive = false) {
  const random = randomSequence(seed ^ 0x51a7),
    from = [0, 0, 0]
  let start = 0,
    index = 0
  const settings = expressive
    ? { ...attentionSettings, transition_ms: [2500, 4500] as const, hold_ms: [600, 1600] as const }
    : attentionSettings
  const between = (range: readonly [number, number], value: number) =>
    range[0] + value * (range[1] - range[0])
  let values: number[]
  for (;;) {
    const duration = between(settings.transition_ms, random()),
      hold = between(settings.hold_ms, random())
    const yaw = (random() * 2 - 1) * settings.degrees[0]
    const pitch =
      index === 0
        ? (random() < 0.5 ? -1 : 1) * settings.degrees[1] * 0.8125
        : (random() * 2 - 1) * settings.degrees[1]
    const target = [
      index === 0 ? (yaw < 0 ? -1 : 1) * settings.degrees[0] * 0.75 : yaw,
      pitch,
      (random() * 2 - 1) * settings.degrees[2],
    ]
    if (ms < start + duration + hold) {
      const t = easeMotion((ms - start) / duration)
      values = from.map((v, i) => v + (target[i]! - v) * t)
      break
    }
    from.splice(0, 3, ...target)
    start += duration + hold
    index++
  }
  const glanceRandom = randomSequence(seed ^ 0x37b8)
  let next = between(settings.glance_interval_ms, glanceRandom())
  let glance: [number, number] = [0, 0],
    follow = 0
  for (;;) {
    const closing = between(settings.glance_move_ms, glanceRandom()),
      hold = between(settings.glance_hold_ms, glanceRandom())
    const opening = between(settings.glance_return_ms, glanceRandom())
    const x = (glanceRandom() < 0.5 ? -1 : 1) * (0.45 + glanceRandom() * 0.25)
    const y = (glanceRandom() * 2 - 1) * 0.22,
      follows = glanceRandom() < settings.head_follow_probability
    if (ms < next) break
    if (
      ms <
      next +
        Math.max(
          closing + hold + opening,
          follows ? settings.head_follow_ms + settings.eye_lead_ms : 0,
        )
    ) {
      const local = ms - next
      const envelope = (time: number) =>
        time < 0
          ? 0
          : time < closing
            ? easeMotion(time / closing)
            : time < closing + hold
              ? 1
              : 1 - easeMotion((time - closing - hold) / opening)
      const t = envelope(local)
      glance = [x * t, y * t]
      // Head follow is a slower, partial response; eye saccades never jerk the head.
      if (follows)
        follow =
          x *
          (local < settings.eye_lead_ms
            ? 0
            : Math.sin(
                Math.PI * Math.min(1, (local - settings.eye_lead_ms) / settings.head_follow_ms),
              ) ** 2)
      break
    }
    next += between(settings.glance_interval_ms, glanceRandom())
  }
  return {
    head: {
      yaw: values[0]! * (1 - Math.abs(follow) * 0.1) + follow * 0.3,
      pitch: values[1]!,
      roll: values[2]!,
    },
    glance,
  }
}

export function gazeOffsets(
  gaze: GazePackage,
  head: HeadPose,
  glance: readonly number[],
): GazeOffset[] {
  const desired = [
    head.yaw * attentionSettings.counter_rotation[0] + glance[0]!,
    head.pitch * attentionSettings.counter_rotation[1] + glance[1]!,
  ]
  // Binocular direction is shared; normalized source limits are the stricter eye's limits.
  const fractions = [0.05, 0.02]
  const safe = desired.map((v, axis) => {
    const sign = v < 0 ? 0 : 1
    const common = Math.min(
      ...gaze.eyes.map((e) => e.bounds_px[axis]![sign]! / (e.width_px * fractions[axis]!)),
    )
    return Math.sign(v) * Math.min(Math.abs(v), common)
  })
  // Elliptical envelope keeps combined horizontal/vertical motion inside axis bounds.
  const length = Math.max(1, Math.hypot(...safe))
  return gaze.eyes.map((e) => [
    (safe[0]! / length) * e.width_px * 0.05,
    (safe[1]! / length) * e.width_px * 0.02,
  ])
}

export function createAttentionDirector(
  seed: number,
  bounds: { yaw: number; pitch: number; roll: number },
  directed = false,
  conversational = false,
  expressive = false,
) {
  const speakingAttention = conversational ? createSpeakingAttention(seed, expressive) : null
  let active = false,
    session = -1,
    changedAt = 0
  let previous: HeadPose = { yaw: 0, pitch: 0, roll: 0 },
    from = { ...previous }
  let idleWeight = 1,
    idleFrom = 1
  return {
    sample(ms: number, speech: BehaviorSpeechState) {
      if (active !== speech.active || session !== speech.session) {
        active = speech.active
        session = speech.session
        changedAt = ms
        from = { ...previous }
        idleFrom = idleWeight
      }
      const blend = easeMotion((ms - changedAt) / attentionSettings.gesture_blend_ms)
      const goal = speech.active
        ? gestureAt(speech.plan, speech.position)
        : { yaw: 0, pitch: 0, roll: 0 }
      previous = {
        yaw: from.yaw * (1 - blend) + goal.yaw * blend,
        pitch: from.pitch * (1 - blend) + goal.pitch * blend,
        roll: 0,
      }
      let attention = attentionAt(ms, seed, expressive)
      if (speakingAttention) {
        attention = speakingAttention.sample(ms, speech.active, attention)
      } else if (directed) {
        idleWeight =
          idleFrom +
          ((speech.active ? 0 : 1) - idleFrom) *
            easeMotion((ms - changedAt) / (speech.active ? 700 : 1800))
        attention.head.yaw *= idleWeight
        attention.head.pitch *= idleWeight
        attention.head.roll *= idleWeight
        attention.glance = attention.glance.map((v) => v * idleWeight) as [number, number]
      }
      const reserve = (base: number, gesture: number, limit: number) =>
        limit <= 0
          ? 0
          : base *
              Math.min(1, limit / Math.max(0.001, Math.abs(base))) *
              (1 - Math.min(1, Math.abs(gesture) / limit)) +
            Math.max(-limit, Math.min(limit, gesture))
      return {
        ...attention,
        head: {
          yaw: reserve(attention.head.yaw, previous.yaw, bounds.yaw),
          pitch: reserve(attention.head.pitch, previous.pitch, bounds.pitch),
          roll: reserve(attention.head.roll, 0, bounds.roll),
        },
      }
    },
  }
}
