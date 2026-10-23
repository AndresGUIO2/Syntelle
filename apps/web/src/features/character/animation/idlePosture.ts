import type { HeadPose } from './headMotion'
import { randomSequence } from './timing'

export interface IdleMotionProfile {
  readonly version: 'idle-posture/1'
  readonly transition_ms: readonly [number, number]
  readonly hold_ms: readonly [number, number]
  readonly amplitude_fraction: number
  readonly minimum_yaw_step_fraction: number
}

export const visibleIdleProfile: IdleMotionProfile = Object.freeze({
  version: 'idle-posture/1',
  transition_ms: [4800, 6800] as const,
  hold_ms: [800, 1800] as const,
  amplitude_fraction: 1,
  minimum_yaw_step_fraction: 0.65,
})

// Sample outside a minimum-distance interval. There is no retry loop, fixed
// left/right alternation or chance of choosing the same yaw for another long hold.
function nextYaw(from: number, limit: number, gap: number, random: number) {
  if (limit <= 0) return 0
  const leftEnd = Math.max(-limit, from - gap)
  const rightStart = Math.min(limit, from + gap)
  const leftLength = leftEnd + limit
  const distance = random * (leftLength + limit - rightStart)
  return distance < leftLength ? -limit + distance : rightStart + distance - leftLength
}

export function idlePostureAt(
  milliseconds: number,
  seed: number,
  bounds: readonly number[],
  profile: IdleMotionProfile,
): HeadPose {
  const random = randomSequence(seed)
  const limits = bounds.map((bound) => Math.abs(bound) * profile.amplitude_fraction)
  const from = [0, 0, 0]
  const ms = Math.max(0, milliseconds)
  let start = 0
  for (;;) {
    const transition =
      profile.transition_ms[0] + random() * (profile.transition_ms[1] - profile.transition_ms[0])
    const hold = profile.hold_ms[0] + random() * (profile.hold_ms[1] - profile.hold_ms[0])
    const target = [
      nextYaw(from[0]!, limits[0]!, limits[0]! * profile.minimum_yaw_step_fraction, random()),
      (random() * 2 - 1) * limits[1]!,
      (random() * 2 - 1) * limits[2]!,
    ]
    if (ms < start + transition + hold) {
      const t = Math.min(1, (ms - start) / transition)
      const eased = t * t * (3 - 2 * t)
      const values = from.map((value, i) => value + (target[i]! - value) * eased)
      return { yaw: values[0]!, pitch: values[1]!, roll: values[2]! }
    }
    from.splice(0, 3, ...target)
    start += transition + hold
  }
}
