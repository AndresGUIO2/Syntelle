import { easeMotion, type HeadPose } from './headMotion'
import { randomSequence } from './timing'

export const speakingAttentionSettings = Object.freeze({
  version: 'conversational-attention/1',
  first_contact_ms: [850, 1500] as const,
  contact_ms: [2200, 4300] as const,
  move_ms: [110, 170] as const,
  away_ms: [900, 1700] as const,
  return_ms: [200, 320] as const,
  head_delay_ms: 180,
  head_move_ms: 700,
  head_return_ms: 850,
  posture_weight: 0.3,
  enter_ms: 700,
  settle_ms: 1200,
  phrase_bridge_ms: 900,
})

// Animation time, rather than clip/session time, keeps progressive speech from
// restarting the same glance at every phrase. No attention or emotion is inferred.
export function speakingAttentionAt(ms: number, seed: number) {
  const random = randomSequence(seed ^ 0x6a23)
  const settings = speakingAttentionSettings
  const between = (range: readonly [number, number]) => range[0] + random() * (range[1] - range[0])
  let start = between(settings.first_contact_ms)
  for (;;) {
    const move = between(settings.move_ms),
      hold = between(settings.away_ms),
      back = between(settings.return_ms)
    const x = (random() < 0.5 ? -1 : 1) * (0.65 + random() * 0.2),
      y = random() < 0.75 ? 0.2 + random() * 0.25 : -random() * 0.12
    const headStart = start + settings.head_delay_ms,
      headReturn = start + move + hold,
      end = headReturn + settings.head_return_ms
    if (ms < end) {
      const eye =
        easeMotion((ms - start) / move) * (1 - easeMotion((ms - start - move - hold) / back))
      const head =
        easeMotion((ms - headStart) / settings.head_move_ms) *
        (1 - easeMotion((ms - headReturn) / settings.head_return_ms))
      return {
        glance: [x * eye, y * eye] as [number, number],
        head: { yaw: x * 0.85 * head, pitch: -y * 0.4 * head, roll: -x * 0.06 * head },
      }
    }
    start = end + between(settings.contact_ms)
  }
}

export function createSpeakingAttention(seed: number, expressive = false) {
  let active = false,
    start = 0,
    changedAt = 0,
    lastSpeakingAt = -Infinity,
    weight = 0,
    from = 0,
    episodeSeed = seed
  return {
    sample(ms: number, speaking: boolean, idle: { head: HeadPose; glance: [number, number] }) {
      const settings = speakingAttentionSettings
      if (speaking !== active) {
        if (speaking && ms - lastSpeakingAt > settings.phrase_bridge_ms) {
          start = ms
          episodeSeed = seed ^ Math.floor(ms / 1000)
        }
        active = speaking
        changedAt = ms
        from = weight
      }
      if (speaking) lastSpeakingAt = ms
      weight =
        from +
        ((speaking ? 1 : 0) - from) *
          easeMotion((ms - changedAt) / (speaking ? settings.enter_ms : settings.settle_ms))
      const speech = speakingAttentionAt(Math.max(0, ms - start), episodeSeed)
      const postureWeight = 1 - weight * (1 - (expressive ? 0.65 : settings.posture_weight))
      return {
        head: {
          yaw: idle.head.yaw * postureWeight + speech.head.yaw * weight,
          pitch: idle.head.pitch * postureWeight + speech.head.pitch * weight,
          roll: idle.head.roll * postureWeight + speech.head.roll * weight,
        },
        glance: idle.glance.map(
          (value, axis) => value * (1 - weight) + speech.glance[axis]! * weight,
        ) as [number, number],
      }
    },
  }
}
