import { computed, onMounted, onScopeDispose, readonly, shallowRef, watch } from 'vue'
import type { MotionPackage } from '../types/motionTypes'

import { blinkAt, randomSequence } from '../animation/timing'
import {
  createContextualBlinkPlanner,
  type BehaviorSettings,
  type BehaviorSpeechState,
} from '../animation/speechBehavior'
export { blinkAt, randomSequence } from '../animation/timing'

export function useMotionPlayback(
  settings: MotionPackage['blink'],
  seed: number,
  options?: {
    behavior: BehaviorSettings
    speech: () => BehaviorSpeechState
  },
) {
  const automaticBlink = shallowRef(true),
    movement = shallowRef(true),
    paused = shallowRef(false),
    reducedMotion = shallowRef(false)
  const elapsed = shallowRef(0),
    manualClosure = shallowRef(0),
    blinkStart = shallowRef<number | null>(null)
  const random = randomSequence(seed),
    interval = () =>
      settings.minimum_interval_ms +
      random() * (settings.maximum_interval_ms - settings.minimum_interval_ms)
  const planner = options ? createContextualBlinkPlanner(seed, options.behavior.blink) : null
  const blinkScale = shallowRef(1)
  const secondAt = shallowRef<number | null>(null)
  let secondIsManual = false
  let deadline = interval(),
    handle = 0,
    last: number | null = null
  const frameTimings: number[] = []
  const duration = settings.closing_ms + settings.closed_ms + settings.opening_ms
  const closure = computed(() =>
    blinkStart.value === null
      ? manualClosure.value
      : blinkAt((elapsed.value - blinkStart.value) / blinkScale.value, settings),
  )
  const doubleBlinkAvailable = computed(() => {
    // Credit advances on the same paused/visibility-aware animation clock.
    void elapsed.value
    return (
      !paused.value &&
      !reducedMotion.value &&
      blinkStart.value === null &&
      secondAt.value === null &&
      Boolean(planner?.canStartDouble())
    )
  })
  function cancelSecond() {
    secondAt.value = null
    planner?.cancelDouble()
  }
  function queueSecond(manual = false) {
    const gap = options?.behavior.blink.double?.open_gap_ms
    if (!gap) return
    secondIsManual = manual
    secondAt.value =
      elapsed.value + duration * blinkScale.value + gap[0] + random() * (gap[1] - gap[0])
  }
  function stop() {
    cancelAnimationFrame(handle)
    handle = 0
    last = null
  }
  function tick(now: number) {
    if (paused.value || document.hidden) {
      stop()
      return
    }
    if (last !== null) {
      const dt = Math.max(0, now - last)
      elapsed.value += dt
      frameTimings.push(dt)
      if (frameTimings.length > 600) frameTimings.shift()
    }
    last = now
    if (
      blinkStart.value !== null &&
      elapsed.value - blinkStart.value >= duration * blinkScale.value
    ) {
      blinkStart.value = null
      manualClosure.value = 0
      deadline = elapsed.value + interval()
    }
    if (secondAt.value !== null && elapsed.value >= secondAt.value) {
      secondAt.value = null
      if (planner?.startReserved(elapsed.value)) {
        blinkScale.value = 0.88 + random() * 0.12
        blinkStart.value = elapsed.value
      }
    }
    if (automaticBlink.value || options?.behavior.blink.double) {
      const shouldBlink =
        planner && options
          ? planner.sample(
              elapsed.value,
              options.speech(),
              automaticBlink.value && blinkStart.value === null && secondAt.value === null,
            )
          : blinkStart.value === null && elapsed.value >= deadline
      if (shouldBlink) {
        blinkScale.value = planner ? 0.88 + random() * 0.24 : 1
        blinkStart.value = elapsed.value
        if (planner?.reserveDouble(elapsed.value)) queueSecond()
      }
    }
    handle = requestAnimationFrame(tick)
  }
  function start() {
    if (!handle && !paused.value && !document.hidden) handle = requestAnimationFrame(tick)
  }
  function visibility() {
    if (document.hidden) stop()
    else start()
  }
  function setClosure(value: number) {
    cancelSecond()
    automaticBlink.value = false
    blinkStart.value = null
    manualClosure.value = Math.max(0, Math.min(1, value))
  }
  function blink() {
    if (!paused.value && !reducedMotion.value) {
      cancelSecond()
      manualClosure.value = 0
      blinkScale.value = 1
      blinkStart.value = elapsed.value
      planner?.record(elapsed.value)
      start()
    }
  }
  function blinkTwice() {
    if (!doubleBlinkAvailable.value || !planner?.beginManualDouble(elapsed.value)) return
    manualClosure.value = 0
    blinkScale.value = 1
    blinkStart.value = elapsed.value
    queueSecond(true)
    start()
  }
  const media = matchMedia('(prefers-reduced-motion: reduce)')
  function preference() {
    reducedMotion.value = media.matches
    if (media.matches) {
      cancelSecond()
      automaticBlink.value = false
      movement.value = false
      paused.value = true
      blinkStart.value = null
      manualClosure.value = 0
    }
  }
  watch(paused, (value) => (value ? stop() : start()))
  watch(automaticBlink, (enabled) => {
    if (!enabled && !secondIsManual) cancelSecond()
    deadline = elapsed.value + interval()
    planner?.reset(elapsed.value)
  })
  onMounted(() => {
    preference()
    document.addEventListener('visibilitychange', visibility)
    media.addEventListener('change', preference)
    start()
  })
  onScopeDispose(() => {
    cancelSecond()
    stop()
    document.removeEventListener('visibilitychange', visibility)
    media.removeEventListener('change', preference)
  })
  return {
    automaticBlink,
    movement,
    paused,
    reducedMotion: readonly(reducedMotion),
    elapsed: readonly(elapsed),
    closure,
    setClosure,
    blink,
    blinkTwice,
    doubleBlinkAvailable,
    frameTimings,
  }
}
