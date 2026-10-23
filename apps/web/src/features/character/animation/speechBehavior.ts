import { easeMotion, type HeadPose } from './headMotion'
import { randomSequence } from './timing'
import { idlePostureAt, type IdleMotionProfile } from './idlePosture'
import type { ResolvedPerformance } from '../types/performanceTypes'

export interface BehaviorSettings {
  version: 'speech-context/1' | 'speech-context/2'
  head: {
    idle_degrees: number[]
    maximum_nod_degrees: number
    maximum_turn_degrees: number
    minimum_gesture_gap_seconds: number
  }
  blink: {
    rate_per_minute: number
    minimum_gap_ms: number
    pause_probability: number
    ending_probability: number
    onset_probability: number
    nod_probability: number
    double?: {
      probability: number
      open_gap_ms: [number, number]
      minimum_interval_ms: number
    }
  }
}
export interface SpeechCue {
  time: number
  kind: 'onset' | 'pause' | 'ending' | 'emphasis' | 'nod'
  strength: number
}
export interface SpeechGesture {
  time: number
  duration: number
  pitch: number
  yaw: number
  kind: SpeechCue['kind'] | 'shake'
}
export interface SpeechBehaviorPlan {
  version: 'speech-context/1'
  duration: number
  cues: readonly SpeechCue[]
  gestures: readonly SpeechGesture[]
  performance?: ResolvedPerformance
}
export interface BehaviorSpeechState {
  active: boolean
  position: number
  session: number
  plan: SpeechBehaviorPlan | null
}

// Acoustic boundaries are cues, not semantic sentence or emotion labels.
export function acousticCues(samples: Float32Array, sampleRate: number): SpeechCue[] {
  if (sampleRate <= 0 || !samples.length) return []
  const step = Math.max(1, Math.round(sampleRate * 0.02)),
    energy: number[] = []
  let peak = 0
  for (let i = 0; i < samples.length; i += step) {
    let sum = 0
    const length = Math.min(step, samples.length - i)
    for (let j = i; j < i + length; j++) sum += samples[j]! ** 2
    const rms = Math.sqrt(sum / length)
    energy.push(rms)
    peak = Math.max(peak, rms)
  }
  if (peak < 0.005) return []
  const seconds = step / sampleRate,
    threshold = Math.max(0.003, peak * 0.09)
  const voiced = energy.map((value) => value >= threshold)
  // Bridge brief unvoiced consonants; they are not phrase pauses.
  for (let i = 0; i < voiced.length;) {
    if (voiced[i]) {
      i++
      continue
    }
    const start = i
    while (i < voiced.length && !voiced[i]) i++
    if (start > 0 && i < voiced.length && (i - start) * seconds < 0.16) voiced.fill(true, start, i)
  }
  const regions: { start: number; end: number }[] = []
  for (let i = 0; i < voiced.length;) {
    if (!voiced[i]) {
      i++
      continue
    }
    const start = i
    while (i < voiced.length && voiced[i]) i++
    if ((i - start) * seconds >= 0.1) regions.push({ start: start * seconds, end: i * seconds })
  }
  if (!regions.length) return []
  const cues: SpeechCue[] = [{ time: regions[0]!.start, kind: 'onset', strength: 0.5 }]
  for (let i = 1; i < regions.length; i++) {
    const previous = regions[i - 1]!,
      gap = regions[i]!.start - previous.end
    if (gap >= 0.32)
      cues.push({ time: previous.end + 0.1, kind: 'pause', strength: Math.min(1, gap / 0.8) })
  }
  let lastAccent = -2
  for (let i = 4; i < energy.length - 4; i++) {
    const value = energy[i]!,
      time = i * seconds
    if (!voiced[i] || time - lastAccent < 1.5 || value < peak * 0.65) continue
    if (energy.slice(i - 4, i + 5).some((other) => other > value)) continue
    if (value < (energy.slice(i - 4, i + 5).reduce((sum, other) => sum + other, 0) / 9) * 1.08)
      continue
    cues.push({ time, kind: 'emphasis', strength: value / peak })
    lastAccent = time
  }
  cues.push({
    time: Math.max(regions[0]!.start, regions.at(-1)!.end - 0.12),
    kind: 'ending',
    strength: 0.8,
  })
  return cues.sort((a, b) => a.time - b.time)
}

export function planSpeechBehavior(
  samples: Float32Array,
  sampleRate: number,
  seed: number,
  settings: BehaviorSettings,
): SpeechBehaviorPlan {
  const cues = acousticCues(samples, sampleRate),
    random = randomSequence(seed),
    gestures: SpeechGesture[] = []
  // Prefer pauses/endings over nearby energy peaks and never nod on each syllable.
  const candidates = [
    ...cues.filter((cue) => cue.kind === 'pause' || cue.kind === 'ending'),
    ...cues.filter((cue) => cue.kind === 'emphasis'),
  ]
  for (const cue of candidates) {
    const chance = cue.kind === 'emphasis' ? 0.22 : cue.kind === 'ending' ? 0.6 : 0.5
    if (
      random() >= chance ||
      cue.time < 0.35 ||
      gestures.some((g) => Math.abs(g.time - cue.time) < settings.head.minimum_gesture_gap_seconds)
    )
      continue
    gestures.push({
      time: Math.max(0, cue.time - 0.16),
      duration: 1.2 + random() * 0.3,
      pitch: -settings.head.maximum_nod_degrees * (0.55 + cue.strength * 0.45),
      yaw: random() < 0.2 ? (random() * 2 - 1) * settings.head.maximum_turn_degrees : 0,
      kind: cue.kind,
    })
  }
  gestures.sort((a, b) => a.time - b.time)
  cues.push(
    ...gestures.map((g): SpeechCue => ({
      time: g.time + g.duration * 0.3,
      kind: 'nod',
      strength: 0.5,
    })),
  )
  return {
    version: 'speech-context/1',
    duration: samples.length / sampleRate,
    cues: cues.sort((a, b) => a.time - b.time),
    gestures,
  }
}

export function gestureAt(plan: SpeechBehaviorPlan | null, position: number): HeadPose {
  let pitch = 0,
    yaw = 0
  for (const gesture of plan?.gestures ?? []) {
    const phase = (position - gesture.time) / gesture.duration
    if (phase <= 0 || phase >= 1) continue
    const amount = phase < 0.42 ? easeMotion(phase / 0.42) : 1 - easeMotion((phase - 0.42) / 0.58)
    pitch += gesture.pitch * amount
    yaw +=
      gesture.yaw *
      (gesture.kind === 'shake'
        ? Math.sin(phase * Math.PI * 4) * Math.sin(phase * Math.PI) ** 2
        : amount)
  }
  return { pitch, yaw, roll: 0 }
}

export function quietPosture(ms: number, seed: number, bounds: number[]): HeadPose {
  const random = randomSequence(seed),
    from = [0, 0, 0]
  let start = 0
  for (;;) {
    const hold = 12000 + random() * 15000,
      transition = 4000 + random() * 2500
    const target = random() < 0.5 ? [...from] : bounds.map((bound) => (random() * 2 - 1) * bound)
    if (ms < start + hold) {
      const p = easeMotion((ms - start) / transition)
      const v = from.map((value, i) => value + (target[i]! - value) * p)
      return { yaw: v[0]!, pitch: v[1]!, roll: v[2]! }
    }
    from.splice(0, 3, ...target)
    start += hold
  }
}

export function createContextualHeadDirector(
  seed: number,
  settings: BehaviorSettings,
  bounds: { idle: number[]; speaking: number[] },
  idle?: IdleMotionProfile,
) {
  let lastActive = false,
    lastSession = -1,
    changedAt = 0
  let previous: HeadPose = { yaw: 0, pitch: 0, roll: 0 },
    from = { ...previous }
  return {
    sample(ms: number, speech: BehaviorSpeechState): HeadPose {
      if (speech.active !== lastActive || speech.session !== lastSession) {
        from = { ...previous }
        changedAt = ms
        lastActive = speech.active
        lastSession = speech.session
      }
      const blend = easeMotion((ms - changedAt) / 600)
      const goal = speech.active
        ? gestureAt(speech.plan, speech.position)
        : { yaw: 0, pitch: 0, roll: 0 }
      previous = {
        yaw: from.yaw * (1 - blend) + goal.yaw * blend,
        pitch: from.pitch * (1 - blend) + goal.pitch * blend,
        roll: 0,
      }
      const base = idle
        ? idlePostureAt(ms, seed, settings.head.idle_degrees, idle)
        : quietPosture(ms, seed, settings.head.idle_degrees)
      // A stopped gesture eases out inside the validated envelope, without an idle clamp jump.
      const limit = bounds.speaking
      return {
        yaw: Math.max(-limit[0]!, Math.min(limit[0]!, base.yaw + previous.yaw)),
        pitch: Math.max(-limit[1]!, Math.min(limit[1]!, base.pitch + previous.pitch)),
        roll: Math.max(-limit[2]!, Math.min(limit[2]!, base.roll)),
      }
    },
  }
}

// Spontaneous and speech-triggered blinks spend the same frequency allowance.
export function createContextualBlinkPlanner(seed: number, settings: BehaviorSettings['blink']) {
  const random = randomSequence(seed),
    rate = settings.rate_per_minute / 60000,
    capacity = settings.double ? 2 : 1.35
  let lastTick = 0,
    lastBlink = -Infinity,
    credit = settings.double ? 2 : 1,
    hazard = 0,
    threshold = -Math.log(1 - random())
  let session = -1,
    previousPosition = -0.001,
    wasActive = false
  let reserved = false,
    lastDouble = -Infinity
  function record(ms: number, spend = true) {
    lastBlink = ms
    if (spend) credit = Math.max(0, credit - 1)
    hazard = 0
    threshold = -Math.log(1 - random())
  }
  return {
    record,
    canStartDouble: () => Boolean(settings.double && credit >= 2 && !reserved),
    reserveDouble(ms: number) {
      if (
        !settings.double ||
        reserved ||
        credit < 1 ||
        ms - lastDouble < settings.double.minimum_interval_ms ||
        random() >= settings.double.probability
      )
        return false
      credit -= 1
      reserved = true
      return true
    },
    beginManualDouble(ms: number) {
      if (!settings.double || reserved || credit < 2) return false
      record(ms)
      credit -= 1
      reserved = true
      return true
    },
    startReserved(ms: number) {
      if (!reserved) return false
      reserved = false
      lastDouble = ms
      record(ms, false)
      return true
    },
    cancelDouble() {
      if (reserved) credit = Math.min(capacity, credit + 1)
      reserved = false
    },
    reset(ms: number) {
      lastTick = ms
      hazard = 0
      previousPosition = -0.001
      session = -1
      wasActive = false
    },
    sample(ms: number, speech: BehaviorSpeechState, available = true) {
      const dt = Math.max(0, Math.min(100, ms - lastTick))
      lastTick = ms
      credit = Math.min(capacity, credit + dt * rate)
      if (
        session !== speech.session ||
        (speech.active && !wasActive) ||
        !speech.active ||
        speech.position < previousPosition
      ) {
        session = speech.session
        previousPosition =
          speech.active && speech.position < 0.25 ? -0.001 : speech.position - 0.001
      }
      wasActive = speech.active
      const cues = speech.active
        ? (speech.plan?.cues ?? []).filter(
            (cue) =>
              cue.time > previousPosition &&
              cue.time <= speech.position &&
              speech.position - cue.time < 0.25,
          )
        : []
      previousPosition = speech.position
      const age = ms - lastBlink
      if (!available || age < settings.minimum_gap_ms) return false
      const contextual = cues.some((cue) => {
        const chance =
          cue.kind === 'pause'
            ? settings.pause_probability
            : cue.kind === 'ending'
              ? settings.ending_probability
              : cue.kind === 'onset'
                ? settings.onset_probability
                : cue.kind === 'nod'
                  ? settings.nod_probability
                  : 0
        return chance > 0 && random() < chance
      })
      hazard += dt * rate * (speech.active ? 0.65 : 1.1)
      if (credit < 1 || (!contextual && hazard < threshold)) return false
      record(ms)
      return true
    },
  }
}
