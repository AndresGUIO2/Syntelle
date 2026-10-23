import { randomSequence } from './timing'

export interface HeadPose {
  yaw: number
  pitch: number
  roll: number
}
export interface RotationProfile {
  version: string
  minimum_yaw_fraction: number
  turn_duration_ms: number
}
export interface SpeechBeat {
  time: number
  strength: number
}
export interface SpeakingMotion {
  active: boolean
  time: number
  beats: SpeechBeat[]
}
export const easeMotion = (t: number) => {
  const p = Math.max(0, Math.min(1, t))
  return p * p * p * (10 + p * (-15 + 6 * p))
}

export function idlePose(
  milliseconds: number,
  seed: number,
  bounds: number[],
  profile?: RotationProfile,
): HeadPose {
  const random = randomSequence(seed)
  let start = 0,
    segment = 0,
    from = [0, 0, 0]
  for (;;) {
    const duration = 3000 + random() * 4000
    if (profile) {
      const direction = (seed % 2 ? -1 : 1) * (segment % 2 ? -1 : 1)
      const target = [
        direction *
          (profile.minimum_yaw_fraction + random() * (1 - profile.minimum_yaw_fraction)) *
          bounds[0]!,
        (random() * 2 - 1) * bounds[1]! * 0.5,
        (random() * 2 - 1) * bounds[2]! * 0.5,
      ]
      if (milliseconds < start + duration) {
        const phase = (milliseconds - start - 150) / profile.turn_duration_ms
        const p =
          phase <= 0 || phase >= 1
            ? 0
            : phase < 0.35
              ? easeMotion(phase / 0.35)
              : phase < 0.47
                ? 1
                : 1 - easeMotion((phase - 0.47) / 0.53)
        return { yaw: target[0]! * p, pitch: target[1]! * p, roll: target[2]! * p }
      }
      start += duration
      segment++
      continue
    }
    const transition = 1200 + random() * 1600
    const target = bounds.map((bound) => (random() * 2 - 1) * bound)
    if (milliseconds < start + duration) {
      const p = easeMotion((milliseconds - start) / transition)
      const value = from.map((v, i) => v + (target[i]! - v) * p)
      return { yaw: value[0]!, pitch: value[1]!, roll: value[2]! }
    }
    from = target
    start += duration
  }
}

export function speechBeats(samples: Float32Array, sampleRate: number): SpeechBeat[] {
  const step = Math.max(1, Math.round(sampleRate * 0.02)),
    energy: number[] = []
  let smoothed = 0
  for (let i = 0; i < samples.length; i += step) {
    let sum = 0
    for (let j = i; j < Math.min(samples.length, i + step); j++) sum += samples[j]! ** 2
    const rms = Math.sqrt(sum / Math.min(step, samples.length - i))
    smoothed += (rms - smoothed) * (rms > smoothed ? 0.45 : 0.12)
    energy.push(smoothed)
  }
  const peak = Math.max(0.01, ...energy),
    beats: SpeechBeat[] = []
  for (let i = 2; i < energy.length - 2; i++) {
    if (energy[i]! < peak * 0.28 || energy[i]! < energy[i - 1]! || energy[i]! <= energy[i + 1]!)
      continue
    const time = (i * step) / sampleRate
    if (time - (beats.at(-1)?.time ?? -2) < 1.2) continue
    beats.push({ time, strength: Math.min(1, energy[i]! / peak) })
  }
  return beats
}

export function createHeadDirector(
  seed: number,
  bounds: { idle: number[]; speaking: number[] },
  profile?: RotationProfile,
) {
  let active = false,
    changedAt = 0,
    from = 0,
    goal = 0
  let lastNod = 0,
    lastTurn = 0
  function weight(ms: number) {
    return from + (goal - from) * easeMotion((ms - changedAt) / 650)
  }
  return {
    sample(ms: number, speech: SpeakingMotion): HeadPose {
      if (active !== speech.active) {
        from = weight(ms)
        goal = speech.active ? 1 : 0
        changedAt = ms
        active = speech.active
      }
      const blend = weight(ms),
        base = idlePose(ms, seed, bounds.idle, profile)
      let nod = 0,
        turn = 0
      for (let i = 0; speech.active && i < speech.beats.length; i++) {
        const beat = speech.beats[i]!,
          p = (speech.time - beat.time + 0.12) / 0.95
        if (p < 0 || p > 1) continue
        const pulse = p < 0.42 ? easeMotion(p / 0.42) : 1 - easeMotion((p - 0.42) / 0.58)
        nod -= pulse * beat.strength * 1.2
        turn +=
          pulse * beat.strength * (i % 2 ? -1 : 1) * (profile ? bounds.speaking[0]! * 0.35 : 0.7)
      }
      if (speech.active) {
        lastNod = nod
        lastTurn = turn
      } else {
        nod = lastNod
        turn = lastTurn
      }
      const values = [base.yaw + turn * blend, base.pitch + nod * blend, base.roll]
      const limit = bounds.idle.map((v, i) => v + (bounds.speaking[i]! - v) * blend)
      const clamped = values.map((v, i) => Math.max(-limit[i]!, Math.min(limit[i]!, v)))
      return { yaw: clamped[0]!, pitch: clamped[1]!, roll: clamped[2]! }
    },
  }
}
