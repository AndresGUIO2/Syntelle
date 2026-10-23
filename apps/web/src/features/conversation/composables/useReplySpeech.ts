import { watch, type Ref } from 'vue'
import type { useSpeechPlayback } from '../../character/composables/useSpeechPlayback'
import type { SpeechPart } from '../types'

type Playback = ReturnType<typeof useSpeechPlayback>

// One turn owns the queue. Each original WAV retains its own native audio clock;
// the next phrase starts as soon as the preceding source ends, without a new
// provider request or the expression lead-in used by manual previews.
export function useReplySpeech(
  playback: Ref<Playback | undefined>,
  callbacks: {
    started: () => void
    waiting: () => void
    ended: () => void
    failed: (message: string) => void
  },
) {
  let queue: SpeechPart[] = [],
    enabled = false,
    complete = false,
    running = false,
    starting = false,
    nextIndex = 0,
    generation = 0

  function reset() {
    generation++
    queue = []
    enabled = complete = running = starting = false
    nextIndex = 0
  }
  function finish() {
    if (enabled && complete && !running && queue.length === 0) {
      enabled = false
      callbacks.ended()
    }
  }
  async function pump() {
    if (!enabled || running || document.hidden) return
    const part = queue.shift()
    if (!part) {
      finish()
      return
    }
    const player = playback.value,
      token = generation
    if (!player) {
      reset()
      callbacks.failed('Audio playback is unavailable.')
      return
    }
    running = starting = true
    await player.playPreparedSpeech(part.speech, part.text, true)
    if (token !== generation) return
    starting = false
    if (player.active.value) callbacks.started()
    else {
      reset()
      callbacks.failed(player.error.value || 'Speech playback could not start.')
    }
  }
  function push(part: SpeechPart) {
    if (part.index < nextIndex) return // duplicate delivery cannot replay audio
    if (part.index !== nextIndex || queue.length >= 32) {
      reset()
      callbacks.failed('Speech arrived out of order. Please start a new reply.')
      return
    }
    enabled = true
    nextIndex++
    queue.push(part)
    void pump()
  }
  function close(expectedParts?: number) {
    if (
      expectedParts !== undefined &&
      (!Number.isInteger(expectedParts) || expectedParts < 1 || expectedParts !== nextIndex)
    ) {
      reset()
      callbacks.failed('The reply ended before all audio arrived. Please ask to continue.')
      return
    }
    complete = true
    finish()
  }

  watch(
    () => playback.value?.active.value,
    (active, previous) => {
      if (!enabled || !previous || active || starting) return
      running = false
      callbacks.waiting()
      void pump()
    },
    { flush: 'sync' },
  )
  return { push, close, reset, active: () => enabled }
}
