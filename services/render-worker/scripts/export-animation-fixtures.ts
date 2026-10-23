import {
  createAttentionDirector,
  gazeOffsets,
} from '../../../apps/web/src/features/character/animation/attention'
import { createBreathingTrack } from '../../../apps/web/src/features/character/animation/breathing'
import { createExpressionDirector } from '../../../apps/web/src/features/character/animation/expressions'
import {
  createContextualBlinkPlanner,
  planSpeechBehavior,
} from '../../../apps/web/src/features/character/animation/speechBehavior'
import {
  directedPlan,
  admitPerformance,
} from '../../../apps/web/src/features/character/animation/performance'
import {
  blinkAt,
  randomSequence,
} from '../../../apps/web/src/features/character/animation/timing'
import {
  decodeSpeechWav,
  speechTimeline,
  mouthAt,
} from '../../../apps/web/src/features/character/speech/speechTimeline'

// This fixture adapter calls the original TS directors and PCM planner. It
// supplies a controlled frame clock, replacing only the DOM/rAF bindings.
export function exportFixtures(input: any) {
  const { motion, gaze, expressions, oral, prepared } = input
  const seed = motion.animation_seed
  const blinkSeed = parseInt(oral.build_id.slice(0, 8), 16)
  const bytes = Uint8Array.from(atob(prepared.audio_base64), (c) =>
    c.charCodeAt(0),
  )
  const pcm = decodeSpeechWav(bytes)
  const timeline = speechTimeline(prepared.visemes, prepared.duration_ms)
  const performance = {
    intent: {
      version: 'performance-intent/1',
      text: 'Hello friend. Yes, absolutely.',
      tone: 'neutral',
      cues: [
        { kind: 'nod', anchor: 'Hello' },
        { kind: 'emphasis', anchor: 'Yes' },
        { kind: 'smile', anchor: 'friend' },
      ],
    },
    resolved: [
      { kind: 'nod', time: 0.6, duration: 1.3 },
      { kind: 'emphasis', time: 3, duration: 1.1 },
      { kind: 'smile', time: 3.2, duration: 1.4 },
    ],
  }
  const plan = directedPlan(pcm.samples, pcm.sampleRate, performance as any)
  const autonomousPlan = planSpeechBehavior(
    pcm.samples,
    pcm.sampleRate,
    123456789,
    motion.behavior,
  )
  const defaults = {
    movement: true,
    blinking: true,
    automatic_expressions: true,
    smile: 0,
    brows: 0,
    reduced_motion: false,
    paused: false,
  }
  function runner() {
    const attention = createAttentionDirector(
      seed,
      motion.projection.bounds,
      true,
      true,
    )
    const breathing = createBreathingTrack(seed, motion.breathing)
    const expression = createExpressionDirector(
      seed,
      Boolean(expressions.smile),
      true,
    )
    const random = randomSequence(blinkSeed)
    const planner = createContextualBlinkPlanner(
      blinkSeed,
      motion.behavior.blink,
    )
    const interval = () =>
      motion.blink.minimum_interval_ms +
      random() *
        (motion.blink.maximum_interval_ms - motion.blink.minimum_interval_ms)
    let deadline = interval(),
      start: number | null = null,
      second: number | null = null,
      scale = 1,
      enabled = true,
      last: any
    function closure(ms: number, speech: any, automatic: boolean) {
      if (automatic !== enabled) {
        enabled = automatic
        if (!enabled) {
          second = null
          planner.cancelDouble()
        }
        deadline = ms + interval()
        planner.reset(ms)
      }
      const duration =
        motion.blink.closing_ms +
        motion.blink.closed_ms +
        motion.blink.opening_ms
      if (start !== null && ms - start >= duration * scale) {
        start = null
        deadline = ms + interval()
      }
      if (second !== null && ms >= second) {
        second = null
        if (planner.startReserved(ms)) {
          scale = 0.88 + random() * 0.12
          start = ms
        }
      }
      if (automatic || motion.behavior.blink.double) {
        if (
          planner.sample(
            ms,
            speech,
            automatic && start === null && second === null,
          )
        ) {
          scale = 0.88 + random() * 0.24
          start = ms
          if (planner.reserveDouble(ms)) {
            const gap = motion.behavior.blink.double.open_gap_ms
            second =
              ms + duration * scale + gap[0] + random() * (gap[1] - gap[0])
          }
        }
      }
      return start === null ? 0 : blinkAt((ms - start) / scale, motion.blink)
    }
    return (ms: number, speech: any, settings: any) => {
      if (settings.paused && last) return last
      let mouth = speech.active
        ? mouthAt(timeline, speech.position)
        : { from: 'Rest', to: 'A', amount: 0 }
      if (mouth.from === mouth.to)
        mouth = {
          from: mouth.from,
          to: mouth.from === 'Rest' ? 'A' : 'Rest',
          amount: 0,
        }
      const moving = settings.movement && !settings.reduced_motion
      const a = moving
        ? attention.sample(ms, speech)
        : { head: { yaw: 0, pitch: 0, roll: 0 }, glance: [0, 0] }
      const offsets = moving
        ? gazeOffsets(gaze, a.head, a.glance)
        : [
            [0, 0],
            [0, 0],
          ]
      const quantized = offsets.map((pair) =>
        pair.map((v) => Math.trunc(v * 20) / 20),
      )
      const c = closure(
        ms,
        speech,
        settings.blinking && !settings.reduced_motion,
      )
      const closed = (id: string) => ['Rest', 'MBP'].includes(id)
      const busy =
        speech.waiting ||
        (speech.active && speech.position < pcm.duration) ||
        (mouth.amount < 1 && !closed(mouth.from)) ||
        (mouth.amount > 0 && !closed(mouth.to))
      const e = expression.sample(
        ms,
        { smile: settings.smile, brows: settings.brows },
        settings.automatic_expressions && !settings.reduced_motion,
        false,
        speech,
        settings.paused,
        busy,
      )
      if (busy) e.smile = 0
      last = {
        head: a.head,
        breath: moving
          ? breathing.sample(ms) *
            motion.anatomy.face_height *
            motion.motion.breath_face_height_fraction
          : 0,
        mouth,
        eye_closure: c,
        gaze: quantized,
        smile: e.smile,
        brows: e.brows,
      }
      return last
    }
  }
  const admissionText = 'Hello friend. Yes, absolutely.'
  const admittedPerformance = {
    intent: {
      version: 'performance-intent/1',
      text: admissionText,
      tone: 'neutral',
      cues: [
        { kind: 'nod', anchor: 'Yes' },
        { kind: 'smile', anchor: 'friend' },
        { kind: 'shake', anchor: 'absolutely' },
      ],
    },
    resolved: [
      { kind: 'nod', time: 0.6, duration: 1.3 },
      { kind: 'smile', time: 2, duration: 1.4 },
      { kind: 'shake', time: 3.2, duration: 1.2 },
    ],
  }
  const admission = [100000, 100100, 140000].map((now) => ({
    now,
    text: admissionText,
    input: admittedPerformance,
    expected: admitPerformance(
      admittedPerformance as any,
      admissionText,
      now,
      false,
    ),
  }))
  const cases = []
  for (const name of ['idle', 'speech', 'controls']) {
    const sample = runner(),
      rows = []
    const duration =
      name === 'idle'
        ? 90000
        : name === 'speech'
          ? (pcm.duration + 3) * 1000
          : 12000
    for (let index = 0; index <= Math.ceil((duration / 1000) * 30); index++) {
      const mediaNS = Math.trunc((index * 1_000_000_000) / 30)
      const now = mediaNS / 1_000_000
      let ms = now
      let settings = { ...defaults }
      let speech = {
        waiting: false,
        active: false,
        position: 0,
        session: 0,
        plan: null as any,
      }
      if (name === 'speech') {
        const position = Math.max(
          0,
          Math.min(pcm.duration, (now - 1000) / 1000),
        )
        speech = {
          waiting: false,
          active: now >= 1000 && position < pcm.duration,
          position,
          session: now >= 1000 ? 1 : 0,
          plan: now >= 1000 ? plan : null,
        }
      }
      if (name === 'controls') {
        speech.waiting = now >= 3000 && now < 3500
        settings.smile = now >= 1000 ? 0.8 : 0
        settings.brows = now >= 2000 ? 0.6 : 0
        settings.movement = !(now >= 4500 && now < 7000)
        settings.blinking = !(now >= 4000 && now < 6000)
        settings.reduced_motion = now >= 8000 && now < 9000
        // Pause/resume freezes the shared clock; its offset stays removed after resume.
        if (now >= 10000 && now < 11000) {
          settings.paused = true
          ms = Math.trunc((10000 - 1000 / 30) * 1_000_000) / 1_000_000
        } else if (now >= 11000) ms = now - 1000
      }
      rows.push({
        media_ns: mediaNS,
        ms,
        settings,
        speech: {
          active: speech.active,
          waiting: speech.waiting,
          position: speech.position,
          session: speech.session,
          has_plan: Boolean(speech.plan),
        },
        expected: sample(ms, speech, settings),
      })
    }
    cases.push({ name, rows })
  }
  return {
    admission,
    version: 'native-animation-ts-reference/1',
    bundle: { motion, gaze, expressions, oral: { build_id: oral.build_id } },
    speech: {
      text: performance.intent.text,
      sample_rate: pcm.sampleRate,
      audio_base64: prepared.audio_base64,
      visemes: prepared.visemes,
      duration_ms: prepared.duration_ms,
      performance,
      plan,
      autonomous_plan: autonomousPlan,
      timeline,
    },
    cases,
  }
}
