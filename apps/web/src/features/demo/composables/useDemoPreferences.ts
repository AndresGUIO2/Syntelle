import { shallowRef, watch } from 'vue'
import { demoCharacters } from '../catalog'

export function useDemoPreferences() {
  const blinking = shallowRef(true)
  const movement = shallowRef(true)
  const automaticExpressions = shallowRef(true)
  const voices = shallowRef<Record<string, string>>({})
  const allowed = new Set<string>(demoCharacters.map((character) => character.voice))
  try {
    const saved = JSON.parse(localStorage.getItem('syntelle.demo.appearance') ?? 'null')
    if (saved) {
      blinking.value = saved.blinking !== false
      movement.value = saved.movement !== false
      automaticExpressions.value = saved.automaticExpressions !== false
      if (saved.voices && typeof saved.voices === 'object') {
        voices.value = Object.fromEntries(
          Object.entries(saved.voices).filter(
            ([key, value]) =>
              demoCharacters.some((c) => c.id === key) &&
              typeof value === 'string' &&
              allowed.has(value),
          ),
        ) as Record<string, string>
      }
    }
  } catch {
    /* Preferences are optional. */
  }
  watch([blinking, movement, automaticExpressions, voices], () => {
    try {
      localStorage.setItem(
        'syntelle.demo.appearance',
        JSON.stringify({
          blinking: blinking.value,
          movement: movement.value,
          automaticExpressions: automaticExpressions.value,
          voices: voices.value,
        }),
      )
    } catch {
      /* Continue without persistence. */
    }
  })
  return { blinking, movement, automaticExpressions, voices }
}
