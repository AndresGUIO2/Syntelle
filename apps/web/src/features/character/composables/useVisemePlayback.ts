import { onMounted, onScopeDispose, readonly, shallowRef } from 'vue'

export function useVisemePlayback() {
  const amount = shallowRef(0)
  const playing = shallowRef(false)
  const duration = shallowRef(120)
  const hold = shallowRef(500)
  const reducedMotion = shallowRef(false)
  const media = matchMedia('(prefers-reduced-motion: reduce)')
  let frame = 0
  let timer = 0

  function pause() {
    playing.value = false
    cancelAnimationFrame(frame)
    clearTimeout(timer)
  }

  function pose(value: number) {
    pause()
    amount.value = Math.max(0, Math.min(1, value))
  }

  function moveTo(target: number) {
    const from = amount.value
    const start = performance.now()
    const milliseconds = duration.value
    function tick(now: number) {
      const progress = milliseconds === 0 ? 1 : Math.min(1, (now - start) / milliseconds)
      amount.value = from + (target - from) * progress
      if (progress < 1) frame = requestAnimationFrame(tick)
      else timer = window.setTimeout(() => moveTo(1 - target), hold.value)
    }
    frame = requestAnimationFrame(tick)
  }

  function toggle() {
    if (playing.value) return pause()
    if (reducedMotion.value) return
    playing.value = true
    moveTo(amount.value >= 0.5 ? 0 : 1)
  }

  function followMotion() {
    reducedMotion.value = media.matches
    if (media.matches) pause()
  }
  function onVisibility() {
    if (document.hidden) pause()
  }
  onMounted(() => {
    followMotion()
    media.addEventListener('change', followMotion)
    document.addEventListener('visibilitychange', onVisibility)
  })
  onScopeDispose(() => {
    pause()
    media.removeEventListener('change', followMotion)
    document.removeEventListener('visibilitychange', onVisibility)
  })

  return {
    amount: readonly(amount),
    playing: readonly(playing),
    reducedMotion: readonly(reducedMotion),
    duration,
    hold,
    pose,
    toggle,
  }
}
