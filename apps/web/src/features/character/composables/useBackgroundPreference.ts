import { computed, shallowRef, watch } from 'vue'
import type { Background } from '../types/motionTypes'

export function useBackgroundPreference(
  catalog: readonly Background[],
  assetUrl: (name: string) => string,
) {
  const background = shallowRef(catalog[0]!.id)
  const blur = shallowRef(0)
  try {
    const saved = JSON.parse(localStorage.getItem('syntelle.motion.background') ?? 'null')
    if (saved && catalog.some((item) => item.id === saved.id)) {
      background.value = saved.id
      blur.value = Math.max(0, Math.min(20, Number(saved.blur) || 0))
    }
  } catch {
    /* Storage is optional. */
  }
  watch([background, blur], ([id, value]) => {
    try {
      localStorage.setItem('syntelle.motion.background', JSON.stringify({ id, blur: value }))
    } catch {
      /* Controls remain usable without storage. */
    }
  })
  const selectedBackground = computed(() =>
    assetUrl(catalog.find((item) => item.id === background.value)!.image),
  )
  return { background, blur, selectedBackground }
}
