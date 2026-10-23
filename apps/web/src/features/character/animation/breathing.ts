import { randomSequence } from './timing'

export interface BreathingSettings {
  version: 'asymmetric-breath/1'
  duration_ms: [number, number]
  inhale_fraction: [number, number]
  amplitude: [number, number]
}

const ease = (t: number) => t * t * t * (t * (t * 6 - 15) + 10)

// Every cycle meets at neutral with zero velocity/acceleration. Parameters vary
// between cycles, never inside an inhale/exhale, and remain inside the rig bound.
export function createBreathingTrack(seed: number, settings: BreathingSettings) {
  const valid = (range: [number, number], low: number, high: number) =>
    range.every(Number.isFinite) && range[0] >= low && range[0] <= range[1] && range[1] <= high
  if (
    !valid(settings.duration_ms, 1000, 15000) ||
    !valid(settings.inhale_fraction, 0.1, 0.49) ||
    !valid(settings.amplitude, 0, 1)
  )
    throw Error('Breathing settings exceed the validated motion envelope')
  let random = randomSequence(seed),
    start = 0
  const between = ([low, high]: [number, number]) => low + random() * (high - low)
  const next = () => ({
    duration: between(settings.duration_ms),
    inhale: between(settings.inhale_fraction),
    amplitude: between(settings.amplitude),
  })
  let cycle = next()
  return {
    sample(milliseconds: number) {
      const ms = Math.max(0, milliseconds)
      if (ms < start) {
        random = randomSequence(seed)
        start = 0
        cycle = next()
      }
      while (ms >= start + cycle.duration) {
        start += cycle.duration
        cycle = next()
      }
      const phase = (ms - start) / cycle.duration
      const coverage =
        phase < cycle.inhale
          ? ease(phase / cycle.inhale)
          : 1 - ease((phase - cycle.inhale) / (1 - cycle.inhale))
      return coverage === 0 ? 0 : -cycle.amplitude * coverage
    },
  }
}
