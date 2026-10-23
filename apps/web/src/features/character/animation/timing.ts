import type { MotionPackage } from '../types/motionTypes'

export function blinkAt(time: number, settings: MotionPackage['blink']) {
  const ease = (p: number) => {
    const n = Math.max(0, Math.min(1, p))
    return n * n * (3 - 2 * n)
  }
  if (time < 0) return 0
  if (time < settings.closing_ms) return ease(time / settings.closing_ms)
  if (time < settings.closing_ms + settings.closed_ms) return 1
  return 1 - ease((time - settings.closing_ms - settings.closed_ms) / settings.opening_ms)
}
export function randomSequence(seed: number) {
  let state = seed >>> 0
  return () => {
    state = (1664525 * state + 1013904223) >>> 0
    return state / 4294967296
  }
}
